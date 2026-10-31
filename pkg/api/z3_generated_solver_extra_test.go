package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedSolverProofReplayOwnsResult(t *testing.T) {
	log := "R\nC 3\n= 0x40\nR\nP 0x40\nS \"proof\"\nS \"true\"\nC 5\n" +
		"R\nP 0x40\nC 6\n= 0x41\nR\nP 0x41\nC 408\n= 0x42\nR\nP 0x41\nP 0x42\nC 416\n" +
		"R\nP 0x41\nC 55\n= 0x43\nR\nP 0x41\nP 0x42\nP 0x43\nC 422\nR\nP 0x41\nP 0x42\nC 427\n" +
		"R\nP 0x41\nP 0x42\nC 432\n= 0x44\nR\nP 0x41\nP 0x42\nC 417\nR\nP 0x41\nP 0x44\nC 900\nR\nP 0x40\nC 4\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedConfigCommands, RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-proof", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		p, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if p.handle.IsNull() {
			t.Fatal("missing proof")
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
		t.Fatal("proof not checked")
	}
}

func TestGeneratedSolverReplayLoadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "formula.smt2")
	if err := os.WriteFile(path, []byte("(declare-const p Bool)\n(assert p)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 408\n= 0x42\nR\nP 0x41\nP 0x42\nC 416\n" +
		fmt.Sprintf("R\nP 0x41\nP 0x42\nS %q\nC 425\n", path) + "R\nP 0x41\nP 0x42\nC 900\nR\nP 0x41\nP 0x42\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-file-assertions", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		refs, e := s.value.Assertions()
		if e != nil {
			return e
		}
		defer func() {
			for i := range refs {
				if e := s.owner.results.manager.DelRef(&refs[i]); e != nil {
					t.Error(e)
				}
			}
		}()
		if len(refs) != 1 {
			t.Fatal("file assertions", len(refs))
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
		t.Fatal("file assertions not checked")
	}
}

func TestGeneratedSolverFromTacticReplaySurvivesReset(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"smt\"\nC 363\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n" +
		"R\nP 0x41\nP 0x42\nC 411\n= 0x43\nR\nP 0x41\nP 0x43\nC 416\nR\nP 0x41\nP 0x42\nC 365\n" +
		"R\nP 0x41\nP 0x43\nC 413\nR\nP 0x41\nP 0x43\nC 420\nR\nP 0x41\nC 55\n= 0x44\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 422\nR\nP 0x41\nP 0x43\nC 427\nR\nP 0x41\nP 0x43\nC 900\nR\nP 0x41\nP 0x43\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-tactic-solver", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.False {
			t.Fatal("reset tactic solver status", status)
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
		t.Fatal("tactic solver not checked")
	}
}
