package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSMTContextRunOPBGuardsAndSessionIsolation(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.RunOPB(nil, nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil OPB reader", err)
	}
	if _, err := f.c.RunOPB(strings.NewReader("1 x1 >= ;"), nil); err == nil {
		t.Fatal("malformed OPB succeeded")
	}
	if _, err := f.c.RunOPBFile("", nil); err == nil {
		t.Fatal("empty OPB path succeeded")
	}
	if _, err := f.m.MkTrue(); err != nil {
		t.Fatal("OPB invocation retired parent session", err)
	}

	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.RunOPBString("1 x1 >= 1;", nil); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed OPB context", err)
	}
}
