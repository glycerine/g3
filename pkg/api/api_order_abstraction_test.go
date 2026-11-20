package api

import (
	"fmt"
	"testing"

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
