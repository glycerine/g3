package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedPseudoBooleanReplaySignedCoefficients(t *testing.T) {
	prefix := "R\nP 0x41\nU 2\nP 0x43\nP 0x44\np 2\n"
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |a|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\n$ |b|\nP 0x42\nC 51\n= 0x44\n" +
		prefix + "U 1\nC 222\n= 0x45\n" + prefix + "U 1\nC 223\n= 0x46\n" +
		prefix + "U 18446744073709551613\nU 2\nu 2\nI -1\nC 224\n= 0x47\n" +
		prefix + "U 18446744073709551613\nU 2\nu 2\nI -1\nC 225\n= 0x48\n" +
		prefix + "U 18446744073709551613\nU 2\nu 2\nI -1\nC 226\n= 0x49\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nP 0x45\nP 0x46\nP 0x47\nP 0x48\nP 0x49\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-pseudo-boolean", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		a, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		b, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		args := []ast.ASTHandle{a.handle, b.handle}
		coeffs := []int64{-3, 2}
		constructors := []func() (ast.ASTHandle, error){
			func() (ast.ASTHandle, error) { return owner.context.PseudoBooleanAtMost(args, 1) },
			func() (ast.ASTHandle, error) { return owner.context.PseudoBooleanAtLeast(args, 1) },
			func() (ast.ASTHandle, error) { return owner.context.PseudoBooleanLE(args, coeffs, -1) },
			func() (ast.ASTHandle, error) { return owner.context.PseudoBooleanGE(args, coeffs, -1) },
			func() (ast.ASTHandle, error) { return owner.context.PseudoBooleanEQ(args, coeffs, -1) },
		}
		for i, makeExpected := range constructors {
			got, e := r.astObject(owner, uint32(i+3))
			if e != nil {
				return e
			}
			want, e := makeExpected()
			if e != nil {
				return e
			}
			if got.handle != want {
				t.Fatal("pseudo-Boolean result", i, got.handle, want)
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
		t.Fatal("pseudo-Boolean results were not checked")
	}
}

func TestGeneratedPseudoBooleanReplayRejectsShortCoefficients(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\n" +
		"R\nP 0x41\nU 2\nP 0x42\nP 0x42\np 2\nU 1\nu 1\nI 1\nC 224\n"
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
		t.Fatal("short coefficient array", err)
	}
}
