package api

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func orderSolver(t *testing.T, input string) (*SMTContext, *APISimpleSolver) {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	s, err := c.NewSolver()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.LoadSMT2String(input))
	return c, s
}

const orderPremises = `(define-sort B () (_ BitVec 2))
(declare-fun p (B) Bool)
(declare-const a B)
(assert (forall ((x B)) (p x)))`

func TestOrderAbstractionPreservesConcreteDecisions(t *testing.T) {
	tests := []struct {
		name, input string
		proved      bool
		concrete    lbool.LBool
	}{
		{"application", orderPremises + "(assert (not (p a)))", true, lbool.False},
		{"satisfiable", orderPremises + "(assert (p a))", false, lbool.True},
		{"unsigned minimum", orderPremises + "(assert (= (p a) (bvult a #b00)))", true, lbool.False},
		{"conditional result", orderPremises + "(declare-const b B)(assert (not (p (ite (p a) a b))))", true, lbool.False},
		{"nested binders", `(define-sort B () (_ BitVec 3))(declare-fun q (B B) Bool)
(assert (forall ((x B)) (exists ((y B)) (q x y))))
(assert (exists ((x B)) (forall ((y B)) (not (q x y)))))`, true, lbool.False},
		{"finite cardinality", orderPremises + `(declare-const b B)(declare-const c B)(declare-const d B)(declare-const e B)
(assert (distinct a b c d e))`, false, lbool.False},
		{"different widths", `(declare-fun f ((_ BitVec 2)) (_ BitVec 3))
(declare-fun q ((_ BitVec 3)) Bool)(assert (forall ((x (_ BitVec 2))) (q (f x))))
(assert (not (q (f #b10))))`, true, lbool.False},
		{"only whole conjuncts may be dropped", orderPremises + "(assert (not (and (p a) (bvult #b11 #b00))))", false, lbool.True},
		{"addition", orderPremises + "(assert (not (p (bvadd a #b01))))", false, lbool.False},
		{"signed comparison", orderPremises + "(assert (= (p a) (bvslt a #b00)))", false, lbool.True},
		{"ordinary Boolean theory", "(declare-const p Bool)(assert p)(assert (not p))", false, lbool.False},
		{"source symbol collisions", `(declare-fun lt0 ((_ BitVec 2)) Bool)
(assert (forall ((f0 (_ BitVec 2))) (lt0 f0)))(declare-const v0 (_ BitVec 2))(assert (not (lt0 v0)))`, true, lbool.False},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, s := orderSolver(t, tc.input)
			yes, err := s.ProveUnsatByOrderAbstraction()
			if err != nil || yes != tc.proved {
				t.Fatalf("order proof = %v, %v; want %v", yes, err, tc.proved)
			}
			result, err := s.Check()
			if err != nil || result != tc.concrete {
				t.Fatalf("concrete check = %v, %v; want %v", result, err, tc.concrete)
			}
			if result == lbool.True {
				model, err := s.Model()
				if err != nil || model == nil {
					t.Fatalf("missing concrete model, %v", err)
				}
				astTestMust(t, model.Close())
			}
		})
	}
}

func TestOrderAbstractionUnsignedComparisons(t *testing.T) {
	for _, op := range []string{"bvult", "bvugt", "bvule", "bvuge"} {
		for _, pair := range [][2]int{{0, 3}, {2, 2}, {3, 1}} {
			a, b := pair[0], pair[1]
			truth := map[string]bool{"bvult": a < b, "bvugt": a > b, "bvule": a <= b, "bvuge": a >= b}[op]
			t.Run(fmt.Sprintf("%s_%d_%d", op, a, b), func(t *testing.T) {
				_, s := orderSolver(t, orderPremises+fmt.Sprintf("(assert (= (p a) (%s #b%02b #b%02b)))", op, a, b))
				yes, err := s.ProveUnsatByOrderAbstraction()
				if err != nil || yes == truth {
					t.Fatal("proof", yes, "comparison", truth, err)
				}
			})
		}
	}
}

func TestOrderAbstractionScopesAndLastModel(t *testing.T) {
	_, s := orderSolver(t, orderPremises)
	if res, err := s.Check(); err != nil || res != lbool.True {
		t.Fatal(res, err)
	}
	first, err := s.Model()
	astTestMust(t, err)
	defer first.Close()
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || yes {
		t.Fatal(yes, err)
	}
	model, err := s.Model()
	if model == nil || err != nil {
		t.Fatal("lost prior model", err)
	}
	astTestMust(t, model.Close())
	astTestMust(t, s.Push())
	// Keep the contradiction connected to a source symbol so the relaxation
	// retains it. Parser scopes are independent across LoadSMT2 calls.
	astTestMust(t, s.LoadSMT2String("(declare-const tag Bool)(assert tag)(assert (not tag))"))
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || !yes {
		t.Fatal(yes, err)
	}
	astTestMust(t, s.Pop(1))
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || yes {
		t.Fatal(yes, err)
	}
	if res, err := s.Check(); err != nil || res != lbool.True {
		t.Fatal(res, err)
	}
}

func TestOrderAbstractionLimits(t *testing.T) {
	c, s := orderSolver(t, orderPremises+"(assert (not (p a)))")
	if yes, err := s.proveUnsatByOrderAbstraction(1); err != nil || yes {
		t.Fatal("resource-bounded attempt", yes, err)
	}
	m, err := c.Manager()
	astTestMust(t, err)
	if len(m.ResourceLimit().Children) != 0 {
		t.Fatal("child registration survived proof attempt")
	}
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || !yes {
		t.Fatal("normal retry", yes, err)
	}
	astTestMust(t, c.Interrupt())
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || yes {
		t.Fatal("interrupt", yes, err)
	}
	m.ResourceLimit().ResetCancel()
	p, err := c.NewParameterSet()
	astTestMust(t, err)
	defer p.Close()
	astTestMust(t, p.SetUInt("timeout", 1))
	astTestMust(t, s.SetParameters(p))
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || yes {
		t.Fatal("timeout", yes, err)
	}
}

func TestOrderProofBatchSeparatesQueries(t *testing.T) {
	c, s := orderSolver(t, orderPremises)
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	m, err := c.Manager()
	astTestMust(t, err)
	done := make(chan struct{})
	m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
	defer func() { m.ResourceLimit().RunSignals = nil }()

	astTestMust(t, s.Push())
	astTestMust(t, s.LoadSMT2String("(declare-const conflict Bool)(declare-const alt Bool)(assert (or conflict alt))(assert (not alt))(assert (not conflict))"))
	if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || !yes {
		t.Fatal("single proof", yes, err)
	}
	var retained *APISimpleSolver
	for i := 0; i < 2; i++ {
		if yes, err := batch.Prove(); err != nil || !yes {
			t.Fatal("batch proof", i, yes, err)
		}
		child := batch.state.child
		if child == nil || i > 0 && child != retained {
			t.Fatal("missing retained kernel")
		}
		retained = child
		childManager, err := batch.state.ctx.Manager()
		astTestMust(t, err)
		level, err := child.ScopeLevel()
		astTestMust(t, err)
		assertions, err := solver.Assertions(child.solver)
		astTestMust(t, err)
		if level != 1 || len(assertions) != strings.Count(batch.state.premises, "\n") {
			t.Fatal("unexpected retained premises", level, len(assertions))
		}
		parsed, err := batch.state.parser.Assertions()
		astTestMust(t, err)
		if len(parsed) != 0 || len(m.ResourceLimit().Children) != 0 || childManager.ResourceLimit().RunSignals != nil {
			t.Fatal("completed query retained parsed assertions or cancellation registration")
		}
	}
	astTestMust(t, s.Pop(1))
	if yes, err := batch.Prove(); err != nil || yes || batch.state.ctx != nil {
		t.Fatal("satisfiable successor", yes, err)
	}
	if answer, err := s.Check(); err != nil || answer != lbool.True {
		t.Fatal("concrete successor", answer, err)
	}

	astTestMust(t, s.Push())
	astTestMust(t, s.LoadSMT2String("(declare-const second Bool)(assert second)(assert (not second))"))
	if yes, err := batch.Prove(); err != nil || !yes {
		t.Fatal("retry", yes, err)
	}
	if yes, err := batch.state.prove(1, true); err != nil || yes || batch.state.ctx != nil {
		t.Fatal("exhausted proof", yes, err)
	}
	if yes, err := batch.Prove(); err != nil || !yes {
		t.Fatal("retry after exhaustion", yes, err)
	}
	close(done)
	if yes, err := batch.Prove(); err != nil || yes || batch.state.ctx != nil {
		t.Fatal("cancelled proof", yes, err)
	}
}

func TestOrderProofBatchClosesWithEitherOwnerFirst(t *testing.T) {
	for _, closeSource := range []bool{false, true} {
		t.Run(fmt.Sprintf("source_first_%t", closeSource), func(t *testing.T) {
			c, s := orderSolver(t, orderPremises+"(assert (not (p a)))")
			m, err := c.Manager()
			astTestMust(t, err)
			if yes, err := s.ProveUnsatByOrderAbstraction(); err != nil || !yes {
				t.Fatal(yes, err)
			}
			base := m.Budget().Used()
			b := s.BeginOrderProofBatch()
			copyOfBatch := *b
			if yes, err := b.Prove(); err != nil || !yes {
				t.Fatal(yes, err)
			}
			if closeSource {
				astTestMust(t, s.Close())
				if yes, err := b.Prove(); yes || !errors.Is(err, ErrSMTContextOwner) {
					t.Fatal("closed source", yes, err)
				}
			}
			astTestMust(t, b.Close())
			astTestMust(t, copyOfBatch.Close())
			if yes, err := copyOfBatch.Prove(); yes || !errors.Is(err, ErrSMTContextOwner) {
				t.Fatal("closed batch", yes, err)
			}
			if used := m.Budget().Used(); used > base {
				t.Fatalf("private kernel retained %d excess budget bytes", used-base)
			}
		})
	}
}
