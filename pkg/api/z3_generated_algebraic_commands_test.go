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

func TestGeneratedAlgebraicPolynomialReplayVectors(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 34\n= 0x42\nR\nP 0x41\nI 2\nP 0x42\nC 150\n= 0x43\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x44\nR\nP 0x41\nU 2\nP 0x44\nP 0x44\np 2\nC 66\n= 0x45\n" +
		"R\nP 0x41\nU 2\nP 0x45\nP 0x43\np 2\nC 67\n= 0x46\nR\nP 0x41\nP 0x46\nU 0\np 0\nC 485\n= 0x47\n" +
		"R\nP 0x41\nP 0x47\nC 448\nR\nP 0x41\nP 0x46\nU 1\nP 0x43\np 1\nC 486\n" +
		"R\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x48\nR\nP 0x41\nU 2\nP 0x48\nP 0x48\np 2\nC 66\n= 0x49\n" +
		"R\nP 0x41\nU 2\nP 0x49\nP 0x43\np 2\nC 67\n= 0x50\nR\nP 0x41\nP 0x50\nP 0x50\nP 0x48\nC 487\n= 0x51\n" +
		"R\nP 0x41\nP 0x51\nC 448\nR\nP 0x41\nP 0x47\nP 0x51\nP 0x43\nC 900\nR\nP 0x41\nP 0x47\nC 449\nR\nP 0x41\nP 0x51\nC 449\n"
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
	if err = r.RegisterCommand(900, "check-polynomial-vectors", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		roots, e := replayVectorAt(r, owner, 1)
		if e != nil {
			return e
		}
		chain, e := replayVectorAt(r, owner, 2)
		if e != nil {
			return e
		}
		two, e := r.astObject(owner, 3)
		if e != nil {
			return e
		}
		n, e := roots.value.Len()
		if e != nil {
			return e
		}
		if n != 2 {
			t.Fatal("root count", n)
		}
		for i := uint32(0); i < n; i++ {
			root, e := roots.value.At(i)
			if e != nil {
				return e
			}
			square, e := owner.context.AlgebraicPower(root, 2)
			if e != nil {
				return e
			}
			equal, e := owner.context.AlgebraicEqual(square, two.handle)
			if e != nil {
				return e
			}
			if !equal {
				t.Fatal("root square", i)
			}
		}
		n, e = chain.value.Len()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("subresultant count", n)
		}
		zero, e := chain.value.At(0)
		if e != nil {
			return e
		}
		text, e := owner.context.NumeralString(zero)
		if e != nil {
			return e
		}
		if text != "0" {
			t.Fatal("equal polynomial subresultant", text)
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
		t.Fatal("polynomial vectors not checked")
	}
}
