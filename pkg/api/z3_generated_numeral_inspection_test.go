package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedNumeralInspectionReplayOutputs(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 34\n= 0x42\nR\nP 0x41\nS \"-7/3\"\nP 0x42\nC 148\n= 0x43\n" +
		"R\nP 0x41\nP 0x43\nC 257\nR\nP 0x41\nP 0x43\nC 261\nR\nP 0x41\nP 0x43\nU 3\nC 262\n" +
		"R\nP 0x41\nP 0x43\nC 263\n= 0x44\nR\nP 0x41\nP 0x43\nC 264\n= 0x45\n" +
		"R\nP 0x41\nP 0x43\nI 0\nI 0\nC 265\nR\nP 0x41\nP 0x43\nI 0\nI 0\nC 270\n" +
		"R\nP 0x41\nP 0x43\nI 0\nI 0\nC 900\nR\nP 0x41\nP 0x43\nI 0\nI 0\nC 901\n" +
		"R\nP 0x41\nC 33\n= 0x46\nR\nP 0x41\nS \"-7\"\nP 0x46\nC 148\n= 0x47\n" +
		"R\nP 0x41\nP 0x47\nI 0\nC 902\nR\nP 0x41\nP 0x47\nI 0\nC 903\n" +
		"R\nP 0x41\nP 0x47\nU 42\nC 904\nR\nP 0x41\nP 0x47\nU 42\nC 905\n" +
		"R\nP 0x41\nP 0x44\nP 0x45\nC 906\n"
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
	for _, entry := range []struct{ test, command uint32 }{{900, 265}, {901, 270}, {902, 266}, {903, 269}, {904, 267}, {905, 268}} {
		command := entry.command
		if err = r.RegisterCommand(entry.test, "check-numeral-output", func(r *Z3Replayer) error {
			if e := replayNumeralInspection(r, command); e != nil {
				return e
			}
			if command == 267 || command == 268 {
				v, e := r.Uint64(2)
				if e != nil {
					return e
				}
				if v != 42 {
					t.Fatal("failed conversion changed output", v)
				}
			} else {
				v, e := r.Int64(2)
				if e != nil {
					return e
				}
				if v != -7 {
					t.Fatal("numerator or integer", command, v)
				}
				if command == 265 || command == 270 {
					d, e := r.Int64(3)
					if e != nil {
						return e
					}
					if d != 3 {
						t.Fatal("denominator", d)
					}
				}
			}
			checks++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(906, "check-rational-parts", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos, want := range map[uint32]string{1: "-7", 2: "3"} {
			v, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			got, e := owner.context.NumeralString(v.handle)
			if e != nil {
				return e
			}
			if got != want {
				t.Fatal("rational part", pos, got)
			}
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 7 {
		t.Fatal("missing numeral checks", checks)
	}
}
