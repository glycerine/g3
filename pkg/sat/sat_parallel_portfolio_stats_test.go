package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATParallelPortfolioChildWinnerPublishesMainStatistics(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	satOK(t, s.MkClause([]Literal{MakeLiteral(0, false), MakeLiteral(1, false)}))
	s.Limit.Cancel()
	got, err := s.Check(nil)
	s.Limit.ResetCancel()
	satOK(t, err)
	if got != lbool.True || s.decision.decisions == 0 {
		t.Fatal("parallel child statistics", got, s.decision.decisions)
	}
}
