// Package httpfile writes IntelliJ HTTP Client requests and merges them into .http and env files,
// the same text the API HTTP Generator plugin writes.
package httpfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Request is one request ready for a .http file, plus the @name = value file variables it uses.
type Request struct {
	// Line is the request line, such as POST {{HOST}}/employees/{{id}}, or the operation line of a
	// GraphQL request: how an existing copy is recognised.
	Line string
	Text string
	Vars []Var
}

// Var is one file variable.
type Var struct{ Name, Value string }

// Name is the request's # @name, or "" when it has none.
func (r Request) Name() string {
	if m := nameLine.FindStringSubmatch(r.Text); m != nil {
		return m[1]
	}
	return ""
}

// Merged is a file after a merge.
type Merged struct {
	Text    string
	Added   []Request
	Skipped []Request
}

var (
	nameLine     = regexp.MustCompile(`(?m)^(?:#|//)\s*@name\s*=?\s*(\S+)`)
	declaredVar  = regexp.MustCompile(`(?m)^@([\w.-]+)\s*=`)
	firstRequest = regexp.MustCompile(`(?m)^###`)
)

// Merge appends the requests a file does not have yet. A request counts as present when a line equals
// its request line, so edits to its headers or body are kept. Missing file variables go above the
// first request, and every added # @name is made unique in the file.
func Merge(existing string, requests []Request) Merged {
	lines := map[string]bool{}
	for _, l := range strings.Split(existing, "\n") {
		lines[strings.TrimSpace(l)] = true
	}
	var added, skipped []Request
	seen := map[string]bool{}
	for _, r := range requests {
		if seen[r.Line] {
			continue
		}
		seen[r.Line] = true
		if lines[r.Line] {
			skipped = append(skipped, r)
		} else {
			added = append(added, r)
		}
	}

	declared := map[string]bool{}
	for _, m := range declaredVar.FindAllStringSubmatch(existing, -1) {
		declared[m[1]] = true
	}
	var vars strings.Builder
	for _, r := range added {
		for _, v := range r.Vars {
			if !declared[v.Name] {
				declared[v.Name] = true
				fmt.Fprintf(&vars, "@%s = %s\n", v.Name, v.Value)
			}
		}
	}

	used := map[string]bool{}
	for _, m := range nameLine.FindAllStringSubmatch(existing, -1) {
		used[m[1]] = true
	}
	for i := range added {
		added[i] = uniqueName(added[i], used)
	}

	text := existing
	if vars.Len() > 0 {
		at := len(text)
		if loc := firstRequest.FindStringIndex(text); loc != nil {
			at = loc[0]
		}
		head := text[:at]
		sep := ""
		if head != "" && !strings.HasSuffix(head, "\n") {
			sep = "\n"
		}
		text = head + sep + vars.String() + "\n" + text[at:]
	}
	for _, r := range added {
		text = strings.TrimRight(text, "\n")
		if text != "" {
			text += "\n\n"
		}
		text += r.Text
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return Merged{Text: text, Added: added, Skipped: skipped}
}

// uniqueName renames the second request called name to name2, and so on: overloaded handlers share a method name.
func uniqueName(r Request, used map[string]bool) Request {
	name := r.Name()
	if name == "" {
		return r
	}
	candidate := name
	for n := 2; used[candidate]; n++ {
		candidate = fmt.Sprintf("%s%d", name, n)
	}
	used[candidate] = true
	if candidate != name {
		r.Text = strings.Replace(r.Text, "# @name "+name+"\n", "# @name "+candidate+"\n", 1)
	}
	return r
}

// Host is http://localhost:<server.port><server.servlet.context-path> from application.properties or
// .yml, reading the default of a ${VAR:default} placeholder. Port 8080 and no context path when unset.
func Host(properties, yaml string) string {
	port := firstOf(property(properties, "server.port"), yamlValue(yaml, "server", "port"), "8080")
	ctx := strings.Trim(firstOf(property(properties, "server.servlet.context-path"),
		yamlValue(yaml, "server", "servlet", "context-path")), "/")
	if ctx != "" {
		ctx = "/" + ctx
	}
	return "http://localhost:" + port + ctx
}

// GraphQLPath is SPQR's endpoint from graphql.spqr.http.endpoint, /graphql when unset.
func GraphQLPath(properties, yaml string) string {
	p := firstOf(property(properties, "graphql.spqr.http.endpoint"), yamlValue(yaml, "graphql", "spqr", "http", "endpoint"))
	if strings.TrimSpace(p) == "" {
		return "/graphql"
	}
	return "/" + strings.Trim(p, "/")
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func property(text, key string) string {
	found := ""
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, key+"=") || strings.HasPrefix(l, key+":") || strings.HasPrefix(l, key+" ") {
			found = resolveDefault(strings.TrimLeft(l[len(key):], " =:"))
		}
	}
	return found
}

// yamlValue reads enough YAML for nested server: keys; no anchors or lists.
func yamlValue(text string, keys ...string) string {
	depth, indent := 0, -1
	for _, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "---" {
			depth, indent = 0, -1
			continue
		}
		lineIndent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if depth > 0 && lineIndent <= indent {
			depth, indent = 0, -1
		}
		key := keys[depth]
		if strings.HasPrefix(trimmed, key+":") && lineIndent > indent {
			value := strings.TrimSpace(trimmed[len(key)+1:])
			if depth == len(keys)-1 {
				if value == "" {
					return ""
				}
				return resolveDefault(value)
			}
			depth++
			indent = lineIndent
		}
	}
	return ""
}

var placeholderDefault = regexp.MustCompile(`^\$\{[^:}]+:([^}]*)}$`)

func resolveDefault(value string) string {
	v := strings.Trim(strings.TrimSpace(value), `"'`)
	if m := placeholderDefault.FindStringSubmatch(v); m != nil {
		return m[1]
	}
	return v
}

// WithEnvKeys adds missing keys to one environment of an env file (created when existing is empty),
// keeping every existing value and the order of keys. It returns false when nothing changes.
func WithEnvKeys(existing, env string, keys []Var) (string, bool, error) {
	root, err := parseObject(existing)
	if err != nil {
		return "", false, err
	}
	envRaw, ok := root.get(env)
	if !ok {
		envRaw = []byte("{}")
	}
	values, err := parseObject(string(envRaw))
	if err != nil {
		return "", false, fmt.Errorf("environment %q is not an object: %w", env, err)
	}
	changed := strings.TrimSpace(existing) == ""
	for _, k := range keys {
		if _, ok := values.get(k.Name); !ok {
			values.set(k.Name, jsonString(k.Value))
			changed = true
		}
	}
	if !changed {
		return existing, false, nil
	}
	root.set(env, values.compact())
	var out bytes.Buffer
	if err := json.Indent(&out, root.compact(), "", "  "); err != nil {
		return "", false, err
	}
	return out.String() + "\n", true, nil
}

// object is a JSON object that keeps its key order.
type object struct {
	keys   []string
	values map[string]json.RawMessage
}

func parseObject(text string) (*object, error) {
	o := &object{values: map[string]json.RawMessage{}}
	if strings.TrimSpace(text) == "" {
		return o, nil
	}
	dec := json.NewDecoder(strings.NewReader(text))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("not a JSON object")
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		o.set(t.(string), raw)
	}
	return o, nil
}

func (o *object) get(k string) (json.RawMessage, bool) {
	v, ok := o.values[k]
	return v, ok
}

func (o *object) set(k string, v json.RawMessage) {
	if _, ok := o.values[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.values[k] = v
}

func (o *object) compact() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(jsonString(k))
		b.WriteByte(':')
		b.Write(o.values[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// jsonString quotes s without HTML escaping, so a URL keeps its & and = as typed.
func jsonString(s string) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(b.Bytes(), "\n")
}
