package api

import (
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each iteration creates a fresh concrete context, loads the original
// assertions, forces the incremental path used by Ivy's push, and verifies
// the result. An earlier iteration cannot supply a learned lemma or model.
func BenchmarkRaftOrderAbstraction(b *testing.B) {
	for _, goal := range []string{"001", "274"} {
		input, err := os.ReadFile("test_vectors/raft_order/" + goal + ".smt2")
		if err != nil {
			b.Fatal(err)
		}
		for _, mode := range []string{"Concrete", "Order"} {
			b.Run(goal+"/"+mode, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if err := benchRaftOrderCheck(b, string(input), mode == "Order"); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func benchRaftOrderCheck(b *testing.B, input string, abstract bool) error {
	c, err := NewSMTContext()
	if err != nil {
		return err
	}
	defer c.Close()
	s, err := c.NewSolver()
	if err != nil {
		return err
	}
	defer s.Close()
	p, err := c.NewParameterSet()
	if err != nil {
		return err
	}
	defer p.Close()
	if err = p.SetBool("smt.macro_finder", true); err != nil {
		return err
	}
	if err = s.SetParameters(p); err != nil {
		return err
	}
	if err = s.LoadSMT2String(input); err != nil {
		return err
	}
	if err = s.Push(); err != nil {
		return err
	}
	if abstract {
		proved, err := s.ProveUnsatByOrderAbstraction()
		if err == nil && !proved {
			b.Fatal("order relaxation failed to prove the goal")
		}
		return err
	}
	answer, err := s.Check()
	if err == nil && answer != lbool.False {
		b.Fatalf("concrete check returned %v", answer)
	}
	return err
}
