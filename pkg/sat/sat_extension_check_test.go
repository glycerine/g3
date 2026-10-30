package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATExtensionFinalCheckOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result SATExtensionCheckResult
		want   lbool.LBool
		abort  bool
	}{{"done", SATExtensionDone, lbool.True, false}, {"continue", SATExtensionContinue, lbool.Undef, false}, {"giveup", SATExtensionGiveUp, lbool.Undef, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := satSolverCheckFixture(t)
			extension := &satExtensionFixture{checkResult: tc.result}
			satOK(t, s.SetExtension(extension))
			got, err := s.finalExtensionCheck()
			var abort satSolverAbort
			if got != tc.want || errors.As(err, &abort) != tc.abort || (!tc.abort && err != nil) {
				t.Fatal(got, err)
			}
		})
	}
}
