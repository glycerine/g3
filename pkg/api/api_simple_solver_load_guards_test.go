package api

import (
	"errors"
	"strings"
	"testing"
)

func TestAPISimpleSolverLoadSMT2LifecycleGuards(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	if err = s.LoadSMT2(nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil reader", err)
	}
	astTestMust(t, s.Close())
	if err = s.LoadSMT2(strings.NewReader("(assert true)")); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed solver", err)
	}
	var absent *APISimpleSolver
	if err = absent.LoadSMT2String("(assert true)"); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil solver", err)
	}
}
