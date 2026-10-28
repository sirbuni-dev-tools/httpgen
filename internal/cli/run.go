package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/sirbuni-dev-tools/httpgen/internal/help"
	"github.com/sirbuni-dev-tools/httpgen/internal/version"
	"golang.org/x/term"
)

// Exit codes, as documented in `httpgen --help`.
const (
	ExitOK         = 0
	ExitNothing    = 1
	ExitUsage      = 2
	ExitParseError = 3
)

// Terminal describes where output goes; tests pass a fake one.
type Terminal struct {
	Out, Err io.Writer
	// IsTTY is true when Out is an interactive terminal.
	IsTTY bool
	Width int
	// Height decides whether help goes through the pager.
	Height int
}

// StdTerminal inspects the real standard output.
func StdTerminal() Terminal {
	t := Terminal{Out: os.Stdout, Err: os.Stderr, Width: 80, Height: 24}
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		t.IsTTY = true
		if w, h, err := term.GetSize(fd); err == nil {
			t.Width, t.Height = w, h
		}
	}
	return t
}

// Run parses the arguments, does the work and returns the exit code.
func Run(args []string, t Terminal) int {
	opts, err := Parse(args)
	if err != nil {
		var ue *UsageError
		if errors.As(err, &ue) {
			fmt.Fprintf(t.Err, "httpgen: %s\n", ue.Msg)
			fmt.Fprintln(t.Err, "Run httpgen --help for usage.")
			return ExitUsage
		}
		fmt.Fprintf(t.Err, "httpgen: %v\n", err)
		return ExitUsage
	}

	switch opts.Action {
	case ActionHelp:
		return showHelp(opts, t)
	case ActionMan:
		fmt.Fprint(t.Out, help.Man(version.Version, time.Now()))
		return ExitOK
	case ActionVersion:
		fmt.Fprintf(t.Out, "httpgen %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		return ExitOK
	case ActionJSON:
		return runJSON(opts, t)
	default:
		fmt.Fprintf(t.Err, "httpgen: --%s is not available in this build yet (see httpgen --help %s)\n", opts.Action, opts.Action)
		return ExitUsage
	}
}

func showHelp(opts Options, t Terminal) int {
	name := opts.HelpTopic
	if name == "" {
		name = help.Overview
	}
	var page string
	if name == "topics" {
		page = help.TopicList() + "\n"
	} else {
		topic, ok := help.Lookup(name)
		if !ok {
			fmt.Fprintf(t.Err, "httpgen: no help topic %q%s\n", name, topicSuggestion(name))
			fmt.Fprintln(t.Err, "Run httpgen --help topics for the list.")
			return ExitUsage
		}
		page = help.Render(topic, help.Style{Width: min(t.Width, 100), Bold: t.IsTTY})
	}
	if t.IsTTY && !opts.NoPager && strings.Count(page, "\n") >= t.Height {
		if pageThrough(page, t) {
			return ExitOK
		}
	}
	fmt.Fprint(t.Out, page)
	return ExitOK
}

// pageThrough sends a long page to $PAGER (default less -R), like git help. False when no pager runs.
func pageThrough(page string, t Terminal) bool {
	pager := strings.Fields(os.Getenv("PAGER"))
	if len(pager) == 0 {
		pager = []string{"less", "-R"}
	}
	path, err := exec.LookPath(pager[0])
	if err != nil {
		return false
	}
	cmd := exec.Command(path, pager[1:]...)
	cmd.Stdin = strings.NewReader(page)
	cmd.Stdout = t.Out
	cmd.Stderr = t.Err
	return cmd.Run() == nil
}

// topicSuggestion returns " (did you mean x?)" for the closest topic names.
func topicSuggestion(name string) string {
	var close []string
	for _, n := range append(help.Names(), "topics") {
		if Distance(name, n) <= 2 || strings.HasPrefix(n, name) {
			close = append(close, n)
		}
	}
	if len(close) == 0 {
		return ""
	}
	sort.Strings(close)
	return fmt.Sprintf(" (did you mean %s?)", strings.Join(close, " or "))
}
