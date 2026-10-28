package endpoints

import (
	"strings"
	"testing"

	"github.com/sirbuni-dev-tools/httpgen/internal/java"
)

func parse(t *testing.T, src string) (*java.Index, *java.TypeDecl) {
	t.Helper()
	f, err := java.Parse("Test.java", src)
	if err != nil {
		t.Fatal(err)
	}
	ix := java.NewIndex()
	ix.Add(f)
	return ix, f.Types[0]
}

func TestEndpoints(t *testing.T) {
	ix, c := parse(t, `package demo;
import org.springframework.data.domain.Pageable;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import jakarta.servlet.http.HttpServletRequest;

@RestController
@RequestMapping("/api/" + DepartmentController.V)
public class DepartmentController {
    static final String V = "v1";

    @PostMapping(value = "/departments", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Object create(@RequestBody Department body, HttpServletRequest request) { return null; }

    @GetMapping({"/departments/{id:[0-9]+}", "/legacy/{id}"})
    public Object find(@PathVariable("id") Long departmentId, @RequestHeader(name = "X-Tenant") String tenant) { return null; }

    @RequestMapping(path = "/departments", method = RequestMethod.DELETE)
    public void purge(@RequestParam(value = "before") String cutoff, int limit, Pageable pageable) { }

    public void helper() { }
}
`)
	if !IsController(c) || IsResolverClass(c) {
		t.Fatal("controller detection")
	}
	eps := Endpoints(ix, c)
	if len(eps) != 3 {
		t.Fatalf("want 3 endpoints, got %d", len(eps))
	}
	create, find, purge := eps[0], eps[1], eps[2]
	if create.Verb != "POST" || create.Path != "/api/v1/departments" || create.Body == nil || len(create.Params) != 0 {
		t.Errorf("create: %+v", create)
	}
	if create.Consumes != "application/json" {
		t.Errorf("consumes: %q", create.Consumes)
	}
	if find.Path != "/api/v1/departments/{id}" || keys(find.PathVars) != "id" || keys(find.Headers) != "X-Tenant" {
		t.Errorf("find: path=%s vars=%s headers=%s", find.Path, keys(find.PathVars), keys(find.Headers))
	}
	if purge.Verb != "DELETE" || keys(purge.Params) != "before,limit" || !purge.Pageable {
		t.Errorf("purge: verb=%s params=%s pageable=%v", purge.Verb, keys(purge.Params), purge.Pageable)
	}
}

func TestOperations(t *testing.T) {
	ix, c := parse(t, `package demo;
import io.leangen.graphql.annotations.*;
import io.leangen.graphql.execution.ResolutionEnvironment;

public class DepartmentResolver {
    @GraphQLQuery(name = "departmentById")
    public Object find(@GraphQLArgument(name = "uid") String id, @GraphQLEnvironment ResolutionEnvironment env) { return null; }

    @GraphQLMutation
    public Object saveDepartment(Department input, @GraphQLIgnore String internal) { return null; }
}
`)
	if !IsResolverClass(c) {
		t.Fatal("resolver detection")
	}
	ops := Operations(ix, c)
	if len(ops) != 2 {
		t.Fatalf("want 2 operations, got %d", len(ops))
	}
	if ops[0].Kind != "query" || ops[0].Name != "departmentById" || keys(ops[0].Args) != "uid" {
		t.Errorf("query: %+v", ops[0])
	}
	if ops[1].Kind != "mutation" || ops[1].Name != "saveDepartment" || keys(ops[1].Args) != "input" {
		t.Errorf("mutation: %+v", ops[1])
	}
}

func TestJoinPath(t *testing.T) {
	cases := map[[2]string]string{
		{"", ""}:                    "/",
		{"/api/", "/x"}:             "/api/x",
		{"api", "{id:\\d+}/items"}:  "/api/{id}/items",
		{"/a/{org}", "{id:[0-9]+}"}: "/a/{org}/{id}",
	}
	for in, want := range cases {
		if got := joinPath(in[0], in[1]); got != want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func keys(args []Arg) string {
	var k []string
	for _, a := range args {
		k = append(k, a.Key)
	}
	return strings.Join(k, ",")
}
