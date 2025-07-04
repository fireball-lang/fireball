package project

import (
	"fireball/analyzer"
	"fireball/ast"
	"fireball/cst"
	"fireball/utils"
	"github.com/pelletier/go-toml/v2"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Project struct {
	AbsolutePath string
	Config       Config

	Data any

	files map[string]*File
}

func OpenProject(path string) (*Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	p := &Project{
		AbsolutePath: abs,
		files:        make(map[string]*File),
	}

	// Parse config

	f, err := os.Open(filepath.Join(abs, "project.toml"))
	if err != nil {
		return nil, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer f.Close()

	err = toml.NewDecoder(f).Decode(&p.Config)
	if err != nil {
		return nil, err
	}

	// Validate config

	if strings.TrimSpace(p.Config.Name) == "" {
		p.Config.Name = "unnamed-project"
	}

	if p.Config.Type != Library && p.Config.Type != Executable {
		p.Config.Type = Library
	}

	return p, nil
}

func (p *Project) AddFile(provider FileContentsProvider) *File {
	if p.HasFile(provider.AbsolutePath()) {
		panic("project.Project.AddFile() - File with this path already exists")
	}

	file := &File{
		project:  p,
		provider: provider,
	}

	p.files[provider.AbsolutePath()] = file
	return file
}

func (p *Project) RemoveFile(path string) {
	if !p.HasFile(path) {
		panic("project.Project.RemoveFile() - File with this path doesn't exist")
	}

	delete(p.files, path)
}

func (p *Project) HasFile(path string) bool {
	_, ok := p.files[path]
	return ok
}

func (p *Project) Files() iter.Seq[*File] {
	return func(yield func(*File) bool) {
		for _, file := range p.files {
			if !yield(file) {
				return
			}
		}
	}
}

func (p *Project) Analyze(forceWithoutParse bool) {
	// Parse

	wg := sync.WaitGroup{}
	changed := false

	for _, file := range p.files {
		if contents, fileChanged := file.provider.Contents(); fileChanged {
			wg.Add(1)
			changed = true

			go parseFile(file, &wg, contents)
		}
	}

	if !changed && !forceWithoutParse {
		return
	}

	wg.Wait()

	// Create context

	ctx := &context{}

	for _, file := range p.files {
		ctx.addFile(file)
	}

	for _, module := range ctx.modules {
		module.checkNameCollisions()
	}

	// Resolve types

	fileDiagnostics := make(map[*File][]utils.Diagnostic)

	for _, file := range p.files {
		modulePath := file.Ast().ModulePath()

		if modulePath.SegmentCount() == 0 {
			var node ast.Node = file.Ast()

			if len(file.Ast().Decls) > 0 {
				node = file.Ast().Decls[0]
			}

			fileDiagnostics[file] = []utils.Diagnostic{{
				Kind:    utils.Error,
				Message: "Expected a module declaration at the top of the file.",
				Range:   node.Range(),
			}}

			continue
		}

		module := ctx.GetAbsoluteModule(modulePath)

		if !utils.IsNil(module) {
			scope := fileScope{ctx: ctx, mod: module}
			fileDiagnostics[file] = analyzer.ResolveTypes(file.ast, ctx, &scope)
		}
	}

	// Analyze

	for _, file := range p.files {
		module := ctx.GetAbsoluteModule(file.Ast().ModulePath())
		diagnostics := fileDiagnostics[file]

		if !utils.IsNil(module) {
			scope := fileScope{ctx: ctx, mod: module}
			diagnostics = append(diagnostics, analyzer.Analyze(file.ast, ctx, &scope)...)
		}

		if didChange(file.analyzeDiagnostics, diagnostics) {
			file.analyzeDiagnostics = diagnostics
			file.analyzeDiagnosticsChanged = true
		}
	}
}

func parseFile(file *File, wg *sync.WaitGroup, contents string) {
	fileCst, diagnostics := cst.Parse(contents)
	file.ast = ast.Convert(&fileCst)

	if didChange(file.parseDiagnostics, diagnostics) {
		file.parseDiagnostics = diagnostics
		file.parseDiagnosticsChanged = true
	}

	wg.Done()
}

func didChange(old, new []utils.Diagnostic) bool {
	return (len(new) == 0 && len(old) > 0) || (len(new) > 0 && len(old) == 0) || (len(new) > 0 && len(old) > 0)
}
