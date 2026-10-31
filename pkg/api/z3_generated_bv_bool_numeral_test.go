package api

import (
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func TestGeneratedBitVectorBoolArrayReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 5\nU 1\nU 1\nU 1\nU 1\nU 1\nu 5\nC 154\n= 0x42\nR\nP 0x41\nP 0x42\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-bool-array-bits", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		n, width, matched, e := owner.results.manager.BitVectorNumeralBigInt(value.handle)
		if e != nil {
			return e
		}
		var bytes [4]byte
		binary.NativeEndian.PutUint32(bytes[:], 1)
		want := uint64(0)
		for i := 0; i < 5; i++ {
			if bytes[i%4] != 0 {
				want |= 1 << i
			}
		}
		if !matched || width != 5 || !n.IsUint64() || n.Uint64() != want {
			t.Fatal("bitvector bool array", n, width, matched, want)
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
		t.Fatal("bitvector numeral not checked")
	}
}

func TestGeneratedBitVectorBoolArrayReplayRejectsShortArray(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nU 2\nU 1\nu 1\nC 154\n"
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
	if err = r.Parse(); err == nil || !strings.Contains(err.Error(), ErrZ3ReplayReference.Error()) {
		t.Fatal("short bool array", err)
	}
}
