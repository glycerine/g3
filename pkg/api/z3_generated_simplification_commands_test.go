package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedSimplificationReplayAndDescriptions(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\n$ |y|\nP 0x42\nC 51\n= 0x44\n" +
		"R\nP 0x41\nC 54\n= 0x45\nR\nP 0x41\nU 2\nP 0x43\nP 0x45\np 2\nC 63\n= 0x46\n" +
		"R\nP 0x41\nP 0x46\nC 287\n= 0x47\nR\nP 0x41\nP 0x43\nP 0x47\nC 900\n" +
		"R\nP 0x41\nC 13\n= 0x48\nR\nP 0x41\nP 0x48\nC 14\n" +
		"R\nP 0x41\nP 0x48\n$ |elim_and|\nI 1\nC 16\n" +
		"R\nP 0x41\nU 2\nP 0x43\nP 0x44\np 2\nC 63\n= 0x49\n" +
		"R\nP 0x41\nP 0x49\nP 0x48\nC 288\n= 0x50\nR\nP 0x41\nP 0x50\nC 901\n" +
		"R\nP 0x41\nC 289\nR\nP 0x41\nC 290\n= 0x51\nR\nP 0x41\nP 0x51\nC 22\n" +
		"R\nP 0x41\nP 0x51\nC 902\nR\nP 0x41\nP 0x51\nC 23\nR\nP 0x41\nP 0x48\nC 15\n"
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
	if err = RegisterZ3GeneratedParameterCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-default-simplify", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		x, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		got, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if x.handle != got.handle {
			t.Fatal("Boolean identity not simplified")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-configured-simplify", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		got, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		family, e := owner.results.manager.GetFamilyID(got.handle)
		if e != nil {
			return e
		}
		kind, e := owner.results.manager.GetDeclKind(got.handle)
		if e != nil {
			return e
		}
		if family != owner.results.manager.BasicFamilyID() || kind != ast.ASTOpNot {
			t.Fatal("elim_and not applied", family, kind)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-simplify-descriptions", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		d, e := replayDescriptions(r, 1, owner)
		if e != nil {
			return e
		}
		kind, e := d.value.Kind("elim_and")
		if e != nil {
			return e
		}
		if kind != ParameterBool {
			t.Fatal("elim_and descriptor", kind)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
