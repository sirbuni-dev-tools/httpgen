// Package endpoints finds Spring MVC handler methods and SPQR GraphQL operations in parsed Java.
package endpoints

import (
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/java"
)

// Arg is a method parameter with the name it travels under.
type Arg struct {
	Key   string
	Param java.Param
}

// Endpoint is one Spring MVC handler method.
type Endpoint struct {
	Verb     string
	Path     string // class and method mapping joined, such as /employees/{id}
	Class    *java.TypeDecl
	Method   java.Method
	PathVars []Arg
	Params   []Arg // @RequestParam and plain simple-typed parameters
	Headers  []Arg
	Body     *java.Param
	Pageable bool
	Consumes string
}

// Operation is one SPQR query or mutation.
type Operation struct {
	Kind   string // query or mutation
	Name   string
	Class  *java.TypeDecl
	Method java.Method
	Args   []Arg
}

var shortcuts = map[string]string{
	"GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT", "PatchMapping": "PATCH", "DeleteMapping": "DELETE",
}

// IsController reports @RestController, @Controller or a class-level @RequestMapping.
func IsController(t *java.TypeDecl) bool {
	return t.Annotation("RestController") != nil || t.Annotation("Controller") != nil || t.Annotation("RequestMapping") != nil
}

// Endpoints lists a class's handler methods in declaration order.
func Endpoints(ix *java.Index, t *java.TypeDecl) []Endpoint {
	var out []Endpoint
	for _, m := range t.Methods {
		if e, ok := endpoint(ix, t, m); ok {
			out = append(out, e)
		}
	}
	return out
}

func endpoint(ix *java.Index, t *java.TypeDecl, m java.Method) (Endpoint, bool) {
	var verb string
	var mapping *java.AnnotationUse
	for simple, v := range shortcuts {
		if a := m.Annotation(simple); a != nil {
			verb, mapping = v, a
			break
		}
	}
	if mapping == nil {
		a := m.Annotation("RequestMapping")
		if a == nil {
			return Endpoint{}, false
		}
		mapping, verb = a, "GET"
		if mv := firstName(a.Arg("method")); mv != "" {
			verb = mv[strings.LastIndexByte(mv, '.')+1:]
		}
	}
	prefix := ""
	if cm := t.Annotation("RequestMapping"); cm != nil {
		prefix = firstPath(ix, cm, t)
	}
	e := Endpoint{Verb: verb, Path: joinPath(prefix, firstPath(ix, mapping, t)), Class: t, Method: m}
	if c, ok := ix.Constant(firstExpr(mapping.Arg("consumes")), t); ok {
		e.Consumes = c
	}
	for i := range m.Params {
		p := m.Params[i]
		typeName := ix.Resolve(p.Type.Name, t).Name
		switch {
		case typeName == "org.springframework.data.domain.Pageable":
			e.Pageable = true
		case p.Annotation("PathVariable") != nil:
			e.PathVars = append(e.PathVars, Arg{nameOf(ix, p, "PathVariable", t), p})
		case p.Annotation("RequestParam") != nil:
			e.Params = append(e.Params, Arg{nameOf(ix, p, "RequestParam", t), p})
		case p.Annotation("RequestHeader") != nil:
			e.Headers = append(e.Headers, Arg{nameOf(ix, p, "RequestHeader", t), p})
		case p.Annotation("RequestBody") != nil:
			e.Body = &m.Params[i]
		case IsFramework(ix, p, t):
		case isSimple(ix, p.Type, t):
			e.Params = append(e.Params, Arg{p.Name, p})
		}
	}
	return e, true
}

// IsResolverClass reports an SPQR resolver: @GraphQLApi, or any method with @GraphQLQuery or @GraphQLMutation.
func IsResolverClass(t *java.TypeDecl) bool {
	if t.Annotation("GraphQLApi") != nil {
		return true
	}
	for _, m := range t.Methods {
		if m.Annotation("GraphQLQuery") != nil || m.Annotation("GraphQLMutation") != nil {
			return true
		}
	}
	return false
}

// Operations lists a class's SPQR operations in declaration order.
func Operations(ix *java.Index, t *java.TypeDecl) []Operation {
	var out []Operation
	for _, m := range t.Methods {
		kind, a := "query", m.Annotation("GraphQLQuery")
		if a == nil {
			kind, a = "mutation", m.Annotation("GraphQLMutation")
		}
		if a == nil {
			continue
		}
		name := m.Name
		if v, ok := ix.Constant(a.Arg("name"), t); ok && v != "" {
			name = v
		}
		op := Operation{Kind: kind, Name: name, Class: t, Method: m}
		for _, p := range m.Params {
			if IsFramework(ix, p, t) || p.Annotation("GraphQLIgnore") != nil {
				continue
			}
			key := p.Name
			if ga := p.Annotation("GraphQLArgument"); ga != nil {
				if v, ok := ix.Constant(ga.Arg("name"), t); ok && v != "" {
					key = v
				}
			}
			op.Args = append(op.Args, Arg{key, p})
		}
		out = append(out, op)
	}
	return out
}

var frameworkPrefixes = []string{
	"jakarta.servlet.", "javax.servlet.", "java.security.Principal", "org.springframework.ui.",
	"org.springframework.validation.", "org.springframework.security.", "org.springframework.web.",
	"org.springframework.http.HttpHeaders", "io.leangen.graphql.execution.",
}

// IsFramework reports request, response, session, principal, model and SPQR environment arguments.
func IsFramework(ix *java.Index, p java.Param, ctx *java.TypeDecl) bool {
	name := ix.Resolve(p.Type.Name, ctx).Name
	for _, prefix := range frameworkPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return p.Annotation("GraphQLEnvironment") != nil || p.Annotation("GraphQLRootContext") != nil
}

var simpleTypes = map[string]bool{
	"java.lang.String": true, "java.lang.Integer": true, "java.lang.Long": true, "java.lang.Short": true,
	"java.lang.Byte": true, "java.lang.Double": true, "java.lang.Float": true, "java.lang.Boolean": true,
	"java.lang.Character": true, "java.math.BigDecimal": true, "java.math.BigInteger": true, "java.util.UUID": true,
	"java.time.LocalDate": true, "java.time.LocalDateTime": true, "java.time.LocalTime": true, "java.time.Instant": true,
	"int": true, "long": true, "short": true, "byte": true, "double": true, "float": true, "boolean": true, "char": true,
}

func isSimple(ix *java.Index, ref java.TypeRef, ctx *java.TypeDecl) bool {
	if ref.Dims > 0 {
		return false
	}
	if simpleTypes[ref.Name] {
		return true
	}
	r := ix.Resolve(ref.Name, ctx)
	return simpleTypes[r.Name] || (r.Decl != nil && r.Decl.Kind == java.Enum)
}

func nameOf(ix *java.Index, p java.Param, annotation string, ctx *java.TypeDecl) string {
	a := p.Annotation(annotation)
	for _, attr := range []string{"value", "name"} {
		if v, ok := ix.Constant(a.Arg(attr), ctx); ok && v != "" {
			return v
		}
	}
	return p.Name
}

// firstPath is the first of value or path, the first entry when it is an array; "" when absent.
func firstPath(ix *java.Index, a *java.AnnotationUse, ctx *java.TypeDecl) string {
	for _, attr := range []string{"value", "path"} {
		if e := firstExpr(a.Arg(attr)); e != nil {
			if v, ok := ix.Constant(e, ctx); ok {
				return v
			}
		}
	}
	return ""
}

func firstExpr(e java.Expr) java.Expr {
	if arr, ok := e.(java.ArrayLit); ok {
		if len(arr.Items) == 0 {
			return nil
		}
		return arr.Items[0]
	}
	return e
}

func firstName(e java.Expr) string {
	if n, ok := firstExpr(e).(java.NameRef); ok {
		return n.Name
	}
	return ""
}

// joinPath joins class and method paths and keeps only the variable name of {id:[0-9]+}.
func joinPath(prefix, path string) string {
	var parts []string
	for _, p := range []string{prefix, path} {
		if t := strings.Trim(p, "/"); t != "" {
			parts = append(parts, t)
		}
	}
	joined := "/" + strings.Join(parts, "/")
	var b strings.Builder
	for i := 0; i < len(joined); i++ {
		if joined[i] == '{' {
			end := strings.IndexByte(joined[i:], '}')
			if end > 0 {
				name := joined[i+1 : i+end]
				if c := strings.IndexByte(name, ':'); c >= 0 {
					name = name[:c]
				}
				b.WriteString("{" + name + "}")
				i += end
				continue
			}
		}
		b.WriteByte(joined[i])
	}
	return b.String()
}
