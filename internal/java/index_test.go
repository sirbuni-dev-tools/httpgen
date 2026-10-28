package java

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A two-module project: the API module uses DTOs from a shared module.
func project(t *testing.T) string {
	root := t.TempDir()
	write(t, root, "pom.xml", "<project/>")
	write(t, root, "api/pom.xml", "<project/>")
	write(t, root, "shared/pom.xml", "<project/>")
	write(t, root, "shared/src/main/java/com/acme/shared/Paths.java", `package com.acme.shared;
public final class Paths { public static final String API = "/api"; public static final String V1 = API + "/v1"; }`)
	write(t, root, "shared/src/main/java/com/acme/shared/dto/EmployeeDto.java", `package com.acme.shared.dto;
public class EmployeeDto extends BaseDto { private String firstName; public static class Address { private String city; } }`)
	write(t, root, "shared/src/main/java/com/acme/shared/dto/BaseDto.java", `package com.acme.shared.dto;
public abstract class BaseDto { public static final String PREFIX = "/base"; private java.util.UUID uid; }`)
	write(t, root, "api/src/main/java/com/acme/api/EmployeeController.java", `package com.acme.api;
import com.acme.shared.dto.*;
import com.acme.shared.Paths;
import static com.acme.shared.Paths.V1;
import java.util.List;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Sort;
public class EmployeeController {
    static final String ROOT = V1 + "/employees";
    List<EmployeeDto> all;
    Page<EmployeeDto.Address> page;
    Helper helper;
    String name;
    Unknown thing;
}`)
	write(t, root, "api/src/main/java/com/acme/api/Helper.java", `package com.acme.api; class Helper {}`)
	write(t, root, "api/target/generated/Ignored.java", `this is not java`)
	return root
}

func TestRootsAndBuild(t *testing.T) {
	root := project(t)
	controllerPath := filepath.Join(root, "api", "src", "main", "java", "com", "acme", "api", "EmployeeController.java")
	if got := ProjectRoot(controllerPath); got != mustAbs(t, root) {
		t.Errorf("project root %s, want %s", got, root)
	}
	if got := ModuleRoot(controllerPath); got != mustAbs(t, filepath.Join(root, "api")) {
		t.Errorf("module root %s", got)
	}
	roots := SourceRoots(root)
	if len(roots) != 2 {
		t.Fatalf("source roots: %v", roots)
	}
	ix := Build(roots...)
	if len(ix.ParseErrors) != 0 {
		t.Fatalf("parse errors: %v", ix.ParseErrors)
	}
	if ix.Lookup("com.acme.shared.dto.EmployeeDto.Address") == nil {
		t.Error("nested type not indexed")
	}
}

func mustAbs(t *testing.T, p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestResolveAndConstants(t *testing.T) {
	root := project(t)
	ix := Build(SourceRoots(root)...)
	c := ix.Lookup("com.acme.api.EmployeeController")
	if c == nil {
		t.Fatal("controller not indexed")
	}
	cases := map[string]string{
		"EmployeeDto":         "com.acme.shared.dto.EmployeeDto", // wildcard import
		"EmployeeDto.Address": "com.acme.shared.dto.EmployeeDto.Address",
		"Paths":               "com.acme.shared.Paths", // single import
		"Helper":              "com.acme.api.Helper",   // same package
		"List":                "java.util.List",        // library, single import
		"Page":                "org.springframework.data.domain.Page",
		"String":              "java.lang.String", // java.lang
		"Sort.Direction":      "org.springframework.data.domain.Sort.Direction",
		"Unknown":             "Unknown",
	}
	for name, want := range cases {
		if got := ix.Resolve(name, c).Name; got != want {
			t.Errorf("Resolve(%s) = %s, want %s", name, got, want)
		}
	}
	if r := ix.Resolve("Unknown", c); r.Known() {
		t.Error("an unresolvable name must not be Known")
	}
	if r := ix.Resolve("List", c); !r.Library || r.Decl != nil {
		t.Errorf("List: %+v", r)
	}

	dto := ix.Lookup("com.acme.shared.dto.EmployeeDto")
	if sup := ix.Superclass(dto); sup == nil || sup.Name != "BaseDto" {
		t.Errorf("superclass: %v", sup)
	}

	consts := map[string]string{
		"ROOT":                             "/api/v1/employees", // static import, then a constant built from another
		"Paths.V1":                         "/api/v1",
		"MediaType.APPLICATION_JSON_VALUE": "application/json",
	}
	for name, want := range consts {
		if got, ok := ix.Constant(NameRef{Name: name}, c); !ok || got != want {
			t.Errorf("Constant(%s) = %q, %v; want %q", name, got, ok, want)
		}
	}
	// A constant inherited from a superclass, referenced unqualified.
	if got, ok := ix.Constant(NameRef{Name: "PREFIX"}, dto); !ok || got != "/base" {
		t.Errorf("inherited constant: %q %v", got, ok)
	}
	if _, ok := ix.Constant(NameRef{Name: "NOPE"}, c); ok {
		t.Error("an unknown constant must not resolve")
	}
}
