package rules

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedRulesLoad(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Hints) < 50 || r.Password == "" || len(r.OwnIDs) == 0 {
		t.Fatalf("unexpectedly small rules: %d hints", len(r.Hints))
	}
}

func TestInvalidRulesAreRejected(t *testing.T) {
	cases := map[string]string{
		`{"version":2}`: "unsupported version",
		`{"version":1,"password":"p","kindGroups":{"text":["STRING"]},"hints":[{"kinds":"text","names":["a"],"faker":"x.y","uuid":true}]}`: "exactly one generator",
		`{"version":1,"password":"p","kindGroups":{"text":["STRING"]},"hints":[{"kinds":"nope","names":["a"],"uuid":true}]}`:               "unknown kind group",
		`{"version":1,"password":"p","kindGroups":{"text":["STRING"]},"hints":[{"kinds":"text","names":["a"],"range":[5,1]}]}`:             "min <= max",
	}
	for data, want := range cases {
		if _, err := Parse([]byte(data)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want error containing %q, got %v", data, want, err)
		}
	}
}

// Every Faker generator a hint uses was checked to resolve in the IntelliJ HTTP Client's $random.
func TestHintFakersAreVerifiedPlaceholders(t *testing.T) {
	r, _ := Load()
	verified := map[string]bool{}
	for _, v := range r.VerifiedPlaceholders {
		verified[v] = true
	}
	for _, h := range r.Hints {
		if h.Faker != "" && !verified[h.Faker] {
			t.Errorf("hint %s uses %s, which is not in verifiedPlaceholders", h.Names[0], h.Faker)
		}
	}
}

// The embedded copy must equal the plugin's rules.json, its source of truth. Skipped where the plugin is not checked out.
func TestCopyMatchesPlugin(t *testing.T) {
	plugin := os.Getenv("HTTPGEN_PLUGIN_RULES")
	if plugin == "" {
		plugin = filepath.Join("..", "..", "..", "..", "INTELLIJ", "custom-plugins", "api-http-generator", "src", "main", "resources", "rules.json")
	}
	want, err := os.ReadFile(plugin)
	if err != nil {
		t.Skipf("plugin rules not found at %s", plugin)
	}
	if !bytes.Equal(want, Raw) {
		t.Fatal("internal/rules/rules.json differs from the plugin's; run make sync-rules")
	}
}
