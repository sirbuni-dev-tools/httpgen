package httpfile

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sirbuni-dev-tools/httpgen/internal/endpoints"
	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
	"github.com/sirbuni-dev-tools/httpgen/internal/java"
	"github.com/sirbuni-dev-tools/httpgen/internal/payload"
	"github.com/sirbuni-dev-tools/httpgen/internal/rules"
)

// The plugin's RequestWriterTest and GraphQlWriterTest sources; the assertions below are the plugin's.
var sources = []string{
	`package demo; public class Employee { private String firstName; private int salary; }`,
	`package demo;
import org.springframework.web.bind.annotation.*;
@RestController
@RequestMapping("/api")
public class EmployeeController {
    @PostMapping("/myemployees-entity-salary")
    public Boolean updateEmployee_salary(@RequestParam("empId") int empId, @RequestParam("salary") double salary) { return true; }

    @GetMapping("/myemployees-entity-find/{id}")
    public Employee findEmployeeById(@PathVariable("id") int empId, @RequestHeader("X-Tenant") String tenant) { return null; }

    @GetMapping("/search")
    public java.util.List<Employee> search(@RequestParam String firstName, int page) { return null; }

    @PostMapping("/myemployees-entity")
    public Employee add(@RequestBody Employee employee, jakarta.servlet.http.HttpServletRequest request) { return employee; }

    @RequestMapping(value = "/legacy", method = RequestMethod.DELETE)
    public void legacy(@RequestParam("empId") long empId) {}

    @GetMapping("/by/{dept}/{fullName}")
    public void byDept(@PathVariable String dept, @PathVariable String fullName) {}

    public void notAnEndpoint() {}
}`,
	`package demo; public class Account { @jakarta.persistence.GeneratedValue private int id; private String email; }`,
	`package demo;
import org.springframework.web.bind.annotation.*;
@RestController
public class Accounts { @PostMapping("/accounts") public Account create(@RequestBody Account a) { return a; } }`,
	`package demo; @lombok.Getter public class ApiResponse<T> { private boolean success; private int code; private String message; private T data; }`,
	`package demo; @lombok.Getter public class Paged<T> { private java.util.List<T> items; private long total; @io.leangen.graphql.annotations.GraphQLIgnore private String internal; }`,
	`package demo; public record DepartmentView(java.util.UUID uid, String name, String code) {}`,
	`package demo; public class PageRequestParam { private Integer page; private Integer pageSize; }`,
	`package demo; public class SearchRequest { private PageRequestParam paging; }`,
	`package demo; public record DepartmentInput(String name, String code) {}`,
	`package demo; public class TypeDto { private java.util.UUID uid; private String name; private java.util.UUID ownerUid; }`,
	`package demo;
import io.leangen.graphql.annotations.*;
import java.util.UUID;
@io.leangen.graphql.spqr.spring.annotations.GraphQLApi
public class DepartmentResolver {
    @GraphQLQuery(name = "departments")
    public ApiResponse<Paged<DepartmentView>> list(@GraphQLArgument(name = "search") SearchRequest search) { return null; }
    @GraphQLMutation(name = "updateDepartment")
    public ApiResponse<DepartmentView> update(@GraphQLArgument(name = "uid") UUID uid, @GraphQLArgument(name = "input") DepartmentInput input) { return null; }
    @GraphQLQuery(name = "ping")
    public String ping() { return "pong"; }
    @GraphQLQuery(name = "typesPage")
    public org.springframework.data.domain.Page<DepartmentView> page(@GraphQLArgument(name = "uid") java.util.UUID uid) { return null; }
    @GraphQLMutation(name = "saveType")
    public DepartmentView save(@GraphQLArgument(name = "dto") TypeDto dto) { return null; }
    public void helper() {}
}`,
}

func fixture(t *testing.T) (*Writer, map[string]*java.TypeDecl) {
	t.Helper()
	ix := java.NewIndex()
	types := map[string]*java.TypeDecl{}
	for i, src := range sources {
		f, err := java.Parse("Fixture"+string(rune('A'+i))+".java", src)
		if err != nil {
			t.Fatal(err)
		}
		ix.Add(f)
		types[f.Types[0].Name] = f.Types[0]
	}
	r, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	return &Writer{Index: ix, Hints: payload.NewHints(r), Source: fake.New(3), Mode: payload.Placeholders}, types
}

func rest(t *testing.T, class, method string) Request {
	w, types := fixture(t)
	for _, e := range endpoints.Endpoints(w.Index, types[class]) {
		if e.Method.Name == method {
			return w.REST(e)
		}
	}
	t.Fatalf("no endpoint %s.%s", class, method)
	return Request{}
}

func graphql(t *testing.T, method string) Request {
	w, types := fixture(t)
	for _, op := range endpoints.Operations(w.Index, types["DepartmentResolver"]) {
		if op.Method.Name == method {
			return w.GraphQL(op)
		}
	}
	t.Fatalf("no operation %s", method)
	return Request{}
}

func contains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("missing %q in:\n%s", p, text)
		}
	}
}

func TestFormBodyForAPostWithRequestParams(t *testing.T) {
	r := rest(t, "EmployeeController", "updateEmployee_salary")
	want := "### POST update employee salary\n# @name updateEmployee_salary\nPOST {{HOST}}/api/myemployees-entity-salary\n" +
		"Content-Type: application/x-www-form-urlencoded\n\nempId={{empId}}&\nsalary={{$random.float(1000, 5000000)}}\n"
	if r.Text != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.Text, want)
	}
	if len(r.Vars) != 1 || r.Vars[0] != (Var{"empId", "1"}) {
		t.Errorf("vars: %v", r.Vars)
	}
}

func TestPathVariablesBecomeFileVariablesAndHeadersAreWritten(t *testing.T) {
	r := rest(t, "EmployeeController", "findEmployeeById")
	if r.Line != "GET {{HOST}}/api/myemployees-entity-find/{{id}}" {
		t.Errorf("line: %s", r.Line)
	}
	if len(r.Vars) != 1 || r.Vars[0] != (Var{"id", "1"}) {
		t.Errorf("vars: %v", r.Vars)
	}
	contains(t, r.Text, "\nX-Tenant: {{$random.lorem.word}}\n")
}

func TestGetPutsParamsInTheQuery(t *testing.T) {
	contains(t, rest(t, "EmployeeController", "search").Text, "GET {{HOST}}/api/search\n    ?firstName={{$random.name.firstName}}\n    &page=0\n")
}

func TestRequestBodyIsJSONAndServletArgumentsAreIgnored(t *testing.T) {
	text := rest(t, "EmployeeController", "add").Text
	contains(t, text, "Content-Type: application/json\n\n{\n  \"firstName\": \"{{$random.name.firstName}}\"")
	if strings.Contains(text, "request") {
		t.Errorf("servlet argument leaked:\n%s", text)
	}
}

func TestFileVariablesAreURLSafe(t *testing.T) {
	safe := regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
	for i := 0; i < 5; i++ {
		for _, v := range rest(t, "EmployeeController", "byDept").Vars {
			if !safe.MatchString(v.Value) {
				t.Errorf("%s = %q", v.Name, v.Value)
			}
		}
	}
}

func TestGeneratedIdsAreLeftOutOfRequestBodies(t *testing.T) {
	text := rest(t, "Accounts", "create").Text
	if !strings.Contains(text, `"email"`) || strings.Contains(text, `"id"`) {
		t.Error(text)
	}
}

func TestRequestMappingWithAMethodAttribute(t *testing.T) {
	if l := rest(t, "EmployeeController", "legacy").Line; l != "DELETE {{HOST}}/api/legacy" {
		t.Error(l)
	}
}

func TestQueryWithInputVariablesAndAThreeLevelSelection(t *testing.T) {
	text := graphql(t, "list").Text
	if !strings.HasPrefix(text, "### GRAPHQL departments\n# @name departments\nGRAPHQL {{HOST}}/graphql\nAuthorization: Bearer {{access_token}}\n\n") {
		t.Errorf("head:\n%s", text)
	}
	contains(t, text,
		"query Departments($search: SearchRequestInput!) {\n  departments(search: $search) {",
		"    data {\n      items {\n        uid\n        name\n        code\n      }\n      total\n    }",
		"{\n  \"search\": {\n    \"paging\": {\n      \"page\": 0,\n      \"pageSize\": 10\n    }\n  }\n}\n",
		"  departments(search: $search) {\n    success\n    code\n    message\n    data {")
	if strings.Contains(text, "internal") {
		t.Errorf("ignored field selected:\n%s", text)
	}
}

func TestMutationIdsAreFileVariablesAndInputsAreTypedBySPQRNaming(t *testing.T) {
	r := graphql(t, "update")
	contains(t, r.Text, "mutation UpdateDepartment($uid: UUID!, $input: DepartmentInputInput!)", `"uid": "{{uid}}"`)
	if len(r.Vars) != 1 || r.Vars[0].Name != "uid" {
		t.Errorf("vars: %v", r.Vars)
	}
	if r.Line != "mutation UpdateDepartment($uid: UUID!, $input: DepartmentInputInput!) {" {
		t.Error(r.Line)
	}
}

func TestSpringDataPageIsSelectedAsAnObjectAndOwnIdsInInputsAreNull(t *testing.T) {
	page := graphql(t, "page").Text
	contains(t, page, "content {\n      uid\n      name\n      code\n    }", "totalElements", "number", `"uid": "{{uid}}"`)
	contains(t, graphql(t, "save").Text, `"uid": null`, `"ownerUid": "{{$uuid}}"`)
}

func TestAScalarResultHasNoSelectionAndNoVariables(t *testing.T) {
	text := graphql(t, "ping").Text
	contains(t, text, "query Ping {\n  ping\n}\n")
	if strings.Contains(text, "\n{\n") {
		t.Error(text)
	}
}

var (
	get  = Request{Line: "GET {{HOST}}/employees/{{id}}", Text: "### GET find\nGET {{HOST}}/employees/{{id}}\n", Vars: []Var{{"id", "1"}}}
	post = Request{Line: "POST {{HOST}}/employees", Text: "### POST add\nPOST {{HOST}}/employees\n"}
)

func TestANewFileGetsTheVariablesFirstThenTheRequests(t *testing.T) {
	m := Merge("", []Request{get, post})
	if want := "@id = 1\n\n### GET find\nGET {{HOST}}/employees/{{id}}\n\n### POST add\nPOST {{HOST}}/employees\n"; m.Text != want {
		t.Errorf("got:\n%q", m.Text)
	}
	if len(m.Added) != 2 {
		t.Error(m.Added)
	}
}

func TestRequestsAlreadyInTheFileAreKeptAsEdited(t *testing.T) {
	existing := "@id = 42\n\n### GET my edited version\nGET {{HOST}}/employees/{{id}}\nAccept: application/json\n"
	m := Merge(existing, []Request{get, post})
	if len(m.Added) != 1 || m.Added[0].Line != post.Line || len(m.Skipped) != 1 || m.Skipped[0].Line != get.Line {
		t.Errorf("added=%v skipped=%v", m.Added, m.Skipped)
	}
	if m.Text != existing+"\n### POST add\nPOST {{HOST}}/employees\n" {
		t.Errorf("got:\n%q", m.Text)
	}
}

func TestRequestNamesStayUniqueInTheFile(t *testing.T) {
	named := func(path string) Request {
		return Request{Line: "GET {{HOST}}" + path, Text: "### GET all\n# @name getAll\nGET {{HOST}}" + path + "\n"}
	}
	m := Merge("### GET kept\n# @name getAll\nGET {{HOST}}/old\n", []Request{named("/a"), named("/b")})
	var names []string
	for _, n := range regexp.MustCompile(`(?m)^# @name (\S+)`).FindAllStringSubmatch(m.Text, -1) {
		names = append(names, n[1])
	}
	if strings.Join(names, ",") != "getAll,getAll2,getAll3" {
		t.Error(names)
	}
}

func TestHostComesFromPropertiesThenYAMLWithPlaceholderDefaults(t *testing.T) {
	cases := []struct{ props, yaml, want string }{
		{"", "", "http://localhost:8080"},
		{"server.port=${SERVER_PORT:8281}\n", "", "http://localhost:8281"},
		{"server.port = 9000\nserver.servlet.context-path=/api/\n", "", "http://localhost:9000/api"},
		{"", "spring:\n  application:\n    name: demo\nserver:\n  port: 8085\n  servlet:\n    context-path: /hr\n", "http://localhost:8085/hr"},
	}
	for _, c := range cases {
		if got := Host(c.props, c.yaml); got != c.want {
			t.Errorf("Host(%q, %q) = %q, want %q", c.props, c.yaml, got, c.want)
		}
	}
	if p := GraphQLPath("graphql.spqr.http.endpoint=/api/graphql/\n", ""); p != "/api/graphql" {
		t.Error(p)
	}
	if p := GraphQLPath("", ""); p != "/graphql" {
		t.Error(p)
	}
}

func TestTitlesReadLikeTheHandWrittenOnes(t *testing.T) {
	if h := Humanize("updateEmployee_salary"); h != "update employee salary" {
		t.Error(h)
	}
	if h := Humanize("findEmployeeById"); h != "find employee by id" {
		t.Error(h)
	}
}

func TestEnvKeysAreAddedAndExistingValuesKept(t *testing.T) {
	text, changed, err := WithEnvKeys("", "local", []Var{{"HOST", "http://localhost:8281"}})
	if err != nil || !changed || text != "{\n  \"local\": {\n    \"HOST\": \"http://localhost:8281\"\n  }\n}\n" {
		t.Fatalf("new file: %q %v %v", text, changed, err)
	}
	edited := "{\n  \"dev\": {\"HOST\": \"https://dev.example.test\"},\n  \"local\": {\n    \"HOST\": \"http://127.0.0.1:9999\"\n  }\n}\n"
	if _, changed, _ := WithEnvKeys(edited, "local", []Var{{"HOST", "x"}}); changed {
		t.Error("an existing key must not change")
	}
	text, changed, _ = WithEnvKeys(edited, "local", []Var{{"HOST", "x"}, {"AUTH_URL", "http://a/b?x=1&y=2"}})
	if !changed || !strings.Contains(text, `"HOST": "http://127.0.0.1:9999"`) || !strings.Contains(text, `"AUTH_URL": "http://a/b?x=1&y=2"`) ||
		strings.Index(text, `"dev"`) > strings.Index(text, `"local"`) {
		t.Errorf("got:\n%s", text)
	}
	if _, _, err := WithEnvKeys("[1]", "local", nil); err == nil {
		t.Error("a non-object file must be an error")
	}
}
