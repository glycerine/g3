package tactic

import (
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestLazyTacticSnapshotsParameters(t *testing.T) {
	m, c := lazyFixture(t)
	p, e := c.NewValue()
	astTestMust(t, e)
	defer c.Del(&p)
	key := "timeout"
	astTestMust(t, c.SetUIntName(&p, &key, 7))
	seen := uint32(0)
	l, e := NewLazyTactic(m, c, &p, func(_ *ast.ASTManager, q *uparams.ParamsRef) (Tactic, error) {
		seen, _ = c.GetUIntName(q, &key, 0)
		return NewSkipTactic(), nil
	})
	astTestMust(t, e)
	astTestMust(t, l.IncRef())
	defer l.DecRef()
	astTestMust(t, c.SetUIntName(&p, &key, 9))
	g, _ := NewGoal(m, false, true, false)
	defer g.Close()
	var out TacticResult
	astTestMust(t, l.Apply(g, &out))
	defer out.Close()
	if seen != 7 {
		t.Fatal(seen)
	}
}
