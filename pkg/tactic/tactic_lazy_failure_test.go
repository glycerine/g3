package tactic

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestLazyTacticFactoryFailureIsRetryable(t *testing.T) {
	m, c := lazyFixture(t)
	calls := 0
	l, e := NewLazyTactic(m, c, nil, func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error) {
		calls++
		return nil, arena.ErrInjectedFailure
	})
	astTestMust(t, e)
	astTestMust(t, l.IncRef())
	defer l.DecRef()
	g, _ := NewGoal(m, false, true, false)
	defer g.Close()
	for i := 0; i < 2; i++ {
		var out TacticResult
		if e = l.Apply(g, &out); !errors.Is(e, arena.ErrInjectedFailure) {
			t.Fatal(e)
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
