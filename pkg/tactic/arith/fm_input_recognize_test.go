package arith

import "testing"

func TestFMInputInequalityNormalizesGEAndNegation(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, false, "fm_ineq_x")
	bound := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "5"))
	xs, coefficients, constant, strict, matched, e := p.inequality(bound, true)
	equationMust(t, e)
	if !matched || strict || len(xs) != 1 || coefficients[0].RatString() != "-1" || constant.RatString() != "-5" {
		t.Fatalf("GE normalized to %v %v <= %s strict=%v", xs, coefficients, constant.RatString(), strict)
	}
	negated, e := m.MkNot(bound)
	equationMust(t, e)
	_, coefficients, constant, strict, matched, e = p.inequality(negated, true)
	equationMust(t, e)
	if !matched || !strict || coefficients[0].RatString() != "1" || constant.RatString() != "5" {
		t.Fatalf("not-GE normalized to %v <= %s strict=%v", coefficients, constant.RatString(), strict)
	}
}

func TestFMInputInequalityRejectsNonNumeralRight(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x, y := fixDLConst(t, m, false, "fm_rhs_x"), fixDLConst(t, m, false, "fm_rhs_y")
	bound := fixDLApp(t, m, 2, x, y)
	_, _, _, _, matched, e := p.inequality(bound, false)
	equationMust(t, e)
	if matched {
		t.Fatal("nonnumeral right side was recognized")
	}
}
