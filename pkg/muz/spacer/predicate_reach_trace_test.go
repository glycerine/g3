package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestReachFactSourceTracePrecedesDuplicateCheck(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	p, m := f.context.transformers[f.query], f.context.manager
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	original := p.owner.contexts[0].config.ModelEvaluator
	override := original
	override.Trace = tags
	p.owner.contexts[0].config.ModelEvaluator = override
	defer func() { p.owner.contexts[0].config.ModelEvaluator = original }()
	truth, err := m.MkTrue()
	depthMust(t, err)
	first, err := NewReachFact(m, f.rule, truth, nil, true)
	depthMust(t, err)
	defer func() { depthMust(t, first.Close()) }()
	added, err := p.AddReachFact(first)
	depthMust(t, err)
	if !added || output.Len() != 0 {
		t.Fatal("disabled trace emitted output or failed insertion")
	}
	tags.Enable("spacer")
	before := m.Stats().LiveReferences
	added, err = p.AddReachFact(first)
	depthMust(t, err)
	if added || before != m.Stats().LiveReferences {
		t.Fatal("duplicate altered facts or references")
	}
	falsity, err := m.MkFalse()
	depthMust(t, err)
	second, err := NewReachFact(m, f.rule, falsity, nil, false)
	depthMust(t, err)
	defer func() { depthMust(t, second.Close()) }()
	added, err = p.AddReachFact(second)
	depthMust(t, err)
	if !added {
		t.Fatal("computed fact not added")
	}
	if !tags.ShouldTrace("spacer") {
		if output.Len() != 0 {
			t.Fatal("erased trace emitted output")
		}
		return
	}
	header := "-------- [spacer] add_reach_fact src/muz/spacer/spacer_context.cpp:410 ---------\n"
	footer := "------------------------------------------------\n"
	want := header + "add_reach_fact: answer-recursive INIT true\n" + footer + header + "add_reach_fact: answer-recursive false\n" + footer
	if got := output.String(); got != want {
		t.Fatalf("source trace = %q, want %q", got, want)
	}
	if strings.Count(output.String(), header) != 2 {
		t.Fatal("wrong trace count")
	}
}
