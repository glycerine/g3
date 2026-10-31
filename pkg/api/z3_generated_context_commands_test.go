package api

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGeneratedContextReplayCopiesConfigAndCloses(t *testing.T) {
	log := "R\nC 3\n= 0x41\nR\nP 0x41\nS \"proof\"\nS \"true\"\nC 5\nR\nP 0x41\nC 6\n= 0x42\nR\nP 0x41\nC 4\nR\nP 0x42\nC 900\nR\nP 0x42\nS \"timeout\"\nS \"5\"\nC 11\nR\nP 0x42\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedConfigCommands, RegisterZ3GeneratedContextCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "check-context", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		m, e := owner.context.Manager()
		if e != nil {
			return e
		}
		proof, e := m.ProofsEnabled()
		if e != nil {
			return e
		}
		if !proof || owner.userRefCount {
			t.Fatal("context configuration", proof, owner.userRefCount)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.contexts) != 0 || len(r.configs) != 0 {
		t.Fatal("live replay objects", len(r.contexts), len(r.configs))
	}
	if _, err = replayContext(r, 0); !errors.Is(err, ErrZ3ReplayReference) {
		t.Fatal("deleted context accepted", err)
	}
}

func TestGeneratedContextReplayReferenceModeAndInterrupt(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x51\nR\nP 0x51\nC 12\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	var context *SMTContext
	if err = r.RegisterCommand(900, "check-interrupt", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		context = owner.context
		m, e := context.Manager()
		if e != nil {
			return e
		}
		if !owner.userRefCount || !m.ResourceLimit().CancelFlag() {
			t.Fatal("reference mode or interrupt")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if context == nil || !context.closed {
		t.Fatal("unreleased context remains open")
	}
}
