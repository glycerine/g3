package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedFixedpointReplayMultipleRelationAnswer(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |P|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\n$ |Q|\nU 0\np 0\nP 0x42\nC 49\n= 0x44\nR\nP 0x41\nC 511\n= 0x45\nR\nP 0x41\nP 0x45\nC 512\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 526\nR\nP 0x41\nP 0x45\nP 0x44\nC 526\nR\nP 0x41\nP 0x45\nP 0x43\nU 0\nu 0\nC 515\n" +
		"R\nP 0x41\nP 0x45\nU 2\nP 0x43\nP 0x44\np 2\nC 518\nR\nP 0x41\nP 0x45\nC 519\n= 0x46\nR\nP 0x41\nP 0x46\nC 900\nR\nP 0x41\nP 0x45\nC 513\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-relation-answer", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		m := owner.results.manager
		n, e := m.GetNumArgs(value.handle)
		if e != nil {
			return e
		}
		if n != 2 {
			t.Fatal("answer arity", n)
		}
		a, e := m.GetArg(value.handle, 0)
		if e != nil {
			return e
		}
		b, e := m.GetArg(value.handle, 1)
		if e != nil {
			return e
		}
		if !m.IsTrue(a) || !m.IsFalse(b) {
			t.Fatal("answer terms", a, b)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("answer not checked")
	}
}
