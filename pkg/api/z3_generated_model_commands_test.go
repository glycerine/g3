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
