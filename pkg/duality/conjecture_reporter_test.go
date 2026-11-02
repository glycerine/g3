package duality

import (
	"bytes"
	"errors"
	"testing"
)

func TestConjectureReporterParametersFormulaAndWriterFailure(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	x, err := ctx.BoolConst("report-x")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	sort, err := x.Sort()
	if err != nil {
		t.Fatal(err)
	}
	defer sort.Close()
	name, err := ctx.StringSymbol("report-P")
	if err != nil {
		t.Fatal(err)
	}
	decl, err := ctx.Function(name, []*Sort{sort}, sort)
	if err != nil {
		t.Fatal(err)
	}
	defer decl.Close()
	term, err := decl.Call(x)
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	graph, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer graph.Close()
	node, err := graph.CreateNode(term)
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Annotation.SetFull(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	reporter := &ConjectureReporter{Output: &output, Params: ctx.config.params, Globals: globals}
	if err := reporter.Update(node, node.Annotation); err != nil {
		t.Fatal(err)
	}
	want := "(define-fun report-P ((report-x Bool)) Bool \ntrue)\n\n"
	if output.String() != want {
		t.Fatalf("conjecture report %q want %q", output.String(), want)
	}
	reporter.Output = dualityRefusingWriter{}
	if err := reporter.Update(node, node.Annotation); !errors.Is(err, errDualityWriter) {
		t.Fatal("report writer failure", err)
	}
	reporter.Output = &output
	output.Reset()
	if err := reporter.Update(node, node.Annotation); err != nil {
		t.Fatal(err)
	}
	if output.String() != want {
		t.Fatal("report writer retry", output.String())
	}
	output.Reset()
	if _, err := graph.CreateLowerBoundEdge(node); err != nil {
		t.Fatal(err)
	}
	stream := &StreamReporter{Format: *reporter, depth: 3}
	if err := stream.Extend(node); err != nil {
		t.Fatal(err)
	}
	if err := stream.Update(node, node.Annotation, true); err != nil {
		t.Fatal(err)
	}
	if err := stream.Bound(node); err != nil {
		t.Fatal(err)
	}
	wantEvents := "[0]node 1: report-P\n[1]update 1 report-P: 0: true (depth=3) (eager)\n[2]check 1\n"
	if output.String() != wantEvents {
		t.Fatalf("search events %q want %q", output.String(), wantEvents)
	}
	stream.Format.Output = dualityRefusingWriter{}
	if err := stream.Bound(node); !errors.Is(err, errDualityWriter) {
		t.Fatal("stream writer failure", err)
	}

	stream.Format.Output = &output
	output.Reset()
	xh, err := x.Raw()
	if err != nil {
		t.Fatal(err)
	}
	shared, err := m.MkAnd(xh, xh)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetRef(node.Annotation.formula, shared); err != nil {
		t.Fatal(err)
	}
	if err := stream.Update(node, node.Annotation, false); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[4]update 1 report-P: 1: report-x (depth=3)\n" {
		t.Fatal("shared summary", output.String())
	}

	output.Reset()
	clone, err := graph.CloneNode(node)
	if err != nil {
		t.Fatal(err)
	}
	edge, err := graph.CreateLowerBoundEdge(clone)
	if err != nil {
		t.Fatal(err)
	}
	stream.Depth(2)
	if err := stream.Expand(edge); err != nil {
		t.Fatal(err)
	}
	if err := stream.Conjecture(node, node.Annotation); err != nil {
		t.Fatal(err)
	}
	if err := stream.UpdateUnderapprox(node, node.Annotation); err != nil {
		t.Fatal(err)
	}
	wantEvents = "[5]expand 1 report-P (depth=2)\n[6]conjecture 1 report-P: 1: report-x\n[7]underapprox 1 report-P: (and report-x report-x)\n"
	if output.String() != wantEvents {
		t.Fatalf("expansion events %q want %q", output.String(), wantEvents)
	}

	output.Reset()
	if err := stream.InductionFailure(node.Outgoing, []*Node{clone, node}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Reject(edge, []*Node{node}); err != nil {
		t.Fatal(err)
	}
	if err := stream.Message("restarted"); err != nil {
		t.Fatal(err)
	}
	wantEvents = "[8]induction failure: report-P, children = 2 1\n[9]reject 2 report-P:  1\n[10]msg restarted\n"
	if output.String() != wantEvents {
		t.Fatalf("candidate events %q want %q", output.String(), wantEvents)
	}

	output.Reset()
	if err := stream.AddCover(clone, []*Node{node, clone}); err != nil {
		t.Fatal(err)
	}
	if err := stream.RemoveCover(clone, node); err != nil {
		t.Fatal(err)
	}
	if err := stream.Forcing(clone, node); err != nil {
		t.Fatal(err)
	}
	if err := stream.Dominates(node, clone); err != nil {
		t.Fatal(err)
	}
	wantEvents = "[11]cover report-P: 2 by 1 2 \n[12]uncover report-P: 2 by 1\n[13]forcing report-P: 2 by 1\n[14]dominates report-P: 1 > 2\n"
	if output.String() != wantEvents {
		t.Fatalf("cover events %q want %q", output.String(), wantEvents)
	}

}
