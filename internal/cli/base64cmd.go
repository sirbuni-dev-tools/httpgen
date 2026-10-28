package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/base64"
)

// runBase64 implements --base64: a file or a built-in sample as Base64, a data URI or a JSON field.
func runBase64(o Options, t Terminal) int {
	if o.Wrap > 0 && o.JSONField != "" {
		fmt.Fprintln(t.Err, "httpgen: --wrap cannot be used with --json-field: a line break inside a JSON string breaks strict Base64 decoders")
		return ExitUsage
	}
	f, err := base64.Open(o.Target)
	if errors.Is(err, base64.ErrUnknown) {
		fmt.Fprintf(t.Err, "httpgen: %s is neither a file nor a sample%s\n", o.Target, sampleSuggestion(o.Target))
		fmt.Fprintf(t.Err, "Samples: %s\n", strings.Join(base64.SampleNames(), ", "))
		return ExitUsage
	}
	if err != nil {
		fmt.Fprintf(t.Err, "httpgen: %v\n", err)
		return ExitUsage
	}
	text := base64.Encode(f, base64.Options{DataURI: o.DataURI, Wrap: o.Wrap, JSONField: o.JSONField}) + "\n"
	if o.Out == "" {
		fmt.Fprint(t.Out, text)
		return ExitOK
	}
	if err := os.WriteFile(o.Out, []byte(text), 0o644); err != nil {
		fmt.Fprintf(t.Err, "httpgen: %v\n", err)
		return ExitUsage
	}
	if !o.Quiet {
		fmt.Fprintf(t.Out, "wrote %s (%s, %d bytes encoded)\n", rel(o.Out), f.Type, len(f.Data))
	}
	return ExitOK
}

func sampleSuggestion(target string) string {
	var close []string
	for _, n := range base64.SampleNames() {
		if Distance(strings.ToLower(target), n) <= 2 {
			close = append(close, n)
		}
	}
	if len(close) == 0 {
		return ""
	}
	return " (did you mean " + strings.Join(close, " or ") + "?)"
}
