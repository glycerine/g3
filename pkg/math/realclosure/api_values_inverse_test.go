package realclosure

import (
	"strings"
	"testing"
)

func (f *apiValuesFixture) roots(coefficients ...string) []*APIValue {
	f.t.Helper()
	p := make([]*APIValue, len(coefficients))
	for i, text := range coefficients {
		p[i] = f.rational(text)
	}
	roots, err := f.v.IsolateRoots(p)
	if err != nil {
		f.t.Fatal(err)
	}
	f.values = append(f.values, roots...)
	return roots
}

// AUDIT math_realclosure_02-F3: when q and the reducible defining polynomial
// share a factor, inv_algebraic refines alpha with p/g and retries.
func TestAPIValuesInverseRefinesReducibleExtension(t *testing.T) {
	f := newAPIValuesFixture(t)
	defer f.close()
	roots := f.roots("6", "0", "-5", "0", "1")
	if len(roots) != 4 {
		t.Fatal(len(roots))
	}
	a := roots[2]
	q := f.keep(f.v.Sub(f.keep(f.v.Mul(a, a)), f.rational("3")))
	result := f.keep(f.v.Div(f.rational("1"), q))
	comparison, err := f.v.Compare(result, f.rational("-1"))
	if err != nil || comparison != 0 {
		t.Fatalf("compare(1/(a^2-3), -1) = %d, %v", comparison, err)
	}
	if got := f.display(result, true); got != "-1" {
		t.Fatalf("1/(a^2-3) = %q, want -1", got)
	}
	if got := f.display(a, true); !strings.HasPrefix(got, "[r!2; r!2 := root(x^2 + -2, (39/2^5, 109/2^6") {
		t.Fatalf("a after the inverse = %q", got)
	}
}

// AUDIT math_realclosure_01b-F1: the Q/R recurrence of inv_algebraic fails
// as soon as some Q divides p, even when gcd(q, p) = 1, and the caller then
// refines alpha. For p = (x-1)(x^2-2) and q = r^2 + 1, Q_1 = 3(x-1).
func TestAPIValuesInverseRecurrenceRefinesCoprimeReducibleExtension(t *testing.T) {
	f := newAPIValuesFixture(t)
	defer f.close()
	roots := f.roots("2", "-2", "-1", "1")
	if len(roots) != 3 {
		t.Fatal(len(roots))
	}
	r := roots[2]
	q := f.keep(f.v.Add(f.keep(f.v.Mul(r, r)), f.rational("1")))
	inverse := f.keep(f.v.Inv(q))
	if inverse.value == nil || !inverse.value.isRational {
		t.Fatalf("1/q is not rational: %q", f.display(inverse, true))
	}
	if got := f.display(inverse, true); got != "1/3" {
		t.Fatalf("1/q = %q, want 1/3", got)
	}
	if got := f.display(r, true); !strings.HasPrefix(got, "[r!2; r!2 := root(x^2 + -2, (19/2^4, 17/2^3") {
		t.Fatalf("r after the inverse = %q", got)
	}
	// r is now defined by x^2 - 2: 1/(r+1) = r - 1.
	if got := f.display(f.keep(f.v.Inv(f.keep(f.v.Add(r, f.rational("1"))))), true); !strings.HasPrefix(got, "[r!2 + -1; r!2 := root(x^2 + -2, ") {
		t.Fatalf("1/(r+1) = %q", got)
	}
}
