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

func TestGeneratedSequenceOperationReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"abc\"\nC 161\n= 0x42\n" +
		"R\nP 0x41\nS \"b\"\nC 161\n= 0x43\nR\nP 0x41\nC 33\n= 0x44\n" +
		"R\nP 0x41\nI 1\nP 0x44\nC 150\n= 0x45\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nC 167\n= 0x46\nR\nP 0x41\nP 0x42\nP 0x43\nC 168\n= 0x47\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nC 169\n= 0x48\nR\nP 0x41\nP 0x42\nP 0x45\nP 0x45\nC 170\n= 0x49\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nP 0x43\nC 171\n= 0x50\nR\nP 0x41\nP 0x42\nP 0x45\nC 172\n= 0x51\n" +
		"R\nP 0x41\nP 0x42\nC 173\n= 0x52\nR\nP 0x41\nP 0x42\nP 0x43\nP 0x45\nC 174\n= 0x53\n" +
		"R\nP 0x41\nP 0x42\nC 175\n= 0x54\nR\nP 0x41\nP 0x45\nC 176\n= 0x55\n" +
		"R\nP 0x41\nP 0x42\nC 177\n= 0x56\nR\nP 0x41\nP 0x42\nP 0x56\nC 178\n= 0x57\n" +
		"R\nP 0x41\nP 0x42\nP 0x46\nP 0x47\nP 0x48\nP 0x49\nP 0x50\nP 0x51\nP 0x52\nP 0x53\nP 0x54\nP 0x56\nP 0x57\nC 900\n" +
		"R\nP 0x41\nP 0x45\nP 0x55\nC 901\n"
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
	for _, item := range []struct {
		id  uint32
		end uint32
	}{{900, 12}, {901, 2}} {
		end := item.end
		if err = r.RegisterCommand(item.id, "check-sequence-operands", func(r *Z3Replayer) error {
			owner, e := replayContext(r, 0)
			if e != nil {
				return e
			}
			input, e := r.astObject(owner, 1)
			if e != nil {
				return e
			}
			for pos := uint32(2); pos <= end; pos++ {
				result, e := r.astObject(owner, pos)
				if e != nil {
					return e
				}
				got, e := owner.results.manager.GetArg(result.handle, 0)
				if e != nil {
					return e
				}
				if got != input.handle {
					t.Fatal("sequence operation first argument", pos, got, input.handle)
				}
			}
			checks++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 2 {
		t.Fatal("missing checks", checks)
	}
}
