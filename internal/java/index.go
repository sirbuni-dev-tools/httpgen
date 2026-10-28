package java

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Index holds every type of a project's sources, by qualified name, and resolves names the way javac does.
type Index struct {
	types     map[string]*TypeDecl
	byPackage map[string][]*TypeDecl
	Files     []*File
	// ParseErrors lists files that could not be read; the rest of the project is still usable.
	ParseErrors []error
}

// NewIndex returns an empty index.
func NewIndex() *Index {
	return &Index{types: map[string]*TypeDecl{}, byPackage: map[string][]*TypeDecl{}}
}

// skipDirs are never source folders.
var skipDirs = map[string]bool{".git": true, "target": true, "build": true, "out": true, "node_modules": true, ".gradle": true, ".idea": true, ".local": true}

// ProjectRoot walks up from path to the outermost folder that still holds a build file
// (pom.xml, build.gradle, build.gradle.kts, settings.gradle, settings.gradle.kts).
func ProjectRoot(path string) string {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	dir, _ = filepath.Abs(dir)
	root := ""
	for d := dir; ; d = filepath.Dir(d) {
		if hasBuildFile(d) {
			root = d
		} else if root != "" {
			break
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	if root == "" {
		return dir
	}
	return root
}

// ModuleRoot is the nearest folder at or above path that holds a build file, else path's folder.
func ModuleRoot(path string) string {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	dir, _ = filepath.Abs(dir)
	for d := dir; ; d = filepath.Dir(d) {
		if hasBuildFile(d) {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

func hasBuildFile(dir string) bool {
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	return false
}

// SourceRoots finds every src/main/java under a project root; without any, the root itself is used.
func SourceRoots(projectRoot string) []string {
	var roots []string
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if skipDirs[d.Name()] && path != projectRoot {
			return filepath.SkipDir
		}
		if strings.HasSuffix(filepath.ToSlash(path), "/src/main/java") {
			roots = append(roots, path)
			return filepath.SkipDir
		}
		return nil
	})
	if len(roots) == 0 {
		roots = []string{projectRoot}
	}
	sort.Strings(roots)
	return roots
}

// Build parses every .java file under the given folders.
func Build(roots ...string) *Index {
	ix := NewIndex()
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipDirs[d.Name()] && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".java") || d.Name() == "module-info.java" {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				ix.ParseErrors = append(ix.ParseErrors, err)
				return nil
			}
			f, err := Parse(path, string(src))
			if err != nil {
				ix.ParseErrors = append(ix.ParseErrors, err)
				return nil
			}
			ix.Add(f)
			return nil
		})
	}
	return ix
}

// Add indexes a parsed file and its nested types.
func (ix *Index) Add(f *File) {
	ix.Files = append(ix.Files, f)
	var add func(t *TypeDecl)
	add = func(t *TypeDecl) {
		ix.types[t.QualifiedName()] = t
		if t.Outer == nil {
			ix.byPackage[f.Package] = append(ix.byPackage[f.Package], t)
		}
		for _, n := range t.Nested {
			add(n)
		}
	}
	for _, t := range f.Types {
		add(t)
	}
}

// Lookup finds a project type by qualified name.
func (ix *Index) Lookup(qualified string) *TypeDecl { return ix.types[qualified] }

// Types lists every indexed type, in a stable order.
func (ix *Index) Types() []*TypeDecl {
	names := make([]string, 0, len(ix.types))
	for n := range ix.types {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*TypeDecl, len(names))
	for i, n := range names {
		out[i] = ix.types[n]
	}
	return out
}

// Resolved is the outcome of resolving a type name.
type Resolved struct {
	// Name is the qualified name when known, else the name as written.
	Name string
	// Decl is the project's declaration, nil for library or unknown types.
	Decl *TypeDecl
	// Library is true for a type in the built-in table (java.util.List, Spring Data Page, ...).
	Library bool
}

// Known reports whether the name was resolved to a project or library type.
func (r Resolved) Known() bool { return r.Decl != nil || r.Library }

// Resolve finds what a type name written inside ctx refers to. Type parameters are the caller's business.
func (ix *Index) Resolve(name string, ctx *TypeDecl) Resolved {
	if strings.Contains(name, ".") {
		if t := ix.types[name]; t != nil {
			return Resolved{Name: name, Decl: t}
		}
		if libraryTypes[name] {
			return Resolved{Name: name, Library: true}
		}
		// Outer.Inner written relative to the context: resolve the first segment, then walk nested types.
		first, rest, _ := strings.Cut(name, ".")
		if head := ix.Resolve(first, ctx); head.Known() {
			if t := ix.types[head.Name+"."+rest]; t != nil {
				return Resolved{Name: head.Name + "." + rest, Decl: t}
			}
			if libraryTypes[head.Name+"."+rest] {
				return Resolved{Name: head.Name + "." + rest, Library: true}
			}
		}
		return Resolved{Name: name}
	}
	// Nested types of the context and its enclosing types, then of their superclasses.
	for c := ctx; c != nil; c = c.Outer {
		for s := c; s != nil; s = ix.superclass(s) {
			if t := ix.types[s.QualifiedName()+"."+name]; t != nil {
				return Resolved{Name: t.QualifiedName(), Decl: t}
			}
		}
	}
	var file *File
	if ctx != nil {
		file = ctx.File
	}
	if file != nil {
		for _, imp := range file.Imports {
			if imp.Static || imp.Wildcard {
				continue
			}
			if imp.Name == name || strings.HasSuffix(imp.Name, "."+name) {
				return ix.byName(imp.Name)
			}
		}
		if t := ix.types[qualify(file.Package, name)]; t != nil {
			return Resolved{Name: t.QualifiedName(), Decl: t}
		}
		for _, imp := range file.Imports {
			if !imp.Wildcard || imp.Static {
				continue
			}
			if r := ix.byName(imp.Name + "." + name); r.Known() {
				return r
			}
		}
	}
	if r := ix.byName("java.lang." + name); r.Known() {
		return r
	}
	return Resolved{Name: name}
}

func (ix *Index) byName(qualified string) Resolved {
	if t := ix.types[qualified]; t != nil {
		return Resolved{Name: qualified, Decl: t}
	}
	return Resolved{Name: qualified, Library: libraryTypes[qualified]}
}

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// Superclass returns the project superclass of a class, if any.
func (ix *Index) Superclass(t *TypeDecl) *TypeDecl { return ix.superclass(t) }

func (ix *Index) superclass(t *TypeDecl) *TypeDecl {
	if t.Kind != Class || len(t.Extends) == 0 {
		return nil
	}
	return ix.resolveDecl(t.Extends[0].Name, t)
}

// resolveDecl resolves a supertype name without the nested-type step, so a class never resolves to its own nested type.
func (ix *Index) resolveDecl(name string, ctx *TypeDecl) *TypeDecl {
	outer := ctx.Outer
	r := ix.Resolve(name, outer)
	if r.Decl == nil && outer == nil {
		// Top-level: resolve in the file's scope.
		r = ix.Resolve(name, &TypeDecl{File: ctx.File})
	}
	return r.Decl
}

// Supertypes returns the project superclass and super-interfaces of a type (not transitively).
func (ix *Index) Supertypes(t *TypeDecl) []*TypeDecl {
	var out []*TypeDecl
	refs := append(append([]TypeRef{}, t.Extends...), t.Implements...)
	for _, ref := range refs {
		if d := ix.resolveDecl(ref.Name, t); d != nil {
			out = append(out, d)
		}
	}
	return out
}

// Constant evaluates an annotation value or constant initializer to text: string and number
// literals, + concatenation, and references to static final constants in the project or the
// built-in library table (such as MediaType.APPLICATION_JSON_VALUE).
func (ix *Index) Constant(e Expr, ctx *TypeDecl) (string, bool) {
	return ix.constant(e, ctx, 0)
}

func (ix *Index) constant(e Expr, ctx *TypeDecl, depth int) (string, bool) {
	if depth > 10 {
		return "", false
	}
	switch v := e.(type) {
	case StrLit:
		return v.Value, true
	case NumLit:
		return strings.TrimRight(v.Text, "lLfFdD"), true
	case BoolLit:
		if v.Value {
			return "true", true
		}
		return "false", true
	case Neg:
		s, ok := ix.constant(v.Value, ctx, depth+1)
		return "-" + s, ok
	case Concat:
		var b strings.Builder
		for _, p := range v.Parts {
			s, ok := ix.constant(p, ctx, depth+1)
			if !ok {
				return "", false
			}
			b.WriteString(s)
		}
		return b.String(), true
	case NameRef:
		return ix.constantRef(v.Name, ctx, depth)
	}
	return "", false
}

func (ix *Index) constantRef(name string, ctx *TypeDecl, depth int) (string, bool) {
	if s, ok := libraryConstants[name]; ok {
		return s, true
	}
	owner, field := "", name
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		owner, field = name[:i], name[i+1:]
	}
	if owner != "" {
		if s, ok := libraryConstants[owner+"."+field]; ok {
			return s, true
		}
		r := ix.Resolve(owner, ctx)
		if s, ok := libraryConstants[r.Name+"."+field]; ok {
			return s, true
		}
		if r.Decl != nil {
			return ix.fieldConstant(r.Decl, field, depth)
		}
		return "", false
	}
	// An unqualified name: the class and its enclosing and super types, then static imports.
	for c := ctx; c != nil; c = c.Outer {
		for s := c; s != nil; s = ix.superclass(s) {
			if v, ok := ix.fieldConstant(s, field, depth); ok {
				return v, true
			}
			for _, sup := range ix.Supertypes(s) {
				if sup.Kind == Interface {
					if v, ok := ix.fieldConstant(sup, field, depth); ok {
						return v, true
					}
				}
			}
		}
	}
	if ctx != nil && ctx.File != nil {
		for _, imp := range ctx.File.Imports {
			if !imp.Static {
				continue
			}
			if imp.Wildcard {
				if t := ix.types[imp.Name]; t != nil {
					if v, ok := ix.fieldConstant(t, field, depth); ok {
						return v, true
					}
				}
			} else if strings.HasSuffix(imp.Name, "."+field) {
				if t := ix.types[strings.TrimSuffix(imp.Name, "."+field)]; t != nil {
					return ix.fieldConstant(t, field, depth)
				}
				if s, ok := libraryConstants[imp.Name]; ok {
					return s, true
				}
			}
		}
	}
	return "", false
}

func (ix *Index) fieldConstant(t *TypeDecl, name string, depth int) (string, bool) {
	for _, f := range t.Fields {
		if f.Name == name && f.Init != nil {
			return ix.constant(f.Init, t, depth+1)
		}
	}
	return "", false
}
