package help

import (
	"fmt"
	"strings"
	"time"
)

// Style controls how a page is drawn.
type Style struct {
	// Width wraps paragraphs; 0 means 80.
	Width int
	// Bold uses ANSI bold for headings and option names (only for a terminal).
	Bold bool
}

const (
	bold  = "\x1b[1m"
	reset = "\x1b[0m"
)

// Render draws a topic as text: headings, re-wrapped paragraphs, options and examples kept as written.
func Render(t Topic, s Style) string {
	width := s.Width
	if width <= 0 {
		width = 80
	}
	var b strings.Builder
	heading := func(text string) {
		if s.Bold {
			b.WriteString(bold + text + reset + "\n")
		} else {
			b.WriteString(text + "\n")
		}
	}
	section := ""
	// Each source line is its own paragraph; blank lines in the topic file separate them visually.
	for _, line := range strings.Split(t.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		switch {
		case trimmed == "":
			b.WriteString("\n")
		case indent == "" && strings.ToUpper(trimmed) == trimmed:
			section = trimmed
			heading(trimmed)
		case strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "|"):
			// Examples and table rows are never re-wrapped.
			b.WriteString(line + "\n")
		case len(indent) == 4 && strings.HasPrefix(trimmed, "-"):
			if s.Bold {
				b.WriteString(indent + bold + trimmed + reset + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		case section == "SYNOPSIS":
			// Usage lines wrap with a hanging indent so each form stays recognisable.
			b.WriteString(wrap(trimmed, indent, indent+"    ", width))
		default:
			b.WriteString(wrap(trimmed, indent, indent, width))
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// wrap breaks text into lines no wider than width: the first starts with indent, the rest with next.
func wrap(text, indent, next string, width int) string {
	var b strings.Builder
	line := indent
	start := len(indent)
	for _, word := range strings.Fields(text) {
		if len(line) > start && len(line)+1+len(word) > width {
			b.WriteString(line + "\n")
			line = next
			start = len(next)
		}
		if len(line) > start {
			line += " "
		}
		line += word
	}
	if len(line) > start {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// Man renders every topic as one roff man page (section 1), for `man httpgen`.
func Man(version string, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, ".TH HTTPGEN 1 %q %q \"httpgen manual\"\n", now.Format("2006-01-02"), "httpgen "+version)
	for _, name := range Names() {
		t := topics[name]
		if name != Overview {
			fmt.Fprintf(&b, ".SH %s\n", roffEscape(strings.ToUpper("TOPIC: "+name)))
		}
		inExample := false
		section := ""
		for _, line := range strings.Split(t.Body, "\n") {
			trimmed := strings.TrimSpace(line)
			indent := len(line) - len(strings.TrimLeft(line, " "))
			isExample := strings.HasPrefix(trimmed, "$ ") || strings.HasPrefix(trimmed, "|")
			if isExample && !inExample {
				b.WriteString(".nf\n")
				inExample = true
			} else if !isExample && inExample {
				b.WriteString(".fi\n")
				inExample = false
			}
			switch {
			case trimmed == "":
				if !inExample {
					b.WriteString(".PP\n")
				}
			case indent == 0 && strings.ToUpper(trimmed) == trimmed:
				section = trimmed
				if name == Overview {
					fmt.Fprintf(&b, ".SH %s\n", roffEscape(trimmed))
				} else {
					fmt.Fprintf(&b, ".SS %s\n", roffEscape(trimmed))
				}
			case indent == 4 && strings.HasPrefix(trimmed, "-"):
				fmt.Fprintf(&b, ".TP\n.B %s\n", roffEscape(trimmed))
			case section == "SYNOPSIS":
				// One usage form per line.
				b.WriteString(roffEscape(trimmed) + "\n.br\n")
			default:
				// Each source line is its own paragraph.
				b.WriteString(roffEscape(trimmed) + "\n.PP\n")
			}
		}
		if inExample {
			b.WriteString(".fi\n")
		}
	}
	return b.String()
}

// roffEscape keeps text literal in roff: backslashes, and a leading dot or quote that roff reads as a request.
func roffEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\e`)
	s = strings.ReplaceAll(s, "-", `\-`)
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "'") {
		s = `\&` + s
	}
	return s
}
