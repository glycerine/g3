package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedSolverReplayIncremental(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\n" +
		"R\nP 0x41\nC 13\n= 0x42\nR\nP 0x41\nP 0x42\nC 14\n" +
		"R\nP 0x41\nP 0x42\n$ |timeout|\nU 10000\nC 17\n" +
		"R\nP 0x41\nC 408\n= 0x43\nR\nP 0x41\nP 0x43\nC 416\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 415\nR\nP 0x41\nC 32\n= 0x44\n" +
		"R\nP 0x41\n$ |x|\nP 0x44\nC 51\n= 0x45\nR\nP 0x41\nP 0x45\nC 58\n= 0x46\n" +
		"R\nP 0x41\nP 0x43\nP 0x45\nC 422\nR\nP 0x41\nP 0x43\nC 427\nR\nP 0x41\nP 0x43\nI 1\nC 900\n" +
		"R\nP 0x41\nP 0x43\nC 418\nR\nP 0x41\nP 0x43\nP 0x46\nC 422\nR\nP 0x41\nP 0x43\nC 427\nR\nP 0x41\nP 0x43\nI -1\nC 900\n" +
		"R\nP 0x41\nP 0x43\nU 1\nC 419\nR\nP 0x41\nP 0x43\nC 421\nR\nP 0x41\nP 0x43\nU 1\nP 0x46\np 1\nC 428\n" +
		"R\nP 0x41\nP 0x43\nI 1\nC 900\nR\nP 0x41\nP 0x43\nC 436\nR\nP 0x41\nP 0x43\nC 417\nR\nP 0x41\nP 0x42\nC 15\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedASTCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "check-status", func(r *Z3Replayer) error {
		s, e := replayGetSolver(r)
		if e != nil {
			return e
		}
		want, e := r.Int(2)
		if e != nil {
			return e
		}
		status, e := s.value.Check()
		if e != nil {
			return e
		}
		if (want == 1 && status != lbool.True) || (want == -1 && status != lbool.False) {
			t.Fatalf("solver status %v, want %d", status, want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.solvers) != 0 || len(r.parameterSets) != 0 {
		t.Fatal("solver objects not cleaned up")
	}
}

func TestGeneratedSolverReplayCleansIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 409\n= 0x42\nR\nP 0x41\nP 0x42\nC 416\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedSolverCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var s *replaySolver
	for _, entry := range r.solvers {
		s = entry
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if s == nil || !s.value.closed || !s.object.closed {
		t.Fatal("solver was not closed")
	}
}
