// Package fake produces the fixed fake values of httpgen's --mode fixed, using gofakeit.
//
// rules.json names generators the way the IntelliJ HTTP Client's $random does (Java Faker names,
// such as name.firstName). Each one maps to a gofakeit call here; the few Java Faker has but
// gofakeit does not (industry, IBAN, BIC, ISBN, university, course, image URL, slug) are built
// from gofakeit primitives so the values keep the same shape.
package fake

import (
	"fmt"
	"math"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
)

// Source generates values; a seed makes them repeatable.
type Source struct {
	f *gofakeit.Faker
}

// New returns a Source; seed 0 means random.
func New(seed uint64) *Source {
	return &Source{f: gofakeit.New(seed)}
}

var industries = []string{"Agriculture", "Banking", "Construction", "Education", "Energy", "Healthcare",
	"Insurance", "Logistics", "Manufacturing", "Mining", "Retail", "Telecommunications", "Tourism"}

var courses = []string{"Bachelor of Computer Science", "Bachelor of Accounting", "Bachelor of Laws",
	"Master of Business Administration", "Diploma in Nursing", "Bachelor of Civil Engineering",
	"Master of Public Health", "Bachelor of Education"}

var generators = map[string]func(f *gofakeit.Faker) string{
	"name.firstName":        func(f *gofakeit.Faker) string { return f.FirstName() },
	"name.lastName":         func(f *gofakeit.Faker) string { return f.LastName() },
	"name.fullName":         func(f *gofakeit.Faker) string { return f.Name() },
	"name.username":         func(f *gofakeit.Faker) string { return f.Username() },
	"name.title":            func(f *gofakeit.Faker) string { return f.JobDescriptor() + " " + f.JobLevel() + " " + f.JobTitle() },
	"internet.emailAddress": func(f *gofakeit.Faker) string { return f.Email() },
	"internet.url":          func(f *gofakeit.Faker) string { return f.URL() },
	"internet.domainName":   func(f *gofakeit.Faker) string { return f.DomainName() },
	"internet.password":     func(f *gofakeit.Faker) string { return f.Password(true, true, true, false, false, 14) },
	"internet.ipV4Address":  func(f *gofakeit.Faker) string { return f.IPv4Address() },
	"internet.image":        func(f *gofakeit.Faker) string { return "https://picsum.photos/seed/" + f.Word() + "/640/480" },
	"internet.slug":         func(f *gofakeit.Faker) string { return f.Word() + "-" + f.Word() },
	"phoneNumber.cellPhone": func(f *gofakeit.Faker) string { return f.PhoneFormatted() },
	"address.city":          func(f *gofakeit.Faker) string { return f.City() },
	"address.streetAddress": func(f *gofakeit.Faker) string { return f.Street() },
	"address.fullAddress": func(f *gofakeit.Faker) string {
		return f.Street() + ", " + f.City() + ", " + f.State() + " " + f.Zip()
	},
	"address.zipCode":      func(f *gofakeit.Faker) string { return f.Zip() },
	"address.country":      func(f *gofakeit.Faker) string { return f.Country() },
	"address.countryCode":  func(f *gofakeit.Faker) string { return f.CountryAbr() },
	"address.state":        func(f *gofakeit.Faker) string { return f.State() },
	"address.latitude":     func(f *gofakeit.Faker) string { return fmt.Sprintf("%.6f", f.Latitude()) },
	"address.longitude":    func(f *gofakeit.Faker) string { return fmt.Sprintf("%.6f", f.Longitude()) },
	"company.name":         func(f *gofakeit.Faker) string { return f.Company() },
	"company.industry":     func(f *gofakeit.Faker) string { return industries[f.IntN(len(industries))] },
	"company.profession":   func(f *gofakeit.Faker) string { return strings.ToLower(f.JobTitle()) },
	"commerce.department":  func(f *gofakeit.Faker) string { return capitalize(f.ProductCategory()) },
	"commerce.productName": func(f *gofakeit.Faker) string { return f.ProductName() },
	"commerce.price":       func(f *gofakeit.Faker) string { return fmt.Sprintf("%.2f", f.Price(1, 1000)) },
	"lorem.word":           func(f *gofakeit.Faker) string { return f.Word() },
	"lorem.sentence":       func(f *gofakeit.Faker) string { return f.Sentence() },
	"lorem.paragraph":      func(f *gofakeit.Faker) string { return f.Paragraph() },
	"color.name":           func(f *gofakeit.Faker) string { return f.Color() },
	"university.name":      func(f *gofakeit.Faker) string { return "University of " + f.City() },
	"educator.course":      func(f *gofakeit.Faker) string { return courses[f.IntN(len(courses))] },
	"finance.iban": func(f *gofakeit.Faker) string {
		return strings.ToUpper(f.CountryAbr()) + f.DigitN(2) + strings.ToUpper(f.LetterN(4)) + f.DigitN(14)
	},
	"finance.bic":    func(f *gofakeit.Faker) string { return strings.ToUpper(f.LetterN(6) + f.LetterN(2)) },
	"code.isbn13":    func(f *gofakeit.Faker) string { return "978" + f.DigitN(10) },
	"idNumber.valid": func(f *gofakeit.Faker) string { return f.SSN() },
}

// Faker returns a value for a `provider.method` generator name.
func (s *Source) Faker(name string) (string, error) {
	g, ok := generators[name]
	if !ok {
		return "", fmt.Errorf("no fixed generator for %q", name)
	}
	return g(s.f), nil
}

// Supports reports whether a generator name has a fixed equivalent.
func Supports(name string) bool {
	_, ok := generators[name]
	return ok
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Long returns a whole number from min to max, both inclusive.
func (s *Source) Long(min, max int64) int64 {
	if max <= min {
		return min
	}
	span := max - min + 1
	if span <= 0 || span > math.MaxInt32 {
		return min + int64(s.f.Float64Range(0, float64(max-min)))
	}
	return min + int64(s.f.IntN(int(span)))
}

// Double returns a number from min to max.
func (s *Source) Double(min, max float64) float64 {
	if max <= min {
		return min
	}
	return s.f.Float64Range(min, max)
}

// Letters returns n lower-case letters, or letters and digits.
func (s *Source) Letters(n int, withDigits bool) string {
	pool := "abcdefghijklmnopqrstuvwxyz"
	if withDigits {
		pool += "0123456789"
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = pool[s.f.IntN(len(pool))]
	}
	return string(b)
}

// Regex returns a string matching a regular expression.
func (s *Source) Regex(pattern string) string { return s.f.Regex(pattern) }

// Bool returns true or false.
func (s *Source) Bool() bool { return s.f.Bool() }

// UUID returns a random UUID.
func (s *Source) UUID() string { return s.f.UUID() }

// Pick returns one of the values.
func (s *Source) Pick(values []string) string { return values[s.f.IntN(len(values))] }
