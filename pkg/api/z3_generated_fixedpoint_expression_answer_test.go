package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedFixedpointReplayExpressionAnswer(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nC 511\n= 0x43\nR\nP 0x41\nP 0x43\nC 512\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 517\nR\nP 0x41\nP 0x43\nC 519\n= 0x44\nR\nP 0x41\nP 0x44\nC 900\nR\nP 0x41\nP 0x43\nC 513\n"
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
	if err = r.RegisterCommand(900, "check-expression-answer", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if !owner.results.manager.IsTrue(value.handle) {
			t.Fatal("answer is not true")
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
