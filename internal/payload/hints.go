package payload

import (
	"regexp"
	"strings"

	"github.com/sirbuni-dev-tools/httpgen/internal/rules"
)

// Kind is a Java type reduced to what decides its fake value; the names match rules.json.
type Kind string

const (
	KString     Kind = "STRING"
	KChar       Kind = "CHAR"
	KInteger    Kind = "INTEGER"
	KLong       Kind = "LONG"
	KByte       Kind = "BYTE"
	KDecimal    Kind = "DECIMAL"
	KBigInteger Kind = "BIG_INTEGER"
	KBoolean    Kind = "BOOLEAN"
	KUUID       Kind = "UUID"
	KDate       Kind = "LOCAL_DATE"
	KDateTime   Kind = "LOCAL_DATE_TIME"
	KTime       Kind = "LOCAL_TIME"
	KInstant    Kind = "INSTANT"
	KEnum       Kind = "ENUM"
	KBytes      Kind = "BYTES"
	KOther      Kind = "OTHER"
)

func (k Kind) wholeNumber() bool {
	return k == KInteger || k == KLong || k == KByte || k == KBigInteger
}

// Constraints are the validation constraints found on a field; nil pointers mean absent.
type Constraints struct {
	Email   bool
	MinSize *int
	MaxSize *int
	Min     *float64
	Max     *float64
	Pattern string
	When    Timing
}

// FieldContext is everything that decides a leaf's value.
type FieldContext struct {
	Name          string
	Kind          Kind
	Owner         string // simple name of the declaring class
	EnumConstants []string
	Constraints   Constraints
}

// Hints picks generators from the shared rules: constraints first, then the field name, then the type.
type Hints struct {
	r      *rules.Rules
	byName map[string][]rules.Hint
}

// NewHints indexes the rules by name.
func NewHints(r *rules.Rules) *Hints {
	h := &Hints{r: r, byName: map[string][]rules.Hint{}}
	for _, hint := range r.Hints {
		for _, n := range hint.Names {
			h.byName[n] = append(h.byName[n], hint)
		}
	}
	return h
}

// Rules returns the rule tables.
func (h *Hints) Rules() *rules.Rules { return h.r }

// Resolve picks the generator for one field.
func (h *Hints) Resolve(ctx FieldContext) Generator {
	if g := h.fromConstraints(ctx); g != nil {
		return g
	}
	key := strings.ReplaceAll(strings.ToLower(ctx.Name), "_", "")
	if hint, ok := h.match(key, ctx.Kind); ok {
		return h.generator(hint.Spec, ctx.Kind, ctx.Owner)
	}
	if hint, ok := h.suffix(key, ctx); ok {
		return h.generator(hint.Spec, ctx.Kind, ctx.Owner)
	}
	return h.fromType(ctx)
}

func (h *Hints) match(name string, kind Kind) (rules.Hint, bool) {
	for _, hint := range h.byName[name] {
		for _, k := range h.r.KindsOf(hint) {
			if Kind(k) == kind {
				return hint, true
			}
		}
	}
	return rules.Hint{}, false
}

var camelSplit = regexp.MustCompile(`([a-z0-9])([A-Z])|_+`)

// suffix tries the last camel-case words when the whole name matched nothing: createdById, customerEmail.
func (h *Hints) suffix(key string, ctx FieldContext) (rules.Hint, bool) {
	spaced := camelSplit.ReplaceAllString(ctx.Name, "$1 $2")
	var words []string
	for _, w := range strings.Fields(spaced) {
		words = append(words, strings.ToLower(w))
	}
	for start := 1; start < len(words); start++ {
		tail := strings.Join(words[start:], "")
		if tail == key {
			continue
		}
		if hint, ok := h.match(tail, ctx.Kind); ok {
			return hint, true
		}
	}
	return rules.Hint{}, false
}

func (h *Hints) fromConstraints(ctx FieldContext) Generator {
	c := ctx.Constraints
	switch {
	case ctx.Kind == KString && c.Pattern != "":
		return Pattern{Regex: c.Pattern}
	case ctx.Kind == KString && c.Email:
		return Faker{Name: "internet.emailAddress", Str: true}
	case ctx.Kind == KString && (c.MinSize != nil || c.MaxSize != nil) && !strings.Contains(strings.ToLower(ctx.Name), "password"):
		return h.sizedText(ctx)
	case ctx.Kind.wholeNumber() && (c.Min != nil || c.Max != nil):
		min := int64(0)
		if c.Min != nil {
			min = int64(*c.Min)
		}
		max := max64(min+1000, 1000)
		if c.Max != nil {
			max = int64(*c.Max)
		}
		return IntRange{Min: min, Max: max}
	case ctx.Kind == KDecimal && (c.Min != nil || c.Max != nil):
		min := 0.0
		if c.Min != nil {
			min = *c.Min
		}
		max := maxF(min+1000, 1000)
		if c.Max != nil {
			max = *c.Max
		}
		return DecimalRange{Min: min, Max: max}
	case ctx.Kind == KDate && c.When != AnyTime:
		return Date{When: c.When}
	case ctx.Kind == KDateTime && c.When != AnyTime:
		return DateTime{When: c.When}
	}
	return nil
}

// sizedText keeps a fitting name hint for @Size strings, otherwise letters of a fitting length.
func (h *Hints) sizedText(ctx FieldContext) Generator {
	c := ctx.Constraints
	min := 0
	if c.MinSize != nil {
		min = *c.MinSize
	}
	max := maxInt(min, 20)
	if c.MaxSize != nil {
		max = *c.MaxSize
	}
	if max >= 30 && min <= 3 {
		plain := ctx
		plain.Constraints = Constraints{}
		if f, ok := h.Resolve(plain).(Faker); ok {
			return f
		}
	}
	n := (min + max) / 2
	n = maxInt(n, maxInt(min, 1))
	n = minInt(n, maxInt(max, 1))
	return Letters{Length: n}
}

func (h *Hints) fromType(ctx FieldContext) Generator {
	if ctx.Kind == KEnum {
		if len(ctx.EnumConstants) == 0 {
			return OneOf{Values: []string{"VALUE"}, Str: true}
		}
		return OneOf{Values: ctx.EnumConstants, Str: true}
	}
	return h.generator(h.r.Types[string(ctx.Kind)], ctx.Kind, "")
}

// generator turns a rules spec into a generator for a field of the given kind.
func (h *Hints) generator(s rules.Spec, kind Kind, owner string) Generator {
	switch {
	case s.Faker != "":
		return Faker{Name: s.Faker, Str: kind == KString}
	case s.ByOwner:
		return h.nameFor(owner)
	case s.Password:
		return OneOf{Values: []string{h.r.Password}, Str: true}
	case len(s.Range) == 2:
		if kind == KDecimal {
			return DecimalRange{Min: s.Range[0], Max: s.Range[1]}
		}
		return IntRange{Min: int64(s.Range[0]), Max: int64(s.Range[1])}
	case len(s.OneOf) > 0:
		return OneOf{Values: s.OneOf, Str: true}
	case s.Letters > 0:
		return Letters{Length: s.Letters, Digits: s.Digits}
	case s.UUID:
		return UUID{}
	case s.Null:
		return NullValue{}
	case s.Boolean:
		return Boolean{}
	case s.Date:
		return Date{}
	case s.DateTime:
		return DateTime{}
	case s.Time:
		return TimeOfDay{}
	case s.IsoTimestamp:
		return IsoTimestamp{}
	case s.Base64:
		return Base64{}
	}
	return NullValue{}
}

func (h *Hints) nameFor(owner string) Generator {
	o := strings.ToLower(owner)
	for _, rule := range h.r.NameByOwner {
		if len(rule.Contains) == 0 {
			return h.generator(rule.Spec, KString, "")
		}
		for _, w := range rule.Contains {
			if strings.Contains(o, w) {
				return h.generator(rule.Spec, KString, "")
			}
		}
	}
	return Faker{Name: "lorem.word", Str: true}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
