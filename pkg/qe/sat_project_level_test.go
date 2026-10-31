package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSATPartialLevelProjectionUpdatesPredecessor(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("project-level-p", f.boolean)
	q := f.constant("project-level-q", f.boolean)
	formula := f.own(m.MkAnd(p, q))
	truth := f.own(m.MkTrue())
	notQ := f.own(m.MkNot(q))
	levels, err := newSATLevels(m, f.r.config, nil, 1, formula, nil)
	fmMust(t, err)
	defer func() { fmMust(t, levels.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	fmMust(t, projectSATPartialLevel(levels, 0, []ast.ASTHandle{p}, formula, f.r.config, rw, nil, false))
	blocked, err := levels.check(0, q)
	fmMust(t, err)
	defer func() { fmMust(t, blocked.Close()) }()
	if blocked.sat {
		t.Fatal("projected successor was not blocked")
	}
	allowed, err := levels.check(0, notQ)
	fmMust(t, err)
	defer func() { fmMust(t, allowed.Close()) }()
	if !allowed.sat {
		t.Fatal("unrelated predecessor assignment was blocked")
	}
	h, err := m.RefGet(levels.levels[0].formula)
	fmMust(t, err)
	if m.IsTrue(h) {
		t.Fatal("predecessor formula was not strengthened")
	}
	if err = projectSATPartialLevel(levels, 1, nil, truth, f.r.config, rw, nil, false); err == nil {
		t.Fatal("invalid level accepted")
	}
}
