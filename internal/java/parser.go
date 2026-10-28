package java

import (
	"fmt"
	"strings"
)

// Parse reads the declarations of one Java source file.
func Parse(path, src string) (file *File, err error) {
	toks, err := Lex(path, src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, path: path, file: &File{Path: path}}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(*ParseError)
			if !ok {
				panic(r)
			}
			file, err = nil, pe
		}
	}()
	p.parseFile()
	return p.file, nil
}

type parser struct {
	toks []Token
	pos  int
	path string
	file *File
}

var modifierWords = map[string]bool{
	"public": true, "protected": true, "private": true, "static": true, "final": true, "abstract": true,
	"native": true, "synchronized": true, "transient": true, "volatile": true, "strictfp": true,
	"default": true, "sealed": true,
}

func (p *parser) peek() Token        { return p.toks[p.pos] }
func (p *parser) peekAt(n int) Token { return p.toks[min(p.pos+n, len(p.toks)-1)] }
func (p *parser) next() Token {
	t := p.toks[p.pos]
	if t.Kind != EOF {
		p.pos++
	}
	return t
}
func (p *parser) is(text string) bool {
	t := p.peek()
	return t.Kind != String && t.Kind != Char && t.Text == text
}
func (p *parser) accept(text string) bool {
	if p.is(text) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) fail(format string, a ...any) {
	t := p.peek()
	what := t.Text
	if t.Kind == EOF {
		what = "end of file"
	}
	panic(&ParseError{Path: p.path, Line: t.Line, Col: t.Col, Msg: fmt.Sprintf(format, a...) + fmt.Sprintf(" (found %q)", what)})
}

func (p *parser) expect(text string) Token {
	if !p.is(text) {
		p.fail("expected %q", text)
	}
	return p.next()
}

func (p *parser) ident() string {
	t := p.peek()
	if t.Kind != Ident {
		p.fail("expected a name")
	}
	p.pos++
	return t.Text
}

// qualifiedName reads a.b.c.
func (p *parser) qualifiedName() string {
	parts := []string{p.ident()}
	for p.is(".") && p.peekAt(1).Kind == Ident {
		p.next()
		parts = append(parts, p.ident())
	}
	return strings.Join(parts, ".")
}

func (p *parser) parseFile() {
	for {
		start := p.pos
		annotations, _ := p.modifiers()
		if p.is("package") {
			p.next()
			p.file.Package = p.qualifiedName()
			p.expect(";")
			continue
		}
		if len(annotations) > 0 || p.pos != start {
			p.pos = start
		}
		break
	}
	for p.is("import") {
		p.next()
		imp := Import{}
		if p.accept("static") {
			imp.Static = true
		}
		imp.Name = p.qualifiedName()
		if p.accept(".") {
			p.expect("*")
			imp.Wildcard = true
		}
		p.expect(";")
		p.file.Imports = append(p.file.Imports, imp)
	}
	for p.peek().Kind != EOF {
		if p.accept(";") {
			continue
		}
		annotations, mods := p.modifiers()
		t := p.typeDecl(annotations, mods, nil)
		if t == nil {
			p.fail("expected a class, interface, enum or record")
		}
		p.file.Types = append(p.file.Types, t)
	}
}

// modifiers reads annotations and modifier words in any order.
func (p *parser) modifiers() ([]AnnotationUse, []string) {
	var annotations []AnnotationUse
	var mods []string
	for {
		switch {
		case p.is("@") && !(p.peekAt(1).Text == "interface" && p.peekAt(1).Kind == Ident):
			annotations = append(annotations, p.annotation())
		case p.peek().Kind == Ident && modifierWords[p.peek().Text]:
			mods = append(mods, p.next().Text)
		case p.is("non") && p.peekAt(1).Text == "-" && p.peekAt(2).Text == "sealed":
			p.pos += 3
			mods = append(mods, "non-sealed")
		default:
			return annotations, mods
		}
	}
}

// isRecordStart tells the contextual keyword record from a type named record.
func (p *parser) isRecordStart() bool {
	return p.is("record") && p.peekAt(1).Kind == Ident && (p.peekAt(2).Text == "(" || p.peekAt(2).Text == "<")
}

// typeDecl reads a type declaration at the current position, or returns nil when none starts here.
func (p *parser) typeDecl(annotations []AnnotationUse, mods []string, outer *TypeDecl) *TypeDecl {
	line := p.peek().Line
	var kind DeclKind
	switch {
	case p.is("class"):
		kind = Class
	case p.is("interface"):
		kind = Interface
	case p.is("enum"):
		kind = Enum
	case p.isRecordStart():
		kind = Record
	case p.is("@") && p.peekAt(1).Text == "interface":
		p.next()
		kind = Annotation
	default:
		return nil
	}
	p.next()
	t := &TypeDecl{Kind: kind, Name: p.ident(), Modifiers: mods, Annotations: annotations, Outer: outer, File: p.file, Line: line}
	if p.is("<") {
		t.TypeParams = p.typeParams()
	}
	if kind == Record {
		t.Components = p.params()
	}
	for {
		switch {
		case p.accept("extends"):
			t.Extends = append(t.Extends, p.typeList()...)
		case p.accept("implements"):
			t.Implements = append(t.Implements, p.typeList()...)
		case p.accept("permits"):
			p.typeList()
		default:
			p.body(t)
			return t
		}
	}
}

func (p *parser) typeList() []TypeRef {
	list := []TypeRef{p.typeRef()}
	for p.accept(",") {
		list = append(list, p.typeRef())
	}
	return list
}

// typeParams reads <T, U extends Foo<U>> and returns the names.
func (p *parser) typeParams() []string {
	p.expect("<")
	var names []string
	for {
		p.modifiers()
		names = append(names, p.ident())
		if p.accept("extends") {
			p.typeRef()
			for p.accept("&") {
				p.typeRef()
			}
		}
		if !p.accept(",") {
			break
		}
	}
	p.expect(">")
	return names
}

func (p *parser) body(t *TypeDecl) {
	p.expect("{")
	if t.Kind == Enum {
		p.enumConstants(t)
	}
	for !p.accept("}") {
		if p.peek().Kind == EOF {
			p.fail("missing } for %s", t.Name)
		}
		p.member(t)
	}
}

func (p *parser) enumConstants(t *TypeDecl) {
	for {
		p.modifiers()
		if p.is(";") || p.is("}") {
			break
		}
		t.Constants = append(t.Constants, p.ident())
		if p.is("(") {
			p.skipBalanced("(", ")")
		}
		if p.is("{") {
			p.skipBalanced("{", "}")
		}
		if !p.accept(",") {
			break
		}
	}
	p.accept(";")
}

func (p *parser) member(t *TypeDecl) {
	if p.accept(";") {
		return
	}
	line := p.peek().Line
	annotations, mods := p.modifiers()
	if p.is("{") { // initializer block
		p.skipBalanced("{", "}")
		return
	}
	if nested := p.typeDecl(annotations, mods, t); nested != nil {
		t.Nested = append(t.Nested, nested)
		return
	}
	var typeParams []string
	if p.is("<") {
		typeParams = p.typeParams()
	}
	// Constructor, or a record's compact constructor.
	if p.is(t.Name) && (p.peekAt(1).Text == "(" || p.peekAt(1).Text == "{") {
		p.next()
		m := Method{Name: t.Name, Modifiers: mods, Annotations: annotations, TypeParams: typeParams, Line: line}
		if p.is("(") {
			m.Params = p.params()
		}
		p.methodRest()
		t.Methods = append(t.Methods, m)
		return
	}
	typ := p.typeRef()
	name := p.ident()
	if p.is("(") {
		m := Method{Name: name, Return: &typ, Params: p.params(), Modifiers: mods, Annotations: annotations, TypeParams: typeParams, Line: line}
		for p.is("[") {
			p.next()
			p.expect("]")
			m.Return.Dims++
		}
		p.methodRest()
		t.Methods = append(t.Methods, m)
		return
	}
	for {
		f := Field{Name: name, Type: typ, Modifiers: mods, Annotations: annotations, Line: line}
		for p.is("[") {
			p.next()
			p.expect("]")
			f.Type.Dims++
		}
		if p.accept("=") {
			f.Init = p.initializer()
		}
		t.Fields = append(t.Fields, f)
		if !p.accept(",") {
			break
		}
		name = p.ident()
	}
	p.expect(";")
}

// methodRest skips throws, an annotation element default and the body.
func (p *parser) methodRest() {
	if p.accept("throws") {
		p.typeList()
	}
	if p.accept("default") {
		p.skipUntil(";")
	}
	if p.is("{") {
		p.skipBalanced("{", "}")
		return
	}
	p.expect(";")
}

// initializer keeps a simple constant (literals, names, +) and skips anything else up to , or ;.
func (p *parser) initializer() Expr {
	start := p.pos
	if e, ok := p.tryExpr(); ok && (p.is(",") || p.is(";")) {
		return e
	}
	p.pos = start
	p.skipUntil(",", ";")
	return nil
}

func (p *parser) tryExpr() (e Expr, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isParse := r.(*ParseError); !isParse {
				panic(r)
			}
			e, ok = nil, false
		}
	}()
	return p.elementValue(), true
}

// skipUntil moves to the first of the given tokens at nesting depth 0, without consuming it.
func (p *parser) skipUntil(stops ...string) {
	depth := 0
	for p.peek().Kind != EOF {
		t := p.peek()
		if depth == 0 && t.Kind == Punct {
			for _, s := range stops {
				if t.Text == s {
					return
				}
			}
		}
		if t.Kind == Punct {
			switch t.Text {
			case "(", "{", "[":
				depth++
			case ")", "}", "]":
				depth--
			}
		}
		p.next()
	}
}

// skipBalanced consumes from an opening token to its matching close.
func (p *parser) skipBalanced(open, close string) {
	p.expect(open)
	depth := 1
	for depth > 0 {
		t := p.next()
		if t.Kind == EOF {
			p.fail("missing %q", close)
		}
		if t.Kind != Punct {
			continue
		}
		switch t.Text {
		case open:
			depth++
		case close:
			depth--
		}
	}
}

func (p *parser) params() []Param {
	p.expect("(")
	var params []Param
	for !p.accept(")") {
		annotations, mods := p.modifiers()
		param := Param{Annotations: annotations, Modifiers: mods, Type: p.typeRef()}
		p.modifiers() // annotations on varargs: String @NonNull ... names
		if p.accept("...") {
			param.Varargs = true
			param.Type.Dims++
		}
		if p.is("this") {
			p.next() // receiver parameter
			param.Name = "this"
		} else {
			param.Name = p.ident()
		}
		for p.is("[") {
			p.next()
			p.expect("]")
			param.Type.Dims++
		}
		if param.Name != "this" {
			params = append(params, param)
		}
		if !p.accept(",") {
			p.expect(")")
			break
		}
	}
	return params
}

// typeRef reads a type: annotations, a qualified name with type arguments, array dimensions.
func (p *parser) typeRef() TypeRef {
	p.modifiers() // type annotations
	if p.accept("?") {
		r := TypeRef{Name: "?"}
		if p.is("extends") || p.is("super") {
			r.Wildcard = p.next().Text
			b := p.typeRef()
			r.Bound = &b
		}
		return r
	}
	r := TypeRef{Name: p.ident()}
	if p.is("<") {
		r.Args = p.typeArgs()
	}
	for p.is(".") && ((p.peekAt(1).Kind == Ident && p.peekAt(1).Text != "class") || p.peekAt(1).Text == "@") {
		p.next()
		p.modifiers()
		r.Name += "." + p.ident()
		if p.is("<") {
			r.Args = p.typeArgs() // Outer<T>.Inner<U>: the inner arguments are the ones that matter
		}
	}
	for p.is("[") && p.peekAt(1).Text == "]" {
		p.pos += 2
		r.Dims++
		p.modifiers()
	}
	return r
}

func (p *parser) typeArgs() []TypeRef {
	p.expect("<")
	var args []TypeRef
	for !p.accept(">") {
		args = append(args, p.typeRef())
		if !p.accept(",") {
			p.expect(">")
			break
		}
	}
	return args
}

// annotation reads @Name or @Name(...) with its element values.
func (p *parser) annotation() AnnotationUse {
	line := p.expect("@").Line
	a := AnnotationUse{Name: p.qualifiedName(), Line: line}
	if !p.accept("(") {
		return a
	}
	a.Args = map[string]Expr{}
	if p.accept(")") {
		return a
	}
	if p.peek().Kind == Ident && p.peekAt(1).Text == "=" {
		for {
			name := p.ident()
			p.expect("=")
			a.Args[name] = p.elementValue()
			if !p.accept(",") {
				break
			}
		}
	} else {
		a.Args["value"] = p.elementValue()
	}
	p.expect(")")
	return a
}

// elementValue reads an annotation value: literals, names, class literals, arrays, nested annotations, +.
func (p *parser) elementValue() Expr {
	if p.is("@") {
		return Nested{Annotation: p.annotation()}
	}
	if p.accept("{") {
		var items []Expr
		for !p.accept("}") {
			items = append(items, p.elementValue())
			if !p.accept(",") {
				p.expect("}")
				break
			}
		}
		return ArrayLit{Items: items}
	}
	parts := []Expr{p.term()}
	for p.accept("+") {
		parts = append(parts, p.term())
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return Concat{Parts: parts}
}

func (p *parser) term() Expr {
	t := p.peek()
	switch {
	case t.Kind == String || t.Kind == Char:
		p.next()
		return StrLit{Value: t.Text}
	case t.Kind == Number:
		p.next()
		return NumLit{Text: t.Text}
	case p.accept("-"):
		return Neg{Value: p.term()}
	case p.accept("("):
		e := p.elementValue()
		p.expect(")")
		return e
	case t.Kind == Ident && (t.Text == "true" || t.Text == "false"):
		p.next()
		return BoolLit{Value: t.Text == "true"}
	case t.Kind == Ident:
		start := p.pos
		r := p.typeRef()
		if p.accept(".") {
			if p.accept("class") {
				return ClassLit{Type: r}
			}
			p.fail("unexpected . after %s", r.Name)
		}
		if len(r.Args) == 0 && r.Dims == 0 {
			return NameRef{Name: r.Name}
		}
		p.pos = start
		p.fail("expected a constant")
	}
	p.fail("expected a value")
	return nil
}
