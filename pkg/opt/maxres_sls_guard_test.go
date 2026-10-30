package opt

// Source: src/opt/opt_context.cpp enable_sls and MaxRes seed guards.

import (
	"errors"
	"testing"
)

func TestMaxResSLSDisabledOrEmptyObjectiveDoesNotStartSearch(t *testing.T) {
	for _, engine := range []*maxResEngine{
		{config: maxSMTConfig{enableSLS: false}, objective: &arithmeticObjective{}},
		{config: maxSMTConfig{enableSLS: true}, objective: &arithmeticObjective{}},
	} {
		if err := engine.trySLSCandidate(&ArithmeticResult{}); err != nil {
			t.Fatal(err)
		}
		if engine.stats.SLSAttempts != 0 {
			t.Fatal("unexpected SLS attempt", engine.stats.SLSAttempts)
		}
	}
}

func TestMaxResSLSSeedRequiresPublishedModel(t *testing.T) {
	engine := &maxResEngine{}
	search, err := engine.newSLSCandidateSearch(&ArithmeticResult{})
	if search != nil || !errors.Is(err, errMaxResState) {
		t.Fatal("missing SLS seed", search, err)
	}
}

func TestMaxResSLSErrorJoinPreservesBothFailures(t *testing.T) {
	left := errors.New("left")
	right := errors.New("right")
	err := joinMaxResSLSErrors(left, right)
	if !errors.Is(err, left) || !errors.Is(err, right) {
		t.Fatal(err)
	}
}
