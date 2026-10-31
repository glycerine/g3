package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedAlgebraicReplayArithmeticAndRelations(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 34\n= 0x42\nR\nP 0x41\nI 2\nP 0x42\nC 150\n= 0x43\n" +
		"R\nP 0x41\nI 3\nP 0x42\nC 150\n= 0x44\nR\nP 0x41\nP 0x43\nC 468\nR\nP 0x41\nP 0x43\nC 469\n" +
		"R\nP 0x41\nP 0x43\nC 470\nR\nP 0x41\nP 0x43\nC 471\nR\nP 0x41\nP 0x43\nC 472\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 473\n= 0x45\nR\nP 0x41\nP 0x43\nP 0x44\nC 474\n= 0x46\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 475\n= 0x47\nR\nP 0x41\nP 0x43\nP 0x44\nC 476\n= 0x48\n" +
		"R\nP 0x41\nP 0x43\nU 2\nC 477\n= 0x49\nR\nP 0x41\nP 0x49\nU 2\nC 478\n= 0x50\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 479\nR\nP 0x41\nP 0x43\nP 0x44\nC 480\nR\nP 0x41\nP 0x43\nP 0x44\nC 481\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 482\nR\nP 0x41\nP 0x43\nP 0x44\nC 483\nR\nP 0x41\nP 0x43\nP 0x44\nC 484\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nP 0x48\nP 0x50\nP 0x43\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-algebraic-results", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		want := []string{"5", "-1", "6", "2/3"}
		for i, w := range want {
			value, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			got, e := owner.context.NumeralString(value.handle)
			if e != nil {
				return e
			}
			if got != w {
				t.Fatal("algebraic value", i, got, w)
			}
		}
		square, e := r.astObject(owner, 5)
		if e != nil {
			return e
		}
		two, e := r.astObject(owner, 6)
		if e != nil {
			return e
		}
		equal, e := owner.context.AlgebraicEqual(square.handle, two.handle)
		if e != nil {
			return e
		}
		if !equal {
			t.Fatal("square root did not square to two")
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
		t.Fatal("algebraic results not checked")
	}
}
