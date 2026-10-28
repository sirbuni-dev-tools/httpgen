package java

import "strings"

// File is one parsed .java file.
type File struct {
	Path    string
	Package string
	Imports []Import
	Types   []*TypeDecl
}

// Import is an import declaration.
type Import struct {
	Name     string // com.acme.Foo, or com.acme for a wildcard
	Static   bool
	Wildcard bool
}

// DeclKind is the kind of a type declaration.
type DeclKind string

const (
	Class      DeclKind = "class"
	Interface  DeclKind = "interface"
	Enum       DeclKind = "enum"
	Record     DeclKind = "record"
	Annotation DeclKind = "@interface"
)

// TypeDecl is a class, interface, enum, record or annotation type.
type TypeDecl struct {
	Kind        DeclKind
	Name        string
	Modifiers   []string
	Annotations []AnnotationUse
	TypeParams  []string
	Extends     []TypeRef // superclass, or super-interfaces of an interface
	Implements  []TypeRef
	Fields      []Field
	Methods     []Method
	Components  []Param  // record components
	Constants   []string // enum constants
	Nested      []*TypeDecl
	Outer       *TypeDecl
	File        *File
	Line        int
}

// QualifiedName is package.Outer.Inner.
func (t *TypeDecl) QualifiedName() string {
	parts := []string{t.Name}
	for o := t.Outer; o != nil; o = o.Outer {
		parts = append([]string{o.Name}, parts...)
	}
	name := strings.Join(parts, ".")
	if t.File != nil && t.File.Package != "" {
		return t.File.Package + "." + name
	}
	return name
}

// Annotation returns the first annotation whose simple name matches (`GetMapping` matches `@org...GetMapping`).
func (t *TypeDecl) Annotation(simple string) *AnnotationUse {
	return findAnnotation(t.Annotations, simple)
}

// HasModifier reports a modifier such as public or static.
func (t *TypeDecl) HasModifier(m string) bool { return contains(t.Modifiers, m) }

// TypeRef is a type as written: name, type arguments, array dimensions.
type TypeRef struct {
	Name     string    // as written: List, java.util.List, ?, int
	Args     []TypeRef // type arguments
	Dims     int       // array dimensions (varargs count as one)
	Wildcard string    // "", "extends" or "super" for ? extends X / ? super X; Bound holds X
	Bound    *TypeRef
}

// String renders the type the way it was written, without annotations.
func (r TypeRef) String() string {
	var b strings.Builder
	if r.Name == "?" {
		b.WriteString("?")
		if r.Bound != nil {
			b.WriteString(" " + r.Wildcard + " " + r.Bound.String())
		}
	} else {
		b.WriteString(r.Name)
	}
	if len(r.Args) > 0 {
		b.WriteString("<")
		for i, a := range r.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.String())
		}
		b.WriteString(">")
	}
	b.WriteString(strings.Repeat("[]", r.Dims))
	return b.String()
}

// SimpleName is the last segment of the name.
func (r TypeRef) SimpleName() string {
	if i := strings.LastIndexByte(r.Name, '.'); i >= 0 {
		return r.Name[i+1:]
	}
	return r.Name
}

// Field is a field declaration (one per declared name).
type Field struct {
	Name        string
	Type        TypeRef
	Modifiers   []string
	Annotations []AnnotationUse
	Init        Expr // only for simple constant initializers; nil otherwise
	Line        int
}

// HasModifier reports a modifier.
func (f Field) HasModifier(m string) bool { return contains(f.Modifiers, m) }

// Annotation returns the first annotation with this simple name.
func (f Field) Annotation(simple string) *AnnotationUse { return findAnnotation(f.Annotations, simple) }

// Method is a method or constructor.
type Method struct {
	Name        string
	Return      *TypeRef // nil for constructors
	Params      []Param
	Modifiers   []string
	Annotations []AnnotationUse
	TypeParams  []string
	Line        int
}

// HasModifier reports a modifier.
func (m Method) HasModifier(mod string) bool { return contains(m.Modifiers, mod) }

// Annotation returns the first annotation with this simple name.
func (m Method) Annotation(simple string) *AnnotationUse {
	return findAnnotation(m.Annotations, simple)
}

// Param is a method parameter or record component.
type Param struct {
	Name        string
	Type        TypeRef
	Annotations []AnnotationUse
	Modifiers   []string
	Varargs     bool
}

// Annotation returns the first annotation with this simple name.
func (p Param) Annotation(simple string) *AnnotationUse { return findAnnotation(p.Annotations, simple) }

// AnnotationUse is an annotation with its element values.
type AnnotationUse struct {
	Name string // as written: GetMapping or org.springframework...GetMapping
	Args map[string]Expr
	Line int
}

// SimpleName is the annotation name without its package.
func (a AnnotationUse) SimpleName() string {
	if i := strings.LastIndexByte(a.Name, '.'); i >= 0 {
		return a.Name[i+1:]
	}
	return a.Name
}

// Arg returns an element value: "value" also answers for a single unnamed argument.
func (a AnnotationUse) Arg(name string) Expr {
	if a.Args == nil {
		return nil
	}
	return a.Args[name]
}

// Expr is an annotation element value or a constant initializer.
type Expr interface{ expr() }

// StrLit is a string literal.
type StrLit struct{ Value string }

// NumLit is a number literal as written (suffixes kept).
type NumLit struct{ Text string }

// BoolLit is true or false.
type BoolLit struct{ Value bool }

// NameRef is an identifier or qualified name: a constant, an enum constant or a class.
type NameRef struct{ Name string }

// ClassLit is Foo.class.
type ClassLit struct{ Type TypeRef }

// ArrayLit is { a, b }.
type ArrayLit struct{ Items []Expr }

// Concat is a + b (strings or numbers).
type Concat struct{ Parts []Expr }

// Nested is an annotation used as a value.
type Nested struct{ Annotation AnnotationUse }

// Neg is a negated number, -1.
type Neg struct{ Value Expr }

func (StrLit) expr()   {}
func (NumLit) expr()   {}
func (BoolLit) expr()  {}
func (NameRef) expr()  {}
func (ClassLit) expr() {}
func (ArrayLit) expr() {}
func (Concat) expr()   {}
func (Nested) expr()   {}
func (Neg) expr()      {}

func findAnnotation(list []AnnotationUse, simple string) *AnnotationUse {
	for i := range list {
		if list[i].SimpleName() == simple {
			return &list[i]
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
