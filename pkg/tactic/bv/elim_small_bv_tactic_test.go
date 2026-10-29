package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestElimSmallBVRetainsSourceWidthBoundEnumerationAndLifecycle(t *testing.T) {
	base := sharingTacticFixture(t, false)
	m := base.manager
	globals := uparams.BorrowProcessGlobalParamsService()
	x, err := NewElimSmallBVTactic(m, base.context, globals, nil, nil)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, x.Close()) })
	s, err := m.BitVectorSort(2)
	sharingMust(t, err)
	v, err := m.MkVar(0, s)
	sharingMust(t, err)
	one, err := m.BitVectorNumeralUnsigned(1, 2)
	sharingMust(t, err)
	eq, err := m.MkEq(v, one)
	sharingMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{s}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: eq})
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, false, false, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	sharingMust(t, g.Assert(q))
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, x.Apply(g, &out))
	if len(out.Goals) != 1 {
		t.Fatal("result goals", len(out.Goals))
	}
	f, err := out.Goals[0].Form(0)
	sharingMust(t, err)
	if !m.IsFalse(f) {
		t.Fatal("source j < bit-width expansion was not retained")
	}
	if d, err := out.Goals[0].Depth(); err != nil || d != 1 {
		t.Fatal("depth", d, err)
	}
}
