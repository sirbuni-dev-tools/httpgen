package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sirbuni-dev-tools/httpgen/internal/help"
)

func TestParseActions(t *testing.T) {
	cases := []struct {
		args   string
		action Action
		target string
		topic  string
	}{
		{"--req src/Foo.java", ActionReq, "src/Foo.java", ""},
		{"--req=src/Foo.java --stdout", ActionReq, "src/Foo.java", ""},
		{"--json Dto.java --mode fixed --seed 7", ActionJSON, "Dto.java", ""},
		{"--base64 pdf --data-uri", ActionBase64, "pdf", ""},
		{"--list src", ActionList, "src", ""},
		{"", ActionHelp, "", ""},
		{"--help", ActionHelp, "", ""},
		{"-h", ActionHelp, "", ""},
		{"--help req", ActionHelp, "", "req"},
		{"--req --help", ActionHelp, "", "req"},
		{"--req src/Foo.java --help", ActionHelp, "src/Foo.java", "req"},
		{"--help --json", ActionHelp, "", "json"},
		{"--man", ActionMan, "", ""},
		{"--version", ActionVersion, "", ""},
	}
	for _, c := range cases {
		o, err := Parse(strings.Fields(c.args))
		if err != nil {
			t.Fatalf("%q: %v", c.args, err)
		}
		if o.Action != c.action || o.Target != c.target || o.HelpTopic != c.topic {
			t.Errorf("%q: got action=%q target=%q topic=%q", c.args, o.Action, o.Target, o.HelpTopic)
		}
	}
}

func TestParseOptions(t *testing.T) {
	o, err := Parse(strings.Fields("--req a.java --method one --method two --depth 4 --format json --no-login --env dev"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(o.Methods, ",") != "one,two" || o.Depth != 4 || o.Format != "json" || !o.NoLogin || o.Env != "dev" {
		t.Errorf("unexpected options: %+v", o)
	}
	o, _ = Parse(strings.Fields("--req a.java"))
	if o.Depth != 3 || o.Format != "text" || o.Lang != "auto" || o.Env != "local" {
		t.Errorf("unexpected defaults: %+v", o)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"--reqq a.java":            "did you mean --req?",
		"--req":                    "--req needs a value",
		"--req a --json b":         "choose one of",
		"--json a --mode random":   "--mode must be one of",
		"--json a --depth -1":      "--depth needs a value",
		"--json a --depth x":       "--depth needs a whole number",
		"stray":                    "unexpected argument",
		"--base64 pdf --wrap many": "--wrap needs a whole number",
	}
	for args, want := range cases {
		_, err := Parse(strings.Fields(args))
		var ue *UsageError
		if !errors.As(err, &ue) || !strings.Contains(ue.Msg, want) {
			t.Errorf("%q: want usage error containing %q, got %v", args, want, err)
		}
	}
}

// Every flag the parser accepts has an entry in some help topic.
func TestEveryFlagIsDocumented(t *testing.T) {
	documented := help.DocumentedFlags()
	for _, name := range FlagNames() {
		if !documented[name] {
			t.Errorf("--%s has no option entry in any help topic", name)
		}
	}
}

// Every `$ httpgen ...` example in the manual is a valid command line.
func TestEveryHelpExampleParses(t *testing.T) {
	examples := help.Examples()
	if len(examples) < 10 {
		t.Fatalf("expected the manual to have examples, found %d", len(examples))
	}
	for _, ex := range examples {
		if _, err := Parse(strings.Fields(ex)); err != nil {
			t.Errorf("example %q does not parse: %v", ex, err)
		}
	}
}

func run(args string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Run(strings.Fields(args), Terminal{Out: &out, Err: &errOut, Width: 80, Height: 1000})
	return code, out.String(), errOut.String()
}

func TestRunHelp(t *testing.T) {
	code, out, _ := run("--req --help")
	if code != ExitOK || !strings.HasPrefix(out, "NAME\n    httpgen --req") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("help to a non-terminal must be plain text")
	}
	code, out, _ = run("--help topics")
	if code != ExitOK || !strings.Contains(out, "base64") || !strings.Contains(out, "agents") {
		t.Fatalf("topics: code=%d out=%q", code, out)
	}
	code, _, errOut := run("--help jsn")
	if code != ExitUsage || !strings.Contains(errOut, "did you mean json") {
		t.Fatalf("unknown topic: code=%d err=%q", code, errOut)
	}
}

func TestRunVersionAndMan(t *testing.T) {
	if code, out, _ := run("--version"); code != ExitOK || !strings.HasPrefix(out, "httpgen dev") {
		t.Errorf("version: %d %q", code, out)
	}
	if code, out, _ := run("--man"); code != ExitOK || !strings.HasPrefix(out, ".TH HTTPGEN 1") || !strings.Contains(out, ".SH TOPIC: REQ") {
		t.Errorf("man: %d %q", code, out[:min(len(out), 200)])
	}
}
