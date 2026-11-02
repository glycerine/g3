//go:build g3trace

package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestSpacerRuleSelectionSourceCandidateTrace(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	p, m := f.context.transformers[f.query], f.context.manager
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()

	tag, err := p.RuleTag(f.rule)
	depthMust(t, err)
	declaration, err := m.GetDecl(tag)
	depthMust(t, err)
	name, err := m.GetASTName(declaration)
	depthMust(t, err)
	tagName, _, err := m.Symbols.BareString(name)
	depthMust(t, err)
	formula, err := f.rule.FormulaString()
	depthMust(t, err)

	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("spacer_verbose")
	original := p.owner.contexts[0].config.ModelEvaluator
	configured := original
	configured.Trace = tags
	p.owner.contexts[0].config.ModelEvaluator = configured
	defer func() { p.owner.contexts[0].config.ModelEvaluator = original }()

	before := m.Stats().LiveReferences
	selection, err := p.SelectRule(md.Core())
	depthMust(t, err)
	if selection.Rule != nil || !selection.IsConcrete {
		depthMust(t, selection.Close())
		t.Fatal("missing tag interpretation did not choose the extension")
	}
	depthMust(t, selection.Close())
	if m.Stats().LiveReferences != before {
		t.Fatal("candidate trace leaked references")
	}
	got := output.String()
	if strings.Count(got, tagName+":\n") != 1 {
		t.Fatal("candidate tag trace", got)
	}
	if !strings.Contains(got, formula+"\n") {
		t.Fatal("candidate rule trace", got)
	}
}
