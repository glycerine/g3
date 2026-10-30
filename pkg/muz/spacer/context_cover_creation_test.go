package spacer

// Source: src/muz/spacer/spacer_context.cpp context::add_cover/get_num_levels.

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSpacerCoverCreatesMissingPredicateAndSurvivesRuleUpdate(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.Reset())
	levels, err := c.NumLevels(f.other)
	if err != nil || levels != 0 {
		t.Error("unknown predicate must report zero levels", levels, err)
	}
	falsehood, err := m.MkFalse()
	must(err)
	must(c.AddCover(2, f.other, falsehood))
	created, found, err := c.Transformer(f.other)
	must(err)
	if !found || created == nil {
		t.Fatal("cover did not create its predicate transformer")
	}
	levels, err = c.NumLevels(f.other)
	must(err)
	if levels != 3 {
		t.Fatal("cover did not ensure its finite frame", levels)
	}
	cover, err := c.CoverDelta(2, f.other)
	must(err)
	h, err := m.RefGet(cover)
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("finite cover was not asserted into the new predicate")
	}
	must(m.DelRef(&cover))
	must(c.AddInvariant(f.other, falsehood))
	cover, err = c.CoverDelta(-1, f.other)
	must(err)
	h, err = m.RefGet(cover)
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("invariant did not promote the existing cover")
	}
	must(m.DelRef(&cover))
	must(f.manager.RegisterPredicate(f.other))
	head, err := m.MkApp(f.other, nil)
	must(err)
	rule, err := f.manager.NewRule(head, []ast.ASTHandle{head}, nil, "covered-recursive-predicate")
	must(err)
	defer func() { must(rule.Close()) }()
	must(f.rules.Add(rule))
	must(c.Initialize(f.rules))
	replacement, found, err := c.Transformer(f.other)
	must(err)
	if !found || replacement == created {
		t.Fatal("rule update did not replace the cover-only transformer")
	}
	if _, err := created.Head(); err == nil {
		t.Fatal("cover-only transformer survived replacement")
	}
	cover, err = c.CoverDelta(-1, f.other)
	must(err)
	h, err = m.RefGet(cover)
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("cover was lost while installing the predicate rules")
	}
	must(m.DelRef(&cover))
	must(c.SetQuery(f.other))
	status, err := c.Solve(0, 3)
	must(err)
	if status != lbool.False {
		t.Fatal("covered recursive-only predicate must stay unreachable", status)
	}
}
