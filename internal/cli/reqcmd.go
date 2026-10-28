package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/endpoints"
	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
	"github.com/sirbuni-dev-tools/httpgen/internal/httpfile"
	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/payload"
)

const (
	envFile        = "http-client.env.json"
	privateEnvFile = "http-client.private.env.json"
)

// httpFile is the requests of one class and where they go.
type httpFile struct {
	class    string
	dir      string
	requests []httpfile.Request
	graphql  bool
	host     string
}

func (f httpFile) path() string { return filepath.Join(f.dir, f.class+".http") }

// fileReport is what --req did, or would do, to one .http file.
type fileReport struct {
	Path    string   `json:"path"`
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Env     []string `json:"env,omitempty"`
}

// runReq implements --req: one .http file per controller or resolver, merged with what is there.
func runReq(o Options, t Terminal) int {
	ws, err := openWorkspace(o.Target)
	if code, done := reportOpenError(err, t); done {
		return code
	}
	files := collectRequests(ws, o)
	if len(files) == 0 {
		fmt.Fprintf(t.Err, "httpgen: no Spring request mappings or SPQR resolvers found in %s%s\n", o.Target, filterNote(o))
		return ExitNothing
	}

	if o.Stdout {
		for i, f := range files {
			if len(files) > 1 {
				if i > 0 {
					fmt.Fprintln(t.Out)
				}
				fmt.Fprintf(t.Out, "# file: %s.http\n\n", f.class)
			}
			fmt.Fprint(t.Out, httpfile.Merge("", f.requests).Text)
		}
		return ExitOK
	}

	var reports []fileReport
	for _, f := range files {
		existing, err := os.ReadFile(f.path())
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(t.Err, "httpgen: %v\n", err)
			return ExitUsage
		}
		merged := httpfile.Merge(string(existing), f.requests)
		r := fileReport{Path: rel(f.path()), Added: names(merged.Added), Skipped: names(merged.Skipped)}
		if !o.DryRun {
			if err := os.MkdirAll(f.dir, 0o755); err != nil {
				fmt.Fprintf(t.Err, "httpgen: %v\n", err)
				return ExitUsage
			}
			if len(merged.Added) > 0 {
				if err := os.WriteFile(f.path(), []byte(merged.Text), 0o644); err != nil {
					fmt.Fprintf(t.Err, "httpgen: %v\n", err)
					return ExitUsage
				}
			}
		}
		env, err := updateEnvFiles(f, o)
		if err != nil {
			fmt.Fprintf(t.Err, "httpgen: %v\n", err)
			return ExitUsage
		}
		r.Env = env
		reports = append(reports, r)
	}
	printReports(reports, o, t)
	return ExitOK
}

// collectRequests writes the requests for every controller and resolver in the target.
func collectRequests(ws *workspace, o Options) []httpFile {
	wanted := map[string]bool{}
	for _, m := range o.Methods {
		wanted[m] = true
	}
	keep := func(m java.Method) bool { return len(wanted) == 0 || wanted[m.Name] }
	mode := payload.Placeholders
	if o.Mode == "fixed" {
		mode = payload.Fixed
	}
	src := fake.New(uint64(o.Seed))

	var files []httpFile
	for _, t := range ws.types(o.Class) {
		module := ws.module
		if t.File != nil && t.File.Path != "" {
			module = java.ModuleRoot(t.File.Path)
		}
		props, yaml := appConfig(module)
		w := &httpfile.Writer{Index: ws.ix, Hints: ws.hints, Source: src, Mode: mode, Depth: o.Depth,
			GraphQLPath: httpfile.GraphQLPath(props, yaml)}
		f := httpFile{class: t.Name, dir: o.Out, host: o.Host}
		if f.dir == "" {
			f.dir = filepath.Join(module, ".local", "http")
		}
		if f.host == "" {
			f.host = httpfile.Host(props, yaml)
		}
		for _, e := range endpoints.Endpoints(ws.ix, t) {
			if keep(e.Method) {
				f.requests = append(f.requests, w.REST(e))
			}
		}
		for _, op := range endpoints.Operations(ws.ix, t) {
			if keep(op.Method) {
				f.requests = append(f.requests, w.GraphQL(op))
				f.graphql = true
			}
		}
		if len(f.requests) == 0 {
			continue
		}
		// GraphQL requests need a token first, so the login request leads the file.
		if f.graphql && !o.NoLogin {
			f.requests = append([]httpfile.Request{httpfile.Login()}, f.requests...)
		}
		files = append(files, f)
	}
	return files
}

// appConfig reads application.properties and application.yml (or .yaml) of a module; missing files are empty.
func appConfig(module string) (props, yaml string) {
	res := filepath.Join(module, "src", "main", "resources")
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(res, name))
		if err != nil {
			return ""
		}
		return string(b)
	}
	props = read("application.properties")
	if yaml = read("application.yml"); yaml == "" {
		yaml = read("application.yaml")
	}
	return props, yaml
}

// updateEnvFiles adds missing keys to the env files next to the .http file and returns the files that change.
func updateEnvFiles(f httpFile, o Options) ([]string, error) {
	shared := []httpfile.Var{{Name: "HOST", Value: f.host}}
	var private []httpfile.Var
	if f.graphql && !o.NoLogin {
		shared = append(shared,
			httpfile.Var{Name: "AUTH_URL", Value: f.host + "/oauth2/token"},
			httpfile.Var{Name: "GRANT_TYPE", Value: "password"},
			httpfile.Var{Name: "CLIENT_ID", Value: ""},
			httpfile.Var{Name: "USERNAME", Value: ""})
		private = []httpfile.Var{{Name: "CLIENT_SECRET", Value: ""}, {Name: "PASSWORD", Value: ""}}
	}
	var changed []string
	for _, e := range []struct {
		name string
		keys []httpfile.Var
	}{{envFile, shared}, {privateEnvFile, private}} {
		if len(e.keys) == 0 {
			continue
		}
		path := filepath.Join(f.dir, e.name)
		existing, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		text, ok, err := httpfile.WithEnvKeys(string(existing), o.Env, e.keys)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel(path), err)
		}
		if !ok {
			continue
		}
		changed = append(changed, rel(path))
		if !o.DryRun {
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return changed, nil
}

func names(requests []httpfile.Request) []string {
	out := []string{}
	for _, r := range requests {
		if n := r.Name(); n != "" {
			out = append(out, n)
		} else {
			out = append(out, r.Line)
		}
	}
	return out
}

func printReports(reports []fileReport, o Options, t Terminal) {
	if o.Format == "json" {
		out, _ := json.Marshal(map[string]any{"files": reports, "dryRun": o.DryRun})
		fmt.Fprintln(t.Out, string(out))
		return
	}
	if o.Quiet {
		return
	}
	verb, envVerb := "added", "updated"
	if o.DryRun {
		verb, envVerb = "would add", "would update"
	}
	for _, r := range reports {
		switch {
		case len(r.Added) == 0:
			fmt.Fprintf(t.Out, "%s: nothing new, %d already there\n", r.Path, len(r.Skipped))
		case len(r.Skipped) == 0:
			fmt.Fprintf(t.Out, "%s: %s %s\n", r.Path, verb, strings.Join(r.Added, ", "))
		default:
			fmt.Fprintf(t.Out, "%s: %s %s; %d already there\n", r.Path, verb, strings.Join(r.Added, ", "), len(r.Skipped))
		}
		for _, e := range r.Env {
			fmt.Fprintf(t.Out, "%s: %s\n", e, envVerb)
		}
	}
}
