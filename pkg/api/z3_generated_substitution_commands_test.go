package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedUpdateAndSubstitutionReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\n$ |y|\nP 0x42\nC 51\n= 0x44\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 56\n= 0x45\nR\nP 0x41\nP 0x44\nP 0x43\nC 56\n= 0x46\n" +
		"R\nP 0x41\nP 0x45\nU 2\nP 0x44\nP 0x43\np 2\nC 291\n= 0x47\nR\nP 0x41\nP 0x46\nP 0x47\nC 900\n" +
		"R\nP 0x41\nP 0x45\nU 2\nP 0x43\nP 0x44\np 2\nP 0x44\nP 0x43\np 2\nC 292\n= 0x48\nR\nP 0x41\nP 0x46\nP 0x48\nC 900\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x49\nR\nP 0x41\nU 1\nP 0x42\nC 191\n= 0x50\n" +
		"R\nP 0x41\nP 0x49\nP 0x50\nC 56\n= 0x51\n" +
		"R\nP 0x41\nP 0x51\nU 2\nP 0x43\nP 0x44\np 2\nC 293\n= 0x52\nR\nP 0x41\nP 0x45\nP 0x52\nC 900\n"
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
	checks := 0
	if err = r.RegisterCommand(900, "check-rewrite", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		want, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		got, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if want.handle != got.handle {
			t.Fatal("wrong rewritten AST", want.handle, got.handle)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 3 {
		t.Fatal("missing checks", checks)
	}
}

func TestGeneratedTranslateASTReplayOwnsTargetResult(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\n" +
		"R\nP 0x41\nC 32\n= 0x43\nR\nP 0x41\n$ |x|\nP 0x43\nC 51\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nC 58\n= 0x45\nR\nP 0x41\nP 0x45\nP 0x42\nC 294\n= 0x46\n" +
		"R\nP 0x41\nC 8\nR\nP 0x42\nC 32\n= 0x47\n" +
		"R\nP 0x42\n$ |x|\nP 0x47\nC 51\n= 0x48\nR\nP 0x42\nP 0x48\nC 58\n= 0x49\n" +
		"R\nP 0x42\nP 0x46\nP 0x49\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-translated", func(r *Z3Replayer) error {
		target, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		got, e := r.astObject(target, 1)
		if e != nil {
			return e
		}
		want, e := r.astObject(target, 2)
		if e != nil {
			return e
		}
		if got.handle != want.handle {
			t.Fatal("wrong translated AST", got.handle, want.handle)
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
		t.Fatal("translation was not checked")
	}
}
