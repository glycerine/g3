package api

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGeneratedSolverReplayResetPreservesParameters(t *testing.T) {
	for _, tc := range []struct {
		id   int
		args string
	}{{408, ""}, {409, ""}, {410, "$ |QF_BV|\n"}} {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 13\n= 0x42\nR\nP 0x41\nP 0x42\nC 14\n" +
				"R\nP 0x41\nP 0x42\n$ |timeout|\nU 10000\nC 17\nR\nP 0x41\n" + tc.args + fmt.Sprintf("C %d\n= 0x43\n", tc.id) +
				"R\nP 0x41\nP 0x43\nC 416\nR\nP 0x41\nP 0x43\nP 0x42\nC 415\n" +
				"R\nP 0x41\nP 0x42\n$ |random_seed|\nU 11\nC 17\nR\nP 0x41\nP 0x43\nP 0x42\nC 415\n" +
				"R\nP 0x41\nP 0x42\n$ |random_seed|\nU 22\nC 17\nR\nP 0x41\nC 55\n= 0x44\n" +
				"R\nP 0x41\nP 0x43\nP 0x44\nC 422\nR\nP 0x41\nP 0x43\nC 418\nR\nP 0x41\nP 0x43\nI -1\nC 900\n" +
				"R\nP 0x41\nP 0x43\nC 420\nR\nP 0x41\nP 0x43\nI 1\nC 900\nR\nP 0x41\nP 0x43\nC 417\nR\nP 0x41\nP 0x42\nC 15\nR\nP 0x41\nC 8\n"
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
			if err = r.RegisterCommand(900, "check-reset", func(r *Z3Replayer) error {
				s, e := replayGetSolver(r)
				if e != nil {
					return e
				}
				expected, e := r.Int(2)
				if e != nil {
					return e
				}
				status, e := s.value.Check()
				if e != nil {
					return e
				}
				if (expected < 0 && status != lbool.False) || (expected > 0 && status != lbool.True) {
					t.Fatalf("status %v want %d", status, expected)
				}
				if expected > 0 {
					level, e := s.value.ScopeLevel()
					if e != nil || level != 0 {
						t.Fatalf("scope level %d: %v", level, e)
					}
					current, e := s.value.solver.Params()
					if e != nil {
						return e
					}
					for name, want := range map[string]uint32{"timeout": 10000, "random_seed": 11} {
						got, e := s.params.params.GetUIntName(current, &name, 0)
						if e != nil || got != want {
							t.Fatalf("parameter %s=%d want %d: %v", name, got, want, e)
						}
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
