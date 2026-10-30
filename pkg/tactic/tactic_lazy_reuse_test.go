package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestLazyTacticMaterializesOnce(t *testing.T) {
	m, c := lazyFixture(t)
	calls := 0
	l, e := NewLazyTactic(m, c, nil, func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error) { calls++; return NewSkipTactic(), nil })
	astTestMust(t, e)
	astTestMust(t, l.IncRef())
	defer l.DecRef()
	g, _ := NewGoal(m, false, true, false)
	defer g.Close()
	for i := 0; i < 2; i++ {
		var out TacticResult
		astTestMust(t, l.Apply(g, &out))
		astTestMust(t, out.Close())
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
