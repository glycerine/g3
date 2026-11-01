package api

import (
	"fmt"
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
		"R\nP 0x41\nI 0\nU 6\n$ |qid|\n# 22\nU 0\np 0\nU 1\nP 0x43\np 1\nU 1\nP 0x42\np 1\n# 7\ns 1\nP 0x43\nC 195\n= 0x50\n" +
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
			noPatterns, x := owner.results.manager.GetNumNoPatterns(value.handle)
			if x != nil || noPatterns != 1 {
				return fmt.Errorf("extended quantifier no-pattern count %d: %w", noPatterns, x)
			}
			noPattern, x := owner.results.manager.GetNoPattern(value.handle, 0)
			if x != nil {
				return x
			}
			body, x := owner.results.manager.GetQuantifierBody(value.handle)
			if x != nil {
				return x
			}
			if noPattern != body {
				return fmt.Errorf("extended quantifier no-pattern %v differs from body %v", noPattern, body)
			}
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

func TestGeneratedConstantQuantifierReplay(t *testing.T) {
	cases := []struct {
		id             uint32
		forall         bool
		patterned      bool
		prefix, suffix string
	}{
		{196, true, true, "U 5\n", ""},
		{197, false, true, "U 5\n", ""},
		{198, true, true, "I 1\nU 5\n", ""},
		{199, false, false, "I 0\nU 5\n# 22\n$ |skid|\n", "U 1\nP 0x45\np 1\n"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\n" +
				"R\nP 0x41\nC 32\n= 0x42\n" +
				"R\nP 0x41\n# 37\nP 0x42\nC 51\n= 0x43\n" +
				"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
				"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\n" +
				"R\nP 0x41\nU 1\nP 0x45\np 1\nC 190\n= 0x46\n" +
				"R\nP 0x41\n" + tc.prefix + "U 1\nP 0x43\np 1\n" + func() string {
				if tc.patterned {
					return "U 1\nP 0x46\np 1\n"
				}
				return "U 0\np 0\n"
			}() + tc.suffix + "P 0x45\n" + fmt.Sprintf("C %d\n= 0x47\n", tc.id) +
				"R\nP 0x41\nP 0x47\nC 900\n"
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
			if err = r.RegisterCommand(900, "check-constant-quantifier", func(r *Z3Replayer) error {
				owner, e := replayContext(r, 0)
				if e != nil {
					return e
				}
				q, e := r.astObject(owner, 1)
				if e != nil {
					return e
				}
				m := owner.results.manager
				forall, e := m.IsForall(q.handle)
				if e != nil {
					return e
				}
				weight, e := m.GetQuantifierWeight(q.handle)
				if e != nil {
					return e
				}
				name, e := m.GetDeclarationName(q.handle, 0)
				if e != nil {
					return e
				}
				if forall != tc.forall || weight != 5 || !name.IsNumerical() || name.Value != 37 {
					t.Fatalf("forall=%t weight=%d name=%v", forall, weight, name)
				}
				body, e := m.GetQuantifierBody(q.handle)
				if e != nil {
					return e
				}
				arg, e := m.GetArg(body, 0)
				if e != nil {
					return e
				}
				index, e := m.GetVarIndex(arg)
				if e != nil || index != 0 {
					t.Fatalf("abstracted body index %d: %v", index, e)
				}
				if tc.patterned {
					pattern, x := m.GetPattern(q.handle, 0)
					if x != nil {
						return x
					}
					term, x := m.GetArg(pattern, 0)
					if x != nil {
						return x
					}
					arg, x = m.GetArg(term, 0)
					if x != nil {
						return x
					}
					index, x = m.GetVarIndex(arg)
					if x != nil || index != 0 {
						t.Fatalf("abstracted pattern index %d: %v", index, x)
					}
				}
				if tc.id == 199 {
					noPatterns, e := m.GetNumNoPatterns(q.handle)
					if e != nil || noPatterns != 1 {
						return fmt.Errorf("extended constant quantifier no-pattern count %d: %w", noPatterns, e)
					}
					noPattern, e := m.GetNoPattern(q.handle, 0)
					if e != nil {
						return e
					}
					if noPattern != body {
						return fmt.Errorf("abstracted no-pattern %v differs from body %v", noPattern, body)
					}
					arg, e := m.GetArg(noPattern, 0)
					if e != nil {
						return e
					}
					index, e := m.GetVarIndex(arg)
					if e != nil || index != 0 {
						return fmt.Errorf("abstracted no-pattern variable index %d: %w", index, e)
					}
					qid, skid, e := m.GetQuantifierIDs(q.handle)
					if e != nil {
						return e
					}
					text, e := m.Symbols.String(skid)
					if e != nil {
						return e
					}
					if !qid.IsNumerical() || qid.Value != 22 || text != "skid" {
						t.Fatal("extended ids", qid, text)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err = r.Parse(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratedConstantQuantifierReplayRejectsEmptyBounds(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 54\n= 0x42\n" +
		"R\nP 0x41\nU 1\nU 0\np 0\nU 0\np 0\nP 0x42\nC 196\n"
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
		t.Fatal("constant quantifier accepted an empty bound list")
	}
}
