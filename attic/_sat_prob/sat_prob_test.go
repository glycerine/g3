package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATProbFiniteClauseStateFlipAndModel(t *testing.T) {
	p := newSATProb(&control.Reslimit{Limit: 10000}, 0)
	p.reserveVariables(3)
	clauses := [][]Literal{
		{MakeLiteral(0, false), MakeLiteral(1, false)},
		{MakeLiteral(0, true), MakeLiteral(1, false)},
		{MakeLiteral(0, false), MakeLiteral(1, true)},
	}
	for _, c := range clauses {
		p.addClause(c)
	}
	if got := p.check(); got != lbool.True {
		t.Fatal("prob search", got, p.bestUnsat)
	}
	for _, c := range clauses {
		sat := false
		for _, l := range c {
			sat = sat || p.model[l.variable()] == lbool.FromBool(!l.sign())
		}
		if !sat {
			t.Fatal("model misses clause", c, p.model)
		}
	}
	if p.bestUnsat != 0 || len(p.model) != 3 {
		t.Fatal(p.bestUnsat, p.model)
	}
}

func TestSATProbRestartAndResourceBoundOnUnsat(t *testing.T) {
	limit := &control.Reslimit{Limit: 30}
	p := newSATProb(limit, 7)
	p.restartOffset = 1
	p.addClause([]Literal{MakeLiteral(0, false)})
	p.addClause([]Literal{MakeLiteral(0, true)})
	if got := p.check(); got != lbool.Undef || limit.Count() != 31 || p.restarts < 2 {
		t.Fatal("bounded contradictory prob search", got, limit.Count(), p.restarts)
	}
}
