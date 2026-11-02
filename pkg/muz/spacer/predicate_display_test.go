//go:build g3trace

package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerInitializedTransformerSourceDisplayTrace(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	p, m := f.context.transformers[f.query], f.context.manager
	formula, err := f.rule.FormulaString()
	depthMust(t, err)
	transition, err := p.Transition()
	depthMust(t, err)
	var transitionOutput bytes.Buffer
	printer, err := ast.NewSimpleASTPrinterContext(m, &transitionOutput, &transitionOutput)
	depthMust(t, err)
	depthMust(t, printer.DisplayExpr(&transitionOutput, transition, 0))
	expected := "rules\n" + formula + "\ntransition\n" + transitionOutput.String() + "\n"

	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("spacer")
	original := p.owner.contexts[0].config.ModelEvaluator
	configured := original
	configured.Trace = tags
	p.owner.contexts[0].config.ModelEvaluator = configured
	defer func() { p.owner.contexts[0].config.ModelEvaluator = original }()

	before := m.Stats().LiveReferences
	depthMust(t, p.traceInitializedSystem())
	if m.Stats().LiveReferences != before {
		t.Fatal("initialized-system display leaked references")
	}
	if !strings.Contains(output.String(), expected) {
		t.Fatalf("initialized-system trace = %q, missing %q", output.String(), expected)
	}
}
