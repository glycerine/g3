package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func checkObservedProof(t *testing.T, c *SMTContext, s *APISimpleSolver, input []ast.ASTHandle, want bool) {
	t.Helper()
	m, err := c.Manager()
	astTestMust(t, err)
	bits, err := order.ProjectGroundEqualities(m, input, 80_000)
	if bits == nil || err != nil {
		t.Fatalf("projection failed, error %v", err)
	}
	state := orderProofState{source: s}
	got, err := state.checkBooleanInput(m, bits, 80_000, input)
	astTestMust(t, state.release())
	if err != nil || got != want {
		t.Fatalf("proved %v, want %v, error %v", got, want, err)
	}
}

func TestObservedUniversalInstances(t *testing.T) {
	const decls = groundEqualityDecls + "(declare-sort I 0)(declare-const i I)(declare-fun pair (I D) Bool)"
	for _, tc := range []struct {
		name, premise, facts string
		proved               bool
	}{
		{"Boolean definition", "(forall ((x D)) (= (q x) (ite (= x c) true (p x))))", "(assert (= c a))(assert (not (q a)))", true},
		{"distinct binder sorts", "(forall ((x D) (y I)) (= (pair y x) (p (f x))))", "(assert (p (f a)))(assert (not (pair i a)))", true},
		{"nested application", "(forall ((x D)) (p (f x)))", "(assert (not (p (f a))))", true},
		{"whole implication", "(forall ((x D)) (=> (p x) (q x)))", "(assert (p a))(assert (not (q a)))", true},
		{"false antecedent", "(forall ((x D)) (=> (p x) (q x)))", "(assert (not (p a)))(assert (not (q a)))", false},
		{"repeated variable", "(forall ((x D)) (r x x))", "(assert (distinct a b))(assert (not (r a b)))", false},
		{"matching repeated variable", "(forall ((x D)) (r x x))", "(assert (not (r a a)))", true},
		{"ground argument retained", "(forall ((x D)) (r a x))", "(assert (distinct a b))(assert (not (r b b)))", false},
		{"recursive definition", "(forall ((x D)) (= (p x) (p (f x))))", "(assert (p a))(assert (not (p b)))", false},
		{"missing binder", "(forall ((x D) (y D)) (or (p x) (q y)))", "(assert (p a))(assert (not (q b)))", false},
		{"existential", "(exists ((x D)) (p x))", "(assert (not (p a)))", false},
		{"negated universal", "(not (forall ((x D)) (p x)))", "(assert (not (p a)))", false},
		{"guarded universal", "(or guard (forall ((x D)) (p x)))", "(assert guard)(assert (not (p a)))", false},
		{"nested binder", "(forall ((x D)) (exists ((y D)) (r x y)))", "(assert (not (r a a)))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, decls+"(assert "+tc.premise+")"+tc.facts)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			checkObservedProof(t, c, s, roots, tc.proved)
			checkObservedProof(t, c, s, roots[1:], false)
			checkObservedProof(t, c, s, roots, tc.proved)
		})
	}
}

// Each function, predicate, guard and named input is interpreted in a known
// two-element model. Changing the observed output must refute that model and
// every other interpretation that agrees with its function and predicate tables.
func TestObservedUniversalFiniteTables(t *testing.T) {
	const rule = "(assert (forall ((x D)) (= (q x) (ite guard (p (f x)) (p x)))))"
	var script strings.Builder
	script.WriteString(groundEqualityDecls + rule)
	atom := func(s string, truth bool) string {
		if truth {
			return s
		}
		return "(not " + s + ")"
	}
	names := [2]string{"a", "b"}
	for mask := range 64 {
		f := [2]int{mask & 1, mask >> 1 & 1}
		p := [2]bool{mask&4 != 0, mask&8 != 0}
		guard, input := mask&16 != 0, mask>>5&1
		fmt.Fprintf(&script, "(assert (and (distinct a b) (= c %s) (= (f a) %s) (= (f b) %s) %s %s %s))\n",
			names[input], names[f[0]], names[f[1]], atom("(p a)", p[0]), atom("(p b)", p[1]), atom("guard", guard))
		want := p[input]
		if guard {
			want = p[f[input]]
		}
		fmt.Fprintf(&script, "(assert %s)(assert %s)\n", atom("(q c)", want), atom("(q c)", !want))
	}
	c, s := orderSolver(t, script.String())
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if len(roots) != 1+3*64 {
		t.Fatalf("unexpected assertion count %d", len(roots))
	}
	for i := 1; i < len(roots); i += 3 {
		t.Run(fmt.Sprint((i-1)/3), func(t *testing.T) {
			for outcome := range 2 {
				checkObservedProof(t, c, s, []ast.ASTHandle{roots[0], roots[i], roots[i+1+outcome]}, outcome == 1)
			}
		})
	}
}

// The only occurrence of k at a concrete argument comes from instantiating q.
// The chosen branch depends on a predicate and on equality in a two-element
// domain. Vary both argument values and every entry in the function tables.
func TestObservedChainedConditionals(t *testing.T) {
	var script strings.Builder
	script.WriteString(groundEqualityDecls + `
(declare-fun k (D) D)
(assert (forall ((x D)) (= (k x) (ite (or guard (not (= x c))) (f x) b))))
(assert (forall ((x D)) (= (q x) (p (k x)))))`)
	atom := func(form string, value bool) string {
		if value {
			return form
		}
		return "(not " + form + ")"
	}
	names := [2]string{"a", "b"}
	for mask := range 128 {
		f := [2]int{mask & 1, mask >> 1 & 1}
		p := [2]bool{mask&4 != 0, mask&8 != 0}
		guard, c, v := mask&16 != 0, mask>>5&1, mask>>6&1
		fmt.Fprintf(&script, "(assert (and (not (= a b)) (= c %s) (= v %s) (= (f a) %s) (= (f b) %s) %s %s %s))\n",
			names[c], names[v], names[f[0]], names[f[1]], atom("(p a)", p[0]), atom("(p b)", p[1]), atom("guard", guard))
		result := p[1]
		if guard || v != c {
			result = p[f[v]]
		}
		fmt.Fprintf(&script, "(assert %s)(assert %s)\n", atom("(q v)", result), atom("(q v)", !result))
	}
	c, s := orderSolver(t, script.String())
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if len(roots) != 2+3*128 {
		t.Fatalf("unexpected assertion count %d", len(roots))
	}
	for i := 2; i < len(roots); i += 3 {
		t.Run(fmt.Sprint((i-2)/3), func(t *testing.T) {
			for j := range 2 {
				// Exercise both directions of a dependency in source order.
				input := []ast.ASTHandle{roots[j], roots[1-j], roots[i], roots[i+1]}
				checkObservedProof(t, c, s, input, false)
				input[3] = roots[i+2]
				checkObservedProof(t, c, s, input, true)
			}
		})
	}
}
