package httpfile

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/sirbuni-dev-tools/httpgen/internal/endpoints"
	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/payload"
)

// Writer turns Spring endpoints and SPQR operations into HTTP Client requests.
type Writer struct {
	Index  *java.Index
	Hints  *payload.Hints
	Source *fake.Source
	// Mode is how bodies and parameters are written: placeholders by default.
	Mode payload.Mode
	// Depth limits nested objects and GraphQL selections; 3 when 0.
	Depth int
	// GraphQLPath is SPQR's endpoint, /graphql when empty.
	GraphQLPath string
}

func (w *Writer) walker() *payload.Walker {
	return &payload.Walker{Index: w.Index, Hints: w.Hints, MaxDepth: w.Depth}
}

// leaf is the value one parameter gets, by the same rules as a field.
func (w *Writer) leaf(key string, p java.Param, ctx *java.TypeDecl, m java.Method) payload.Node {
	obj := w.walker().Params([]payload.Param{{Key: key, Name: p.Name, Type: p.Type, Annotations: p.Annotations}}, ctx, m.TypeParams)
	return obj.Fields[0].Value
}

type pair struct{ name, value string }

// REST writes one Spring MVC endpoint.
func (w *Writer) REST(e endpoints.Endpoint) Request {
	var vars []Var
	addVar := func(name, value string) {
		for _, v := range vars {
			if v.Name == name {
				return
			}
		}
		vars = append(vars, Var{name, value})
	}
	path := e.Path
	for _, a := range e.PathVars {
		path = strings.ReplaceAll(path, "{"+a.Key+"}", "{{"+a.Key+"}}")
		addVar(a.Key, w.fixedValue(a.Key, w.leaf(a.Key, a.Param, e.Class, e.Method)))
	}

	// An id picks the record to update or delete, so it is a file variable you set, never a random value.
	var params []pair
	for _, a := range e.Params {
		node := w.leaf(a.Key, a.Param, e.Class, e.Method)
		if isID(a.Key, node) {
			addVar(a.Key, w.fixedValue(a.Key, node))
			params = append(params, pair{a.Key, "{{" + a.Key + "}}"})
		} else {
			params = append(params, pair{a.Key, payload.RenderPlain(node, w.Mode, w.Source)})
		}
	}
	if e.Pageable {
		params = append(params, pair{"page", "0"}, pair{"size", "20"})
	}
	// A JSON body leaves no room for a form, so parameters then go to the query string as well.
	inQuery := e.Verb == "GET" || e.Body != nil

	line := e.Verb + " {{HOST}}" + path
	var b strings.Builder
	b.WriteString("### " + e.Verb + " " + Humanize(e.Method.Name) + "\n")
	// The name lists the request in the Services tool window; Merge keeps it unique per file.
	b.WriteString("# @name " + e.Method.Name + "\n")
	b.WriteString(line)
	if inQuery {
		for i, p := range params {
			sep := "    &"
			if i == 0 {
				sep = "    ?"
			}
			b.WriteString("\n" + sep + p.name + "=" + p.value)
		}
	}
	b.WriteString("\n")
	for _, h := range e.Headers {
		b.WriteString(h.Key + ": " + payload.RenderPlain(w.leaf(h.Key, h.Param, e.Class, e.Method), w.Mode, w.Source) + "\n")
	}
	switch {
	case e.Body != nil:
		body := w.walker()
		body.SkipGenerated = true
		b.WriteString("Content-Type: application/json\n\n")
		b.WriteString(payload.Render(body.Type(e.Body.Type, e.Class, e.Method.TypeParams), w.Mode, w.Source) + "\n")
	case !inQuery && len(params) > 0:
		b.WriteString("Content-Type: application/x-www-form-urlencoded\n\n")
		for i, p := range params {
			if i > 0 {
				b.WriteString("&\n")
			}
			b.WriteString(p.name + "=" + p.value)
		}
		b.WriteString("\n")
	}
	return Request{Line: line, Text: b.String(), Vars: vars}
}

func isID(name string, node payload.Node) bool {
	v, ok := node.(payload.Value)
	if !ok {
		return false
	}
	switch v.Gen.(type) {
	case payload.IntRange, payload.UUID:
	default:
		return false
	}
	n := strings.ToLower(name)
	return strings.HasSuffix(n, "id") || n == "uid" || n == "uuid"
}

// fixedValue is a path variable or id value: edited by hand to point at a real record, so ids start at 1.
func (w *Writer) fixedValue(name string, node payload.Node) string {
	if v, ok := node.(payload.Value); ok {
		if _, isRange := v.Gen.(payload.IntRange); isRange && strings.HasSuffix(strings.ToLower(name), "id") {
			return "1"
		}
	}
	return urlSafe(payload.RenderPlain(node, payload.Fixed, w.Source))
}

var urlUnsafe = regexp.MustCompile(`[^A-Za-z0-9._~-]+`)

// urlSafe keeps the first URL-safe word: file variables land in paths and query strings, so
// "Outdoors & Tools" becomes "Outdoors".
func urlSafe(value string) string {
	for _, part := range urlUnsafe.Split(value, -1) {
		if part != "" {
			return part
		}
	}
	return "1"
}

// GraphQL writes one SPQR operation as a GRAPHQL request with operation variables and a selection set.
func (w *Writer) GraphQL(op endpoints.Operation) Request {
	opName := op.Name
	if opName != "" {
		opName = strings.ToUpper(opName[:1]) + opName[1:]
	}
	var decls, args []string
	for _, a := range op.Args {
		decls = append(decls, "$"+a.Key+": "+w.inputType(a.Param.Type, op.Class, op.Method.TypeParams)+"!")
		args = append(args, a.Key+": $"+a.Key)
	}
	header := op.Kind + " " + opName
	if len(decls) > 0 {
		header += "(" + strings.Join(decls, ", ") + ")"
	}
	header += " {"
	call := op.Name
	if len(args) > 0 {
		call += "(" + strings.Join(args, ", ") + ")"
	}
	selection := ""
	if op.Method.Return != nil {
		out := w.walker()
		out.IncludeGetters = true
		selection = selectionSet(out.Type(*op.Method.Return, op.Class, op.Method.TypeParams), 2)
	}

	var vars []Var
	var values []pair
	input := w.walker()
	input.SkipGenerated = true
	for _, a := range op.Args {
		obj := input.Params([]payload.Param{{Key: a.Key, Name: a.Param.Name, Type: a.Param.Type, Annotations: a.Param.Annotations}}, op.Class, op.Method.TypeParams)
		node := obj.Fields[0].Value
		if v, ok := node.(payload.Value); ok && isID(a.Key, node) {
			// Like REST path variables: you choose the record, it is never random.
			value, quoted := "1", "{{"+a.Key+"}}"
			if v.Gen.IsString() {
				value, quoted = "00000000-0000-0000-0000-000000000000", `"{{`+a.Key+`}}"`
			}
			if !hasVar(vars, a.Key) {
				vars = append(vars, Var{a.Key, value})
			}
			values = append(values, pair{a.Key, quoted})
			continue
		}
		values = append(values, pair{a.Key, payload.Render(node, w.Mode, w.Source)})
	}

	path := w.GraphQLPath
	if path == "" {
		path = "/graphql"
	}
	var b strings.Builder
	b.WriteString("### GRAPHQL " + Humanize(op.Name) + "\n")
	b.WriteString("# @name " + op.Name + "\n")
	b.WriteString("GRAPHQL {{HOST}}" + path + "\n")
	b.WriteString("Authorization: Bearer {{access_token}}\n\n")
	b.WriteString(header + "\n")
	b.WriteString("  " + call)
	if selection != "" {
		b.WriteString(" " + selection)
	}
	b.WriteString("\n}\n")
	if len(values) > 0 {
		b.WriteString("\n{\n")
		for i, v := range values {
			if i > 0 {
				b.WriteString(",\n")
			}
			b.WriteString(`  "` + v.name + `": ` + strings.ReplaceAll(v.value, "\n", "\n  "))
		}
		b.WriteString("\n}\n")
	}
	return Request{Line: header, Text: b.String(), Vars: vars}
}

func hasVar(vars []Var, name string) bool {
	for _, v := range vars {
		if v.Name == name {
			return true
		}
	}
	return false
}

// selectionSet is { a b c { d } } over every field with something to select; nested objects need fields of their own.
func selectionSet(n payload.Node, indent int) string {
	var obj payload.Object
	switch v := n.(type) {
	case payload.Object:
		obj = v
	case payload.Array:
		return selectionSet(v.Elem, indent)
	default:
		return ""
	}
	pad := strings.Repeat(" ", indent+2)
	var lines []string
	for _, f := range obj.Fields {
		switch f.Value.(type) {
		case payload.Value:
			lines = append(lines, pad+f.Key)
		case payload.Object, payload.Array:
			if s := selectionSet(f.Value, indent+2); s != "" {
				lines = append(lines, pad+f.Key+" "+s)
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "{\n" + strings.Join(lines, "\n") + "\n" + strings.Repeat(" ", indent) + "}"
}

var graphQLPrimitives = map[string]string{
	"int": "Int", "long": "Long", "short": "Short", "byte": "Byte", "double": "Float", "float": "Float",
	"boolean": "Boolean", "char": "Char",
}

var graphQLScalars = map[string]string{
	"java.lang.String": "String", "java.lang.Integer": "Int", "java.lang.Long": "Long", "java.lang.Short": "Short",
	"java.lang.Byte": "Byte", "java.lang.Double": "Float", "java.lang.Float": "Float", "java.lang.Boolean": "Boolean",
	"java.lang.Character": "Char", "java.math.BigDecimal": "BigDecimal", "java.math.BigInteger": "BigInteger",
	"java.util.UUID": "UUID", "java.time.LocalDate": "LocalDate", "java.time.LocalDateTime": "LocalDateTime",
	"java.time.LocalTime": "LocalTime", "java.time.Instant": "Instant", "java.time.OffsetDateTime": "OffsetDateTime",
	"java.time.ZonedDateTime": "ZonedDateTime", "java.util.Date": "Date", "java.lang.Object": "Object",
}

var iterables = map[string]bool{
	"java.lang.Iterable": true, "java.util.Collection": true, "java.util.List": true, "java.util.ArrayList": true,
	"java.util.LinkedList": true, "java.util.Set": true, "java.util.HashSet": true, "java.util.LinkedHashSet": true,
	"java.util.TreeSet": true, "java.util.SortedSet": true, "java.util.Queue": true, "java.util.Deque": true,
}

// inputType is the GraphQL type SPQR gives a Java argument type: scalars by their usual names, enums by
// their simple name, other classes as <SimpleName>Input, collections and arrays as lists.
func (w *Writer) inputType(ref java.TypeRef, ctx *java.TypeDecl, typeParams []string) string {
	if ref.Dims > 0 {
		elem := ref
		elem.Dims--
		return "[" + w.inputType(elem, ctx, typeParams) + "]"
	}
	if s, ok := graphQLPrimitives[ref.Name]; ok {
		return s
	}
	for _, p := range typeParams {
		if p == ref.Name {
			return "String"
		}
	}
	r := w.Index.Resolve(ref.Name, ctx)
	if s, ok := graphQLScalars[r.Name]; ok {
		return s
	}
	if iterables[r.Name] {
		if len(ref.Args) > 0 {
			return "[" + w.inputType(ref.Args[0], ctx, typeParams) + "]"
		}
		return "[String]"
	}
	if r.Name == "java.util.Optional" {
		if len(ref.Args) > 0 {
			return w.inputType(ref.Args[0], ctx, typeParams)
		}
		return "String"
	}
	if _, ok := java.LibraryEnums[r.Name]; ok || (r.Decl != nil && r.Decl.Kind == java.Enum) {
		return ref.SimpleName()
	}
	return ref.SimpleName() + "Input"
}

// Humanize turns updateEmployee_salary into "update employee salary", as titles in a hand-written file read.
func Humanize(name string) string {
	var words []string
	var cur []rune
	runes := []rune(name)
	for i, r := range runes {
		if r == '_' {
			if len(cur) > 0 {
				words = append(words, string(cur))
				cur = nil
			}
			continue
		}
		if unicode.IsUpper(r) && i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) && len(cur) > 0 {
			words = append(words, string(cur))
			cur = nil
		}
		cur = append(cur, unicode.ToLower(r))
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return strings.Join(words, " ")
}

// Login logs in once per file and keeps the token for every request below it. Values live in the env files.
func Login() Request {
	return Request{
		Line: "POST {{AUTH_URL}}",
		Text: `### POST login (run first; the token lasts until it expires)
# @name login
POST {{AUTH_URL}}
Authorization: Basic {{CLIENT_ID}} {{CLIENT_SECRET}}
Content-Type: application/x-www-form-urlencoded

grant_type={{GRANT_TYPE}}&
username={{USERNAME}}&
password={{PASSWORD}}

> {% client.global.set("access_token", response.body.access_token); %}
`,
	}
}
