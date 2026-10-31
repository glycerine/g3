package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedDeclarationReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\n" +
		"R\nP 0x41\n$ |T|\nC 31\n= 0x42\n" +
		"R\nP 0x41\n# 42\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\n" +
		"R\nP 0x41\nS \"fresh\"\nU 1\nP 0x42\np 1\nP 0x42\nC 52\n= 0x46\n" +
		"R\nP 0x41\nS \"fresh\"\nP 0x42\nC 53\n= 0x47\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nP 0x44\nP 0x45\nP 0x46\nP 0x47\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-declarations", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		sort, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		name, e := owner.context.SortName(sort.handle)
		if e != nil || name != "T" {
			t.Fatalf("sort name %q: %v", name, e)
		}
		constant, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		decl, e := owner.context.ApplicationDeclaration(constant.handle)
		if e != nil {
			return e
		}
		sym, e := owner.results.manager.GetASTName(decl)
		if e != nil || !sym.IsNumerical() || sym.Value != 42 {
			t.Fatalf("numeric symbol %v: %v", sym, e)
		}
		fun, e := r.astObject(owner, 3)
		if e != nil {
			return e
		}
		app, e := r.astObject(owner, 4)
		if e != nil {
			return e
		}
		actual, e := owner.context.ApplicationDeclaration(app.handle)
		if e != nil || actual != fun.handle {
			t.Fatalf("application declaration: %v", e)
		}
		arg, e := owner.context.ApplicationArgument(app.handle, 0)
		if e != nil || arg != constant.handle {
			t.Fatalf("application argument: %v", e)
		}
		fresh, e := r.astObject(owner, 5)
		if e != nil {
			return e
		}
		arity, e := owner.context.DeclarationArity(fresh.handle)
		if e != nil || arity != 1 {
			t.Fatalf("fresh function arity %d: %v", arity, e)
		}
		freshConst, e := r.astObject(owner, 6)
		if e != nil {
			return e
		}
		actualSort, e := owner.context.SortOf(freshConst.handle)
		if e != nil || actualSort != sort.handle {
			t.Fatalf("fresh constant sort: %v", e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
