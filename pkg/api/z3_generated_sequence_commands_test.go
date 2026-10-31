package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedSequenceAndStringReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 155\n= 0x43\nR\nP 0x41\nP 0x43\nC 156\n" +
		"R\nP 0x41\nP 0x43\nC 157\n= 0x44\nR\nP 0x41\nP 0x44\nC 158\n" +
		"R\nP 0x41\nC 159\n= 0x45\nR\nP 0x41\nP 0x45\nC 160\n" +
		"R\nP 0x41\nS \"hello\"\nC 161\n= 0x46\nR\nP 0x41\nP 0x46\nC 162\nR\nP 0x41\nP 0x46\nC 163\n" +
		"R\nP 0x41\nP 0x43\nC 164\n= 0x47\nR\nP 0x41\nC 54\n= 0x48\n" +
		"R\nP 0x41\nP 0x48\nC 165\n= 0x49\nR\nP 0x41\nU 2\nP 0x47\nP 0x49\np 2\nC 166\n= 0x50\n" +
		"R\nP 0x41\nP 0x43\nP 0x46\nP 0x49\nP 0x50\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-sequences", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		sort, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		str, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		text, ok, e := owner.context.StringValue(str.handle)
		if e != nil {
			return e
		}
		if !ok || text != "hello" {
			t.Fatal("string replay", text, ok)
		}
		for _, pos := range []uint32{3, 4} {
			v, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			got, e := owner.context.SortOf(v.handle)
			if e != nil {
				return e
			}
			if got != sort.handle {
				t.Fatal("sequence sort", pos, got, sort.handle)
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
		t.Fatal("sequences were not checked")
	}
}
