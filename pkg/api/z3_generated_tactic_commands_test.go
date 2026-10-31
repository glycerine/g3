package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedTacticReplayLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"skip\"\nC 363\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n" +
		"R\nP 0x41\nC 377\n= 0x43\nR\nP 0x41\nP 0x43\nC 364\nR\nP 0x41\nC 378\n= 0x44\nR\nP 0x41\nP 0x44\nC 364\n" +
		"R\nP 0x41\nC 380\n= 0x45\nR\nP 0x41\nP 0x45\nC 364\nR\nP 0x41\nP 0x42\nC 900\n" +
		"R\nP 0x41\nP 0x42\nC 365\nR\nP 0x41\nP 0x43\nC 365\nR\nP 0x41\nP 0x44\nC 365\nR\nP 0x41\nP 0x45\nC 365\nR\nP 0x41\nC 8\n"
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
	var checked *replayTactic
	if err = r.RegisterCommand(900, "check-tactic", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		checked, e = replayTacticAt(r, owner, 1)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checked == nil || !checked.value.closed || len(r.tactics) != 0 {
		t.Fatal("tactic cleanup")
	}
}

func TestGeneratedTacticReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 377\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var value *replayTactic
	for _, tactic := range r.tactics {
		value = tactic
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if value == nil || !value.value.closed || !value.object.closed {
		t.Fatal("unreleased tactic not closed")
	}
}
