package arith

import "testing"

func TestFMInputPolynomialExtractsOrderedMonomials(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x, y := fixDLConst(t, m, false, "fm_poly_x"), fixDLConst(t, m, false, "fm_poly_y")
	two := fixDLNumber(t, m, false, "2")
	polynomial := fixDLApp(t, m, 6, fixDLApp(t, m, 9, two, x), y)
	xs, coefficients, useful, e := p.polynomial(polynomial, true)
	equationMust(t, e)
	if !useful || len(xs) != 2 || coefficients[0].RatString() != "2" || coefficients[1].RatString() != "1" {
		t.Fatalf("polynomial = %v %v useful=%v", xs, coefficients, useful)
	}
}

func TestFMInputPolynomialRejectsDuplicateVariable(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, false, "fm_duplicate_x")
	polynomial := fixDLApp(t, m, 6, x, x)
	_, _, useful, e := p.polynomial(polynomial, false)
	equationMust(t, e)
	if useful {
		t.Fatal("duplicate variable polynomial was recognized")
	}
}
