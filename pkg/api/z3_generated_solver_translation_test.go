package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedDirectSolverTranslationAcrossContexts(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\n" +
		"R\nP 0x41\n$ |translation-p|\nP 0x43\nC 51\n= 0x44\nR\nP 0x41\nC 409\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\n" +
		"R\nP 0x41\nC 13\n= 0x47\nR\nP 0x41\nP 0x47\nC 14\nR\nP 0x41\nP 0x47\n$ |random_seed|\nU 17\nC 17\nR\nP 0x41\nP 0x45\nP 0x47\nC 415\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nC 422\nR\nP 0x41\nP 0x45\nP 0x42\nC 412\n= 0x46\nR\nP 0x42\nP 0x46\nC 416\nR\nP 0x41\nC 8\n" +
		"R\nP 0x42\nP 0x46\nC 900\nR\nP 0x42\nP 0x46\nC 420\nR\nP 0x42\nP 0x46\nC 901\nR\nP 0x42\nP 0x46\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	checks := 0
	if err = r.RegisterCommand(900, "check-translated-solver", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		n, e := s.value.solver.NumAssertions()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("assertions", n)
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.True {
			t.Fatal("status", status)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-translated-reset-parameters", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		p, e := s.value.solver.Params()
		if e != nil {
			return e
		}
		globals, e := s.owner.context.command.GlobalParameters()
		if e != nil {
			return e
		}
		key := "random_seed"
		seed, e := globals.Context.GetUIntName(p, &key, 0)
		if e != nil {
			return e
		}
		if seed != 17 {
			t.Fatal("reset seed", seed)
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
		t.Fatal("checks", checks)
	}
}

func TestGeneratedStrategicSolverTranslationWithinContext(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\nP 0x43\nC 58\n= 0x44\n" +
		"R\nP 0x41\nC 408\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\nR\nP 0x41\nP 0x45\nP 0x43\nC 422\nR\nP 0x41\nP 0x45\nP 0x41\nC 412\n= 0x46\n" +
		"R\nP 0x41\nP 0x46\nC 416\nR\nP 0x41\nP 0x45\nC 417\nR\nP 0x41\nP 0x46\nP 0x44\nC 422\nR\nP 0x41\nP 0x46\nC 900\nR\nP 0x41\nP 0x46\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-strategic-translation", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.False {
			t.Fatal("status", status)
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
		t.Fatal("translation not checked")
	}
}

func TestGeneratedStrategicSolverTranslationAcrossContexts(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\nR\nP 0x41\n$ |p|\nP 0x43\nC 51\n= 0x44\n" +
		"R\nP 0x41\nC 408\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\nR\nP 0x41\nP 0x45\nP 0x44\nC 422\n" +
		"R\nP 0x41\nP 0x45\nP 0x42\nC 412\n= 0x46\nR\nP 0x42\nP 0x46\nC 416\nR\nP 0x41\nC 8\nR\nP 0x42\nP 0x46\nC 900\nR\nP 0x42\nP 0x46\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-cross-context-strategy", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		if s.kind != 0 {
			t.Fatal("factory kind", s.kind)
		}
		n, e := s.value.solver.NumAssertions()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("assertions", n)
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.True {
			t.Fatal("status", status)
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
		t.Fatal("translation not checked")
	}
}

func TestGeneratedFiniteDomainSolverTranslationAcrossContexts(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\n$ |QF_FD|\nC 410\n= 0x43\nR\nP 0x41\nP 0x43\nC 416\n" +
		"R\nP 0x41\nC 55\n= 0x44\nR\nP 0x41\nP 0x43\nP 0x44\nC 422\nR\nP 0x41\nP 0x43\nC 427\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 412\n= 0x45\nR\nP 0x42\nP 0x45\nC 416\nR\nP 0x41\nC 8\nR\nP 0x42\nP 0x45\nC 900\nR\nP 0x42\nP 0x45\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-finite-translation", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.False {
			t.Fatal("status", status)
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
		t.Fatal("translation not checked")
	}
}

func TestGeneratedSMTTacticSolverTranslationAcrossContexts(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nS \"smt\"\nC 363\n= 0x43\nR\nP 0x41\nP 0x43\nC 364\n" +
		"R\nP 0x41\nP 0x43\nC 411\n= 0x44\nR\nP 0x41\nP 0x44\nC 416\nR\nP 0x41\nP 0x43\nC 365\nR\nP 0x41\nC 55\n= 0x45\nR\nP 0x41\nP 0x44\nP 0x45\nC 422\n" +
		"R\nP 0x41\nP 0x44\nP 0x42\nC 412\n= 0x46\nR\nP 0x42\nP 0x46\nC 416\nR\nP 0x41\nC 8\nR\nP 0x42\nP 0x46\nC 900\n" +
		"R\nP 0x42\nP 0x46\nC 420\nR\nP 0x42\nC 55\n= 0x47\nR\nP 0x42\nP 0x46\nP 0x47\nC 422\nR\nP 0x42\nP 0x46\nC 900\nR\nP 0x42\nP 0x46\nC 417\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	checks := 0
	if err = r.RegisterCommand(900, "check-tactic-translation", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		if s.kind != 3 || s.tactic == nil {
			t.Fatal("lost target tactic")
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if status != lbool.False {
			t.Fatal("status", status)
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
		t.Fatal("checks", checks)
	}
}
