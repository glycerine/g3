package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATParallelPortfolioChildWinnerPublishesModel(t *testing.T) {
	s := satParallelPortfolioFixture(t, 2)
	v, err := s.MkVar(false, true)
	satOK(t, err)
	satOK(t, s.MkClause([]Literal{MakeLiteral(v, false)}))
	// Existing parent cancellation is not copied into a newly registered source
	// child. The child therefore wins while the owner lane returns undefined.
	s.Limit.Cancel()
	got, err := s.Check(nil)
	s.Limit.ResetCancel()
	satOK(t, err)
	if got != lbool.True {
		t.Fatal("parallel child result", got)
	}
	satParallelAssertModel(t, s, v, lbool.True)
}
