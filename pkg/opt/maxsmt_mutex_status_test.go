package opt

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// AUDIT.md opt_01-F6: init_local (maxres) and wmax::operator() return
// find_mutexes' status when it is not l_true; the engine run then reports
// that status instead of a construction error.
func TestMaxSMTMutexDiscoveryStatusIsReturned(t *testing.T) {
	for _, status := range []lbool.LBool{lbool.Undef, lbool.False} {
		w := &wmaxEngine{mutexStatus: status, mutexStopped: true}
		got, optimal, err := w.optimize(&ArithmeticResult{})
		if err != nil || optimal || got != status {
			t.Fatal("wmax mutex status", got, optimal, err)
		}
	}
}
