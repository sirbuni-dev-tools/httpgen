// Package help holds httpgen's manual: one embedded text file per topic, rendered for a terminal,
// as plain text when piped, or as a roff man page.
//
// Topic file format:
//
//	name: req
//	summary: one line shown in the topic list
//	---
//	SYNOPSIS
//	    indented body lines
//
// Lines in capitals with no indent are section headings. Body lines indented by four spaces are
// paragraphs (re-wrapped to the terminal), lines indented by eight are option descriptions,
// `    --flag` lines name an option, and `    $ httpgen ...` lines are examples (never wrapped).
package help

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed topics/*.txt
var files embed.FS

// Topic is one page of the manual.
type Topic struct {
	Name    string
	Summary string
	Body    string
}

// Overview is the topic shown by a bare `httpgen --help`.
const Overview = "httpgen"

var topics = load()

func load() map[string]Topic {
	entries, err := files.ReadDir("topics")
	if err != nil {
		panic(err)
	}
	all := map[string]Topic{}
	for _, e := range entries {
		raw, err := files.ReadFile("topics/" + e.Name())
		if err != nil {
			panic(err)
		}
		t, err := parse(string(raw))
		if err != nil {
			panic(fmt.Sprintf("help topic %s: %v", e.Name(), err))
		}
		all[t.Name] = t
	}
	return all
}

func parse(raw string) (Topic, error) {
	head, body, ok := strings.Cut(raw, "\n---\n")
	if !ok {
		return Topic{}, fmt.Errorf("missing --- line between header and body")
	}
	var t Topic
	for _, line := range strings.Split(head, "\n") {
		key, value, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(key) {
		case "name":
			t.Name = strings.TrimSpace(value)
		case "summary":
			t.Summary = strings.TrimSpace(value)
		}
	}
	if t.Name == "" || t.Summary == "" {
		return Topic{}, fmt.Errorf("header needs name and summary")
	}
	t.Body = strings.TrimRight(body, "\n")
	return t, nil
}

// Lookup returns a topic by name.
func Lookup(name string) (Topic, bool) {
	t, ok := topics[name]
	return t, ok
}

// Names lists every topic, the overview first.
func Names() []string {
	var names []string
	for n := range topics {
		if n != Overview {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return append([]string{Overview}, names...)
}

// TopicList is the `httpgen --help topics` page: every topic with its summary.
func TopicList() string {
	var b strings.Builder
	b.WriteString("HELP TOPICS\n")
	for _, n := range Names() {
		label := n
		if n == Overview {
			label = "(none)"
		}
		fmt.Fprintf(&b, "    %-10s %s\n", label, topics[n].Summary)
	}
	b.WriteString("\n    Read one with: httpgen --help <topic>")
	return b.String()
}

// Examples returns the `$ httpgen ...` command lines of every topic, for tests that keep them valid.
func Examples() []string {
	var out []string
	for _, t := range topics {
		for _, line := range strings.Split(t.Body, "\n") {
			if cmd, ok := strings.CutPrefix(strings.TrimSpace(line), "$ httpgen"); ok {
				out = append(out, strings.TrimSpace(cmd))
			}
		}
	}
	return out
}

// DocumentedFlags returns every `--flag` named at the start of an option line in any topic.
func DocumentedFlags() map[string]bool {
	found := map[string]bool{}
	for _, t := range topics {
		for _, line := range strings.Split(t.Body, "\n") {
			if !strings.HasPrefix(line, "    --") || strings.HasPrefix(line, "     ") {
				continue
			}
			for _, field := range strings.Fields(line) {
				if name, ok := strings.CutPrefix(strings.Trim(field, ",[]"), "--"); ok {
					name, _, _ = strings.Cut(name, "=")
					found[name] = true
				}
			}
		}
	}
	return found
}
