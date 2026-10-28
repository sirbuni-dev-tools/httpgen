package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/payload"
	"github.com/sirbuni-dev-tools/httpgen/internal/rules"
)

// workspace is the project around a target path: its index and the files the user pointed at.
type workspace struct {
	ix     *java.Index
	module string // nearest folder with a build file, where .local/ goes
	files  []*java.File
	hints  *payload.Hints
}

// errNothing means the target holds nothing to generate (exit code 1).
var errNothing = errors.New("nothing to generate")

// openWorkspace indexes the project that contains target and parses the target files.
func openWorkspace(target string) (*workspace, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, &UsageError{Msg: fmt.Sprintf("cannot read %s: %v", target, err)}
	}
	r, err := rules.Load()
	if err != nil {
		return nil, err
	}
	ix := java.Build(java.SourceRoots(java.ProjectRoot(abs))...)
	byPath := map[string]*java.File{}
	for _, f := range ix.Files {
		p, _ := filepath.Abs(f.Path)
		byPath[p] = f
	}

	var paths []string
	if info.IsDir() {
		_ = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".java") && d.Name() != "module-info.java" {
				paths = append(paths, p)
			}
			return nil
		})
	} else {
		if !strings.HasSuffix(abs, ".java") {
			return nil, &UsageError{Msg: fmt.Sprintf("%s is not a .java file", target)}
		}
		paths = []string{abs}
	}
	sort.Strings(paths)

	ws := &workspace{ix: ix, module: java.ModuleRoot(abs), hints: payload.NewHints(r)}
	for _, p := range paths {
		if f := byPath[p]; f != nil {
			ws.files = append(ws.files, f)
			continue
		}
		// Outside the project's src/main/java (a test folder, a loose file): parse it on its own.
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := java.Parse(p, string(src))
		if err != nil {
			return nil, err
		}
		ix.Add(f)
		ws.files = append(ws.files, f)
	}
	return ws, nil
}

// types lists the top-level types of the target files, or the type named by class (top-level or nested).
func (ws *workspace) types(class string) []*java.TypeDecl {
	var out []*java.TypeDecl
	var visit func(t *java.TypeDecl)
	visit = func(t *java.TypeDecl) {
		if class == "" {
			if t.Outer == nil {
				out = append(out, t)
			}
			return
		}
		if t.Name == class || t.QualifiedName() == class {
			out = append(out, t)
		}
		for _, n := range t.Nested {
			visit(n)
		}
	}
	for _, f := range ws.files {
		for _, t := range f.Types {
			visit(t)
		}
	}
	return out
}

func (ws *workspace) walker() *payload.Walker {
	return &payload.Walker{Index: ws.ix, Hints: ws.hints}
}

// rel shows a path relative to the working directory when that is shorter.
func rel(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if r, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}
