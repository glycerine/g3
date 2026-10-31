package clp_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/clp"
	"testing"
)

func TestCLPLifecycleAndStatistics(t *testing.T) {
	if _, e := clp.NewEngine(nil, nil, nil, 0); !errors.Is(e, clp.ErrCLP) {
		t.Fatal(e)
	}
	f := newFixture(t)
	e, x := clp.NewEngine(f.m, f.set, f.solver, 20)
	if x != nil {
		t.Fatal(x)
	}
	if x = e.ResetStatistics(); x != nil {
		t.Fatal(x)
	}
	if x = e.Close(); x != nil {
		t.Fatal(x)
	}
	if x = e.Close(); x != nil {
		t.Fatal(x)
	}
	if _, x = e.Answer(); !errors.Is(x, clp.ErrCLP) {
		t.Fatal(x)
	}
}
