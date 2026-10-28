package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// project writes a small Maven project and returns its root.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"pom.xml": "<project/>",
		"src/main/java/demo/Department.java": `package demo;

import jakarta.persistence.GeneratedValue;
import jakarta.persistence.Id;

public class Department {
    @Id @GeneratedValue
    private Long id;
    private String name;
    private Integer headcount;
}
`,
		"src/main/java/demo/DepartmentController.java": `package demo;

import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/departments")
public class DepartmentController {
    @PostMapping
    public Department create(@RequestBody Department department) { return department; }

    @GetMapping("/{id}")
    public Department find(@PathVariable("id") Long departmentId) { return null; }

    @GetMapping
    public java.util.List<Department> all() { return null; }
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
	return root
}

func TestJSONClassKeepsGeneratedId(t *testing.T) {
	root := project(t)
	code, out, errOut := run("--json " + filepath.Join(root, "src/main/java/demo/Department.java") + " --stdout --seed 1")
	if code != ExitOK {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not one JSON document: %v\n%s", err, out)
	}
	for _, key := range []string{"id", "name", "headcount"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("missing %s in %s", key, out)
		}
	}
}

func TestJSONControllerWritesRequestPayloads(t *testing.T) {
	root := project(t)
	code, out, errOut := run("--json " + filepath.Join(root, "src/main/java/demo/DepartmentController.java") + " --format json")
	if code != ExitOK {
		t.Fatalf("code=%d err=%s", code, errOut)
	}
	var res struct{ Written []string }
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(res.Written) != 2 {
		t.Fatalf("want create and find, got %v", res.Written)
	}
	body, err := os.ReadFile(filepath.Join(root, ".local/json/DepartmentController.create.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"id"`) || !strings.Contains(string(body), `"name"`) {
		t.Errorf("request body must leave out the generated id: %s", body)
	}
	params, err := os.ReadFile(filepath.Join(root, ".local/json/DepartmentController.find.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(params), `"id"`) {
		t.Errorf("path variable must use its annotation name: %s", params)
	}
}

func TestJSONStdoutNamesEachPayload(t *testing.T) {
	root := project(t)
	_, out, _ := run("--json " + filepath.Join(root, "src/main/java/demo/DepartmentController.java") + " --stdout --mode placeholders")
	for _, want := range []string{"// DepartmentController.create\n{", "\n\n// DepartmentController.find\n{", "{{$random."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestJSONNothingAndBadPath(t *testing.T) {
	root := project(t)
	code, _, errOut := run("--json " + filepath.Join(root, "src/main/java/demo/DepartmentController.java") + " --method nope")
	if code != ExitNothing || !strings.Contains(errOut, "looking for method nope") {
		t.Errorf("code=%d err=%s", code, errOut)
	}
	if code, _, _ := run("--json " + filepath.Join(root, "missing.java")); code != ExitUsage {
		t.Errorf("missing file: code=%d", code)
	}
	if code, _, _ := run("--json " + filepath.Join(root, "pom.xml")); code != ExitUsage {
		t.Errorf("non-java file: code=%d", code)
	}
}

func TestJSONParseError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Broken.java")
	if err := os.WriteFile(p, []byte("public class Broken { private int ; "), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := run("--json " + p + " --stdout"); code != ExitParseError {
		t.Errorf("code=%d err=%s", code, errOut)
	}
}
