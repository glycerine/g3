package api

import "testing"

func TestAPIRCFRationalArithmetic(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a, b := f.rational("3/2"), f.rational("2/3")
	results := map[string]*RCFNumber{
		"13/6": f.keep(f.context.RCFAdd(a, b)),
		"5/6":  f.keep(f.context.RCFSub(a, b)),
		"1":    f.keep(f.context.RCFMul(a, b)),
		"9/4":  f.keep(f.context.RCFDiv(a, b)),
	}
	for want, value := range results {
		if got := f.text(value); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestAPIRCFRationalUnary(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	a := f.rational("-2/3")
	if got := f.text(f.keep(f.context.RCFNeg(a))); got != "2/3" {
		t.Fatal(got)
	}
	if got := f.text(f.keep(f.context.RCFInv(a))); got != "-3/2" {
		t.Fatal(got)
	}
	if got := f.text(f.keep(f.context.RCFPower(a, 3))); got != "-8/27" {
		t.Fatal(got)
	}
}

// AUDIT math_realclosure_01-F8: refining an algebraic value uses
// interval(p[i]) for coefficients of every kind (polynomial_interval).
func TestAPIRCFAlgebraicTimesPiDecimal(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	roots, err := f.context.RCFRoots([]*RCFNumber{f.rational("-2"), f.rational("0"), f.rational("1")})
	if err != nil || len(roots) != 2 {
		t.Fatal(len(roots), err)
	}
	for _, root := range roots {
		f.keep(root, nil)
	}
	r := roots[1]
	pi := f.keep(f.context.RCFPi())
	for _, c := range []struct {
		label string
		value *RCFNumber
		want  string
	}{
		{"r*pi", f.keep(f.context.RCFMul(r, pi)), "4.4428829381?"},
		{"r+pi", f.keep(f.context.RCFAdd(r, pi)), "4.5558062159?"},
	} {
		decimal, err := f.context.RCFDecimalString(c.value, 10)
		if err != nil || decimal != c.want {
			t.Fatalf("%s decimal = %q, %v; want %q", c.label, decimal, err, c.want)
		}
	}
}

// AUDIT math_realclosure_01b-F1 and 02-F3: inv_algebraic refines the
// reducible defining polynomial (x-1)(x^2-2) of r, so 1/(r^2+1) is 1/3.
func TestAPIRCFInverseRefinesExtension(t *testing.T) {
	f := newRCFFixture(t)
	defer f.close()
	roots, err := f.context.RCFRoots([]*RCFNumber{f.rational("2"), f.rational("-2"), f.rational("-1"), f.rational("1")})
	if err != nil || len(roots) != 3 {
		t.Fatal(len(roots), err)
	}
	for _, root := range roots {
		f.keep(root, nil)
	}
	r := roots[2]
	q := f.keep(f.context.RCFAdd(f.keep(f.context.RCFMul(r, r)), f.rational("1")))
	inverse := f.keep(f.context.RCFInv(q))
	if got := f.text(inverse); got != "1/3" {
		t.Fatalf("1/(r^2+1) = %q, want 1/3", got)
	}
	numerator, denominator, err := f.context.RCFNumeratorDenominator(inverse)
	f.keep(numerator, err)
	f.keep(denominator, err)
	if f.text(numerator) != "1" || f.text(denominator) != "3" {
		t.Fatal(f.text(numerator), f.text(denominator))
	}
}
