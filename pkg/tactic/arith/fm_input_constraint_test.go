package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMInputParseBuildsAndRegistersConstraint(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, false, "fm_constraint_x")
	bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "7/2"))
	c, registration, matched, e := p.Parse(bound, ast.ASTDependency{})
	equationMust(t, e)
	if !matched || registration != FMRegistrationIndexed || c == nil || len(c.Variables) != 1 || c.Constant.RatString() != "7" || c.Coefficients[0].RatString() != "2" {
		t.Fatalf("parsed constraint = %#v registration=%v matched=%v", c, registration, matched)
	}
}

func TestFMInputParseLeavesUnrecognizedFormulaUntouched(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	formula := fmBoolConst(t, m, "fm_unrecognized")
	c, _, matched, e := p.Parse(formula, ast.ASTDependency{})
	equationMust(t, e)
	if matched || c != nil || len(p.Variables.Integer) != 0 {
		t.Fatal("unrecognized input mutated parser state")
	}
}
