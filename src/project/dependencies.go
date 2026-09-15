package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fireball/core"
	fbcore "fireball/fb-core"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

const MetadataVersion = 1

type MetadataFile struct {
	Version      int                        `json:"version"`
	Repositories map[string]GitRepoMetadata `json:"repositories"` // Key: url + "@" + revision
}

type GitRepoMetadata struct {
	URL             string    `json:"url"`
	Revision        string    `json:"revision"`
	RelPath         string    `json:"path"`
	ProjectName     string    `json:"project_name"`
	CommitHash      string    `json:"commit_hash"`
	CommitTimestamp time.Time `json:"commit_timestamp"`
}

type gitVersionCandidate struct {
	proj     *Project
	metadata GitRepoMetadata
}

func LoadHierarchy(main *Project) (map[string]*Project, map[Dependency]*Project, error) {
	defer core.Scope()()

	depsPath := filepath.Join(main.Path, "build", "dependencies")
	if err := os.MkdirAll(depsPath, 0750); err != nil {
		return nil, nil, err
	}

	// 1. Load embedded core project
	coreProj, err := loadCore(depsPath)
	if err != nil {
		return nil, nil, err
	}

	// 2. Load metadata cache
	metaFile, err := loadMetadata(depsPath)
	if err != nil {
		return nil, nil, err
	}

	// 3. Phase 1: Discover dependencies, clone/update git repos, and pick the newest versions
	gitCandidates, localProjects, err := discoverDependencies(main, depsPath, metaFile)
	if err != nil {
		return nil, nil, err
	}

	// 4. Phase 2: Build final dependency hierarchy using only reachable, winning versions
	return buildHierarchy(main, coreProj, metaFile, gitCandidates, localProjects)
}

func discoverDependencies(
	main *Project,
	depsPath string,
	metaFile *MetadataFile,
) (map[string]gitVersionCandidate, map[string]*Project, error) {
	defer core.Scope()()

	gitCandidates := make(map[string]gitVersionCandidate)
	localProjects := make(map[string]*Project)
	gitProjects := make(map[*Project]struct{})

	queue := []*Project{main}
	visited := make(map[*Project]struct{})
	metaDirty := false

	for len(queue) > 0 {
		proj := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		// Skip if this git project was superseded by a newer version while waiting in queue
		if _, isGit := gitProjects[proj]; isGit {
			if winning, ok := gitCandidates[proj.Config.Name]; ok && winning.proj != proj {
				continue
			}
		}

		if _, ok := visited[proj]; ok {
			continue
		}

		visited[proj] = struct{}{}

		for _, dep := range proj.Config.Dependencies {
			if dep.Path != "" {
				// Local dependency
				absPath := filepath.Clean(filepath.Join(proj.Path, dep.Path))
				localProj, ok := localProjects[absPath]

				if !ok {
					var err error
					localProj, err = Open(absPath)
					if err != nil {
						return nil, nil, err
					}

					localProjects[absPath] = localProj
				}

				queue = append(queue, localProj)
			} else {
				// Git dependency
				depProj, updated, err := processGitDep(depsPath, metaFile, gitCandidates, dep)
				if err != nil {
					return nil, nil, err
				}

				if updated {
					metaDirty = true
				}
				if depProj != nil {
					gitProjects[depProj] = struct{}{}
					queue = append(queue, depProj)
				}
			}
		}
	}

	if metaDirty {
		if err := saveMetadata(depsPath, metaFile); err != nil {
			return nil, nil, err
		}
	}

	return gitCandidates, localProjects, nil
}

func buildHierarchy(
	main *Project,
	coreProj *Project,
	metaFile *MetadataFile,
	gitCandidates map[string]gitVersionCandidate,
	localProjects map[string]*Project,
) (map[string]*Project, map[Dependency]*Project, error) {
	defer core.Scope()()

	projMap := make(map[string]*Project)
	depMap := make(map[Dependency]*Project)

	projMap[main.Config.Name] = main
	projMap["core"] = coreProj
	depMap[Dependency{Path: "core"}] = coreProj

	walkQueue := []*Project{main}
	visited := make(map[*Project]struct{})
	visited[main] = struct{}{}
	visited[coreProj] = struct{}{}

	for len(walkQueue) > 0 {
		curr := walkQueue[len(walkQueue)-1]
		walkQueue = walkQueue[:len(walkQueue)-1]

		for _, dep := range curr.Config.Dependencies {
			var target *Project

			if dep.Path != "" {
				// Local dependency
				absPath := filepath.Clean(filepath.Join(curr.Path, dep.Path))
				target = localProjects[absPath]

				if target == nil {
					return nil, nil, fmt.Errorf("unresolved local dependency '%s' for project '%s'", dep.Path, curr.Config.Name)
				}
			} else {
				// Git dependency: resolve to winning candidate
				key := dep.Url + "@" + dep.Revision
				meta, ok := metaFile.Repositories[key]

				if !ok {
					return nil, nil, fmt.Errorf("missing metadata for git dependency: %s", key)
				}

				candidate, ok := gitCandidates[meta.ProjectName]
				if !ok {
					return nil, nil, fmt.Errorf("unresolved git dependency for project: %s", meta.ProjectName)
				}

				target = candidate.proj
			}

			depMap[dep] = target

			// Collision check against active project graph
			if existing, exists := projMap[target.Config.Name]; exists {
				if existing != target {
					return nil, nil, fmt.Errorf("project with the name '%s' already exists in the dependency tree", target.Config.Name)
				}
			} else {
				projMap[target.Config.Name] = target
			}

			if _, ok := visited[target]; !ok {
				visited[target] = struct{}{}
				walkQueue = append(walkQueue, target)
			}
		}
	}

	return projMap, depMap, nil
}

func loadCore(depsPath string) (*Project, error) {
	defer core.Scope()()

	path := filepath.Join(depsPath, "core")

	err := core.ExtractVersionedEmbedFs(path, ".", fbcore.Fs)
	if err != nil {
		return nil, err
	}

	return Open(path)
}

func processGitDep(
	depsPath string,
	metaFile *MetadataFile,
	candidates map[string]gitVersionCandidate,
	dep Dependency,
) (*Project, bool, error) {
	defer core.Scope()()

	key := dep.Url + "@" + dep.Revision
	meta, cached := metaFile.Repositories[key]
	metaDirty := false

	targetDir := meta.RelPath

	if targetDir == "" {
		var err error
		if targetDir, err = isolatedDirName(dep.Url, dep.Revision); err != nil {
			return nil, false, err
		}
	}

	fullPath := filepath.Join(depsPath, targetDir)

	// Fast path: use cache without Git operations if directory exists
	exists, err := pathExists(fullPath)
	if err != nil {
		return nil, false, err
	}

	if !cached || !exists {
		newMeta, err := cloneAndCheckout(depsPath, targetDir, dep.Url, dep.Revision)
		if err != nil {
			return nil, false, err
		}

		meta = newMeta
		metaFile.Repositories[key] = meta
		metaDirty = true
	}

	proj, err := Open(fullPath)
	if err != nil {
		return nil, false, err
	}

	incoming := gitVersionCandidate{
		proj:     proj,
		metadata: meta,
	}

	// Version selection: candidate with newer commit timestamp wins
	if existing, exists := candidates[meta.ProjectName]; exists {
		if incoming.metadata.CommitTimestamp.After(existing.metadata.CommitTimestamp) {
			candidates[meta.ProjectName] = incoming
			return proj, metaDirty, nil
		}

		// Older revision is ignored
		return nil, metaDirty, nil
	}

	candidates[meta.ProjectName] = incoming
	return proj, metaDirty, nil
}

func cloneAndCheckout(depsPath, relPath, url, revision string) (GitRepoMetadata, error) {
	defer core.Scope()()

	fullPath := filepath.Join(depsPath, relPath)

	repo, err := openOrClone(fullPath, url)
	if err != nil {
		return GitRepoMetadata{}, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer repo.Close()

	commit, err := resolveCommit(repo, revision)
	if err != nil {
		return GitRepoMetadata{}, err
	}

	if err := checkoutHash(repo, commit.Hash); err != nil {
		return GitRepoMetadata{}, err
	}

	proj, err := Open(fullPath)
	if err != nil {
		return GitRepoMetadata{}, err
	}

	return GitRepoMetadata{
		URL:             url,
		Revision:        revision,
		RelPath:         relPath,
		ProjectName:     proj.Config.Name,
		CommitHash:      commit.Hash.String(),
		CommitTimestamp: commit.Committer.When,
	}, nil
}

func openOrClone(repoPath, url string) (*git.Repository, error) {
	defer core.Scope()()

	exists, err := pathExists(repoPath)
	if err != nil {
		return nil, err
	}

	if exists {
		repo, err := git.PlainOpen(repoPath)
		if err != nil {
			return nil, err
		}

		err = repo.Fetch(&git.FetchOptions{})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			_ = repo.Close()
			return nil, err
		}

		return repo, nil
	}

	return git.PlainClone(repoPath, &git.CloneOptions{
		URL:        url,
		NoCheckout: true,
	})
}

func resolveCommit(repo *git.Repository, revision string) (*object.Commit, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return nil, err
	}

	return repo.CommitObject(*hash)
}

func checkoutHash(repo *git.Repository, hash plumbing.Hash) error {
	defer core.Scope()()

	tree, err := repo.Worktree()
	if err != nil {
		return err
	}

	return tree.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func isolatedDirName(url, revision string) (string, error) {
	raw := fmt.Sprintf("%s@%s", url, revision)
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])[:8]

	base, err := repoName(url)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s", base, hash), nil
}

func repoName(url string) (string, error) {
	if !strings.HasPrefix(url, "https://") || !strings.HasSuffix(url, ".git") {
		return "", fmt.Errorf("invalid git url '%s': must start with https:// and end with .git", url)
	}

	i := strings.LastIndex(url, "/")
	if i == -1 || i >= len(url)-4 {
		return "", fmt.Errorf("invalid git url '%s'", url)
	}

	return url[i+1 : len(url)-4], nil
}

func loadMetadata(depsPath string) (*MetadataFile, error) {
	defer core.Scope()()

	filePath := filepath.Join(depsPath, "dependencies.json")
	file, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return &MetadataFile{
			Version:      MetadataVersion,
			Repositories: make(map[string]GitRepoMetadata),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var meta MetadataFile
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return &MetadataFile{
			Version:      MetadataVersion,
			Repositories: make(map[string]GitRepoMetadata),
		}, nil
	}

	if meta.Repositories == nil {
		meta.Repositories = make(map[string]GitRepoMetadata)
	}

	return &meta, nil
}

func saveMetadata(depsPath string, meta *MetadataFile) error {
	defer core.Scope()()

	meta.Version = MetadataVersion

	filePath := filepath.Join(depsPath, "dependencies.json")

	tmpFile, err := os.CreateTemp(depsPath, "dependencies-*.tmp")
	if err != nil {
		return err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(tmpFile.Name())

	//goland:noinspection GoUnhandledErrorResult
	defer tmpFile.Close()

	if err := tmpFile.Chmod(0o644); err != nil {
		return err
	}

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "    ")

	if err := enc.Encode(meta); err != nil {
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpFile.Name(), filePath); err != nil {
		return err
	}

	return nil
}
