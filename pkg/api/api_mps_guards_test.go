package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSMTContextRunMPSGuardsAndSessionIsolation(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.RunMPS(nil, "nil.mps", nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil MPS reader", err)
	}
	if result, err := f.c.RunMPS(strings.NewReader("not MPS"), "bad.mps", nil); err == nil || !strings.Contains(result.Output, "cannot process bad.mps") {
		t.Fatal("malformed MPS", result.Output, err)
	}
	if _, err := f.c.RunMPSFile("", nil); err == nil {
		t.Fatal("empty MPS path succeeded")
	}
	if _, err := f.m.MkTrue(); err != nil {
		t.Fatal("MPS invocation retired parent session", err)
	}

	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.RunMPSString(apiOneVariableMPS("L", 5, ""), "closed.mps", nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed MPS context", err)
	}
}
