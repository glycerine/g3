package spacer

// Source: spacer_context.cpp derivation::create_next_child returns before
// changing the active premise when its strengthened transition is unreachable.

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
)

func TestDerivationNextChildRetriesAfterUnreachableSummary(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	p, m := f.context.transformers[f.query], f.context.manager
	truth, err := m.MkTrue()
	depthMust(t, err)
	falsity, err := m.MkFalse()
	depthMust(t, err)
	root, err := p.NewProofObligation(nil, 2, 0, truth, nil)
	depthMust(t, err)
	d, err := NewDerivation(root, f.rule, truth, nil)
	depthMust(t, err)
	defer func() { depthMust(t, d.Close()) }()
	depthMust(t, d.AddPremise(p, 0, truth, false, nil))
	depthMust(t, d.AddPremise(p, 1, falsity, false, nil))
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	first, err := d.CreateFirstChild(md.Core())
	depthMust(t, err)
	if first == nil {
		t.Fatal("missing first child")
	}
	transition, err := d.Transition()
	depthMust(t, err)
	child, err := d.CreateNextChild()
	depthMust(t, err)
	if child != nil || d.active != 0 || d.premises[0].Must() {
		t.Fatal("unreachable later summary advanced derivation")
	}
	after, err := d.Transition()
	depthMust(t, err)
	if after != transition {
		t.Fatal("unreachable summary changed transition")
	}
	summary, err := d.premises[0].Summary()
	depthMust(t, err)
	if summary != truth {
		t.Fatal("unreachable summary changed active premise")
	}
	depthMust(t, d.premises[1].SetSummary(truth, false, nil))
	child, err = d.CreateNextChild()
	depthMust(t, err)
	if child == nil || d.active != 1 || !d.premises[0].Must() {
		t.Fatal("repaired summary did not advance derivation")
	}
	summary, err = d.premises[0].Summary()
	depthMust(t, err)
	if !m.IsTrue(summary) {
		t.Fatal("active summary does not match selected initial reach fact")
	}
	child, err = d.CreateNextChild()
	depthMust(t, err)
	if child != nil {
		t.Fatal("exhausted derivation returned another child")
	}
}
