package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestLazyTacticDefersFactoryUntilApply(t *testing.T) {
	m, c := lazyFixture(t)
	calls := 0
	l, e := NewLazyTactic(m, c, nil, func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error) { calls++; return NewSkipTactic(), nil })
	astTestMust(t, e)
	astTestMust(t, l.IncRef())
	defer l.DecRef()
	if calls != 0 {
		t.Fatal(calls)
	}
	g, e := NewGoal(m, false, true, false)
	astTestMust(t, e)
	defer g.Close()
	var out TacticResult
	astTestMust(t, l.Apply(g, &out))
	defer out.Close()
	if calls != 1 {
		t.Fatal(calls)
	}
}
