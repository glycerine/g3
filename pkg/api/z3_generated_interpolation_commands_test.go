package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedInterpolationReplayContextAndMarker(t *testing.T) {
	log := "R\nC 3\n= 0x40\nR\nP 0x40\nS \"proof\"\nS \"false\"\nC 5\n" +
		"R\nP 0x40\nC 564\n= 0x41\nR\nP 0x40\nC 6\n= 0x42\nR\nP 0x40\nC 4\n" +
		"R\nP 0x41\nC 54\n= 0x43\nR\nP 0x41\nP 0x43\nC 563\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nP 0x43\nC 900\nR\nP 0x42\nC 901\n" +
		"R\nP 0x41\nC 8\nR\nP 0x42\nC 8\nR\nP 0\nC 564\n= 0x45\nR\nP 0x45\nC 901\nR\nP 0x45\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedConfigCommands, RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checks := 0
	if err = r.RegisterCommand(900, "check-interpolation-marker", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		marker, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		input, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		kind, e := owner.results.manager.GetDeclKind(marker.handle)
		if e != nil {
			return e
		}
		arg, e := owner.results.manager.GetArg(marker.handle, 0)
		if e != nil {
			return e
		}
		if kind != ast.ASTOpInterp || arg != input.handle {
			t.Fatal("marker differs from input")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-interpolation-proof-mode", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		enabled, e := owner.results.manager.ProofsEnabled()
		if e != nil {
			return e
		}
		if !enabled {
			t.Fatal("interpolation config did not enable proofs")
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
		t.Fatal("proof-mode checks", checks)
	}
}
