package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

type gitVersion struct {
	proj *Project
	hash plumbing.Hash
	time time.Time

	deps []Dependency
}

func LoadHierarchy(main *Project) (map[string]*Project, map[Dependency]*Project, error) {
	projMap := make(map[string]*Project)
	projMap[main.Config.Name] = main

	depMap := make(map[Dependency]*Project)

	depsPath := filepath.Join(main.Path, "build", "dependencies")
	if err := os.MkdirAll(depsPath, 0750); err != nil {
		return nil, nil, err
	}

	gitVersions := make(map[string]gitVersion)
	currentHash := make(map[string]plumbing.Hash)

	// Recursively process all projects
	queue := []*Project{main}
	visited := make(map[Dependency]any)

	for len(queue) > 0 {
		proj := queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		for _, dep := range proj.Config.Dependencies {
			if _, ok := visited[dep]; ok {
				continue
			}
			visited[dep] = nil

			if dep.Path != "" {
				// Local
				depProj, err := processLocalDep(projMap, depMap, proj, dep)
				if err != nil {
					return nil, nil, err
				}

				queue = append(queue, depProj)
			} else {
				// Git
				depProj, err := processGitDep(gitVersions, currentHash, depsPath, dep)
				if err != nil {
					return nil, nil, err
				}

				queue = append(queue, depProj)
			}
		}
	}

	// Select newest git versions
	if err := finalizeGitVersions(projMap, depMap, currentHash, gitVersions); err != nil {
		return nil, nil, err
	}

	return projMap, depMap, nil
}

func processLocalDep(projMap map[string]*Project, depMap map[Dependency]*Project, parent *Project, dep Dependency) (*Project, error) {
	// Open project
	proj, err := Open(filepath.Join(parent.Path, dep.Path))
	if err != nil {
		return nil, err
	}

	// Check duplicate name
	if _, ok := projMap[proj.Config.Name]; ok {
		return nil, fmt.Errorf("project with the name '%s' already exists in the dependency tree", proj.Config.Name)
	}

	projMap[proj.Config.Name] = proj

	// Add project into dependency map
	depMap[dep] = proj

	return proj, nil
}

func processGitDep(gitVersions map[string]gitVersion, currentHash map[string]plumbing.Hash, depsPath string, dep Dependency) (*Project, error) {
	path := filepath.Join(depsPath, repoName(dep.Url))

	// Open repo
	repo, err := openOrClone(path, dep.Url)
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer repo.Close()

	// Commit
	commit, err := resolveCommit(repo, dep.Revision)
	if err != nil {
		return nil, err
	}

	// Checkout
	if currentHash[path] != commit.Hash {
		err := checkoutHash(repo, commit.Hash)
		if err != nil {
			return nil, err
		}

		currentHash[path] = commit.Hash
	}

	// Open project
	proj, err := Open(path)
	if err != nil {
		return nil, err
	}

	incoming := gitVersion{
		proj: proj,
		hash: commit.Hash,
		time: commit.Committer.When,
		deps: []Dependency{dep},
	}

	// Store version
	if old, ok := gitVersions[proj.Config.Name]; ok {
		if incoming.time.After(old.time) {
			incoming.deps = append(incoming.deps, old.deps...)
			gitVersions[proj.Config.Name] = incoming
		} else {
			old.deps = append(old.deps, dep)
			gitVersions[proj.Config.Name] = old
		}
	} else {
		gitVersions[proj.Config.Name] = incoming
	}

	return proj, nil
}

func finalizeGitVersions(projMap map[string]*Project, depMap map[Dependency]*Project, currentHash map[string]plumbing.Hash, gitVersions map[string]gitVersion) error {
	for name, version := range gitVersions {
		// Check duplicate project name
		if _, ok := projMap[name]; ok {
			return fmt.Errorf("project with the name '%s' already exists in the dependency tree", name)
		}

		projMap[name] = version.proj

		// Checkout
		if currentHash[version.proj.Path] != version.hash {
			repo, err := git.PlainOpen(version.proj.Path)
			if err != nil {
				return err
			}

			err = checkoutHash(repo, version.hash)
			_ = repo.Close()

			if err != nil {
				return err
			}
		}

		// Add project into dependency map
		for _, dep := range version.deps {
			depMap[dep] = version.proj
		}
	}

	return nil
}

func openOrClone(repoPath, url string) (*git.Repository, error) {
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

func repoName(url string) string {
	i := strings.LastIndex(url, "/")
	if i == -1 {
		panic("project.repoName() - Invalid git url")
	}

	return url[i+1 : len(url)-4]
}
