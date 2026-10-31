package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATLevelsInitializeAndClose(t *testing.T) {
	f := qsatKernelFixture(t)
	m := f.r.manager
	p := f.constant("levels-p", f.boolean)
	notP := f.own(m.MkNot(p))
	count := 0
	levels, err := newSATLevels(m, f.r.config, nil, 2, p, func() error { count++; return nil })
	fmMust(t, err)
	if len(levels.levels) != 3 || count != 1 {
		t.Fatalf("levels=%d checkpoints=%d", len(levels.levels), count)
	}
	for i, level := range levels.levels {
		h, e := m.RefGet(level.formula)
		fmMust(t, e)
		if i < 2 && !m.IsTrue(h) {
			t.Fatal("initial level is not true")
		}
		if i == 2 && h != p {
			t.Fatal("last level lost formula")
		}
		fmMust(t, level.solver.Push())
		fmMust(t, level.solver.AssertFormula(notP))
		status, e := level.solver.CheckSat(nil)
		fmMust(t, e)
		want := lbool.True
		if i == 2 {
			want = lbool.False
		}
		if status != want {
			t.Fatalf("level %d status %v", i, status)
		}
		fmMust(t, level.solver.Pop(1))
	}
	fmMust(t, levels.Close())
	fmMust(t, levels.Close())
	stopped := errors.New("stop")
	_, err = newSATLevels(m, f.r.config, nil, 1, p, func() error { return stopped })
	if !errors.Is(err, stopped) {
		t.Fatal("checkpoint failure", err)
	}
}
