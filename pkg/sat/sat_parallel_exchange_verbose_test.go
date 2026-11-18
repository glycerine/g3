//go:build g3trace

package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md sat_03-F3: exchange_par reports the units it exchanged.
func TestSATParallelExchangeSourceSyncVerbose(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	out := satInspectionStream(t, arena.NewBudget(8192))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	satOK(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	satOK(t, err)
	t.Cleanup(func() {
		satOK(t, diagnostic.SetProcessVerboseStream(oldStream))
		satOK(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
	})
	satOK(t, diagnostic.SetProcessVerboseStream(out))
	satOK(t, diagnostic.SetProcessVerbosityLevel(1))
	_, err = s.MkVar(false, true)
	satOK(t, err)
	satOK(t, s.MkClause([]Literal{MakeLiteral(0, false)}))
	satOK(t, s.SetParallelExchange(NewParallelUnitExchange()))
	satOK(t, s.conflict.exchangeParallel())
	satOK(t, s.conflict.exchangeParallel())
	got, err := out.String()
	satOK(t, err)
	if got != "(sat-sync out: 1 in: 0)\n" {
		t.Fatalf("%q", got)
	}
}
