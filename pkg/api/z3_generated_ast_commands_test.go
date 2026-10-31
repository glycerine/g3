package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedASTReplayReferenceCountedConstruction(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\nP 0x42\nC 9\nR\nP 0x41\nC 54\n= 0x43\nR\nP 0x41\nP 0x43\nC 9\nR\nP 0x41\nP 0x43\nC 58\n= 0x44\nR\nP 0x41\nP 0x44\nC 9\nR\nP 0x41\nP 0x43\nP 0x44\nC 56\n= 0x45\nR\nP 0x41\nP 0x45\nC 900\nR\nP 0x41\nP 0x42\nC 10\nR\nP 0x41\nP 0x43\nC 10\nR\nP 0x41\nP 0x44\nC 10\nR\nP 0x41\nC 8\n"
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
	if err = r.RegisterCommand(900, "check-equality", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		entry, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		m := owner.results.manager
		op, e := m.GetDeclKind(entry.handle)
		if e != nil {
			return e
		}
		if op != ast.ASTOpEq {
			t.Fatal("wrong AST operator", op)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.asts) != 0 || len(r.contexts) != 0 {
		t.Fatal("replay objects not closed", len(r.asts), len(r.contexts))
	}
}

func TestGeneratedASTReplayCleansIncompleteReferences(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nP 0x42\nC 9\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var context *SMTContext
	for _, owner := range r.contexts {
		context = owner.context
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if context == nil || !context.closed {
		t.Fatal("context not closed")
	}
}

func TestGeneratedASTReplayExpiresUnreferencedLastResult(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\nR\nP 0x41\nP 0x42\nC 9\nR\nP 0x41\nP 0x42\nC 58\n= 0x43\nR\nP 0x41\nC 55\nR\nP 0x41\nP 0x43\n"
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
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	owner, err := replayContext(r, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.astObject(owner, 1); err == nil {
		t.Fatal("unreferenced prior result remained valid")
	}
}
