package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/endpoints"
)

// listed is one endpoint or operation, as --list --format json writes it.
type listed struct {
	Kind   string `json:"kind"`           // rest, query or mutation
	Verb   string `json:"verb,omitempty"` // REST only
	Path   string `json:"path,omitempty"` // REST only
	Name   string `json:"name,omitempty"` // GraphQL operation name
	Class  string `json:"class"`
	Method string `json:"method"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// runList implements --list: one line per endpoint and per operation.
func runList(o Options, t Terminal) int {
	ws, err := openWorkspace(o.Target)
	if code, done := reportOpenError(err, t); done {
		return code
	}
	var items []listed
	for _, c := range ws.types(o.Class) {
		file := ""
		if c.File != nil {
			file = rel(c.File.Path)
		}
		for _, e := range endpoints.Endpoints(ws.ix, c) {
			items = append(items, listed{Kind: "rest", Verb: e.Verb, Path: e.Path, Class: c.Name, Method: e.Method.Name, File: file, Line: e.Method.Line})
		}
		for _, op := range endpoints.Operations(ws.ix, c) {
			items = append(items, listed{Kind: op.Kind, Name: op.Name, Class: c.Name, Method: op.Method.Name, File: file, Line: op.Method.Line})
		}
	}
	if len(items) == 0 {
		fmt.Fprintf(t.Err, "httpgen: no Spring request mappings or SPQR resolvers found in %s%s\n", o.Target, filterNote(o))
		return ExitNothing
	}
	if o.Format == "json" {
		out, _ := json.Marshal(items)
		fmt.Fprintln(t.Out, string(out))
		return ExitOK
	}
	rows := make([][3]string, len(items))
	width := [2]int{}
	for i, it := range items {
		if it.Kind == "rest" {
			rows[i] = [3]string{it.Verb, it.Path, it.Class + "." + it.Method}
		} else {
			rows[i] = [3]string{it.Kind, it.Name, it.Class + "." + it.Method}
		}
		width[0] = max(width[0], len(rows[i][0]))
		width[1] = max(width[1], len(rows[i][1]))
	}
	for _, r := range rows {
		line := fmt.Sprintf("%-*s  %-*s  %s", width[0], r[0], width[1], r[1], r[2])
		fmt.Fprintln(t.Out, strings.TrimRight(line, " "))
	}
	return ExitOK
}
