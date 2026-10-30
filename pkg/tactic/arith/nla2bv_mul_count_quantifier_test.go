package arith

// Source: src/ast/for_each_expr.h:65-88 and
// src/tactic/arith/nla2bv_tactic.cpp:366-397.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestNLA2BVCountMulVisitsQuantifierBodyAndAnnotations(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	arithmetic, err := m.ArithmeticFamilyID()
	equationMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	equationMust(t, err)
	v, err := m.MkVar(0, integer)
	equationMust(t, err)
	one := fixDLNumber(t, m, true, "1")
	bodyMul, err := m.MkArithmeticMul(v, one)
	equationMust(t, err)
	body, err := m.MkEq(bodyMul, v)
	equationMust(t, err)
	annotationMul, err := m.MkArithmeticMul(v, v)
	equationMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{annotationMul})
	equationMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body, Patterns: []ast.ASTHandle{pattern}})
	equationMust(t, err)
	fixDLPin(t, m, q)
	g := fixDLGoal(t, m, false, false, false, q)
	got, err := (&nla2BVInvocation{t: r}).countMultiplications(g)
	equationMust(t, err)
	if got != 2 {
		t.Fatal("body/pattern annotation count", got)
	}
}
