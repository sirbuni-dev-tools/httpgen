// Package cli parses httpgen's command line and runs the chosen action.
package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Action is the one thing a run does.
type Action string

const (
	ActionNone    Action = ""
	ActionReq     Action = "req"
	ActionJSON    Action = "json"
	ActionBase64  Action = "base64"
	ActionList    Action = "list"
	ActionHelp    Action = "help"
	ActionMan     Action = "man"
	ActionVersion Action = "version"
)

// Options holds everything the command line set.
type Options struct {
	Action Action
	// Target is the path or sample name given to --req, --json, --base64 or --list.
	Target string
	// HelpTopic is the topic for --help; empty means the overview.
	HelpTopic string

	Methods   []string
	Class     string
	Out       string
	Stdout    bool
	DryRun    bool
	Mode      string
	Host      string
	Env       string
	NoLogin   bool
	Depth     int
	Seed      int64
	HasSeed   bool
	DataURI   bool
	Wrap      int
	JSONField string
	Format    string
	Lang      string
	Quiet     bool
	Verbose   bool
	NoPager   bool
}

// flagSpec describes one flag. Every flag here must be documented in a help topic (see the help tests).
type flagSpec struct {
	name     string
	short    string
	value    string // placeholder for the value, empty for a switch
	action   Action // set when the flag chooses the action
	optional bool   // the value may be left out (only --help)
	apply    func(o *Options, v string) error
}

func intValue(name string, dst *int) func(*Options, string) error {
	return func(_ *Options, v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return fmt.Errorf("%s needs a whole number of 0 or more, got %q", name, v)
		}
		*dst = n
		return nil
	}
}

func oneOf(name string, allowed []string, dst *string) func(*Options, string) error {
	return func(_ *Options, v string) error {
		for _, a := range allowed {
			if v == a {
				*dst = v
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %s, got %q", name, strings.Join(allowed, ", "), v)
	}
}

func specs(o *Options) []flagSpec {
	set := func(dst *string) func(*Options, string) error {
		return func(_ *Options, v string) error { *dst = v; return nil }
	}
	on := func(dst *bool) func(*Options, string) error {
		return func(_ *Options, _ string) error { *dst = true; return nil }
	}
	target := func(_ *Options, v string) error { o.Target = v; return nil }
	return []flagSpec{
		{name: "req", value: "path", action: ActionReq, apply: target},
		{name: "json", value: "path", action: ActionJSON, apply: target},
		{name: "base64", value: "file|sample", action: ActionBase64, apply: target},
		{name: "list", value: "path", action: ActionList, apply: target},
		{name: "help", short: "h", value: "topic", action: ActionHelp, optional: true, apply: set(&o.HelpTopic)},
		{name: "man", action: ActionMan, apply: func(*Options, string) error { return nil }},
		{name: "version", action: ActionVersion, apply: func(*Options, string) error { return nil }},

		{name: "method", value: "name", apply: func(_ *Options, v string) error { o.Methods = append(o.Methods, v); return nil }},
		{name: "class", value: "name", apply: set(&o.Class)},
		{name: "out", value: "path", apply: set(&o.Out)},
		{name: "stdout", apply: on(&o.Stdout)},
		{name: "dry-run", apply: on(&o.DryRun)},
		{name: "mode", value: "placeholders|fixed", apply: oneOf("--mode", []string{"placeholders", "fixed"}, &o.Mode)},
		{name: "host", value: "url", apply: set(&o.Host)},
		{name: "env", value: "name", apply: set(&o.Env)},
		{name: "no-login", apply: on(&o.NoLogin)},
		{name: "depth", value: "n", apply: intValue("--depth", &o.Depth)},
		{name: "seed", value: "n", apply: func(_ *Options, v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("--seed needs a whole number, got %q", v)
			}
			o.Seed, o.HasSeed = n, true
			return nil
		}},
		{name: "data-uri", apply: on(&o.DataURI)},
		{name: "wrap", value: "n", apply: intValue("--wrap", &o.Wrap)},
		{name: "json-field", value: "name", apply: set(&o.JSONField)},
		{name: "format", value: "text|json", apply: oneOf("--format", []string{"text", "json"}, &o.Format)},
		{name: "lang", value: "auto|java", apply: oneOf("--lang", []string{"auto", "java"}, &o.Lang)},
		{name: "quiet", short: "q", apply: on(&o.Quiet)},
		{name: "verbose", short: "v", apply: on(&o.Verbose)},
		{name: "no-pager", apply: on(&o.NoPager)},
	}
}

// FlagNames lists every long flag name, for the help tests and suggestions.
func FlagNames() []string {
	var names []string
	for _, s := range specs(&Options{}) {
		names = append(names, s.name)
	}
	sort.Strings(names)
	return names
}

// UsageError is a mistake on the command line (exit code 2).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usage(format string, a ...any) error { return &UsageError{Msg: fmt.Sprintf(format, a...)} }

// Parse reads the arguments. `--help` after an action (`--req --help`) asks for that action's help page.
func Parse(args []string) (Options, error) {
	o := Options{Mode: "", Depth: 3, Format: "text", Lang: "auto", Env: "local"}
	byName := map[string]flagSpec{}
	for _, s := range specs(&o) {
		byName["--"+s.name] = s
		if s.short != "" {
			byName["-"+s.short] = s
		}
	}
	var actions []Action
	helpRequested := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.IndexByte(arg, '='); eq > 0 {
				name, value, hasValue = arg[:eq], arg[eq+1:], true
			}
		}
		spec, ok := byName[name]
		if !ok {
			if strings.HasPrefix(arg, "-") {
				return o, usage("unknown option %s%s", arg, suggestion(name))
			}
			return o, usage("unexpected argument %q (options start with --; see httpgen --help)", arg)
		}
		if spec.value != "" && !hasValue {
			next := ""
			if i+1 < len(args) {
				next = args[i+1]
			}
			switch {
			case next != "" && !strings.HasPrefix(next, "-"):
				value, hasValue = next, true
				i++
			case spec.optional, helpRequested, next == "--help" || next == "-h":
				// `--help` alone, `--help --req` or `--req --help`: the value may be missing.
			default:
				return o, usage("%s needs a value: %s <%s>", name, name, spec.value)
			}
		}
		if spec.action == ActionHelp {
			helpRequested = true
		} else if spec.action != ActionNone {
			actions = append(actions, spec.action)
		}
		if hasValue || spec.value == "" {
			if err := spec.apply(&o, value); err != nil {
				return o, &UsageError{Msg: err.Error()}
			}
		}
	}

	if helpRequested {
		o.Action = ActionHelp
		if o.HelpTopic == "" && len(actions) == 1 {
			o.HelpTopic = string(actions[0])
		}
		return o, nil
	}
	switch len(actions) {
	case 0:
		o.Action = ActionHelp
	case 1:
		o.Action = actions[0]
	default:
		return o, usage("choose one of --req, --json, --base64, --list, --man, --version (got %d)", len(actions))
	}
	if (o.Action == ActionReq || o.Action == ActionJSON || o.Action == ActionBase64 || o.Action == ActionList) && o.Target == "" {
		return o, usage("--%s needs a value (see httpgen --help %s)", o.Action, o.Action)
	}
	return o, nil
}

// suggestion returns " (did you mean --x?)" for a close flag name, or "".
func suggestion(name string) string {
	bare := strings.TrimLeft(name, "-")
	best, bestDist := "", 3
	for _, n := range FlagNames() {
		if d := Distance(bare, n); d < bestDist {
			best, bestDist = n, d
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf(" (did you mean --%s?)", best)
}

// Distance is the Levenshtein edit distance, used for "did you mean" suggestions.
func Distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
