package api

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const booleanCoverage = "(define-sort B () (_ BitVec 1))(declare-fun f (B) Bool)(assert (forall ((x B)) (f x)))"
const booleanSquare = "(declare-const g3_witness_a Bool)(declare-const g3_witness_b Bool)(assert (or g3_witness_a g3_witness_b))(assert (or g3_witness_a (not g3_witness_b)))(assert (or (not g3_witness_a) g3_witness_b))(assert (or (not g3_witness_a) (not g3_witness_b)))"

func TestBooleanProofScopesAndLimits(t *testing.T) {
	c, s := orderSolver(t, booleanCoverage)
	m, err := c.Manager()
	astTestMust(t, err)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	for i := 0; i < 3; i++ {
		astTestMust(t, s.Push())
		astTestMust(t, s.LoadSMT2String(booleanSquare))
		if ok, e := batch.state.prove(1, true); e != nil || ok {
			t.Fatal("step budget", ok, e)
		}
		for j := 0; j < 3; j++ {
			if ok, e := batch.Prove(); e != nil || !ok {
				t.Fatal("Boolean refutation", i, j, ok, e)
			}
			if batch.state.bits == nil || batch.state.bits.checks != j+1 || batch.state.ctx != nil {
				t.Fatal("Boolean batch lifetime", i, j)
			}
		}
		done := make(chan struct{})
		close(done)
		m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
		if ok, e := batch.Prove(); e != nil || ok || batch.state.bits != nil {
			t.Fatal("canceled batch", ok, e)
		}
		m.ResourceLimit().RunSignals = nil
		astTestMust(t, s.Pop(1))
		if ok, e := batch.Prove(); e != nil || ok {
			t.Fatal("leaked clauses", ok, e)
		}
		if ans, e := s.Check(); e != nil || ans != lbool.True {
			t.Fatal("lost model", ans, e)
		}
		if len(m.ResourceLimit().Children) != 0 {
			t.Fatal("registered child after proof")
		}
	}
}

func TestBooleanProofRespectsInferredWidths(t *testing.T) {
	for _, tc := range []struct {
		extra  string
		binary bool
		proved bool
	}{
		{"(declare-const t B)(assert (not (f t)))", true, true},
		{"(declare-fun g ((_ BitVec 2)) Bool)(assert (g #b00))", false, false},
		{"(declare-const a B)(declare-const b B)(declare-const c B)(assert (distinct a b c))", true, true},
		{"(declare-fun q (B B) Bool)(assert (forall ((x B))(exists ((y B))(q x y))))(assert (exists ((a B))(forall ((b B))(not (q a b)))))", true, true},
		{"(declare-fun q (B B) Bool)(assert (forall ((x B))(exists ((y B))(q x y))))(assert (exists ((a B))(exists ((b B))(not (q a b)))))", true, false},
	} {
		c, s := orderSolver(t, booleanCoverage+tc.extra)
		m, e := c.Manager()
		astTestMust(t, e)
		a, e := solver.Assertions(s.solver)
		astTestMust(t, e)
		_, bits, app, e := order.ForProof(m, a, orderProofSteps)
		if e != nil || !app || (bits != nil) != tc.binary {
			t.Fatal(tc.extra, app, bits != nil, e)
		}
		if yes, e := s.ProveUnsatByOrderAbstraction(); e != nil || yes != tc.proved {
			t.Fatal(tc.extra, yes, e)
		}
	}
}

func TestBooleanProofInterruptedLoadAndBatchRollover(t *testing.T) {
	f, err := os.Open("test_vectors/raft_order/onebit.jsonl.gz")
	astTestMust(t, err)
	defer f.Close()
	z, err := gzip.NewReader(f)
	astTestMust(t, err)
	defer z.Close()
	var row [2]string
	astTestMust(t, json.NewDecoder(z).Decode(&row))
	c, s := orderSolver(t, row[1])
	m, e := c.Manager()
	astTestMust(t, e)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	assertions, e := solver.Assertions(s.solver)
	astTestMust(t, e)
	_, k, applicable, e := order.ForProof(m, assertions, orderProofSteps)
	if e != nil || !applicable || k == nil {
		t.Fatal(row[0], applicable, e)
	}
	var reduce control.Reslimit
	if conflict, complete := k.Reduce(16, &reduce); conflict || !complete {
		t.Fatal("expected residual clauses")
	}
	// Allow reduction, variable creation, and permanent rows. The final
	// guarded rows must exhaust the budget before the refutation.
	prefix := len(k.Clauses) - booleanProofTail
	if prefix <= 0 {
		t.Fatal("fixture lacks permanent rows")
	}
	steps := k.Work + uint32(reduce.Count()) + uint32(k.Vars+prefix)
	if ok, e := batch.state.prove(steps, true); ok || e != nil || batch.state.bits != nil {
		t.Fatal("partially loaded proof", ok, e)
	}
	for i := 0; i < booleanProofChecks+3; i++ {
		ok, e := batch.Prove()
		if e != nil || !ok || batch.state.bits == nil {
			t.Fatal(i, ok, e)
		}
		if b := batch.state.bits; b.checks != i%booleanProofChecks+1 {
			t.Fatal(i, b.checks, "retained rows", len(b.prefix))
		}
	}
	if len(m.ResourceLimit().Children) != 0 {
		t.Fatal("registered child")
	}
}
