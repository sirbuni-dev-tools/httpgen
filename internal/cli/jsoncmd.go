package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/endpoints"
	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/payload"
)

// namedPayload is one JSON document and the file name stem it is written under.
type namedPayload struct {
	Name string
	Node payload.Node
}

// runJSON implements --json: one payload per class, or per endpoint of a controller or resolver.
func runJSON(o Options, t Terminal) int {
	ws, err := openWorkspace(o.Target)
	if code, done := reportOpenError(err, t); done {
		return code
	}
	payloads := collectPayloads(ws, o)
	if len(payloads) == 0 {
		fmt.Fprintf(t.Err, "httpgen: no classes or endpoints found in %s%s\n", o.Target, filterNote(o))
		return ExitNothing
	}

	mode := payload.Fixed
	if o.Mode == "placeholders" {
		mode = payload.Placeholders
	}
	src := fake.New(uint64(o.Seed))

	if o.Stdout {
		for i, p := range payloads {
			if len(payloads) > 1 {
				if i > 0 {
					fmt.Fprintln(t.Out)
				}
				fmt.Fprintf(t.Out, "// %s\n", p.Name)
			}
			fmt.Fprintln(t.Out, payload.Render(p.Node, mode, src))
		}
		return ExitOK
	}

	dir := o.Out
	if dir == "" {
		dir = filepath.Join(ws.module, ".local", "json")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(t.Err, "httpgen: %v\n", err)
		return ExitUsage
	}
	var written []string
	for _, p := range payloads {
		path := filepath.Join(dir, p.Name+".json")
		if err := os.WriteFile(path, []byte(payload.Render(p.Node, mode, src)+"\n"), 0o644); err != nil {
			fmt.Fprintf(t.Err, "httpgen: %v\n", err)
			return ExitUsage
		}
		written = append(written, rel(path))
	}
	if o.Format == "json" {
		out, _ := json.Marshal(map[string]any{"written": written})
		fmt.Fprintln(t.Out, string(out))
	} else if !o.Quiet {
		for _, w := range written {
			fmt.Fprintf(t.Out, "wrote %s\n", w)
		}
	}
	return ExitOK
}

// collectPayloads picks what to render: classes, or endpoint bodies and parameters.
func collectPayloads(ws *workspace, o Options) []namedPayload {
	// A class on its own is copied as is; endpoint bodies are requests, as in the plugin's writers.
	w, req := ws.walker(), ws.walker()
	req.SkipGenerated = true
	if o.Depth > 0 {
		w.MaxDepth, req.MaxDepth = o.Depth, o.Depth
	}
	wanted := map[string]bool{}
	for _, m := range o.Methods {
		wanted[m] = true
	}
	var out []namedPayload
	for _, t := range ws.types(o.Class) {
		eps := endpoints.Endpoints(ws.ix, t)
		ops := endpoints.Operations(ws.ix, t)
		if len(eps) == 0 && len(ops) == 0 {
			if len(wanted) > 0 || t.Kind == java.Interface || t.Kind == java.Annotation || t.Kind == java.Enum {
				continue
			}
			out = append(out, namedPayload{Name: t.Name, Node: w.Decl(t)})
			continue
		}
		seen := map[string]int{}
		add := func(m java.Method, node payload.Node) {
			if node == nil || (len(wanted) > 0 && !wanted[m.Name]) {
				return
			}
			name := t.Name + "." + m.Name
			if seen[m.Name]++; seen[m.Name] > 1 {
				name = fmt.Sprintf("%s%d", name, seen[m.Name])
			}
			out = append(out, namedPayload{Name: name, Node: node})
		}
		for _, e := range eps {
			if e.Body != nil {
				add(e.Method, req.Type(e.Body.Type, t, e.Method.TypeParams))
				continue
			}
			args := append(append(append([]endpoints.Arg{}, e.PathVars...), e.Params...), e.Headers...)
			add(e.Method, paramsNode(req, args, t, e.Method))
		}
		for _, op := range ops {
			add(op.Method, paramsNode(req, op.Args, t, op.Method))
		}
	}
	return out
}

func paramsNode(w *payload.Walker, args []endpoints.Arg, t *java.TypeDecl, m java.Method) payload.Node {
	if len(args) == 0 {
		return nil
	}
	var params []payload.Param
	for _, a := range args {
		params = append(params, payload.Param{Key: a.Key, Name: a.Param.Name, Type: a.Param.Type, Annotations: a.Param.Annotations})
	}
	return w.Params(params, t, m.TypeParams)
}

func filterNote(o Options) string {
	var parts []string
	if o.Class != "" {
		parts = append(parts, "class "+o.Class)
	}
	if len(o.Methods) > 0 {
		parts = append(parts, "method "+strings.Join(o.Methods, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (looking for " + strings.Join(parts, " and ") + ")"
}

// reportOpenError prints a workspace error with the right exit code.
func reportOpenError(err error, t Terminal) (int, bool) {
	if err == nil {
		return 0, false
	}
	var ue *UsageError
	var pe *java.ParseError
	switch {
	case errors.As(err, &ue):
		fmt.Fprintf(t.Err, "httpgen: %s\n", ue.Msg)
		return ExitUsage, true
	case errors.As(err, &pe):
		fmt.Fprintf(t.Err, "httpgen: cannot parse %s\n", pe.Error())
		return ExitParseError, true
	default:
		fmt.Fprintf(t.Err, "httpgen: %v\n", err)
		return ExitUsage, true
	}
}
