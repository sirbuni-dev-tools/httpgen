// Package rules loads the fake-value tables shared with the API HTTP Generator plugin.
//
// rules.json is a copy of the plugin's src/main/resources/rules.json (refresh it with
// `make sync-rules`). The decisions that use these tables live in the walker, as in the plugin.
package rules

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed rules.json
var Raw []byte

// Spec says how one value is generated. Exactly one generator field is set.
type Spec struct {
	Faker        string    `json:"faker,omitempty"`
	ByOwner      bool      `json:"byOwner,omitempty"`
	Password     bool      `json:"password,omitempty"`
	Range        []float64 `json:"range,omitempty"`
	OneOf        []string  `json:"oneOf,omitempty"`
	Letters      int       `json:"letters,omitempty"`
	Digits       bool      `json:"digits,omitempty"`
	UUID         bool      `json:"uuid,omitempty"`
	Null         bool      `json:"null,omitempty"`
	Boolean      bool      `json:"boolean,omitempty"`
	Date         bool      `json:"date,omitempty"`
	DateTime     bool      `json:"dateTime,omitempty"`
	Time         bool      `json:"time,omitempty"`
	IsoTimestamp bool      `json:"isoTimestamp,omitempty"`
	Base64       bool      `json:"base64,omitempty"`
}

// Hint matches field names (lower case, underscores removed) for the value kinds of its group.
type Hint struct {
	Spec
	Kinds string   `json:"kinds"`
	Names []string `json:"names"`
}

// OwnerRule picks the value of a plain `name` field from the declaring class; an empty Contains always matches.
type OwnerRule struct {
	Spec
	Contains []string `json:"contains"`
}

// Rules is the whole table.
type Rules struct {
	Version       int                 `json:"version"`
	Password      string              `json:"password"`
	KindGroups    map[string][]string `json:"kindGroups"`
	Hints         []Hint              `json:"hints"`
	NameByOwner   []OwnerRule         `json:"nameByOwner"`
	Types         map[string]Spec     `json:"types"`
	OwnIDs        []string            `json:"ownIds"`
	CriteriaLists struct {
		Contains []string `json:"contains"`
		Exact    []string `json:"exact"`
	} `json:"criteriaLists"`
	VerifiedPlaceholders []string `json:"verifiedPlaceholders"`
}

// Kinds the walker reduces Java types to; the same names as the plugin's ValueKind.
var Kinds = []string{
	"STRING", "CHAR", "INTEGER", "LONG", "BYTE", "DECIMAL", "BIG_INTEGER", "BOOLEAN",
	"UUID", "LOCAL_DATE", "LOCAL_DATE_TIME", "LOCAL_TIME", "INSTANT", "ENUM", "BYTES", "OTHER",
}

// Load parses and validates the embedded rules.
func Load() (*Rules, error) {
	return Parse(Raw)
}

// Parse reads rules from JSON and checks them.
func Parse(data []byte) (*Rules, error) {
	var r Rules
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("rules.json: %w", err)
	}
	if err := r.validate(); err != nil {
		return nil, fmt.Errorf("rules.json: %w", err)
	}
	return &r, nil
}

// KindsOf returns the kinds a hint applies to.
func (r *Rules) KindsOf(h Hint) []string { return r.KindGroups[h.Kinds] }

// FakerNames lists every `provider.method` the rules use, for mapping checks.
func (r *Rules) FakerNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(s Spec) {
		if s.Faker != "" && !seen[s.Faker] {
			seen[s.Faker] = true
			out = append(out, s.Faker)
		}
	}
	for _, h := range r.Hints {
		add(h.Spec)
	}
	for _, o := range r.NameByOwner {
		add(o.Spec)
	}
	for _, s := range r.Types {
		add(s)
	}
	return out
}

func (r *Rules) validate() error {
	if r.Version != 1 {
		return fmt.Errorf("unsupported version %d", r.Version)
	}
	if r.Password == "" {
		return fmt.Errorf("password is empty")
	}
	known := map[string]bool{}
	for _, k := range Kinds {
		known[k] = true
	}
	for group, kinds := range r.KindGroups {
		for _, k := range kinds {
			if !known[k] {
				return fmt.Errorf("kind group %q: unknown kind %q", group, k)
			}
		}
	}
	for i, h := range r.Hints {
		if _, ok := r.KindGroups[h.Kinds]; !ok {
			return fmt.Errorf("hint %d (%s): unknown kind group %q", i, strings.Join(h.Names, ","), h.Kinds)
		}
		if len(h.Names) == 0 {
			return fmt.Errorf("hint %d has no names", i)
		}
		if err := h.Spec.check(true); err != nil {
			return fmt.Errorf("hint %d (%s): %w", i, h.Names[0], err)
		}
	}
	if len(r.NameByOwner) == 0 || len(r.NameByOwner[len(r.NameByOwner)-1].Contains) != 0 {
		return fmt.Errorf("nameByOwner must end with an entry whose contains is empty")
	}
	for i, o := range r.NameByOwner {
		if err := o.Spec.check(false); err != nil {
			return fmt.Errorf("nameByOwner %d: %w", i, err)
		}
	}
	for _, k := range Kinds {
		if k == "ENUM" {
			continue // enums take their own constants
		}
		s, ok := r.Types[k]
		if !ok {
			return fmt.Errorf("types: no default for %s", k)
		}
		if err := s.check(false); err != nil {
			return fmt.Errorf("types %s: %w", k, err)
		}
	}
	return nil
}

// check makes sure exactly one generator is set (byOwner and password only make sense in hints).
func (s Spec) check(inHint bool) error {
	set := 0
	for _, on := range []bool{
		s.Faker != "", s.ByOwner && inHint, s.Password && inHint, len(s.Range) > 0, len(s.OneOf) > 0, s.Letters > 0,
		s.UUID, s.Null, s.Boolean, s.Date, s.DateTime, s.Time, s.IsoTimestamp, s.Base64,
	} {
		if on {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("needs exactly one generator, found %d", set)
	}
	if len(s.Range) > 0 && (len(s.Range) != 2 || s.Range[0] > s.Range[1]) {
		return fmt.Errorf("range must be [min, max] with min <= max")
	}
	if s.Faker != "" && !strings.Contains(s.Faker, ".") {
		return fmt.Errorf("faker %q must be provider.method", s.Faker)
	}
	return nil
}
