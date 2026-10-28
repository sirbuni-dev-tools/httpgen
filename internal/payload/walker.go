package payload

import (
	"strconv"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/java"
)

// Walker reduces Java types to the JSON Jackson (or SPQR) would read or write for them.
//
// It follows fields (superclass first) or record components, skips static, transient,
// @Transient, @JsonIgnore and @GraphQLIgnore members, renames by @JsonProperty, reads validation
// constraints, binds type parameters through generics and superclasses, stops at MaxDepth and
// turns a class already on the path into {}.
type Walker struct {
	Index *java.Index
	Hints *Hints
	// MaxDepth limits nested objects; 3 by default.
	MaxDepth int
	// SkipGenerated is the request mode: @GeneratedValue fields are left out and a payload's own
	// id, uid or uuid is null, since the server generates it or uses it to find the record.
	SkipGenerated bool
	// IncludeGetters is the GraphQL output mode: only what SPQR exposes (public fields, Lombok
	// getters, explicit getters) plus computed getters such as getCode().
	IncludeGetters bool
}

// binding is what a type parameter stands for, with the scope it was written in.
type binding struct {
	ref   java.TypeRef
	ctx   *java.TypeDecl
	subst map[string]binding
}

type member struct {
	name        string
	owner       string
	annotations []java.AnnotationUse
	inObject    bool
}

func (w *Walker) maxDepth() int {
	if w.MaxDepth <= 0 {
		return 3
	}
	return w.MaxDepth
}

// Decl walks a class itself.
func (w *Walker) Decl(t *java.TypeDecl) Node {
	return w.object(t, nil, map[string]bool{t.QualifiedName(): true}, 1)
}

// Type walks a type as written inside ctx (a field, parameter or return type).
func (w *Walker) Type(ref java.TypeRef, ctx *java.TypeDecl, typeParams []string) Node {
	return w.node(ref, ctx, unbound(typeParams), member{}, map[string]bool{}, 0)
}

// Param is a named value such as a method parameter.
type Param struct {
	Key         string
	Name        string
	Type        java.TypeRef
	Annotations []java.AnnotationUse
}

// Params walks parameters as one object keyed by Key; values follow the same rules as fields.
func (w *Walker) Params(params []Param, ctx *java.TypeDecl, typeParams []string) Object {
	var o Object
	owner := ""
	if ctx != nil {
		owner = ctx.Name
	}
	for _, p := range params {
		m := member{name: p.Name, owner: owner, annotations: p.Annotations}
		o.Fields = append(o.Fields, Field{Key: p.Key, Value: w.node(p.Type, ctx, unbound(typeParams), m, map[string]bool{}, 0)})
	}
	return o
}

// unbound marks method type parameters as known names with nothing bound: they render as null.
func unbound(names []string) map[string]binding {
	if len(names) == 0 {
		return nil
	}
	m := map[string]binding{}
	for _, n := range names {
		m[n] = binding{}
	}
	return m
}

var primitiveKinds = map[string]Kind{
	"int": KInteger, "short": KInteger, "long": KLong, "byte": KByte, "double": KDecimal, "float": KDecimal,
	"boolean": KBoolean, "char": KChar,
}

var simpleKinds = map[string]Kind{
	"java.lang.String": KString, "java.lang.CharSequence": KString, "java.lang.Character": KChar,
	"java.lang.Integer": KInteger, "java.lang.Short": KInteger, "java.lang.Long": KLong, "java.lang.Byte": KByte,
	"java.lang.Double": KDecimal, "java.lang.Float": KDecimal, "java.math.BigDecimal": KDecimal,
	"java.math.BigInteger": KBigInteger, "java.lang.Boolean": KBoolean, "java.util.UUID": KUUID,
	"java.time.LocalDate": KDate, "java.sql.Date": KDate, "java.time.LocalDateTime": KDateTime,
	"java.time.LocalTime": KTime, "java.time.Instant": KInstant, "java.time.OffsetDateTime": KInstant,
	"java.time.ZonedDateTime": KInstant, "java.util.Date": KInstant, "java.sql.Timestamp": KInstant,
}

var collectionTypes = map[string]bool{
	"java.util.List": true, "java.util.ArrayList": true, "java.util.LinkedList": true, "java.util.Set": true,
	"java.util.HashSet": true, "java.util.LinkedHashSet": true, "java.util.TreeSet": true, "java.util.SortedSet": true,
	"java.util.Collection": true, "java.util.Queue": true, "java.util.Deque": true,
}

var mapTypes = map[string]bool{
	"java.util.Map": true, "java.util.HashMap": true, "java.util.LinkedHashMap": true, "java.util.TreeMap": true,
	"java.util.SortedMap": true,
}

func (w *Walker) node(ref java.TypeRef, ctx *java.TypeDecl, subst map[string]binding, m member, path map[string]bool, depth int) Node {
	if ref.Dims > 0 {
		if ref.Name == "byte" && ref.Dims == 1 {
			return w.leaf(KBytes, m, nil)
		}
		if w.isCriteriaList(m.name) {
			return EmptyArray{}
		}
		elem := ref
		elem.Dims--
		return Array{Elem: w.node(elem, ctx, subst, m, path, depth)}
	}
	if k, ok := primitiveKinds[ref.Name]; ok {
		return w.leaf(k, m, nil)
	}
	if ref.Name == "void" {
		return Null{}
	}
	if ref.Name == "?" {
		if ref.Bound != nil && ref.Wildcard == "extends" {
			return w.node(*ref.Bound, ctx, subst, m, path, depth)
		}
		return Null{}
	}
	if b, ok := subst[ref.Name]; ok && !strings.Contains(ref.Name, ".") {
		if b.ctx == nil && b.ref.Name == "" {
			return Null{}
		}
		return w.node(b.ref, b.ctx, b.subst, m, path, depth)
	}

	r := w.Index.Resolve(ref.Name, ctx)
	if k, ok := simpleKinds[r.Name]; ok {
		return w.leaf(k, m, nil)
	}
	if consts, ok := java.LibraryEnums[r.Name]; ok {
		return w.leaf(KEnum, m, consts)
	}
	arg := func(i int) (java.TypeRef, bool) {
		if i < len(ref.Args) {
			return ref.Args[i], true
		}
		return java.TypeRef{}, false
	}
	switch {
	case r.Name == "java.util.Optional" || r.Name == "org.springframework.http.ResponseEntity":
		if a, ok := arg(0); ok {
			return w.node(a, ctx, subst, m, path, depth)
		}
		return Null{}
	case mapTypes[r.Name]:
		if a, ok := arg(1); ok {
			return Map{Value: w.node(a, ctx, subst, m, path, depth)}
		}
		return Map{Value: Null{}}
	case collectionTypes[r.Name] || r.Name == "java.lang.Iterable":
		if w.isCriteriaList(m.name) {
			return EmptyArray{}
		}
		if a, ok := arg(0); ok {
			return Array{Elem: w.node(a, ctx, subst, m, path, depth)}
		}
		return Array{Elem: Null{}}
	case r.Name == "org.springframework.data.domain.Page" || r.Name == "org.springframework.data.domain.Slice" ||
		r.Name == "org.springframework.data.domain.PageImpl":
		return w.page(ref, ctx, subst, m, path, depth)
	}
	if r.Decl == nil {
		return Null{}
	}
	if r.Decl.Kind == java.Enum {
		return w.leaf(KEnum, m, r.Decl.Constants)
	}
	if r.Decl.Kind == java.Annotation {
		return Null{}
	}
	name := r.Decl.QualifiedName()
	if path[name] || depth >= w.maxDepth() {
		return Cut{}
	}
	// Bind the class's type parameters to the arguments written here.
	var bound map[string]binding
	if len(r.Decl.TypeParams) > 0 {
		bound = map[string]binding{}
		for i, p := range r.Decl.TypeParams {
			if i < len(ref.Args) {
				bound[p] = binding{ref: ref.Args[i], ctx: ctx, subst: subst}
			} else {
				bound[p] = binding{}
			}
		}
	}
	next := map[string]bool{}
	for k := range path {
		next[k] = true
	}
	next[name] = true
	return w.object(r.Decl, bound, next, depth+1)
}

// page is Spring Data's Page or Slice: an object with content and paging getters in GraphQL output, {} as input.
func (w *Walker) page(ref java.TypeRef, ctx *java.TypeDecl, subst map[string]binding, m member, path map[string]bool, depth int) Node {
	if !w.IncludeGetters {
		return Object{}
	}
	content := Node(Array{Elem: Null{}})
	if len(ref.Args) > 0 {
		content = Array{Elem: w.node(ref.Args[0], ctx, subst, member{name: "content"}, path, depth+1)}
	}
	whole := func(name string) Field { return Field{Key: name, Value: w.leaf(KLong, member{name: "count"}, nil)} }
	flag := func(name string) Field { return Field{Key: name, Value: w.leaf(KBoolean, member{name: name}, nil)} }
	return Object{Fields: []Field{
		{Key: "content", Value: content}, whole("totalElements"), whole("totalPages"), whole("number"), whole("size"),
		whole("numberOfElements"), flag("first"), flag("last"), flag("empty"),
	}}
}

type level struct {
	decl  *java.TypeDecl
	subst map[string]binding
}

// hierarchy returns the class and its project superclasses, top-most first, each with its type bindings.
func (w *Walker) hierarchy(t *java.TypeDecl, subst map[string]binding) []level {
	var chain []level
	seen := map[*java.TypeDecl]bool{}
	for cur, cs := t, subst; cur != nil && !seen[cur]; {
		seen[cur] = true
		chain = append([]level{{cur, cs}}, chain...)
		if cur.Kind != java.Class || len(cur.Extends) == 0 {
			break
		}
		sup := w.Index.Superclass(cur)
		if sup == nil {
			break
		}
		var next map[string]binding
		if len(sup.TypeParams) > 0 {
			next = map[string]binding{}
			args := cur.Extends[0].Args
			for i, p := range sup.TypeParams {
				if i < len(args) {
					next[p] = binding{ref: args[i], ctx: cur, subst: cs}
				} else {
					next[p] = binding{}
				}
			}
		}
		cur, cs = sup, next
	}
	return chain
}

func (w *Walker) object(t *java.TypeDecl, subst map[string]binding, path map[string]bool, depth int) Node {
	ignored := map[string]bool{}
	for _, lv := range w.hierarchy(t, subst) {
		if a := lv.decl.Annotation("JsonIgnoreProperties"); a != nil {
			for _, s := range w.strings(a.Arg("value"), lv.decl) {
				ignored[s] = true
			}
		}
	}
	var fields []Field
	has := func(key string) bool {
		for _, f := range fields {
			if f.Key == key {
				return true
			}
		}
		return false
	}

	if t.Kind == java.Record {
		for _, c := range t.Components {
			if w.skipped(c.Annotations) {
				continue
			}
			key := w.jsonName(c.Annotations, t, c.Name)
			if ignored[key] || has(key) {
				continue
			}
			m := member{name: c.Name, owner: t.Name, annotations: c.Annotations, inObject: true}
			fields = append(fields, Field{Key: key, Value: w.node(c.Type, t, subst, m, path, depth)})
		}
		return Object{Fields: fields}
	}

	for _, lv := range w.hierarchy(t, subst) {
		for _, f := range lv.decl.Fields {
			if f.HasModifier("static") || f.HasModifier("transient") || w.skipped(f.Annotations) {
				continue
			}
			if w.IncludeGetters && !w.exposed(f, lv.decl, t) {
				continue
			}
			key := w.jsonName(f.Annotations, lv.decl, f.Name)
			if ignored[key] || has(key) {
				continue
			}
			m := member{name: f.Name, owner: t.Name, annotations: f.Annotations, inObject: true}
			fields = append(fields, Field{Key: key, Value: w.node(f.Type, lv.decl, lv.subst, m, path, depth)})
		}
	}
	if w.IncludeGetters {
		for _, lv := range w.allSupertypes(t, subst) {
			for _, mt := range lv.decl.Methods {
				key := propertyName(mt, lv.decl)
				if key == "" || has(key) || ignored[key] || w.skipped(mt.Annotations) {
					continue
				}
				m := member{name: key, owner: t.Name, annotations: mt.Annotations, inObject: true}
				fields = append(fields, Field{Key: key, Value: w.node(*mt.Return, lv.decl, withMethodParams(lv.subst, mt.TypeParams), m, path, depth)})
			}
		}
	}
	return Object{Fields: fields}
}

// allSupertypes is the class, its superclasses and every project interface above them, each with bindings.
func (w *Walker) allSupertypes(t *java.TypeDecl, subst map[string]binding) []level {
	var out []level
	seen := map[*java.TypeDecl]bool{}
	var visit func(d *java.TypeDecl, s map[string]binding)
	visit = func(d *java.TypeDecl, s map[string]binding) {
		if d == nil || seen[d] {
			return
		}
		seen[d] = true
		out = append(out, level{d, s})
		refs := append(append([]java.TypeRef{}, d.Extends...), d.Implements...)
		for _, ref := range refs {
			r := w.Index.Resolve(ref.Name, outerOrFile(d))
			if r.Decl == nil {
				continue
			}
			var next map[string]binding
			if len(r.Decl.TypeParams) > 0 {
				next = map[string]binding{}
				for i, p := range r.Decl.TypeParams {
					if i < len(ref.Args) {
						next[p] = binding{ref: ref.Args[i], ctx: d, subst: s}
					} else {
						next[p] = binding{}
					}
				}
			}
			visit(r.Decl, next)
		}
	}
	visit(t, subst)
	return out
}

// outerOrFile is the scope a type's own extends and implements clauses are written in.
func outerOrFile(d *java.TypeDecl) *java.TypeDecl {
	if d.Outer != nil {
		return d.Outer
	}
	return &java.TypeDecl{File: d.File}
}

func withMethodParams(s map[string]binding, params []string) map[string]binding {
	if len(params) == 0 {
		return s
	}
	out := map[string]binding{}
	for k, v := range s {
		out[k] = v
	}
	for _, p := range params {
		out[p] = binding{}
	}
	return out
}

// propertyName turns getCode() into code and isActive() on a boolean into active; "" for anything else.
func propertyName(m java.Method, d *java.TypeDecl) string {
	if m.Return == nil || len(m.Params) > 0 || m.HasModifier("static") || m.Return.Name == "void" {
		return ""
	}
	if d.Kind != java.Interface && !m.HasModifier("public") {
		return ""
	}
	var raw string
	switch {
	case strings.HasPrefix(m.Name, "get") && len(m.Name) > 3 && m.Name != "getClass":
		raw = m.Name[3:]
	case strings.HasPrefix(m.Name, "is") && len(m.Name) > 2 && (m.Return.Name == "boolean" || m.Return.SimpleName() == "Boolean"):
		raw = m.Name[2:]
	default:
		return ""
	}
	return strings.ToLower(raw[:1]) + raw[1:]
}

// exposed reports whether SPQR shows a field: public, Lombok getter on the field or class, or an explicit getter.
func (w *Walker) exposed(f java.Field, declaring, cls *java.TypeDecl) bool {
	if f.HasModifier("public") {
		return true
	}
	for _, a := range []string{"Getter", "Data", "Value"} {
		if f.Annotation(a) != nil || declaring.Annotation(a) != nil {
			return true
		}
	}
	suffix := strings.ToUpper(f.Name[:1]) + f.Name[1:]
	for _, lv := range w.allSupertypes(cls, nil) {
		for _, m := range lv.decl.Methods {
			if (m.Name == "get"+suffix || m.Name == "is"+suffix) && len(m.Params) == 0 {
				return true
			}
		}
	}
	return false
}

func (w *Walker) isCriteriaList(name string) bool {
	n := strings.ToLower(name)
	r := w.Hints.Rules()
	for _, c := range r.CriteriaLists.Contains {
		if strings.Contains(n, c) {
			return true
		}
	}
	for _, e := range r.CriteriaLists.Exact {
		if n == e {
			return true
		}
	}
	return false
}

func (w *Walker) leaf(kind Kind, m member, constants []string) Node {
	if w.SkipGenerated && m.inObject {
		for _, id := range w.Hints.Rules().OwnIDs {
			if strings.ToLower(m.name) == id {
				return Null{}
			}
		}
	}
	ctx := FieldContext{Name: m.name, Kind: kind, Owner: m.owner, EnumConstants: constants, Constraints: w.constraints(m.annotations)}
	return Value{Gen: w.Hints.Resolve(ctx)}
}

func (w *Walker) skipped(annotations []java.AnnotationUse) bool {
	for _, a := range annotations {
		switch a.SimpleName() {
		case "Transient", "GraphQLIgnore":
			return true
		case "JsonIgnore":
			if v, ok := a.Arg("value").(java.BoolLit); !ok || v.Value {
				return true
			}
		case "GeneratedValue":
			if w.SkipGenerated {
				return true
			}
		}
	}
	return false
}

func (w *Walker) jsonName(annotations []java.AnnotationUse, ctx *java.TypeDecl, name string) string {
	for _, a := range annotations {
		if a.SimpleName() == "JsonProperty" {
			if v, ok := w.Index.Constant(a.Arg("value"), ctx); ok && v != "" {
				return v
			}
		}
	}
	return name
}

func (w *Walker) strings(e java.Expr, ctx *java.TypeDecl) []string {
	if arr, ok := e.(java.ArrayLit); ok {
		var out []string
		for _, item := range arr.Items {
			if s, ok := w.Index.Constant(item, ctx); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := w.Index.Constant(e, ctx); ok {
		return []string{s}
	}
	return nil
}

// constraints reads jakarta and javax validation annotations by simple name.
func (w *Walker) constraints(annotations []java.AnnotationUse) Constraints {
	var c Constraints
	num := func(a *java.AnnotationUse, attr string) *float64 {
		if a == nil {
			return nil
		}
		s, ok := w.Index.Constant(a.Arg(attr), nil)
		if !ok {
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return &f
	}
	find := func(name string) *java.AnnotationUse {
		for i := range annotations {
			if annotations[i].SimpleName() == name {
				return &annotations[i]
			}
		}
		return nil
	}
	c.Email = find("Email") != nil
	if size := find("Size"); size != nil {
		if v := num(size, "min"); v != nil && *v > 0 {
			n := int(*v)
			c.MinSize = &n
		}
		if v := num(size, "max"); v != nil && *v < 2147483647 {
			n := int(*v)
			c.MaxSize = &n
		}
	}
	one, zero, minusOne := 1.0, 0.0, -1.0
	switch {
	case find("Min") != nil:
		c.Min = num(find("Min"), "value")
	case find("DecimalMin") != nil:
		c.Min = num(find("DecimalMin"), "value")
	case find("Positive") != nil:
		c.Min = &one
	case find("PositiveOrZero") != nil:
		c.Min = &zero
	}
	switch {
	case find("Max") != nil:
		c.Max = num(find("Max"), "value")
	case find("DecimalMax") != nil:
		c.Max = num(find("DecimalMax"), "value")
	case find("Negative") != nil:
		c.Max = &minusOne
	case find("NegativeOrZero") != nil:
		c.Max = &zero
	}
	if p := find("Pattern"); p != nil {
		if s, ok := w.Index.Constant(p.Arg("regexp"), nil); ok {
			c.Pattern = s
		}
	}
	switch {
	case find("Past") != nil || find("PastOrPresent") != nil:
		c.When = Past
	case find("Future") != nil || find("FutureOrPresent") != nil:
		c.When = Future
	}
	return c
}
