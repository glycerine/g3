package api

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Each iteration checks three successive obligations from issue73/ord.ivy on
// a fresh solver. The donor keeps the original, parsed premises and goals live.
func BenchmarkFiniteEnumerationVerification(b *testing.B) {
	benchmarkVerificationGoals(b, "test_vectors/issue73_order_goals.smt2.gz", 3)
}

// Three consecutive sys_live goals share a premise constraining several
// arguments of a relation to unary functions of the same argument.
func BenchmarkEnumerationPredicateDependencies(b *testing.B) {
	benchmarkVerificationGoals(b, "test_vectors/apple_sys_live_goals.smt2.gz", 3)
}

// Three consecutive flash_cubicle goals use finite cache states, but the
// premises quantify over at most two cache identifiers at once.
func BenchmarkNarrowEnumerationVerification(b *testing.B) {
	benchmarkVerificationGoals(b, "test_vectors/flash_cubicle_goals.smt2.gz", 3)
}

func benchmarkVerificationGoals(b *testing.B, fixture string, goals int) {
	c, forms := loadVerificationGoals(b, fixture, goals)
	for _, mode := range []string{"Concrete", "Batch"} {
		b.Run(mode, func(b *testing.B) {
			var private int
			for b.Loop() {
				n, err := checkVerificationSiblings(c, forms, mode == "Batch")
				if err != nil {
					b.Fatal(err)
				}
				private += n
			}
			b.ReportMetric(float64(goals), "goals/op")
			b.ReportMetric(float64(private)/float64(b.N), "private-proofs/op")
		})
	}
}

func loadVerificationGoals(b *testing.B, fixture string, goals int) (*SMTContext, []ast.ASTHandle) {
	b.Helper()
	f, err := os.Open(fixture)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	defer z.Close()
	script, err := io.ReadAll(z)
	if err != nil {
		b.Fatal(err)
	}
	c, err := NewSMTContext()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := c.Close(); err != nil {
			b.Error(err)
		}
	})
	donor, err := c.NewSolver()
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := donor.Close(); err != nil {
			b.Error(err)
		}
	})
	if err = donor.LoadSMT2String(string(script)); err != nil {
		b.Fatal(err)
	}
	forms, err := solver.Assertions(donor.solver)
	if err != nil {
		b.Fatal(err)
	}
	if len(forms) != goals+1 {
		b.Fatalf("got %d assertions, expected one premise and %d goals", len(forms), goals)
	}
	return c, forms
}

func checkVerificationSiblings(c *SMTContext, forms []ast.ASTHandle, useBatch bool) (proved int, err error) {
	s, err := c.NewSolverForIncrementalChecks()
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	params, err := c.NewParameterSet()
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, params.Close()) }()
	if err = params.SetBool("smt.macro_finder", true); err != nil {
		return 0, err
	}
	if err = s.SetParameters(params); err != nil {
		return 0, err
	}
	if err = s.Assert(forms[0]); err != nil {
		return 0, err
	}
	var batch *OrderProofBatch
	if useBatch {
		batch = s.BeginOrderProofBatch()
		defer func() { err = errors.Join(err, batch.Close()) }()
	}
	prePush := batch != nil
	for _, goal := range forms[1:] {
		firstPush := prePush
		if prePush {
			yes, err := batch.ProveWithGoal(goal)
			if err != nil {
				return proved, err
			}
			if yes {
				proved++
				continue
			}
			prePush = false
		}
		if err = s.Push(); err != nil {
			return proved, err
		}
		if err = s.Assert(goal); err != nil {
			return proved, err
		}
		var yes bool
		if batch != nil && !firstPush {
			yes, err = batch.Prove()
			if err != nil {
				return proved, err
			}
		}
		if yes {
			proved++
		} else {
			result, err := s.Check()
			if err != nil {
				return proved, err
			}
			if result != lbool.False {
				return proved, errors.New("expected an unsatisfiable verification condition")
			}
		}
		if err = s.Pop(1); err != nil {
			return proved, err
		}
	}
	return proved, nil
}
