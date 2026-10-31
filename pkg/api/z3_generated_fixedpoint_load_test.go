package api

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGeneratedFixedpointReplayLoadQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "load.smt2")
	if err := os.WriteFile(path, []byte(`(declare-rel S ()) (query S)`), 0600); err != nil {
		t.Fatal(err)
	}
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n" +
		"R\nP 0x41\nP 0x42\nS \"(declare-rel R (Int)) (query R)\"\nC 534\n= 0x43\nR\nP 0x41\nP 0x43\nC 448\n" +
		"R\nP 0x41\nP 0x42\nS " + strconv.Quote(path) + "\nC 535\n= 0x44\nR\nP 0x41\nP 0x44\nC 448\nR\nP 0x41\nP 0x42\nC 513\n" +
		"R\nP 0x41\nP 0x43\nP 0x44\nC 900\nR\nP 0x41\nP 0x43\nC 449\nR\nP 0x41\nP 0x44\nC 449\n"
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
	if err = r.RegisterCommand(900, "check-query-vectors", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for i, want := range []uint32{1, 0} {
			v, e := replayVectorAt(r, owner, uint32(i+1))
			if e != nil {
				return e
			}
			values, e := v.value.Values()
			if e != nil {
				return e
			}
			if len(values) != 1 {
				t.Fatalf("vector %d length %d", i, len(values))
			}
			n, e := owner.results.manager.GetNumArgs(values[0])
			if e != nil {
				return e
			}
			if n != want {
				t.Fatalf("query %d arity %d", i, n)
			}
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
		t.Fatal("vectors not checked")
	}
}

func TestGeneratedFixedpointReplayLoadErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.smt2")
	for _, tc := range []struct {
		name, value string
		command     int
	}{
		{"invalid", "(declare-rel R ()) (query R) (assert", 534},
		{"missing", missing, 535},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nS " + strconv.Quote(tc.value) + "\nC " + strconv.Itoa(tc.command) + "\n= 0x0\nR\nP 0x41\nC 900\n"
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
			if err = r.RegisterCommand(900, "check-load-error", func(r *Z3Replayer) error {
				owner, e := replayContext(r, 0)
				if e != nil {
					return e
				}
				if owner.results.errorCode != 4 {
					t.Fatal("error code", owner.results.errorCode)
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
				t.Fatal("error not checked")
			}
		})
	}
}
