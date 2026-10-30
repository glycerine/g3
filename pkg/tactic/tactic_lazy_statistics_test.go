package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestLazyTacticStatisticsDoNotMaterialize(t *testing.T) {
	m, c := lazyFixture(t)
	calls := 0
	l, e := NewLazyTactic(m, c, nil, func(*ast.ASTManager, *uparams.ParamsRef) (Tactic, error) { calls++; return NewSkipTactic(), nil })
	astTestMust(t, e)
	astTestMust(t, l.IncRef())
	defer l.DecRef()
	if len(l.CollectStatistics()) != 0 || calls != 0 {
		t.Fatal(calls)
	}
}
