package duality

import (
	"bytes"
	"errors"
	"testing"
)

func TestCoveringNativeContainmentAndUpdateInvalidation(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("cover-R")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	source, err := original.CreateNode(term)
	must(err)
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	var nodes []*Node
	for n := 0; n < 5; n++ {
		node, err := working.CloneNode(source)
		must(err)
		_, err = working.CreateLowerBoundEdge(node)
		must(err)
		nodes = append(nodes, node)
	}
	covering, err := NewCovering(working, aux)
	must(err)
	added, err := covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if !added {
		t.Fatal("valid covering rejected")
	}
	covered, err := covering.IsCovered(nodes[2])
	must(err)
	if !covered {
		t.Fatal("covering not recorded")
	}
	added, err = covering.CoverByNode(nodes[1], nodes[2])
	must(err)
	if added {
		t.Fatal("reverse source order accepted")
	}
	// A covering candidate whose descendant is covered is unavailable.
	must(working.DeleteEdge(nodes[3].Outgoing))
	_, err = working.CreateEdge(nodes[3], nodes[3].Annotation, []*Node{nodes[2]})
	must(err)
	added, err = covering.CoverByNode(nodes[4], nodes[3])
	must(err)
	if added {
		t.Fatal("transitively covered candidate accepted")
	}
	must(covering.Update(nodes[1]))
	if !covering.someUpdates {
		t.Fatal("update flag not set")
	}
	covered, err = covering.IsCovered(nodes[2])
	must(err)
	if covered {
		t.Fatal("stale covering survived annotation update")
	}
	// A stronger covering annotation must really contain the covered fact.
	must(nodes[1].Annotation.SetEmpty())
	added, err = covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if added {
		t.Fatal("non-containing annotation accepted")
	}
	must(nodes[1].Annotation.SetFull())
	added, err = covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if !added {
		t.Fatal("covering not restored")
	}
	// Underapproximation nodes may cover their associated original out of order,
	// but an underapproximation node may never itself be covered.
	must(covering.MapUnderapproximation(nodes[4], nodes[0]))
	if !covering.CoverOrder(nodes[4], nodes[0]) || covering.CoverOrder(nodes[0], nodes[4]) {
		t.Fatal("underapproximation covering order")
	}
	// Ascendant invalidation visits parents and stops on graph cycles.
	must(working.DeleteEdge(nodes[0].Outgoing))
	_, err = working.CreateEdge(nodes[0], nodes[0].Annotation, []*Node{nodes[3]})
	must(err)
	must(working.DeleteEdge(nodes[3].Outgoing))
	_, err = working.CreateEdge(nodes[3], nodes[3].Annotation, []*Node{nodes[0]})
	must(err)
	must(covering.RemoveAscendantCoverings(nodes[3]))
	covered, err = covering.IsCovered(nodes[2])
	must(err)
	if covered {
		t.Fatal("ancestor invalidation missed later instance")
	}
	covered, err = covering.IsCovered(nodes[0])
	must(err)
	if covered {
		t.Fatal("uncovered cycle reported covered")
	}
}

func TestCoveringReporterWriterFailureAndRetry(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	term, err := ctx.BoolConst("report-cover")
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	source, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	original, err := source.CreateNode(term)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	if err != nil {
		t.Fatal(err)
	}
	defer graph.Close()
	earlier, err := graph.CloneNode(original)
	if err != nil {
		t.Fatal(err)
	}
	later, err := graph.CloneNode(original)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := graph.CreateLowerBoundEdge(earlier); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.CreateLowerBoundEdge(later); err != nil {
		t.Fatal(err)
	}
	covering, err := NewCovering(graph, aux)
	if err != nil {
		t.Fatal(err)
	}
	reporter := &StreamReporter{Format: ConjectureReporter{Output: dualityRefusingWriter{}}}
	graph.streamReporter = reporter
	if covered, err := covering.CoverByNode(later, earlier); covered || !errors.Is(err, errDualityWriter) {
		t.Fatal("cover writer refusal", covered, err)
	}
	var output bytes.Buffer
	reporter.Format.Output = &output
	if err := covering.Update(earlier); err != nil {
		t.Fatal(err)
	}
	covered, err := covering.CoverByNode(later, earlier)
	if err != nil || !covered {
		t.Fatal("cover retry", covered, err)
	}
	if output.String() != "[1]uncover report-cover: 2 by 1\n[2]cover report-cover: 2 by 1 \n" {
		t.Fatal("covering report", output.String())
	}
	reporter.Format.Output = dualityRefusingWriter{}
	if err := covering.Update(earlier); !errors.Is(err, errDualityWriter) {
		t.Fatal("uncover writer refusal", err)
	}
	graph.streamReporter = nil
	if err := covering.Update(earlier); err != nil {
		t.Fatal(err)
	}
	covered, err = covering.IsCovered(later)
	if err != nil || covered {
		t.Fatal("partial uncover left stale relation", covered, err)
	}
}
