package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func domMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type domFixture struct {
	m       *ast.ASTManager
	a, b, c ast.ASTHandle
}

func newDomFixture(t *testing.T) domFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	domMust(t, err)
	numbers, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	domMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	domMust(t, err)
	t.Cleanup(func() {
		domMust(t, m.Close())
		domMust(t, numbers.Close())
		domMust(t, symbols.Close())
		if budget.Used() != 0 {
			t.Fatal("dom fixture retained", budget.Used())
		}
	})
	s, err := m.MkBoolSort()
	domMust(t, err)
	mk := func(name string) ast.ASTHandle {
		n, e := m.Symbols.Intern(name)
		domMust(t, e)
		h, e := m.MkConst(n, s)
		domMust(t, e)
		ref, e := m.NewRef(h)
		domMust(t, e)
		t.Cleanup(func() { domMust(t, m.DelRef(&ref)) })
		return h
	}
	return domFixture{m: m, a: mk("dom-a"), b: mk("dom-b"), c: mk("dom-c")}
}
func (f domFixture) goal(t *testing.T, proofs bool, forms ...ast.ASTHandle) *tactic.Goal {
	t.Helper()
	if proofs {
		domMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	}
	g, e := tactic.NewGoal(f.m, proofs, false, false)
	domMust(t, e)
	t.Cleanup(func() { domMust(t, g.Close()) })
	for _, h := range forms {
		domMust(t, g.Assert(h))
	}
	return g
}
func (f domFixture) run(t *testing.T, g *tactic.Goal) *DomSimplifyTactic {
	t.Helper()
	r, e := NewDomSimplifyTactic(f.m, nil)
	domMust(t, e)
	t.Cleanup(func() { domMust(t, r.Close()) })
	var out tactic.TacticResult
	t.Cleanup(func() { domMust(t, out.Close()) })
	domMust(t, r.Apply(g, &out))
	if len(out.Goals) != 1 || out.Goals[0] != g {
		t.Fatal("goal publication")
	}
	return r
}
