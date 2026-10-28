// Package payload builds and renders JSON payloads with fake values: the same model, rules and
// output as the API HTTP Generator plugin, over httpgen's own Java index.
package payload

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/sirbuni-dev-tools/httpgen/internal/fake"
)

// Node is a JSON shape with a generator on every leaf.
type Node interface{ node() }

// Object is a JSON object; fields keep declaration order.
type Object struct{ Fields []Field }

// Field is one key of an object.
type Field struct {
	Key   string
	Value Node
}

// Array is a list rendered with one sample element.
type Array struct{ Elem Node }

// Map is a Map<K, V>, rendered with a single sample key.
type Map struct{ Value Node }

// Value is a leaf.
type Value struct{ Gen Generator }

// Cut is a class already on the walk path, or past the depth limit: rendered as {}.
type Cut struct{}

// Null is an explicit null.
type Null struct{}

// EmptyArray is a list left empty on purpose, such as search filters.
type EmptyArray struct{}

func (Object) node()     {}
func (Array) node()      {}
func (Map) node()        {}
func (Value) node()      {}
func (Cut) node()        {}
func (Null) node()       {}
func (EmptyArray) node() {}

// Generator produces one leaf as an HTTP Client placeholder, when one exists, and as a fixed value.
type Generator interface {
	// IsString is true when the value is a JSON string.
	IsString() bool
	// Placeholder is the expression inside {{ }}, or "" when the HTTP Client has no generator for it.
	Placeholder() string
	// Fixed is a fake value chosen now.
	Fixed(s *fake.Source) string
}

// Faker is a provider.method generator, the same name in $random and in the fixed generators.
type Faker struct {
	Name string
	Str  bool
}

// IntRange is whole numbers from Min to Max, both inclusive.
type IntRange struct{ Min, Max int64 }

// DecimalRange is numbers from Min to Max.
type DecimalRange struct{ Min, Max float64 }

// UUID is a random UUID.
type UUID struct{}

// IsoTimestamp is an instant with an offset.
type IsoTimestamp struct{}

// Letters is Length random letters, with digits when Digits.
type Letters struct {
	Length int
	Digits bool
}

// Timing is when a date may fall.
type Timing int

const (
	AnyTime Timing = iota
	Past
	Future
)

// Date is an ISO date near today.
type Date struct{ When Timing }

// DateTime is an ISO local date-time near today.
type DateTime struct{ When Timing }

// TimeOfDay is an ISO local time.
type TimeOfDay struct{}

// Boolean is true or false.
type Boolean struct{}

// OneOf picks one of a fixed set, such as enum constants.
type OneOf struct {
	Values []string
	Str    bool
}

// Pattern is a string matching a @Pattern expression.
type Pattern struct{ Regex string }

// NullValue is an explicit null, for values no fake can get right.
type NullValue struct{}

// Base64 is a byte[], which Jackson writes as Base64.
type Base64 struct{}

func (g Faker) IsString() bool      { return g.Str }
func (g Faker) Placeholder() string { return "$random." + g.Name }
func (g Faker) Fixed(s *fake.Source) string {
	v, err := s.Faker(g.Name)
	if err != nil {
		return g.Name
	}
	return v
}

func (IntRange) IsString() bool { return false }

// Placeholder: $random.integer works on int and excludes its upper bound; a single value is written as is.
func (g IntRange) Placeholder() string {
	if g.Min == g.Max || g.Min < math.MinInt32 || g.Max >= math.MaxInt32 {
		return ""
	}
	return fmt.Sprintf("$random.integer(%d, %d)", g.Min, g.Max+1)
}
func (g IntRange) Fixed(s *fake.Source) string { return strconv.FormatInt(s.Long(g.Min, g.Max), 10) }

func (DecimalRange) IsString() bool { return false }
func (g DecimalRange) Placeholder() string {
	return fmt.Sprintf("$random.float(%s, %s)", plain(g.Min), plain(g.Max))
}
func (g DecimalRange) Fixed(s *fake.Source) string {
	return fmt.Sprintf("%.2f", s.Double(g.Min, g.Max))
}

// plain writes 1000.0 as 1000, the way the plugin does.
func plain(d float64) string {
	if d == math.Floor(d) && !math.IsInf(d, 0) {
		return strconv.FormatInt(int64(d), 10)
	}
	return strconv.FormatFloat(d, 'f', -1, 64)
}

func (UUID) IsString() bool              { return true }
func (UUID) Placeholder() string         { return "$uuid" }
func (UUID) Fixed(s *fake.Source) string { return s.UUID() }

func (IsoTimestamp) IsString() bool      { return true }
func (IsoTimestamp) Placeholder() string { return "$isoTimestamp" }
func (IsoTimestamp) Fixed(s *fake.Source) string {
	return time.Now().UTC().Add(-time.Duration(s.Long(0, 60*24*30)) * time.Minute).Truncate(time.Second).Format(time.RFC3339)
}

func (Letters) IsString() bool { return true }
func (g Letters) Placeholder() string {
	if g.Digits {
		return fmt.Sprintf("$random.alphanumeric(%d)", g.Length)
	}
	return fmt.Sprintf("$random.alphabetic(%d)", g.Length)
}
func (g Letters) Fixed(s *fake.Source) string { return s.Letters(g.Length, g.Digits) }

func offsetDays(s *fake.Source, when Timing) int {
	switch when {
	case Past:
		return -int(s.Long(1, 730))
	case Future:
		return int(s.Long(1, 730))
	}
	return int(s.Long(-365, 365))
}

func (Date) IsString() bool      { return true }
func (Date) Placeholder() string { return "" }
func (g Date) Fixed(s *fake.Source) string {
	return time.Now().AddDate(0, 0, offsetDays(s, g.When)).Format("2006-01-02")
}

func (DateTime) IsString() bool      { return true }
func (DateTime) Placeholder() string { return "" }
func (g DateTime) Fixed(s *fake.Source) string {
	d := time.Now().AddDate(0, 0, offsetDays(s, g.When))
	return fmt.Sprintf("%sT%02d:%02d:00", d.Format("2006-01-02"), s.Long(7, 18), s.Long(0, 59))
}

func (TimeOfDay) IsString() bool      { return true }
func (TimeOfDay) Placeholder() string { return "" }
func (TimeOfDay) Fixed(s *fake.Source) string {
	return fmt.Sprintf("%02d:%02d:00", s.Long(7, 18), s.Long(0, 59))
}

func (Boolean) IsString() bool              { return false }
func (Boolean) Placeholder() string         { return "" }
func (Boolean) Fixed(s *fake.Source) string { return strconv.FormatBool(s.Bool()) }

func (g OneOf) IsString() bool              { return g.Str }
func (OneOf) Placeholder() string           { return "" }
func (g OneOf) Fixed(s *fake.Source) string { return s.Pick(g.Values) }

func (Pattern) IsString() bool                { return true }
func (Pattern) Placeholder() string           { return "" }
func (g Pattern) Fixed(s *fake.Source) string { return s.Regex(g.Regex) }

func (NullValue) IsString() bool            { return false }
func (NullValue) Placeholder() string       { return "" }
func (NullValue) Fixed(*fake.Source) string { return "null" }

func (Base64) IsString() bool      { return true }
func (Base64) Placeholder() string { return "" }
func (Base64) Fixed(s *fake.Source) string {
	return base64.StdEncoding.EncodeToString([]byte(s.Letters(12, true)))
}

// Mode is how values are written.
type Mode int

const (
	// Fixed writes fake values chosen now: valid JSON that pastes anywhere.
	Fixed Mode = iota
	// Placeholders writes {{$random...}} where the HTTP Client has a generator, fixed values elsewhere.
	Placeholders
)

// Render writes a node as JSON with two-space indentation.
func Render(n Node, mode Mode, s *fake.Source) string {
	var b strings.Builder
	write(&b, n, 0, mode, s)
	return b.String()
}

// RenderPlain writes one value for a URL, form body or header: placeholder or raw value, never quoted.
func RenderPlain(n Node, mode Mode, s *fake.Source) string {
	v, ok := n.(Value)
	if !ok {
		lines := strings.Split(Render(n, mode, s), "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		return strings.Join(lines, "")
	}
	if p := v.Gen.Placeholder(); mode == Placeholders && p != "" {
		return "{{" + p + "}}"
	}
	return v.Gen.Fixed(s)
}

func write(b *strings.Builder, n Node, depth int, mode Mode, s *fake.Source) {
	switch v := n.(type) {
	case Object:
		writeObject(b, v.Fields, depth, mode, s)
	case Map:
		writeObject(b, []Field{{Key: "key", Value: v.Value}}, depth, mode, s)
	case Array:
		b.WriteString("[\n" + indent(depth+1))
		write(b, v.Elem, depth+1, mode, s)
		b.WriteString("\n" + indent(depth) + "]")
	case Value:
		b.WriteString(value(v.Gen, mode, s))
	case Cut:
		b.WriteString("{}")
	case Null:
		b.WriteString("null")
	case EmptyArray:
		b.WriteString("[]")
	}
}

func writeObject(b *strings.Builder, fields []Field, depth int, mode Mode, s *fake.Source) {
	if len(fields) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	for i, f := range fields {
		b.WriteString(indent(depth+1) + Quote(f.Key) + ": ")
		write(b, f.Value, depth+1, mode, s)
		if i < len(fields)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(indent(depth) + "}")
}

func value(g Generator, mode Mode, s *fake.Source) string {
	if p := g.Placeholder(); mode == Placeholders && p != "" {
		if g.IsString() {
			return `"{{` + p + `}}"`
		}
		return "{{" + p + "}}"
	}
	raw := g.Fixed(s)
	if g.IsString() {
		return Quote(raw)
	}
	return raw
}

func indent(depth int) string { return strings.Repeat("  ", depth) }

// Quote writes a JSON string literal, escaping quotes, backslashes and control characters.
func Quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
