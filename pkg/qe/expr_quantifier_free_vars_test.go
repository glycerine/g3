package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExprQuantifierInstantiationAndAbstraction(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	v0 := f.own(m.MkVar(0, f.boolean))
	v2 := f.own(m.MkVar(2, f.boolean))
	root := f.own(m.MkOr(v0, v2))
	nestedFree := f.own(m.MkVar(3, f.boolean))
	nestedBody := f.own(m.MkOr(v0, nestedFree))
	quantified := f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{f.boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: nestedBody}))
	for _, original := range []ast.ASTHandle{root, quantified} {
		state, err := instantiateExprFreeVars(m, original)
		fmMust(t, err)
		if len(state.bound) != 3 {
			t.Fatalf("got %d free-variable slots", len(state.bound))
		}
		instantiated, err := m.RefGet(state.formula)
		fmMust(t, err)
		hasFree, err := m.HasFreeVars(instantiated)
		fmMust(t, err)
		if hasFree {
			t.Fatal("free variables remained after instantiation")
		}
		restored, err := state.abstract(instantiated)
		fmMust(t, err)
		value, err := m.RefGet(restored)
		fmMust(t, err)
		if value != original {
			t.Fatal("free variables did not round-trip")
		}
		fmMust(t, m.DelRef(&restored))
		fmMust(t, state.Close())
	}
	closed := f.own(m.MkTrue())
	state, err := instantiateExprFreeVars(m, closed)
	fmMust(t, err)
	if len(state.bound) != 0 {
		t.Fatal("closed formula received bound constants")
	}
	result, err := state.abstract(closed)
	fmMust(t, err)
	value, err := m.RefGet(result)
	fmMust(t, err)
	if value != closed {
		t.Fatal("closed formula changed")
	}
	fmMust(t, m.DelRef(&result))
	fmMust(t, state.Close())
}
