package java

import (
	"errors"
	"strings"
	"testing"
)

const controller = `
// A line comment with "quotes" and { braces }
package com.acme.web;

import java.util.*;
import java.util.UUID;
import static com.acme.web.Paths.BASE;
import org.springframework.web.bind.annotation.*;

/** Javadoc with @RequestMapping("not real") inside. */
@RestController
@RequestMapping(value = {BASE + "/employees", "/v2/employees"}, produces = "application/json")
public class EmployeeController extends BaseController<Employee> implements Auditable, java.io.Serializable {
    private static final String TAG = "emp" + "loyee";
    private static final long serialVersionUID = 1L;
    private final EmployeeService service;
    private Map<String, List<Employee>> cache = new HashMap<>(), other = Map.of("a", List.of());
    int[] scores, matrix[];

    static { System.out.println("init { not a member }"); }

    public EmployeeController(EmployeeService service) { this.service = service; }

    @GetMapping("/{id}")
    public ResponseEntity<Employee> find(@PathVariable("id") Long id, @RequestHeader(name = "X-Tenant", required = false) String tenant) {
        if (id < 0) { return null; }
        String s = """
            text block with } and "quotes"
            """;
        return ResponseEntity.ok(service.find(id));
    }

    @PostMapping(path = "/search", consumes = MediaType.APPLICATION_JSON_VALUE)
    public <T extends Comparable<T>> List<? extends Employee> search(@RequestBody final SearchRequest<T> request, String... tags) throws java.io.IOException, IllegalStateException {
        Runnable r = () -> { int x = 1; };
        return List.of();
    }

    @Deprecated
    protected abstract void hook(@SuppressWarnings({"a", "b"}) char c, String legacy[]);

    public enum Status { ACTIVE("a") { @Override public String toString() { return "x"; } }, ON_LEAVE, ; Status() {} Status(String s) {} }

    public record Page<T>(List<T> items, @JsonProperty("total") long count) implements Serializable {
        public Page { if (count < 0) throw new IllegalArgumentException(); }
    }

    @interface Marker { String value() default ""; int[] codes() default {1, 2}; }

    sealed interface Shape permits Circle {}
    non-sealed class Circle implements Shape {}
}
`

func TestParseDeclarations(t *testing.T) {
	f, err := Parse("EmployeeController.java", controller)
	if err != nil {
		t.Fatal(err)
	}
	if f.Package != "com.acme.web" || len(f.Imports) != 4 || !f.Imports[0].Wildcard || !f.Imports[2].Static {
		t.Fatalf("package/imports: %+v", f)
	}
	if len(f.Types) != 1 {
		t.Fatalf("want 1 top-level type, got %d", len(f.Types))
	}
	c := f.Types[0]
	if c.Kind != Class || c.Name != "EmployeeController" || c.QualifiedName() != "com.acme.web.EmployeeController" {
		t.Fatalf("class: %+v", c)
	}
	if c.Extends[0].String() != "BaseController<Employee>" || len(c.Implements) != 2 || c.Implements[1].Name != "java.io.Serializable" {
		t.Errorf("extends/implements: %v %v", c.Extends, c.Implements)
	}

	rm := c.Annotation("RequestMapping")
	arr, ok := rm.Arg("value").(ArrayLit)
	if !ok || len(arr.Items) != 2 {
		t.Fatalf("RequestMapping value: %#v", rm.Arg("value"))
	}
	if cat, ok := arr.Items[0].(Concat); !ok || cat.Parts[0].(NameRef).Name != "BASE" || cat.Parts[1].(StrLit).Value != "/employees" {
		t.Errorf("concat: %#v", arr.Items[0])
	}

	names := []string{}
	for _, fl := range c.Fields {
		names = append(names, fl.Name+":"+fl.Type.String())
	}
	if got := strings.Join(names, " "); got != "TAG:String serialVersionUID:long service:EmployeeService cache:Map<String, List<Employee>> other:Map<String, List<Employee>> scores:int[] matrix:int[][]" {
		t.Errorf("fields: %s", got)
	}
	if cat, ok := c.Fields[0].Init.(Concat); !ok || len(cat.Parts) != 2 {
		t.Errorf("TAG initializer: %#v", c.Fields[0].Init)
	}
	if c.Fields[3].Init != nil {
		t.Errorf("a non-constant initializer must be skipped, got %#v", c.Fields[3].Init)
	}

	methods := map[string]Method{}
	for _, m := range c.Methods {
		methods[m.Name] = m
	}
	if ctor := methods["EmployeeController"]; ctor.Return != nil || len(ctor.Params) != 1 {
		t.Errorf("constructor: %+v", ctor)
	}
	find := methods["find"]
	if find.Return.String() != "ResponseEntity<Employee>" || len(find.Params) != 2 || find.Params[0].Annotation("PathVariable") == nil {
		t.Errorf("find: %+v", find)
	}
	if req := find.Params[1].Annotation("RequestHeader"); req.Arg("name").(StrLit).Value != "X-Tenant" || req.Arg("required").(BoolLit).Value {
		t.Errorf("RequestHeader args: %#v", req.Args)
	}
	search := methods["search"]
	if search.TypeParams[0] != "T" || search.Return.String() != "List<? extends Employee>" || !search.Params[1].Varargs || search.Params[1].Type.String() != "String[]" {
		t.Errorf("search: %+v", search)
	}
	if consumes, ok := search.Annotation("PostMapping").Arg("consumes").(NameRef); !ok || consumes.Name != "MediaType.APPLICATION_JSON_VALUE" {
		t.Errorf("consumes: %#v", search.Annotation("PostMapping").Arg("consumes"))
	}
	if hook := methods["hook"]; hook.Params[1].Type.String() != "String[]" || !hook.HasModifier("abstract") {
		t.Errorf("hook: %+v", hook)
	}

	nested := map[string]*TypeDecl{}
	for _, n := range c.Nested {
		nested[n.Name] = n
	}
	if s := nested["Status"]; s == nil || s.Kind != Enum || strings.Join(s.Constants, ",") != "ACTIVE,ON_LEAVE" {
		t.Errorf("enum: %+v", s)
	}
	if pg := nested["Page"]; pg == nil || pg.Kind != Record || len(pg.Components) != 2 || pg.Components[1].Annotation("JsonProperty") == nil ||
		pg.QualifiedName() != "com.acme.web.EmployeeController.Page" {
		t.Errorf("record: %+v", pg)
	}
	if m := nested["Marker"]; m == nil || m.Kind != Annotation || len(m.Methods) != 2 {
		t.Errorf("annotation type: %+v", m)
	}
	if circle := nested["Circle"]; circle == nil || !circle.HasModifier("non-sealed") {
		t.Errorf("non-sealed class: %+v", circle)
	}
}

func TestParseErrorsPointAtTheLine(t *testing.T) {
	_, err := Parse("Broken.java", "package a;\n\npublic class Broken {\n  int x = ;\n  void m( {\n}\n")
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Path != "Broken.java" || pe.Line < 4 {
		t.Fatalf("want a ParseError with a line, got %v", err)
	}
}

func TestPackageAnnotationsAndEmptyFile(t *testing.T) {
	f, err := Parse("package-info.java", "@NonNullApi\npackage com.acme;\n")
	if err != nil || f.Package != "com.acme" || len(f.Types) != 0 {
		t.Fatalf("package-info: %+v %v", f, err)
	}
}
