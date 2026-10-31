package api

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
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

func TestGeneratedParserReplayParsesBindingsAndFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "formula.smt2")
	if err := os.WriteFile(path, []byte("(assert p)"), 0600); err != nil {
		t.Fatal(err)
	}
	args := "U 0\ns 0\np 0\nU 1\n$ |p|\ns 1\nP 0x44\np 1\n"
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nU 0\np 0\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 0\np 0\nC 50\n= 0x43\n" +
		"R\nP 0x41\nS \"(assert p)\"\n" + args + "C 335\n= 0x45\n" +
		"R\nP 0x41\nS " + strconv.Quote(path) + "\n" + args + "C 336\n= 0x46\n" +
		"R\nP 0x41\nP 0x43\nP 0x45\nP 0x46\nC 900\n" +
		"R\nP 0x41\nS \"(assert\"\nU 0\ns 0\np 0\nU 0\ns 0\np 0\nC 335\n= 0x47\n" +
		"R\nP 0x41\nP 0x47\nC 901\n" +
		"R\nP 0x41\nS " + strconv.Quote(path+".missing") + "\nU 0\ns 0\np 0\nU 0\ns 0\np 0\nC 336\nC 902\n"
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
	if err = r.RegisterCommand(900, "check-parsed-bindings", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		want, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		for pos := uint32(2); pos <= 3; pos++ {
			got, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			if got.handle != want.handle {
				t.Fatal("parsed formula changed", pos)
			}
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-parse-failure", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		if !owner.results.manager.IsTrue(value.handle) || owner.results.errorCode != 4 || owner.parserError == "" {
			t.Fatal("parse failure state", owner.results.errorCode, owner.parserError)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-missing-parser-file", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if r.result != 0 || owner.results.errorCode != 4 {
			t.Fatal("missing file state", r.result, owner.results.errorCode)
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
		t.Fatal("parse checks", checks)
	}
}

func TestGeneratedParserReplayRejectsUnsupportedBindingSymbol(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\nS \"\"\nU 1\n# 7\ns 1\nP 0x42\np 1\nU 0\ns 0\np 0\nC 335\n"
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
	if err = r.Parse(); err == nil || !strings.Contains(err.Error(), "binding symbols are not supported") {
		t.Fatal("numeric parser binding", err)
	}
}
