package duality

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDualityRefinementFailureReportsAndContinues(t *testing.T) {
	var report bytes.Buffer
	original := &RPFP{streamReporter: &StreamReporter{Format: ConjectureReporter{Output: &report}}}
	d := &BacktrackingDerivation{DerivationTree: &DerivationTree{original: original}}
	if err := d.refinementFailure(ErrGreedyReduceFailed); err != nil {
		t.Fatal("recoverable reduction failure stopped learning", err)
	}
	if !strings.Contains(report.String(), "interpolant verification failed") {
		t.Fatal("missing source diagnostic", report.String())
	}
	for _, failure := range []error{fmt.Errorf("wrapped: %w", ErrGreedyReduceFailed), errors.Join(ErrGreedyReduceFailed)} {
		if err := d.refinementFailure(failure); err != nil {
			t.Fatal("wrapped recoverable failure stopped learning", err)
		}
	}
	report.Reset()
	sentinel := errors.New("unrelated failure")
	if d.refinementFailure(sentinel) != sentinel {
		t.Fatal("unrelated error was suppressed")
	}
	if report.Len() != 0 {
		t.Fatal("unrelated failure reported as verification failure")
	}
	for _, failure := range []error{
		errors.Join(ErrGreedyReduceFailed, sentinel),
		fmt.Errorf("refinement cleanup: %w", errors.Join(ErrGreedyReduceFailed, sentinel)),
		errors.Join(fmt.Errorf("reduction: %w", ErrGreedyReduceFailed), sentinel),
	} {
		got := d.refinementFailure(failure)
		if !errors.Is(got, sentinel) || !errors.Is(got, ErrGreedyReduceFailed) {
			t.Fatal("joined resource failure suppressed", got)
		}
	}
	original.streamReporter.Format.Output = dualityRefusingWriter{}
	err := d.refinementFailure(ErrGreedyReduceFailed)
	if !errors.Is(err, ErrGreedyReduceFailed) || !errors.Is(err, errDualityWriter) {
		t.Fatal("reporting failure lost its causes", err)
	}
}
