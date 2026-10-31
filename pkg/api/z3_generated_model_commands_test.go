package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedModelReplayEvaluation(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 408\n= 0x42\nR\nP 0x41\nP 0x42\nC 416\n" +
		"R\nP 0x41\nC 32\n= 0x43\nR\nP 0x41\n$ |x|\nP 0x43\nC 51\n= 0x44\n" +
		"R\nP 0x41\nP 0x42\nP 0x44\nC 422\nR\nP 0x41\nP 0x42\nC 427\n" +
		"R\nP 0x41\nP 0x42\nC 431\n= 0x45\nR\nP 0x41\nP 0x45\nC 296\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nI 0\nP 0\nC 298\n* 0x46 4\n" +
		"R\nP 0x41\nP 0x45\nC 302\nR\nP 0x41\nP 0x45\nU 0\nC 303\n= 0x47\n" +
		"R\nP 0x41\nP 0x45\nP 0x47\nC 299\n= 0x48\nR\nP 0x41\nP 0x45\nP 0x47\nC 300\n" +
		"R\nP 0x41\nP 0x45\nC 333\nR\nP 0x41\nP 0x45\nP 0x46\nP 0x48\nC 900\n" +
		"R\nP 0x41\nP 0x45\nC 297\nR\nP 0x41\nP 0x42\nC 417\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "check-evaluation", func(r *Z3Replayer) error {
		m, e := replayGetModel(r)
		if e != nil {
			return e
		}
		for _, pos := range []uint32{2, 3} {
			a, e := r.astObject(m.owner, pos)
			if e != nil {
				return e
			}
			kind, e := m.owner.results.manager.GetDeclKind(a.handle)
			if e != nil || kind != ast.ASTOpTrue {
				t.Fatalf("model result %v: %v", kind, e)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.models) != 0 || len(r.solvers) != 0 {
		t.Fatal("model or solver not cleaned up")
	}
}
func TestGeneratedModelReplayCleansIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 295\n= 0x42\nR\nP 0x41\nP 0x42\nC 296\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedModelCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var m *replayModel
	for _, entry := range r.models {
		m = entry
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if m == nil || !m.value.closed || !m.object.closed {
		t.Fatal("model not closed")
	}
}

func TestGeneratedModelTranslationAndArrayQueries(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\n" +
		"R\nP 0x41\nC 32\n= 0x43\nR\nP 0x41\n$ |x|\nU 0\np 0\nP 0x43\nC 49\n= 0x44\n" +
		"R\nP 0x41\nC 54\n= 0x45\nR\nP 0x41\nC 295\n= 0x46\nR\nP 0x41\nP 0x46\nC 296\n" +
		"R\nP 0x41\nP 0x46\nP 0x44\nP 0x45\nC 313\n" +
		"R\nP 0x41\n$ |U|\nC 31\n= 0x47\nR\nP 0x41\n$ |u|\nP 0x47\nC 51\n= 0x48\n" +
		"R\nP 0x41\nP 0x46\nP 0x47\nP 0x48\nC 900\nR\nP 0x41\nP 0x46\nC 306\n" +
		"R\nP 0x41\nP 0x46\nU 0\nC 307\n= 0x49\nR\nP 0x41\nP 0x47\nP 0x49\nC 901\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x43\np 1\nP 0x43\nC 49\n= 0x50\n" +
		"R\nP 0x41\nP 0x50\nC 135\n= 0x51\nR\nP 0x41\nP 0x51\nC 310\n" +
		"R\nP 0x41\nP 0x51\nC 311\n= 0x52\nR\nP 0x41\nP 0x50\nP 0x52\nC 901\n" +
		"R\nP 0x41\nP 0x46\nP 0x42\nC 309\n= 0x53\nR\nP 0x42\nP 0x53\nC 296\n" +
		"R\nP 0x41\nP 0x46\nC 297\nR\nP 0x41\nC 8\n" +
		"R\nP 0x42\nC 32\n= 0x54\nR\nP 0x42\n$ |x|\nU 0\np 0\nP 0x54\nC 49\n= 0x55\n" +
		"R\nP 0x42\nP 0x53\nP 0x55\nC 299\n= 0x56\nR\nP 0x42\nP 0x56\nC 902\nR\nP 0x42\nP 0x53\nC 297\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "register-universe", func(r *Z3Replayer) error {
		m, e := replayGetModel(r)
		if e != nil {
			return e
		}
		sort, e := r.astObject(m.owner, 2)
		if e != nil {
			return e
		}
		value, e := r.astObject(m.owner, 3)
		if e != nil {
			return e
		}
		return m.value.RegisterUniverse(sort.handle, []ast.ASTHandle{value.handle})
	}); err != nil {
		t.Fatal(err)
	}
	checks := 0
	if err = r.RegisterCommand(901, "check-borrowed", func(r *Z3Replayer) error {
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
		if a.handle != b.handle {
			t.Fatal("unexpected borrowed result", a.handle, b.handle)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-translated-value", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		kind, e := owner.results.manager.GetDeclKind(value.handle)
		if e != nil {
			return e
		}
		if kind != ast.ASTOpTrue {
			t.Fatal("incorrect model translation", kind)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 3 {
		t.Fatal("missing checks", checks)
	}
}
