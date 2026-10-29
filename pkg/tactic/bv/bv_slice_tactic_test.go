package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"testing"
)

func TestBVSliceSourceTacticExampleSlicesLaterUses(t *testing.T) {
	f := newBVSliceFixture(t)
	g, e := tactic.NewGoal(f.m, false, false, false)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, g.Close()) })
	zero, e := f.m.BitVectorNumeralUnsigned(123, 16)
	sharingMust(t, e)
	first := f.eq(t, f.extract(t, 31, 16, f.x), zero)
	second := f.eq(t, f.extract(t, 15, 0, f.x), f.extract(t, 16, 1, f.y))
	sum, e := f.m.MkBitVectorAdd(f.x, f.x)
	sharingMust(t, e)
	third := f.eq(t, sum, f.y)
	for _, h := range []ast.ASTHandle{first, second, third} {
		sharingMust(t, g.Assert(h))
	}
	var out tactic.TacticResult
	t.Cleanup(func() { sharingMust(t, out.Close()) })
	sharingMust(t, f.tactic.Apply(g, &out))
	got, e := g.Form(2)
	sharingMust(t, e)
	if got == third {
		t.Fatal("later occurrence was not sliced")
	}
	foundConcat := false
	stack := []ast.ASTHandle{got}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		yes, e := newBVSliceFrame(f.tactic).isConcat(h)
		sharingMust(t, e)
		foundConcat = foundConcat || yes
		k, e := f.m.GetKind(h)
		sharingMust(t, e)
		if k == ast.ASTApp {
			n, e := f.m.GetNumArgs(h)
			sharingMust(t, e)
			for i := uint32(0); i < n; i++ {
				a, e := f.m.GetArg(h, i)
				sharingMust(t, e)
				stack = append(stack, a)
			}
		}
	}
	if !foundConcat {
		t.Fatal("no slice concat in rewritten formula")
	}
}
