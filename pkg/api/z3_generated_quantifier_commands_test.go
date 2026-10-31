package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedQuantifierReplayBoundPatternAndSymbols(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\n" +
		"R\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x43\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\n" +
		"R\nP 0x41\nU 1\nP 0x45\np 1\nC 190\n= 0x46\n" +
		"R\nP 0x41\nU 5\nU 1\nP 0x46\np 1\nU 1\nP 0x42\np 1\n# 7\ns 1\nP 0x45\nC 192\n= 0x47\n" +
		"R\nP 0x41\nP 0x47\nI 1\nI 5\nI 1\nC 900\n" +
		"R\nP 0x41\nU 3\nU 0\np 0\nU 1\nP 0x42\np 1\n$ |x|\ns 1\nP 0x43\nC 193\n= 0x48\n" +
		"R\nP 0x41\nP 0x48\nI 0\nI 3\nI 0\nC 900\n" +
		"R\nP 0x41\nI 1\nU 4\nU 0\np 0\nU 1\nP 0x42\np 1\n# 7\ns 1\nP 0x43\nC 194\n= 0x49\n" +
		"R\nP 0x41\nP 0x49\nI 1\nI 4\nI 0\nC 900\n" +
		"R\nP 0x41\nI 0\nU 6\n$ |qid|\n# 22\nU 0\np 0\nU 0\np 0\nU 1\nP 0x42\np 1\n# 7\ns 1\nP 0x43\nC 195\n= 0x50\n" +
		"R\nP 0x41\nP 0x50\nI 0\nI 6\nI 0\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-quantifier", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		value, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		forall, e := owner.results.manager.IsForall(value.handle)
		if e != nil {
			return e
		}
		wantForall, e := r.Int(2)
		if e != nil {
			return e
		}
		wantWeight, e := r.Int(3)
		if e != nil {
			return e
		}
		wantPatterns, e := r.Int(4)
		if e != nil {
			return e
		}
		weight, e := owner.results.manager.GetQuantifierWeight(value.handle)
		if e != nil {
			return e
		}
		patterns, e := owner.results.manager.GetNumPatterns(value.handle)
		if e != nil {
			return e
		}
		if forall != (wantForall != 0) || int(weight) != int(wantWeight) || int(patterns) != int(wantPatterns) {
			t.Fatalf("forall=%t weight=%d patterns=%d", forall, weight, patterns)
		}
		name, e := owner.results.manager.GetDeclarationName(value.handle, 0)
		if e != nil {
			return e
		}
		if wantForall != 0 && (!name.IsNumerical() || name.Value != 7) {
			t.Fatalf("numeric binder name lost: %v", name)
		}
		if wantWeight == 6 {
			qid, skid, e := owner.results.manager.GetQuantifierIDs(value.handle)
			if e != nil {
				return e
			}
			q, e := owner.results.manager.Symbols.String(qid)
			if e != nil {
				return e
			}
			if q != "qid" || !skid.IsNumerical() || skid.Value != 22 {
				t.Fatalf("quantifier identifiers: %q %v", q, skid)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedQuantifierReplayRejectsInvalidPattern(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\n" +
		"R\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nC 54\n= 0x43\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\n" +
		"R\nP 0x41\nU 1\nP 0x45\np 1\nC 190\n= 0x46\n" +
		"R\nP 0x41\nU 1\nU 1\nP 0x46\np 1\nU 1\nP 0x42\np 1\n$ |x|\ns 1\nP 0x43\nC 192\n"
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
	if err = r.Parse(); err == nil {
		t.Fatal("pattern without quantified variables was accepted")
	}
}

func TestGeneratedQuantifierReplayZeroBindersReturnsBody(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\n" +
		"R\nP 0x41\nU 1\nU 0\np 0\nU 0\np 0\ns 0\nP 0x42\nC 192\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nC 900\n"
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
	if err = r.RegisterCommand(900, "check-zero-binders", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		body, e := r.astObject(owner, 1)
		if e != nil {
			return e
		}
		result, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if result.handle != body.handle {
			t.Fatal("zero binders changed body")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPatternReplayRejectsBoundVariableTerm(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x43\n" +
		"R\nP 0x41\nU 1\nP 0x43\np 1\nC 190\n"
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
	if err = r.Parse(); err == nil {
		t.Fatal("pattern containing a bare variable was accepted")
	}
}
