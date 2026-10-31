package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedSetReplayConstructionAndMembership(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 33\n= 0x42\nR\nP 0x41\nI 1\nP 0x42\nC 150\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nC 136\n= 0x44\nR\nP 0x41\nP 0x42\nC 137\n= 0x45\nR\nP 0x41\nP 0x42\nC 138\n= 0x46\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 139\n= 0x47\nR\nP 0x41\nP 0x47\nP 0x43\nC 140\n= 0x48\n" +
		"R\nP 0x41\nU 2\nP 0x45\nP 0x47\np 2\nC 141\n= 0x49\nR\nP 0x41\nU 2\nP 0x46\nP 0x47\np 2\nC 142\n= 0x50\n" +
		"R\nP 0x41\nP 0x47\nP 0x45\nC 143\n= 0x51\nR\nP 0x41\nP 0x45\nC 144\n= 0x52\n" +
		"R\nP 0x41\nP 0x43\nP 0x47\nC 145\n= 0x53\nR\nP 0x41\nP 0x43\nP 0x48\nC 145\n= 0x54\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nC 146\n= 0x55\nR\nP 0x41\nP 0x45\nP 0x46\nC 147\n= 0x56\n" +
		"R\nP 0x41\nP 0x53\nC 287\n= 0x57\nR\nP 0x41\nP 0x54\nC 287\n= 0x58\n" +
		"R\nP 0x41\nP 0x44\nP 0x45\nP 0x46\nP 0x49\nP 0x50\nP 0x51\nP 0x52\nP 0x55\nP 0x56\nP 0x57\nP 0x58\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-set-results", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		values := make([]ast.ASTHandle, 11)
		for i := range values {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			values[i] = v.handle
		}
		if !owner.results.manager.IsTrue(values[9]) || !owner.results.manager.IsFalse(values[10]) {
			t.Fatal("set membership did not simplify")
		}
		sort, e := owner.results.manager.GetSort(values[1])
		if e != nil {
			return e
		}
		if sort != values[0] {
			t.Fatal("set sort mismatch")
		}
		kind, e := owner.results.manager.GetDeclKind(values[8])
		if e != nil {
			return e
		}
		if kind != ast.ASTArrayOpExt {
			t.Fatal("array extensionality kind", kind)
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
		t.Fatal("set results not checked")
	}
}
