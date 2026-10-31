package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedASTInspectionReplayBorrowedResults(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\nR\nP 0x41\nC 900\n" +
		"R\nP 0x41\nP 0x45\nC 245\n= 0x46\nR\nP 0x41\nP 0x45\nC 246\n= 0x47\n" +
		"R\nP 0x41\nP 0x45\nC 247\nR\nP 0x41\nP 0x45\nU 0\nC 248\n= 0x48\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nC 249\nR\nP 0x41\nP 0x45\nC 250\n" +
		"R\nP 0x41\nP 0x45\nC 252\n= 0x49\nR\nP 0x41\nP 0x45\nC 253\n" +
		"R\nP 0x41\nP 0x45\nC 254\nR\nP 0x41\nP 0x45\nC 255\nR\nP 0x41\nP 0x45\nC 256\n" +
		"R\nP 0x41\nP 0x45\nC 259\n= 0x50\nR\nP 0x41\nP 0x44\nC 260\n= 0x51\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x44\nP 0x47\nP 0x43\nP 0x48\nP 0x42\nP 0x49\nP 0x45\nP 0x50\nP 0x44\nP 0x51\nC 901\n"
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
	var before int
	if err = r.RegisterCommand(900, "capture-trail", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		before = len(owner.results.astTrail)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(901, "check-inspection", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if len(owner.results.astTrail) != before {
			t.Fatal("inspection changed AST trail")
		}
		for pos := uint32(1); pos <= 12; pos += 2 {
			a, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			b, e := r.astObject(owner, pos+1)
			if e != nil {
				return e
			}
			if a.handle != b.handle {
				t.Fatal("inspection identity", pos, a.handle, b.handle)
			}
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
		t.Fatal("inspection was not checked")
	}
}
