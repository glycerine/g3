package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverNNFWeakeningAndScope(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	c, err := newSATSolverContext(m, solver, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, c.Close()) }()
	rw, err := rewriter.NewTheoryRewriter(m, f.r.config.Context, f.r.config.Globals, nil)
	fmMust(t, err)
	defer func() { fmMust(t, rw.Close()) }()
	p := f.constant("weaken-p", f.boolean)
	q := f.constant("weaken-q", f.boolean)
	b := f.own(m.MkAnd(p, q))
	count := 0
	result, err := satWeaken(m, c.normalizer, solver, rw, p, b, func() error { count++; return nil })
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	if h != q {
		t.Fatal("expected q after weakening", h, q)
	}
	if count != 2 {
		t.Fatalf("got %d checkpoints", count)
	}
	stopped := errors.New("stop")
	_, err = satWeaken(m, c.normalizer, solver, rw, p, b, func() error { return stopped })
	if !errors.Is(err, stopped) {
		t.Fatal("checkpoint failure not propagated", err)
	}
	fmMust(t, solver.Push())
	notP := f.own(m.MkNot(p))
	fmMust(t, solver.AssertFormula(notP))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("weakening leaked assertions", status)
	}
	fmMust(t, solver.Pop(1))
}
