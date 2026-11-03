package duality

import (
	"bytes"
	"errors"
	"testing"
)

var errRecursionReportWriter = errors.New("recursion report writer refused")

type recursionReportRefusingWriter struct{}

func (recursionReportRefusingWriter) Write([]byte) (int, error) {
	return 0, errRecursionReportWriter
}

func TestDualityRecursionResultReportsOnlyActiveCutoffs(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("recursion-report-R")
	must(err)
	closeExpr(t, term)
	original, err := source.CreateNode(term)
	must(err)
	original.RecursionBound = 2
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	var output bytes.Buffer
	graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &output}, depth: -1}
	covering, err := NewCovering(graph, native)
	must(err)
	unwinding, err := NewUnwinding(source, graph, covering)
	must(err)
	instance, err := unwinding.CreateNodeInstance(original, 0)
	must(err)
	_, err = graph.CreateLowerBoundEdge(instance)
	must(err)
	unwinding.RecursionBound = 0
	unwinding.backEdges[instance] = map[*Node]int{original: 3}

	// Pending instances do not contribute to the published solution.
	must(unwinding.TestRecursionBounded())
	if unwinding.IsResultRecursionBounded() || output.Len() != 0 {
		t.Fatal("pending cutoff affected result or report", unwinding.IsResultRecursionBounded(), output.String())
	}
	must(unwinding.ActivateInstance(instance))
	must(unwinding.TestRecursionBounded())
	if !unwinding.IsResultRecursionBounded() {
		t.Fatal("active cutoff omitted from result")
	}
	if got, want := output.String(), "[0]msg cut off recursion-report-R at depth 3\n"; got != want {
		t.Fatalf("cutoff report %q, want %q", got, want)
	}

	// Covered, inclusive-limit, and globally disabled cases stay silent.
	output.Reset()
	covering.coveredBy[instance] = instance
	must(unwinding.TestRecursionBounded())
	if unwinding.IsResultRecursionBounded() || output.Len() != 0 {
		t.Fatal("covered cutoff affected result or report", unwinding.IsResultRecursionBounded(), output.String())
	}
	delete(covering.coveredBy, instance)
	unwinding.backEdges[instance][original] = 2
	must(unwinding.TestRecursionBounded())
	if unwinding.IsResultRecursionBounded() || output.Len() != 0 {
		t.Fatal("inclusive limit affected result or report", unwinding.IsResultRecursionBounded(), output.String())
	}
	unwinding.backEdges[instance][original] = 3
	unwinding.RecursionBound = -1
	must(unwinding.TestRecursionBounded())
	if unwinding.IsResultRecursionBounded() || output.Len() != 0 {
		t.Fatal("disabled limit affected result or report", unwinding.IsResultRecursionBounded(), output.String())
	}

	// Reporting occurs before the result flag is published, so a destination
	// failure cannot leave a falsely successful bounded-result status.
	unwinding.RecursionBound = 0
	graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: recursionReportRefusingWriter{}}, depth: -1}
	if err = unwinding.TestRecursionBounded(); !errors.Is(err, errRecursionReportWriter) {
		t.Fatalf("report failure %v, want writer error", err)
	}
	if unwinding.IsResultRecursionBounded() {
		t.Fatal("report failure published recursion-bounded result")
	}
}
