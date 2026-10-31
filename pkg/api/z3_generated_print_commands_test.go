package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedASTPrintReplayContextModes(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\n" +
		"R\nP 0x41\n$ |x|\nP 0x43\nC 51\n= 0x44\nR\nP 0x41\n$ |f|\nU 1\nP 0x43\np 1\nP 0x43\nC 49\n= 0x45\n" +
		"R\nP 0x41\nP 0x45\nU 1\nP 0x44\np 1\nC 50\n= 0x46\nR\nP 0x41\nU 1\nP 0x46\np 1\nC 190\n= 0x47\n" +
		"R\nP 0x41\nP 0x46\nC 329\nR\nP 0x41\nP 0x47\nC 330\nR\nP 0x41\nP 0x43\nC 331\nR\nP 0x41\nP 0x45\nC 332\n" +
		"R\nP 0x41\nU 1\nC 328\nR\nP 0x41\nP 0x46\nC 329\nR\nP 0x41\nP 0x43\nC 331\n" +
		"R\nP 0x41\nP 0x42\nC 900\nR\nP 0x41\nU 2\nC 328\nR\nP 0x41\nP 0x45\nC 332\n"
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
	if err = r.RegisterCommand(900, "check-print-modes", func(r *Z3Replayer) error {
		a, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		b, e := replayContext(r, 1)
		if e != nil {
			return e
		}
		if a.printMode != 1 || b.printMode != 0 {
			t.Fatal("print modes are not context scoped", a.printMode, b.printMode)
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
		t.Fatal("print mode not checked")
	}
}
