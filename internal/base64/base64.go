// Package base64 encodes files and the built-in samples for JSON upload fields, the same way on every OS.
package base64

import (
	"embed"
	stdb64 "encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed samples
var samples embed.FS

// sampleFiles maps a sample name to its embedded file; jpg is accepted for jpeg.
var sampleFiles = map[string]string{
	"pdf": "sample.pdf", "csv": "sample.csv", "png": "sample.png", "jpeg": "sample.jpeg", "jpg": "sample.jpeg",
}

// SampleNames lists the sample names, without aliases.
func SampleNames() []string { return []string{"pdf", "csv", "jpeg", "png"} }

// ErrUnknown means the target is neither a file nor a sample name.
var ErrUnknown = errors.New("no such file or sample")

// File is the bytes to encode and what to call them.
type File struct {
	Name string
	Type string
	Data []byte
}

// Open reads target as a file when one exists there, else as a sample name.
func Open(target string) (File, error) {
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return File{}, errors.New(target + " is a folder")
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return File{}, err
		}
		return File{Name: filepath.Base(target), Type: MIMEType(target, data), Data: data}, nil
	}
	name, ok := sampleFiles[strings.ToLower(target)]
	if !ok {
		return File{}, ErrUnknown
	}
	data, err := samples.ReadFile("samples/" + name)
	if err != nil {
		return File{}, err
	}
	return File{Name: name, Type: MIMEType(name, data), Data: data}, nil
}

// types are the common upload types, fixed so the answer is the same on every OS.
var types = map[string]string{
	".pdf": "application/pdf", ".csv": "text/csv", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml", ".txt": "text/plain", ".json": "application/json",
	".xml": "application/xml", ".html": "text/html", ".zip": "application/zip",
	".doc": "application/msword", ".xls": "application/vnd.ms-excel",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".odt":  "application/vnd.oasis.opendocument.text", ".ods": "application/vnd.oasis.opendocument.spreadsheet",
}

// MIMEType is the type for a file name's extension, else what the content looks like.
func MIMEType(name string, data []byte) string {
	if t, ok := types[strings.ToLower(filepath.Ext(name))]; ok {
		return t
	}
	t := http.DetectContentType(data)
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = t[:i]
	}
	return t
}

// Options shape the output.
type Options struct {
	DataURI bool
	// Wrap breaks the output into lines of this many characters; 0 keeps one line.
	Wrap int
	// JSONField prints {"<f>": ..., "<f>Name": ..., "<f>Type": ...} instead of the bare string.
	JSONField string
}

// Encode returns the text to print, without a final newline.
func Encode(f File, o Options) string {
	s := stdb64.StdEncoding.EncodeToString(f.Data)
	if o.DataURI {
		s = "data:" + f.Type + ";base64," + s
	}
	if o.JSONField != "" {
		return jsonObject([][2]string{{o.JSONField, s}, {o.JSONField + "Name", f.Name}, {o.JSONField + "Type", f.Type}})
	}
	return wrap(s, o.Wrap)
}

func wrap(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	var b strings.Builder
	for len(s) > n {
		b.WriteString(s[:n] + "\n")
		s = s[n:]
	}
	b.WriteString(s)
	return b.String()
}

// jsonObject writes fields in order, two-space indented like the other payloads.
func jsonObject(fields [][2]string) string {
	var b strings.Builder
	b.WriteString("{\n")
	for i, f := range fields {
		k, _ := json.Marshal(f[0])
		v, _ := json.Marshal(f[1])
		b.WriteString("  " + string(k) + ": " + string(v))
		if i < len(fields)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}
