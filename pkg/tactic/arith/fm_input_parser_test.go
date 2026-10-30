package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func fmParserFixture(t *testing.T, realOnly, clauses bool) (*FMInputParser, *ast.ASTManager) {
	t.Helper()
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	p, e := NewFMInputParser(m, store, realOnly, clauses)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, p.Close()) })
	return p, m
}

func TestFMInputParserInternsToRealBaseOnce(t *testing.T) {
	p, m := fmParserFixture(t, false, false)
	x := fixDLConst(t, m, true, "fm_parse_int")
	toReal := fixDLApp(t, m, 14, x)
	a, e := p.internVariable(x)
	equationMust(t, e)
	b, e := p.internVariable(toReal)
	equationMust(t, e)
	if a != b || len(p.Variables.Integer) != 1 || !p.Variables.IsInteger(a) {
		t.Fatal("to_real did not share its integer base variable")
	}
}

func TestFMInputParserRealOnlyForbidsIntegers(t *testing.T) {
	p, m := fmParserFixture(t, true, false)
	x := fixDLConst(t, m, true, "fm_real_only_int")
	v, e := p.internVariable(x)
	equationMust(t, e)
	if !p.Variables.IsForbidden(v) {
		t.Fatal("real-only parser retained integer candidate")
	}
}
