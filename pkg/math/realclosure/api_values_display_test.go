package realclosure

import (
	"strings"
	"testing"
)

// AUDIT math_realclosure_02-F2: display writes the whole numerator
// polynomial over the extension, parenthesizes coefficients as
// use_parenthesis does, and lists the referenced extensions in compact form.
func TestAPIValuesDisplayAlgebraicPolynomials(t *testing.T) {
	f := newAPIValuesFixture(t)
	defer f.close()
	coefficients := []*APIValue{f.rational("6"), f.rational("0"), f.rational("-5"), f.rational("0"), f.rational("1")}
	roots, err := f.v.IsolateRoots(coefficients)
	if err != nil || len(roots) != 4 {
		t.Fatal(len(roots), err)
	}
	f.values = append(f.values, roots...)
	a := roots[2]
	// The isolating interval's brackets are a separate item
	// (math_realclosure_01-F9), so take the definition from a itself.
	definition := f.display(a, false)
	if !strings.HasPrefix(definition, "root(x^4 + -5*x^2 + 6, (39/2^5, 109/2^6") || !strings.HasSuffix(definition, ", {})") {
		t.Fatalf("a = %q", definition)
	}
	if got, want := f.display(a, true), "[r!2; r!2 := "+definition+"]"; got != want {
		t.Fatalf("compact a = %q, want %q", got, want)
	}
	twoA := f.keep(f.v.Mul(f.rational("2"), a))
	if got, want := f.display(twoA, false), "2*"+definition; got != want {
		t.Fatalf("2*a = %q, want %q", got, want)
	}
	if got, want := f.display(twoA, true), "[2*r!2; r!2 := "+definition+"]"; got != want {
		t.Fatalf("compact 2*a = %q, want %q", got, want)
	}
	minusA := f.keep(f.v.Neg(a))
	if got, want := f.display(minusA, false), "-1*"+definition; got != want {
		t.Fatalf("-a = %q, want %q", got, want)
	}
	square := f.keep(f.v.Sub(f.keep(f.v.Mul(a, a)), f.rational("3")))
	if got, want := f.display(square, false), definition+"^2 + -3"; got != want {
		t.Fatalf("a*a-3 = %q, want %q", got, want)
	}
	if got, want := f.display(square, true), "[r!2^2 + -3; r!2 := "+definition+"]"; got != want {
		t.Fatalf("compact a*a-3 = %q, want %q", got, want)
	}
	pi := f.keep(f.v.Pi())
	if got, want := f.display(f.keep(f.v.Add(a, pi)), true), "[r!2 + pi; r!2 := "+definition+"]"; got != want {
		t.Fatalf("compact a+pi = %q, want %q", got, want)
	}
	// use_parenthesis also holds for a coefficient whose denominator is not one.
	one := f.rational("1")
	quotient := f.keep(f.v.Div(f.keep(f.v.Add(a, one)), f.keep(f.v.Add(pi, one))))
	if got, want := f.display(quotient, true), "[(1/(pi + 1))*r!2 + 1/(pi + 1); r!2 := "+definition+"]"; got != want {
		t.Fatalf("compact (a+1)/(pi+1) = %q, want %q", got, want)
	}
	var pretty strings.Builder
	if err := f.v.Display(&pretty, twoA, true, true); err != nil {
		t.Fatal(err)
	}
	if got, want := pretty.String(), "[2 &alpha;<sub>2</sub>; &alpha;<sub>2</sub> := root(x<sup>4</sup> + -5 x<sup>2</sup> + 6, "; !strings.HasPrefix(got, want) {
		t.Fatalf("pretty compact 2*a = %q, want prefix %q", got, want)
	}
}
