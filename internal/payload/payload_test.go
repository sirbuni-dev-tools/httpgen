package payload

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/rules"
)

func hints(t *testing.T) *Hints {
	t.Helper()
	r, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	return NewHints(r)
}

func intp(n int) *int           { return &n }
func floatp(f float64) *float64 { return &f }

// The same cases as the plugin's FieldHintsTest, so both tools pick the same generators.
func TestHintsMatchThePlugin(t *testing.T) {
	h := hints(t)
	res := func(name string, kind Kind, owner string, c Constraints) Generator {
		return h.Resolve(FieldContext{Name: name, Kind: kind, Owner: owner, Constraints: c})
	}
	cases := []struct {
		got  Generator
		want Generator
	}{
		{res("firstName", KString, "", Constraints{}), Faker{"name.firstName", true}},
		{res("first_name", KString, "", Constraints{}), Faker{"name.firstName", true}},
		{res("SURNAME", KString, "", Constraints{}), Faker{"name.lastName", true}},
		{res("name", KString, "Institution", Constraints{}), Faker{"company.name", true}},
		{res("name", KString, "Department", Constraints{}), Faker{"commerce.department", true}},
		{res("name", KString, "Employee", Constraints{}), Faker{"name.fullName", true}},
		{res("name", KString, "Category", Constraints{}), Faker{"lorem.word", true}},
		{res("id", KString, "", Constraints{}), UUID{}},
		{res("id", KLong, "", Constraints{}), IntRange{1, 1000}},
		{res("salary", KInteger, "", Constraints{}), IntRange{1000, 5000000}},
		{res("salary", KDecimal, "", Constraints{}), DecimalRange{1000, 5000000}},
		{res("customerEmail", KString, "", Constraints{}), Faker{"internet.emailAddress", true}},
		{res("createdById", KLong, "", Constraints{}), IntRange{1, 1000}},
		{res("zebra", KString, "", Constraints{}), Faker{"lorem.word", true}},
		{res("active", KBoolean, "", Constraints{}), Boolean{}},
		{res("startDate", KDate, "", Constraints{}), Date{}},
		{res("createdAt", KInstant, "", Constraints{}), IsoTimestamp{}},
		{res("firstName", KString, "", Constraints{Pattern: "[A-Z]{3}"}), Pattern{"[A-Z]{3}"}},
		{res("contact", KString, "", Constraints{Email: true}), Faker{"internet.emailAddress", true}},
		{res("age", KInteger, "", Constraints{Min: floatp(18), Max: floatp(60)}), IntRange{18, 60}},
		{res("birthDate", KDate, "", Constraints{When: Past}), Date{When: Past}},
		{res("firstName", KString, "", Constraints{MinSize: intp(2), MaxSize: intp(50)}), Faker{"name.firstName", true}},
		{res("firstName", KString, "", Constraints{MinSize: intp(3), MaxSize: intp(5)}), Letters{Length: 4}},
		{res("page", KInteger, "", Constraints{}), IntRange{0, 0}},
		{res("pageSize", KInteger, "", Constraints{}), IntRange{10, 10}},
		{res("sortBy", KString, "", Constraints{}), NullValue{}},
		{res("password", KString, "", Constraints{MinSize: intp(8), MaxSize: intp(20)}), OneOf{[]string{"Str0ng!Passw0rd"}, true}},
		{res("confirmPassword", KString, "", Constraints{}), OneOf{[]string{"Str0ng!Passw0rd"}, true}},
	}
	for i, c := range cases {
		if !equalGen(c.got, c.want) {
			t.Errorf("case %d: got %#v, want %#v", i, c.got, c.want)
		}
	}
}

func equalGen(a, b Generator) bool {
	oa, okA := a.(OneOf)
	ob, okB := b.(OneOf)
	if okA && okB {
		return strings.Join(oa.Values, ",") == strings.Join(ob.Values, ",") && oa.Str == ob.Str
	}
	return a == b
}

func TestPlaceholdersAndRendering(t *testing.T) {
	if p := (IntRange{18, 60}).Placeholder(); p != "$random.integer(18, 61)" {
		t.Errorf("integer placeholder: %s", p)
	}
	if p := (IntRange{0, 0}).Placeholder(); p != "" {
		t.Errorf("a single value has no placeholder: %s", p)
	}
	if p := (DecimalRange{0, 100}).Placeholder(); p != "$random.float(0, 100)" {
		t.Errorf("float placeholder: %s", p)
	}
	n := Object{Fields: []Field{
		{"firstName", Value{Faker{"name.firstName", true}}},
		{"salary", Value{IntRange{1000, 5000}}},
		{"skills", Array{Value{OneOf{[]string{"java"}, true}}}},
		{"manager", Cut{}},
		{"filters", EmptyArray{}},
		{"extra", Map{Null{}}},
	}}
	got := Render(n, Placeholders, fake.New(7))
	want := "{\n  \"firstName\": \"{{$random.name.firstName}}\",\n  \"salary\": {{$random.integer(1000, 5001)}},\n  \"skills\": [\n    \"java\"\n  ],\n  \"manager\": {},\n  \"filters\": [],\n  \"extra\": {\n    \"key\": null\n  }\n}"
	if got != want {
		t.Errorf("render:\n%s\nwant:\n%s", got, want)
	}
	fixed := Render(n, Fixed, fake.New(7))
	if strings.Contains(fixed, "{{") || !regexp.MustCompile(`"salary": \d{4}`).MatchString(fixed) {
		t.Errorf("fixed render: %s", fixed)
	}
	if q := Quote("a\"b\\c\nd"); q != `"a\"b\\c\nd"` {
		t.Errorf("quote: %s", q)
	}
}

func index(t *testing.T, sources ...string) *java.Index {
	t.Helper()
	ix := java.NewIndex()
	for i, src := range sources {
		f, err := java.Parse("F"+string(rune('A'+i))+".java", src)
		if err != nil {
			t.Fatal(err)
		}
		ix.Add(f)
	}
	return ix
}

func object(t *testing.T, n Node) Object {
	t.Helper()
	o, ok := n.(Object)
	if !ok {
		t.Fatalf("want an object, got %#v", n)
	}
	return o
}

func get(o Object, key string) Node {
	for _, f := range o.Fields {
		if f.Key == key {
			return f.Value
		}
	}
	return nil
}

func keys(o Object) string {
	var k []string
	for _, f := range o.Fields {
		k = append(k, f.Key)
	}
	return strings.Join(k, ",")
}

// The same cases as the plugin's ClassWalkerTest.
func TestWalkerMatchesThePlugin(t *testing.T) {
	ix := index(t,
		`package demo; public abstract class BaseEntity { private Long id; private java.time.Instant createdAt; }`,
		`package demo; public enum Status { ACTIVE, ON_LEAVE }`,
		`package demo; import java.util.List; public class Department { private String name; private List<Employee> employees; }`,
		`package demo;
import java.util.*;
import jakarta.validation.constraints.*;
import com.fasterxml.jackson.annotation.*;
@JsonIgnoreProperties({"internalNote"})
public class Employee extends BaseEntity {
    public static final String TABLE = "employees";
    private transient int cache;
    @jakarta.persistence.Transient private String computed;
    @JsonIgnore private String password;
    private String internalNote;
    private String firstName;
    @Email private String contact;
    @Min(18) @Max(60) private int age;
    private Status status;
    @JsonProperty("dept") private Department department;
    private List<String> tags;
    private Map<String, Integer> scores;
    @Past private java.time.LocalDate birthDate;
    @Pattern(regexp = "EMP[0-9]{4}") private String code;
    private UUID uid;
}`,
		`package demo; public record CreateEmployee(String firstName, @jakarta.validation.constraints.Size(min = 3, max = 5) String code, java.util.UUID uid) {}`,
		`package demo; public class Item { private String productName; }`,
		`package demo; public class Page<T> { private java.util.List<T> content; private long totalElements; }`,
		`package demo; public class Holder { private Page<Item> page; }`,
		`package demo; public class D { private String word; }`,
		`package demo; public class C { private D d; }`,
		`package demo; public class B { private C c; }`,
		`package demo; public class A { private B b; }`,
	)
	w := &Walker{Index: ix, Hints: hints(t)}
	emp := object(t, w.Decl(ix.Lookup("demo.Employee")))
	if got := keys(emp); got != "id,createdAt,firstName,contact,age,status,dept,tags,scores,birthDate,code,uid" {
		t.Fatalf("keys: %s", got)
	}
	gen := func(o Object, k string) Generator { return get(o, k).(Value).Gen }
	if g := gen(emp, "id"); g != (IntRange{1, 1000}) {
		t.Errorf("id: %#v", g)
	}
	if g := gen(emp, "age"); g != (IntRange{18, 60}) {
		t.Errorf("age: %#v", g)
	}
	if g := gen(emp, "status").(OneOf); strings.Join(g.Values, ",") != "ACTIVE,ON_LEAVE" {
		t.Errorf("status: %#v", g)
	}
	if g := gen(emp, "birthDate"); g != (Date{When: Past}) {
		t.Errorf("birthDate: %#v", g)
	}
	if g := gen(emp, "code"); g != (Pattern{"EMP[0-9]{4}"}) {
		t.Errorf("code: %#v", g)
	}
	dept := object(t, get(emp, "dept"))
	if g := gen(dept, "name"); g != (Faker{"commerce.department", true}) {
		t.Errorf("department name: %#v", g)
	}
	if _, ok := get(dept, "employees").(Array).Elem.(Cut); !ok {
		t.Error("Employee -> Department -> employees must be cut")
	}
	if _, ok := get(emp, "scores").(Map); !ok {
		t.Error("scores must be a map")
	}

	rec := object(t, w.Decl(ix.Lookup("demo.CreateEmployee")))
	if keys(rec) != "firstName,code,uid" || gen(rec, "code") != (Letters{Length: 4}) || gen(rec, "uid") != (UUID{}) {
		t.Errorf("record: %s %#v", keys(rec), rec)
	}

	holder := object(t, w.Decl(ix.Lookup("demo.Holder")))
	item := object(t, get(object(t, get(holder, "page")), "content").(Array).Elem)
	if gen(item, "productName") != (Faker{"commerce.productName", true}) {
		t.Errorf("generic argument not followed: %#v", item)
	}

	a := object(t, w.Decl(ix.Lookup("demo.A")))
	c := object(t, get(object(t, get(a, "b")), "c"))
	if _, ok := get(c, "d").(Cut); !ok {
		t.Errorf("depth limit: %#v", get(c, "d"))
	}
}

func TestRequestAndOutputModes(t *testing.T) {
	ix := index(t,
		`package demo; public class Account {
    @jakarta.persistence.Id @jakarta.persistence.GeneratedValue private Long id;
    private java.util.UUID uid; private String email; private java.util.UUID ownerUid;
    private java.util.List<String> searchFields; private String[] filters;
}`,
		`package demo; @lombok.Getter public class ApiResponse<T> { private boolean success; private T data; private Object hidden; public int getCode() { return 0; } }`,
		`package demo; public class Plain { private String a; public String b; private String c; public String getC() { return c; } }`,
		`package demo; @lombok.Getter public class View { private org.springframework.data.domain.Page<Plain> page; }`,
	)
	req := &Walker{Index: ix, Hints: hints(t), SkipGenerated: true}
	acc := object(t, req.Decl(ix.Lookup("demo.Account")))
	if keys(acc) != "uid,email,ownerUid,searchFields,filters" {
		t.Errorf("generated id must be left out: %s", keys(acc))
	}
	if _, ok := get(acc, "uid").(Null); !ok {
		t.Errorf("own uid must be null in a request: %#v", get(acc, "uid"))
	}
	if get(acc, "ownerUid").(Value).Gen != (UUID{}) {
		t.Error("a reference id keeps a fake value")
	}
	for _, k := range []string{"searchFields", "filters"} {
		if _, ok := get(acc, k).(EmptyArray); !ok {
			t.Errorf("%s must be empty", k)
		}
	}

	out := &Walker{Index: ix, Hints: hints(t), IncludeGetters: true}
	resp := object(t, out.Type(java.TypeRef{Name: "ApiResponse", Args: []java.TypeRef{{Name: "Plain"}}}, ix.Lookup("demo.View"), nil))
	if keys(resp) != "success,data,hidden,code" {
		t.Errorf("getters: %s", keys(resp))
	}
	if keys(object(t, get(resp, "data"))) != "b,c" {
		t.Errorf("only exposed fields: %s", keys(object(t, get(resp, "data"))))
	}
	page := object(t, get(object(t, out.Decl(ix.Lookup("demo.View"))), "page"))
	if !strings.HasPrefix(keys(page), "content,totalElements") {
		t.Errorf("Page in output mode: %s", keys(page))
	}
}
