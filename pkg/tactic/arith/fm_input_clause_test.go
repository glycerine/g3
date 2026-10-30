package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFMInputParseOCCClause(t *testing.T) {
	p, m := fmParserFixture(t, false, true)
	x := fixDLConst(t, m, false, "fm_clause_x")
	q := fmBoolConst(t, m, "fm_clause_literal")
	notQ, e := m.MkNot(q)
	equationMust(t, e)
	bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "3"))
	clause, e := m.MkOrArray([]ast.ASTHandle{notQ, bound})
	equationMust(t, e)
	c, registration, matched, e := p.Parse(clause, ast.ASTDependency{})
	equationMust(t, e)
	if !matched || registration != FMRegistrationIndexed || len(c.Literals) != 1 || c.Literals[0] != -1 {
		t.Fatalf("OCC clause = %#v registration=%v matched=%v", c, registration, matched)
	}
}

func TestFMInputParseRejectsTwoInequalitiesInClause(t *testing.T) {
	p, m := fmParserFixture(t, false, true)
	x := fixDLConst(t, m, false, "fm_two_x")
	a := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1"))
	b := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0"))
	clause, e := m.MkOr(a, b)
	equationMust(t, e)
	c, _, matched, e := p.Parse(clause, ast.ASTDependency{})
	equationMust(t, e)
	if matched || c != nil {
		t.Fatal("two-inequality clause was recognized")
	}
}
