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

// AUDIT.md tactic_bv_01-F1: C++ reduce_quantifier restarts num_steps for every
// quantifier, so a later quantifier is still expanded instead of becoming true.
func TestElimSmallBVStepLimitIsPerQuantifier(t *testing.T) {
	base := sharingTacticFixture(t, false)
	m := base.manager
	globals := uparams.BorrowProcessGlobalParamsService()
	x, err := NewElimSmallBVTactic(m, base.context, globals, nil, nil)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, x.Close()) })
	p, err := base.context.NewValue()
	sharingMust(t, err)
	key := "max_steps"
	sharingMust(t, base.context.SetUIntName(&p, &key, 5))
	sharingMust(t, x.UpdateParams(&p))
	sharingMust(t, base.context.Del(&p))
	s, err := m.BitVectorSort(2)
	sharingMust(t, err)
	b, err := m.MkBoolSort()
	sharingMust(t, err)
	v, err := m.MkVar(0, s)
	sharingMust(t, err)
	g, err := tactic.NewGoal(m, false, false, false)
	sharingMust(t, err)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	want := map[ast.ASTHandle]bool{}
	for k := 0; k < 4; k++ {
		decl, err := m.MkFreshFuncDecl(usymbol.Numerical(uint32(k)), usymbol.Symbol{}, []ast.ASTHandle{s}, b)
		sharingMust(t, err)
		body, err := m.MkApp(decl, []ast.ASTHandle{v})
		sharingMust(t, err)
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{s}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
		sharingMust(t, err)
		sharingMust(t, g.Assert(q))
		for j := uint32(0); j < 2; j++ {
			n, err := m.BitVectorNumeralUnsigned(j, 2)
			sharingMust(t, err)
			a, err := m.MkApp(decl, []ast.ASTHandle{n})
			sharingMust(t, err)
			want[a] = true
		}
	}
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, x.Apply(g, &out))
	if len(out.Goals) != 1 {
		t.Fatal("result goals", len(out.Goals))
	}
	n, err := out.Goals[0].Size()
	sharingMust(t, err)
	if n != 8 {
		t.Fatal("expected 8 expanded formulas, got", n)
	}
	for i := uint32(0); i < n; i++ {
		f, err := out.Goals[0].Form(i)
		sharingMust(t, err)
		if !want[f] {
			t.Fatal("unexpected formula", i)
		}
	}
}
