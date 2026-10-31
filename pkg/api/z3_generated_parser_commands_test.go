package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedParserReplayBenchmarkAndPersistentEvaluation(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\nS \"bench\"\nS \"QF_UF\"\nS \"unknown\"\nS \"\"\nU 1\nP 0x43\np 1\nP 0x43\nC 334\n" +
		"R\nP 0x41\nS \"(declare-const p Bool)(assert p)\"\nC 337\nR\nP 0x41\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-persistent-evaluation", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		out, e := owner.context.EvaluateSMT2("(assert (not p))(check-sat)")
		if e != nil {
			return e
		}
		if strings.TrimSpace(out) != "unsat" {
			t.Fatal("persistent evaluation", out)
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
		t.Fatal("evaluation not checked")
	}
}

func TestGeneratedParserReplayReportsAndClearsError(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"(assert\"\nC 337\nR\nP 0x41\nC 900\nR\nP 0x41\nC 338\nC 901\n" +
		"R\nP 0x41\nU 3\nC 340\nR\nP 0x41\nC 339\nC 902\nR\nP 0\nU 3\nC 341\n"
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
	checks := 0
	for _, entry := range []struct {
		id   uint32
		code uint32
	}{{900, 4}, {901, 0}, {902, 3}} {
		id, code := entry.id, entry.code
		if err = r.RegisterCommand(id, "check-parser-error", func(r *Z3Replayer) error {
			owner, e := replayContext(r, 0)
			if e != nil {
				return e
			}
			if owner.parserError == "" || owner.results.errorCode != code {
				t.Fatal("parser error state", owner.parserError, owner.results.errorCode)
			}
			checks++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 3 {
		t.Fatal("parser error checks", checks)
	}
}
