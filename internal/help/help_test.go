package help

import (
	"strings"
	"testing"
	"time"
)

func TestTopicsLoad(t *testing.T) {
	for _, name := range []string{Overview, "req", "json", "base64", "list", "env", "agents"} {
		if _, ok := Lookup(name); !ok {
			t.Errorf("missing topic %q", name)
		}
	}
	if Names()[0] != Overview {
		t.Error("the overview must come first")
	}
}

func TestRenderWrapsParagraphsAndKeepsExamples(t *testing.T) {
	topic := Topic{Name: "x", Summary: "s", Body: "HEADING\n    one two three four five six seven eight\n    $ httpgen --req a/very/long/path/that/must/not/wrap.java --stdout\n    --flag <v>\n        described here"}
	out := Render(topic, Style{Width: 24})
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.Contains(line, "$ httpgen") && len(line) > 24 {
			t.Errorf("line longer than 24: %q", line)
		}
	}
	if !strings.Contains(out, "    $ httpgen --req a/very/long/path/that/must/not/wrap.java --stdout\n") {
		t.Errorf("example was wrapped:\n%s", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("plain style must not contain escape codes")
	}
	if !strings.Contains(Render(topic, Style{Width: 80, Bold: true}), "\x1b[1mHEADING\x1b[0m") {
		t.Error("bold style should embolden headings")
	}
}

func TestManPageIsValidRoff(t *testing.T) {
	out := Man("1.2.3", time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC))
	if !strings.HasPrefix(out, ".TH HTTPGEN 1 \"2026-10-28\" \"httpgen 1.2.3\"") {
		t.Errorf("bad header: %q", strings.SplitN(out, "\n", 2)[0])
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, ".") && !strings.HasPrefix(line, ".TH") && !strings.HasPrefix(line, ".SH") &&
			!strings.HasPrefix(line, ".SS") && !strings.HasPrefix(line, ".PP") && !strings.HasPrefix(line, ".TP") &&
			!strings.HasPrefix(line, ".B ") && !strings.HasPrefix(line, ".nf") && !strings.HasPrefix(line, ".fi") &&
			!strings.HasPrefix(line, ".br") {
			t.Errorf("text line read as a roff request: %q", line)
		}
	}
	if !strings.Contains(out, `\-\-req`) {
		t.Error("dashes should be escaped so man keeps them as hyphens")
	}
}
