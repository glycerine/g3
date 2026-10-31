package tactic

// Source: src/tactic/goal.cpp:402-410 and src/ast/ast_smt2_pp.cpp.

import (
	"io"
	"testing"
)

func TestGoalDisplayReleasesTemporaryFormatTree(t *testing.T) {
	m := astTestManager(t)
	goalDisplayBindParams(t, m)
	g := goalTestNew(t, m, false, false)
	p := goalTestAtom(t, m, "format-p")
	astTestMust(t, g.Assert(p))
	f, err := m.GetFormatManager()
	astTestMust(t, err)
	doc, err := m.SMT2PPExprTree(p, nil)
	astTestMust(t, err)
	ref, err := f.NewRef(doc)
	astTestMust(t, err)
	astTestMust(t, f.DelRef(&ref))
	before := f.Stats()
	for i := 0; i < 3; i++ {
		astTestMust(t, g.Display(io.Discard))
		after := f.Stats()
		if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
			t.Fatal("goal display left format tree alive", before, after)
		}
	}
}
