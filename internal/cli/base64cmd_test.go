package cli

import (
	stdb64 "encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBase64FileAndOut(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "contract.pdf")
	if err := os.WriteFile(in, []byte("%PDF-1.7 tiny"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run("--base64 " + in + " --data-uri")
	if code != ExitOK || out != "data:application/pdf;base64,"+stdb64.StdEncoding.EncodeToString([]byte("%PDF-1.7 tiny"))+"\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
	dest := filepath.Join(dir, "contract.b64")
	code, out, _ = run("--base64 " + in + " --out " + dest)
	if code != ExitOK || !strings.Contains(out, "wrote ") || !strings.Contains(out, "application/pdf, 13 bytes encoded") {
		t.Errorf("code=%d out=%q", code, out)
	}
	if b, _ := os.ReadFile(dest); string(b) != stdb64.StdEncoding.EncodeToString([]byte("%PDF-1.7 tiny"))+"\n" {
		t.Errorf("file: %q", b)
	}
}

func TestBase64SamplesAndErrors(t *testing.T) {
	code, out, _ := run("--base64 pdf --json-field attachment")
	if code != ExitOK || !strings.HasPrefix(out, "{\n  \"attachment\": \"JVBERi0") || !strings.Contains(out, "\"attachmentType\": \"application/pdf\"\n}\n") {
		t.Errorf("code=%d out=%q", code, out)
	}
	if code, _, errOut := run("--base64 pnj"); code != ExitUsage || !strings.Contains(errOut, "png?)") || !strings.Contains(errOut, "Samples: pdf, csv, jpeg, png") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
	if code, _, errOut := run("--base64 csv --wrap 20 --json-field f"); code != ExitUsage || !strings.Contains(errOut, "--wrap cannot be used with --json-field") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}
