package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQEMaxRelevantGroupingAndNestedDescent(t *testing.T) {
	f := newQENNFFixture(t)
	m := f.r.manager
	z := f.constant("irrelevant-z", f.boolean)
	notX := f.own(m.MkNot(f.x))
	grouped := f.own(m.MkAndArgs([]ast.ASTHandle{f.x, z, notX}))
	result, err := extractMaxRelevantExpr(m, f.adapter, grouped)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	outer, err := m.RefGet(result.formula)
	fmMust(t, err)
	sub, err := m.RefGet(result.subformula)
	fmMust(t, err)
	first, err := m.GetArg(outer, 0)
	fmMust(t, err)
	second, err := m.GetArg(outer, 1)
	fmMust(t, err)
	if first != z || second != sub {
		t.Fatal("relevant terms were not grouped after irrelevant terms")
	}
	a, err := m.GetArg(sub, 0)
	fmMust(t, err)
	b, err := m.GetArg(sub, 1)
	fmMust(t, err)
	if a != f.x || b != notX {
		t.Fatal("relevant term order changed")
	}
	inner := f.own(m.MkOr(f.x, z))
	root := f.own(m.MkAnd(inner, f.y))
	nested, err := extractMaxRelevantExpr(m, f.adapter, root)
	fmMust(t, err)
	defer func() { fmMust(t, nested.Close()) }()
	sub, err = m.RefGet(nested.subformula)
	fmMust(t, err)
	if sub != f.x {
		t.Fatal("did not descend to relevant atom")
	}
	outer, err = m.RefGet(nested.formula)
	fmMust(t, err)
	first, err = m.GetArg(outer, 0)
	fmMust(t, err)
	if first != f.y {
		t.Fatal("irrelevant outer argument order")
	}
	innerResult, err := m.GetArg(outer, 1)
	fmMust(t, err)
	first, err = m.GetArg(innerResult, 0)
	fmMust(t, err)
	second, err = m.GetArg(innerResult, 1)
	fmMust(t, err)
	if first != z || second != f.x {
		t.Fatal("nested relevant argument order")
	}
	unchanged, err := extractMaxRelevantExpr(m, f.adapter, z)
	fmMust(t, err)
	defer func() { fmMust(t, unchanged.Close()) }()
	outer, err = m.RefGet(unchanged.formula)
	fmMust(t, err)
	sub, err = m.RefGet(unchanged.subformula)
	fmMust(t, err)
	if outer != z || sub != z {
		t.Fatal("irrelevant atom changed")
	}
}
