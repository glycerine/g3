package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

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
	if n != 1 {
		t.Fatal("parallel child core size", n)
	}
	lit, err := core.Read(0)
	satOK(t, err)
	if lit != negative {
		t.Fatal("parallel child core", lit, negative)
	}
}
