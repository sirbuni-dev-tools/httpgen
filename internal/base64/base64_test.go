package base64

import (
	"bytes"
	stdb64 "encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSamplesAreValidFiles(t *testing.T) {
	magic := map[string][]byte{
		"pdf":  []byte("%PDF-1.4"),
		"png":  []byte("\x89PNG\r\n\x1a\n"),
		"jpeg": {0xFF, 0xD8, 0xFF},
		"csv":  []byte("code,name,"),
	}
	for _, name := range SampleNames() {
		f, err := Open(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.HasPrefix(f.Data, magic[name]) {
			t.Errorf("%s does not start with its signature", name)
		}
		if bytes.Contains(f.Data, []byte("Exif")) || bytes.Contains(f.Data, []byte("Photoshop")) {
			t.Errorf("%s carries metadata", name)
		}
	}
	if f, _ := Open("JPG"); f.Name != "sample.jpeg" || f.Type != "image/jpeg" {
		t.Errorf("jpg alias: %+v", f.Name)
	}
	if _, err := Open("pfd"); err != ErrUnknown {
		t.Errorf("unknown target: %v", err)
	}
}

func TestAFileWinsOverASampleName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pdf")
	if err := os.WriteFile(path, []byte("not a sample"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	f, err := Open("pdf")
	if err != nil || string(f.Data) != "not a sample" || f.Type != "text/plain" {
		t.Errorf("%+v %v", f, err)
	}
	if _, err := Open(dir); err == nil {
		t.Error("a folder must be an error")
	}
}

func TestEncodeRoundTripsAndShapes(t *testing.T) {
	f, _ := Open("png")
	plain := Encode(f, Options{})
	if back, err := stdb64.StdEncoding.DecodeString(plain); err != nil || !bytes.Equal(back, f.Data) {
		t.Fatalf("round trip: %v", err)
	}
	if uri := Encode(f, Options{DataURI: true}); uri != "data:image/png;base64,"+plain {
		t.Errorf("data URI: %.40s", uri)
	}
	wrapped := Encode(f, Options{Wrap: 76})
	lines := strings.Split(wrapped, "\n")
	for _, l := range lines[:len(lines)-1] {
		if len(l) != 76 {
			t.Errorf("line of %d", len(l))
		}
	}
	if strings.ReplaceAll(wrapped, "\n", "") != plain {
		t.Error("wrapping changed the content")
	}
	var obj map[string]string
	if err := json.Unmarshal([]byte(Encode(f, Options{JSONField: "logo", DataURI: true})), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["logo"] != "data:image/png;base64,"+plain || obj["logoName"] != "sample.png" || obj["logoType"] != "image/png" {
		t.Errorf("json field: %v", obj)
	}
}

func TestMIMEType(t *testing.T) {
	cases := map[string]string{"a.PDF": "application/pdf", "b.csv": "text/csv", "c.docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
	for name, want := range cases {
		if got := MIMEType(name, nil); got != want {
			t.Errorf("%s: %s", name, got)
		}
	}
	if got := MIMEType("noext", []byte("%PDF-1.7")); got != "application/pdf" {
		t.Errorf("sniffed: %s", got)
	}
	if got := MIMEType("blob", []byte{0, 1, 2, 3}); got != "application/octet-stream" {
		t.Errorf("binary: %s", got)
	}
}
