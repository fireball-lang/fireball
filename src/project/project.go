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

func (p *Project) AddFile(provider FileContentsProvider) {
	if _, ok := p.files[provider.AbsolutePath()]; ok {
		panic("project.Project.AddFile() - File with this path already exists")
	}

	p.files[provider.AbsolutePath()] = &File{
		project:  p,
		provider: provider,
	}
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

func (p *Project) Analyze() {
	// Parse

	wg := sync.WaitGroup{}

	for _, file := range p.files {
		if contents, changed := file.provider.Contents(); changed {
			wg.Add(1)
			go parseFile(file, &wg, contents)
		}
	}

	wg.Wait()

	// Create global scope
	scope := newGlobalScope()

	for _, file := range p.files {
		collectSymbols(file, scope)
	}

	// Analyze

	for _, file := range p.files {
		diagnostics := analyzer.Analyze(file.ast, scope)

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

func collectSymbols(file *File, scope *globalScope) {
	var diagnostics []utils.Diagnostic

	for _, decl := range file.ast.Decls {
		switch decl := decl.(type) {
		case *ast.Struct:
			if _, ok := scope.types[decl.Name()]; ok {
				diagnostics = append(diagnostics, utils.Diagnostic{
					Kind:    utils.Error,
					Message: "Type with the name '" + decl.Name() + "' already exists.",
					Range:   decl.NameN.Range(),
				})
			} else {
				scope.types[decl.Name()] = decl
			}

		case *ast.Func:
			if _, ok := scope.functions[decl.Name()]; ok {
				diagnostics = append(diagnostics, utils.Diagnostic{
					Kind:    utils.Error,
					Message: "Function with the name '" + decl.Name() + "' already exists.",
					Range:   decl.NameN.Range(),
				})
			} else {
				scope.functions[decl.Name()] = decl
			}
		}
	}

	if didChange(file.analyzeDiagnostics, diagnostics) {
		file.collectSymbolsDiagnostics = diagnostics
		file.collectSymbolsDiagnosticsChanged = true
	}
}

func didChange(old, new []utils.Diagnostic) bool {
	return (len(new) == 0 && len(old) > 0) || (len(new) > 0 && len(old) == 0)
}
