package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedFunctionInterpretationReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 54\n= 0x44\nR\nP 0x41\nC 55\n= 0x45\nR\nP 0x41\nC 295\n= 0x46\nR\nP 0x41\nP 0x46\nC 296\n" +
		"R\nP 0x41\nP 0x46\nP 0x43\nP 0x45\nC 312\n= 0x47\nR\nP 0x41\nP 0x47\nC 314\n" +
		"R\nP 0x41\nC 447\n= 0x48\nR\nP 0x41\nP 0x48\nC 448\nR\nP 0x41\nP 0x48\nP 0x44\nC 454\n" +
		"R\nP 0x41\nP 0x47\nP 0x48\nP 0x44\nC 321\nR\nP 0x41\nP 0x47\nP 0x44\nC 319\n" +
		"R\nP 0x41\nP 0x47\nC 316\nR\nP 0x41\nP 0x47\nC 320\nR\nP 0x41\nP 0x47\nC 318\n= 0x49\n" +
		"R\nP 0x41\nP 0x46\nP 0x43\nC 301\n= 0x50\nR\nP 0x41\nP 0x50\nC 314\nR\nP 0x41\nP 0x46\nC 297\n" +
		"R\nP 0x41\nP 0x50\nP 0x44\nP 0x49\nC 900\nR\nP 0x41\nP 0x47\nC 315\nR\nP 0x41\nP 0x50\nC 315\nR\nP 0x41\nP 0x48\nC 449\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-function-interp", func(r *Z3Replayer) error {
		f, e := replayGetFunctionInterpretation(r)
		if e != nil {
			return e
		}
		expected, e := r.astObject(f.owner, 2)
		if e != nil {
			return e
		}
		actual, e := r.astObject(f.owner, 3)
		if e != nil {
			return e
		}
		arity, e := f.value.Arity()
		if e != nil {
			return e
		}
		count, e := f.value.NumEntries()
		if e != nil {
			return e
		}
		otherwise, e := f.value.Else()
		if e != nil {
			return e
		}
		if arity != 1 || count != 1 || expected.handle != actual.handle || otherwise != expected.handle {
			t.Fatal("function interpretation", arity, count, otherwise)
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
		t.Fatal("function interpretation not checked")
	}
}

func TestGeneratedFunctionInterpretationReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x43\nR\nP 0x41\nC 55\n= 0x44\n" +
		"R\nP 0x41\nC 295\n= 0x45\nR\nP 0x41\nP 0x45\nC 296\nR\nP 0x41\nP 0x45\nP 0x43\nP 0x44\nC 312\n= 0x46\nR\nP 0x41\nP 0x46\nC 314\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var f *replayFunctionInterpretation
	for _, value := range r.functions {
		f = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if f == nil || !f.value.closed || !f.object.closed {
		t.Fatal("unreleased function interpretation")
	}
}

func TestGeneratedFunctionEntryReplaySurvivesModelRelease(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 54\n= 0x44\nR\nP 0x41\nC 55\n= 0x45\nR\nP 0x41\nC 295\n= 0x46\nR\nP 0x41\nP 0x46\nC 296\n" +
		"R\nP 0x41\nP 0x46\nP 0x43\nP 0x45\nC 312\n= 0x47\nR\nP 0x41\nP 0x47\nC 314\n" +
		"R\nP 0x41\nC 447\n= 0x48\nR\nP 0x41\nP 0x48\nC 448\nR\nP 0x41\nP 0x48\nP 0x44\nC 454\n" +
		"R\nP 0x41\nP 0x47\nP 0x48\nP 0x45\nC 321\nR\nP 0x41\nP 0x47\nU 0\nC 317\n= 0x49\n" +
		"R\nP 0x41\nP 0x49\nC 322\nR\nP 0x41\nP 0x47\nC 315\nR\nP 0x41\nP 0x46\nC 297\nR\nP 0x41\nP 0x48\nC 449\n" +
		"R\nP 0x41\nP 0x49\nC 325\nR\nP 0x41\nP 0x49\nC 324\n= 0x50\nR\nP 0x41\nP 0x49\nU 0\nC 326\n= 0x51\n" +
		"R\nP 0x41\nP 0x49\nP 0x45\nP 0x50\nP 0x44\nP 0x51\nC 900\nR\nP 0x41\nP 0x49\nC 323\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-function-entry", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		id, e := r.Object(1)
		if e != nil {
			return e
		}
		entry := r.functionEntries[id]
		if entry == nil {
			return ErrZ3ReplayReference
		}
		arity, e := entry.value.NumArguments()
		if e != nil {
			return e
		}
		if arity != 1 {
			t.Fatal("entry arity", arity)
		}
		for pos := uint32(2); pos <= 4; pos += 2 {
			a, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			b, e := r.astObject(owner, pos+1)
			if e != nil {
				return e
			}
			if a.handle != b.handle {
				t.Fatal("entry values", pos)
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
		t.Fatal("entry not checked")
	}
}
