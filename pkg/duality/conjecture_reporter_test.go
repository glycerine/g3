package duality

import (
	"bytes"
	"errors"
	"testing"
)

func TestConjectureReporterParametersFormulaAndWriterFailure(t *testing.T) {
	ctx, _, _, globals := dualityNativeSolverServicesFixture(t)
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
}
