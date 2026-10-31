package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedTacticRegistryReplayDescriptions(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 391\nR\nP 0x41\nU 0\nC 392\n" +
		"R\nP 0x41\nC 393\nR\nP 0x41\nU 0\nC 394\nR\nP 0x41\nC 377\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n" +
		"R\nP 0x41\nP 0x42\nC 395\nR\nP 0x41\nP 0x42\nC 396\n= 0x43\nR\nP 0x41\nP 0x43\nC 22\n" +
		"R\nP 0x41\nS \"skip\"\nC 397\nR\nP 0x41\nS \"size\"\nC 398\nR\nP 0x41\nP 0x43\nC 900\n" +
		"R\nP 0x41\nP 0x43\nC 23\nR\nP 0x41\nP 0x42\nC 365\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedASTCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-tactic-descriptions", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		d, e := replayDescriptions(r, 1, owner)
		if e != nil {
			return e
		}
		if _, e = d.value.Size(); e != nil {
			return e
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
		t.Fatal("descriptions not checked")
	}
}
