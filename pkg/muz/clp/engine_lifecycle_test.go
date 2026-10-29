package clp

import (
	"errors"
	"testing"
)

func TestCLPLifecycleAndStatistics(t *testing.T) {
	if _, e := NewEngine(nil, nil, nil, 0); !errors.Is(e, ErrCLP) {
		t.Fatal(e)
	}
	f := newFixture(t)
	e, x := NewEngine(f.m, f.set, f.solver, 20)
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
	if _, x = e.Answer(); !errors.Is(x, ErrCLP) {
		t.Fatal(x)
	}
}
