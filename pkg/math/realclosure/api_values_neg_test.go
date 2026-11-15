package realclosure

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// apiValuesFixture drives the APIValues owner the way the Z3_rcf_* API does.
type apiValuesFixture struct {
	t      *testing.T
	v      *APIValues
	values []*APIValue
}

func newAPIValuesFixture(t *testing.T) *apiValuesFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 24)
	service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewAPIValues(service)
	if err != nil {
		t.Fatal(err)
	}
	return &apiValuesFixture{t: t, v: v}
}

func (f *apiValuesFixture) keep(value *APIValue, err error) *APIValue {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, value)
	return value
}

func (f *apiValuesFixture) rational(text string) *APIValue { return f.keep(f.v.Rational(text)) }

func (f *apiValuesFixture) display(value *APIValue, compact bool) string {
	f.t.Helper()
	var b strings.Builder
	if err := f.v.Display(&b, value, compact, false); err != nil {
		f.t.Fatal(err)
	}
	return b.String()
}

func (f *apiValuesFixture) close() {
	f.t.Helper()
	for i := len(f.values) - 1; i >= 0; i-- {
		if err := f.values[i].Close(); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := f.v.Close(); err != nil {
		f.t.Fatal(err)
	}
}

// AUDIT math_realclosure_02-F1: neg_rf negates the numerator and keeps the
// denominator of a transcendental or infinitesimal value.
func TestAPIValuesNegKeepsTranscendentalDenominator(t *testing.T) {
	for _, kind := range []string{"pi", "eps!1"} {
		t.Run(kind, func(t *testing.T) {
			f := newAPIValuesFixture(t)
			defer f.close()
			one := f.rational("1")
			var extension *APIValue
			if kind == "pi" {
				extension = f.keep(f.v.Pi())
			} else {
				extension = f.keep(f.v.Infinitesimal())
			}
			x := f.keep(f.v.Div(one, f.keep(f.v.Add(extension, one))))
			n := f.keep(f.v.Neg(x))
			if got, want := f.display(n, false), "(-1)/("+kind+" + 1)"; got != want {
				t.Fatalf("-(1/(%s+1)) = %q, want %q", kind, got, want)
			}
			comparison, err := f.v.Compare(n, f.rational("-1"))
			if err != nil || comparison != 1 {
				t.Fatalf("compare(-(1/(%s+1)), -1) = %d, %v; want 1", kind, comparison, err)
			}
			difference := f.keep(f.v.Sub(one, x))
			zero, err := f.v.IsZero(difference)
			if err != nil || zero {
				t.Fatalf("1 - 1/(%s+1) is zero = %v, %v", kind, zero, err)
			}
			if got, want := f.display(difference, false), "("+kind+")/("+kind+" + 1)"; got != want {
				t.Fatalf("1 - 1/(%s+1) = %q, want %q", kind, got, want)
			}
		})
	}
}
