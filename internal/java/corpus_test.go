package java

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseCorpus parses every .java file under the folders in HTTPGEN_CORPUS (separated by the
// OS path list separator). It is skipped by default; run it against real projects to find gaps:
//
//	HTTPGEN_CORPUS=/path/to/project/src go test ./internal/java -run TestParseCorpus -v
func TestParseCorpus(t *testing.T) {
	dirs := os.Getenv("HTTPGEN_CORPUS")
	if dirs == "" {
		t.Skip("set HTTPGEN_CORPUS to run")
	}
	files, failed := 0, 0
	annotated := map[string]int{}
	var count func(types []*TypeDecl)
	count = func(types []*TypeDecl) {
		for _, t := range types {
			for _, m := range t.Methods {
				for _, a := range m.Annotations {
					annotated[a.SimpleName()]++
				}
			}
			count(t.Nested)
		}
	}
	for _, dir := range filepath.SplitList(dirs) {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".java") || strings.HasSuffix(path, "module-info.java") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			files++
			f, err := Parse(path, string(src))
			if err == nil {
				count(f.Types)
			}
			if err != nil {
				failed++
				if failed <= 30 {
					t.Errorf("%v", err)
				}
			}
			return nil
		})
	}
	t.Logf("parsed %d files, %d failed", files, failed)
	for _, name := range []string{"GetMapping", "PostMapping", "PutMapping", "PatchMapping", "DeleteMapping", "RequestMapping", "GraphQLQuery", "GraphQLMutation"} {
		t.Logf("methods with @%s: %d", name, annotated[name])
	}
}

// TestResolveCorpus indexes each project in HTTPGEN_CORPUS (project roots, separated by the OS path
// list separator) and logs the field and parameter type names that do not resolve, most frequent first.
func TestResolveCorpus(t *testing.T) {
	dirs := os.Getenv("HTTPGEN_CORPUS_PROJECTS")
	if dirs == "" {
		t.Skip("set HTTPGEN_CORPUS_PROJECTS to run")
	}
	for _, root := range filepath.SplitList(dirs) {
		ix := Build(SourceRoots(root)...)
		unknown := map[string]int{}
		total := 0
		check := func(ref TypeRef, ctx *TypeDecl, typeParams []string) {
			var walk func(r TypeRef)
			walk = func(r TypeRef) {
				if r.Name == "?" {
					if r.Bound != nil {
						walk(*r.Bound)
					}
					return
				}
				for _, a := range r.Args {
					walk(a)
				}
				if primitives[r.Name] || containsString(typeParams, r.Name) {
					return
				}
				total++
				if !ix.Resolve(r.Name, ctx).Known() {
					unknown[r.Name]++
				}
			}
			walk(ref)
		}
		for _, td := range ix.Types() {
			params := td.TypeParams
			for o := td.Outer; o != nil; o = o.Outer {
				params = append(params, o.TypeParams...)
			}
			for _, f := range td.Fields {
				check(f.Type, td, params)
			}
			for _, c := range td.Components {
				check(c.Type, td, params)
			}
			for _, m := range td.Methods {
				for _, p := range m.Params {
					check(p.Type, td, append(params, m.TypeParams...))
				}
			}
		}
		type kv struct {
			k string
			v int
		}
		var list []kv
		missing := 0
		for k, v := range unknown {
			list = append(list, kv{k, v})
			missing += v
		}
		sortSlice(list, func(a, b kv) bool { return a.v > b.v })
		t.Logf("%s: %d types, %d type uses, %d unresolved", filepath.Base(root), len(ix.Types()), total, missing)
		for i, e := range list {
			if i == 15 {
				break
			}
			t.Logf("    %-40s %d", e.k, e.v)
		}
	}
}

var primitives = map[string]bool{"int": true, "long": true, "short": true, "byte": true, "double": true, "float": true, "boolean": true, "char": true, "void": true}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sortSlice[T any](s []T, less func(a, b T) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
