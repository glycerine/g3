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

func TestGeneratedFixedpointReplaySpacerExpressionGroundAndTrace(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nC 511\n= 0x43\nR\nP 0x41\nP 0x43\nC 512\nR\nP 0x41\nP 0x43\nC 901\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 517\nR\nP 0x41\nP 0x43\nC 652\n= 0x44\nR\nP 0x41\nP 0x43\nC 653\n= 0x45\nR\nP 0x41\nP 0x45\nC 448\nR\nP 0x41\nP 0x44\nP 0x45\nC 900\nR\nP 0x41\nP 0x45\nC 449\nR\nP 0x41\nP 0x43\nC 513\n"
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
	if err = r.RegisterCommand(901, "set-spacer", func(r *Z3Replayer) error {
		f, e := replayGetFixedpoint(r)
		if e != nil {
			return e
		}
		return f.owner.context.SetFixedpointEngine("spacer")
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-expression-ground-trace", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if _, e = r.astObject(owner, 1); e != nil {
			return e
		}
		v, e := replayVectorAt(r, owner, 2)
		if e != nil {
			return e
		}
		n, e := v.value.Len()
		if e != nil {
			return e
		}
		if n == 0 {
			t.Fatal("empty trace")
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
		t.Fatal("ground answer not checked")
	}
}
