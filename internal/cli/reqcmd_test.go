package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// withResolver adds an SPQR resolver and server settings to the test project.
func withResolver(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"src/main/resources/application.properties": "server.port=${PORT:8281}\ngraphql.spqr.http.endpoint=/api/graphql\n",
		"src/main/java/demo/DepartmentResolver.java": `package demo;

import io.leangen.graphql.annotations.*;

public class DepartmentResolver {
    @GraphQLQuery(name = "departmentById")
    public Department find(@GraphQLArgument(name = "uid") java.util.UUID uid) { return null; }

    @GraphQLMutation(name = "saveDepartment")
    public Department save(@GraphQLArgument(name = "input") Department input) { return null; }
}
`,
	}
	for name, src := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReqWritesFilesAndEnvAndDoesNotDuplicate(t *testing.T) {
	root := project(t)
	withResolver(t, root)
	src := filepath.Join(root, "src/main/java")
	if code, _, errOut := run("--req " + src); code != ExitOK {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	dir := filepath.Join(root, ".local/http")
	rest := read(t, filepath.Join(dir, "DepartmentController.http"))
	if !strings.HasPrefix(rest, "@id = 1\n\n### POST create\n# @name create\nPOST {{HOST}}/departments\n") {
		t.Errorf("controller file:\n%s", rest)
	}
	gql := read(t, filepath.Join(dir, "DepartmentResolver.http"))
	if !strings.HasPrefix(gql, "@uid = 00000000-0000-0000-0000-000000000000\n\n### POST login") ||
		!strings.Contains(gql, "GRAPHQL {{HOST}}/api/graphql\n") || !strings.Contains(gql, "mutation SaveDepartment($input: DepartmentInput!) {") {
		t.Errorf("resolver file:\n%s", gql)
	}
	env := read(t, filepath.Join(dir, "http-client.env.json"))
	if !strings.Contains(env, `"HOST": "http://localhost:8281"`) || !strings.Contains(env, `"AUTH_URL": "http://localhost:8281/oauth2/token"`) {
		t.Errorf("env:\n%s", env)
	}
	if p := read(t, filepath.Join(dir, "http-client.private.env.json")); !strings.Contains(p, `"PASSWORD": ""`) {
		t.Errorf("private env:\n%s", p)
	}

	// A second run adds nothing and keeps edits.
	edited := strings.Replace(rest, "@id = 1", "@id = 42", 1)
	if err := os.WriteFile(filepath.Join(dir, "DepartmentController.http"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("--req " + src + " --format json")
	var res struct {
		Files []fileReport
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || code != ExitOK {
		t.Fatalf("code=%d %v: %s", code, err, out)
	}
	for _, f := range res.Files {
		if len(f.Added) != 0 || len(f.Env) != 0 {
			t.Errorf("second run changed %+v", f)
		}
	}
	if got := read(t, filepath.Join(dir, "DepartmentController.http")); got != edited {
		t.Errorf("edits lost:\n%s", got)
	}
	if n := len(regexp.MustCompile(`(?m)^# @name login$`).FindAllString(read(t, filepath.Join(dir, "DepartmentResolver.http")), -1)); n != 1 {
		t.Errorf("%d login requests", n)
	}
}

func TestReqDryRunWritesNothing(t *testing.T) {
	root := project(t)
	code, out, _ := run("--req " + filepath.Join(root, "src/main/java") + " --dry-run")
	if code != ExitOK || !strings.Contains(out, "would add create, find, all") || !strings.Contains(out, "http-client.env.json: would update") {
		t.Errorf("code=%d out=%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".local")); !os.IsNotExist(err) {
		t.Errorf(".local was created: %v", err)
	}
}

// A dry run reports exactly what the real run then does, even when several classes share the env files.
func TestReqDryRunMatchesTheRealRun(t *testing.T) {
	root := project(t)
	// A resolver read before the controller adds every env key, so the controller adds none.
	p := filepath.Join(root, "src/main/java/api/AccountResolver.java")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package api;\nimport io.leangen.graphql.annotations.*;\npublic class AccountResolver {\n" +
		"    @GraphQLQuery(name = \"accountCount\") public int count() { return 0; }\n}\n"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "src/main/java")
	_, dry, _ := run("--req " + dir + " --dry-run")
	_, real, _ := run("--req " + dir)
	if want := strings.NewReplacer("would add", "added", "would update", "updated").Replace(dry); want != real {
		t.Errorf("dry run:\n%s\nreal run:\n%s", dry, real)
	}
	if n := strings.Count(dry, "http-client.env.json: would update"); n != 1 {
		t.Errorf("env file reported %d times:\n%s", n, dry)
	}
}

func TestReqStdoutMethodFilterAndOptions(t *testing.T) {
	root := project(t)
	withResolver(t, root)
	src := filepath.Join(root, "src/main/java")
	_, out, _ := run("--req " + src + " --method find --stdout")
	for _, want := range []string{"# file: DepartmentController.http\n\n@id = 1\n", "# file: DepartmentResolver.http\n\n@uid = ", "query DepartmentById($uid: UUID!) {"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "# @name create") || strings.Contains(out, "# @name save") {
		t.Errorf("method filter ignored:\n%s", out)
	}
	_, out, _ = run("--req " + filepath.Join(src, "demo/DepartmentResolver.java") + " --no-login --stdout --mode fixed --seed 4")
	if strings.Contains(out, "login") || strings.Contains(out, "{{$") || strings.Contains(out, "# file:") {
		t.Errorf("--no-login, --mode fixed or single-file output:\n%s", out)
	}
	out2 := filepath.Join(root, "requests")
	if code, _, _ := run("--req " + filepath.Join(src, "demo/DepartmentController.java") + " --out " + out2 + " --host http://api.test:9000 --env dev -q"); code != ExitOK {
		t.Fatal(code)
	}
	if env := read(t, filepath.Join(out2, "http-client.env.json")); !strings.Contains(env, "\"dev\": {\n    \"HOST\": \"http://api.test:9000\"") {
		t.Errorf("env:\n%s", env)
	}
	if code, _, _ := run("--req " + filepath.Join(src, "demo/Department.java")); code != ExitNothing {
		t.Errorf("a DTO has nothing to request: code=%d", code)
	}
}

func TestList(t *testing.T) {
	root := project(t)
	withResolver(t, root)
	src := filepath.Join(root, "src/main/java")
	code, out, _ := run("--list " + src)
	want := "POST      /departments       DepartmentController.create\n" +
		"GET       /departments/{id}  DepartmentController.find\n" +
		"GET       /departments       DepartmentController.all\n" +
		"query     departmentById     DepartmentResolver.find\n" +
		"mutation  saveDepartment     DepartmentResolver.save\n"
	if code != ExitOK || out != want {
		t.Errorf("code=%d got:\n%s", code, out)
	}
	_, out, _ = run("--list " + src + " --format json")
	var items []listed
	if err := json.Unmarshal([]byte(out), &items); err != nil || len(items) != 5 || items[3].Kind != "query" || items[0].Line == 0 {
		t.Errorf("%v: %s", err, out)
	}
	if code, _, _ := run("--list " + filepath.Join(src, "demo/Department.java")); code != ExitNothing {
		t.Errorf("code=%d", code)
	}
}
