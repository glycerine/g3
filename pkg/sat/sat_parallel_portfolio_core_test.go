package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// The child lane finishes first and its core is copied into the owner at
// once (check_par's first-finisher branch). The owner lane then still runs:
// init_search resets m_core, and the canceled owner's first propagate
// checkpoint throws, which check_par swallows for a lane other than 0. So
// the published result is the child's l_false with the owner's empty core,
// as in the C++ (AUDIT.md sat_03-F2; before that fix the child's core was
// copied after the owner lane and this test expected it).
func TestSATParallelPortfolioChildWinnerPublishesCore(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	v, err := s.MkVar(true, true)
	satOK(t, err)
	positive := MakeLiteral(v, false)
	negative := positive.Negated()
	satOK(t, s.MkClause([]Literal{positive}))
	s.Limit.Cancel()
	got, err := s.Check([]Literal{negative})
	s.Limit.ResetCancel()
	satOK(t, err)
	if got != lbool.False {
		t.Fatal("parallel child result", got)
	}
	core, err := s.BorrowCore()
	satOK(t, err)
	n, err := core.Size()
	satOK(t, err)
	if n != 0 {
		t.Fatal("owner lane's init_search must reset the transferred core", n)
	}
}
