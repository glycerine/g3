package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedASTVectorReplayOperationsAndTranslation(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\n" +
		"R\nP 0x41\n$ |x|\nP 0x43\nC 51\n= 0x44\nR\nP 0x41\n$ |y|\nP 0x43\nC 51\n= 0x45\n" +
		"R\nP 0x41\nC 447\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\nR\nP 0x41\nP 0x46\nP 0x44\nC 454\n" +
		"R\nP 0x41\nP 0x46\nU 3\nC 453\nR\nP 0x41\nP 0x46\nU 2\nC 451\n" +
		"R\nP 0x41\nP 0x46\nU 1\nP 0x45\nC 452\nR\nP 0x41\nP 0x46\nU 2\nC 453\n" +
		"R\nP 0x41\nP 0x46\nC 450\nR\nP 0x41\nP 0x46\nU 1\nC 451\n= 0x47\nR\nP 0x41\nP 0x46\nC 456\n" +
		"R\nP 0x41\nP 0x46\nP 0x45\nP 0x47\nC 900\nR\nP 0x41\nP 0x46\nP 0x42\nC 455\n= 0x48\n" +
		"R\nP 0x42\nP 0x48\nC 448\nR\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nC 8\n" +
		"R\nP 0x42\nP 0x48\nU 1\nC 451\n= 0x49\nR\nP 0x42\nP 0x48\nP 0x49\nC 901\nR\nP 0x42\nP 0x48\nC 449\n"
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
	if err = r.RegisterCommand(900, "check-vector", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		a, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		b, e := r.astObject(v.owner, 3)
		if e != nil {
			return e
		}
		size, e := v.value.Len()
		if e != nil {
			return e
		}
		if size != 2 || a.handle != b.handle {
			t.Fatal("vector contents", size, a.handle, b.handle)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-translated-vector", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		a, e := r.astObject(v.owner, 2)
		if e != nil {
			return e
		}
		size, e := v.value.Len()
		if e != nil {
			return e
		}
		text, e := v.owner.context.ASTString(a.handle, APIASTPrintSMT2)
		if e != nil {
			return e
		}
		if size != 2 || text != "y" {
			t.Fatal("translated vector", size, text)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatal("missing vector checks", checks)
	}
}

func TestGeneratedASTVectorReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 447\n= 0x42\nR\nP 0x41\nP 0x42\nC 448\n"
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
	var vector *replayASTVector
	for _, v := range r.vectors {
		vector = v
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if vector == nil || vector.value.owner != nil || !vector.object.closed {
		t.Fatal("unreleased vector not closed")
	}
}
