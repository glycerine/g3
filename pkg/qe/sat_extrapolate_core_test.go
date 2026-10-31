package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATSolverCoreExtrapolation(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	k := qsatKernelOwner(t, f)
	solver, err := k.solver()
	fmMust(t, err)
	p := f.constant("core-p", f.boolean)
	q := f.constant("core-q", f.boolean)
	count := 0
	result, err := satCoreExtrapolate(m, solver, p, []ast.ASTHandle{p, q}, func() error { count++; return nil })
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&result)) }()
	h, err := m.RefGet(result)
	fmMust(t, err)
	if h != p {
		n, e := m.GetNumArgs(h)
		fmMust(t, e)
		if n != 1 {
			t.Fatalf("core has %d assignments, want one", n)
		}
		argument, e := m.GetArg(h, 0)
		fmMust(t, e)
		if argument != p {
			t.Fatal("core selected the wrong assignment")
		}
	}
	if count != 1 {
		t.Fatal("checkpoint count", count)
	}
	if _, err = satCoreExtrapolate(m, solver, p, []ast.ASTHandle{q}, nil); err == nil {
		t.Fatal("accepted assignments that do not imply the formula")
	}
	stopped := errors.New("stop")
	_, err = satCoreExtrapolate(m, solver, p, nil, func() error { return stopped })
	if !errors.Is(err, stopped) {
		t.Fatal("checkpoint failure not propagated", err)
	}
	fmMust(t, solver.Push())
	notP := f.own(m.MkNot(p))
	fmMust(t, solver.AssertFormula(notP))
	status, err := solver.CheckSat(nil)
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("core extrapolation leaked constraints", status)
	}
	fmMust(t, solver.Pop(1))
}
