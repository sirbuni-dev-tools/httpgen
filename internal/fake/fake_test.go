package fake

import (
	"testing"

	"github.com/sirbuni-dev-tools/httpgen/internal/rules"
)

// Every Faker generator in rules.json has a fixed equivalent that produces a value.
func TestEveryRuleFakerHasAFixedGenerator(t *testing.T) {
	r, err := rules.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := New(1)
	for _, name := range r.FakerNames() {
		v, err := s.Faker(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if v == "" {
			t.Errorf("%s produced an empty value", name)
		}
	}
}

func TestSeedMakesValuesRepeatable(t *testing.T) {
	a, _ := New(42).Faker("name.fullName")
	b, _ := New(42).Faker("name.fullName")
	if a != b {
		t.Errorf("same seed gave %q and %q", a, b)
	}
}

func TestUnknownGenerator(t *testing.T) {
	if _, err := New(1).Faker("job.title"); err == nil {
		t.Error("expected an error for a generator rules.json does not use")
	}
}
