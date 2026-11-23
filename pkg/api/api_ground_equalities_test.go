package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

const groundEqualityDecls = `
(declare-sort D 0)
(declare-const a D)(declare-const b D)(declare-const c D)(declare-const v D)
(declare-fun f (D) D)(declare-fun p (D) Bool)(declare-fun q (D) Bool)
(declare-fun r (D D) Bool)(declare-const guard Bool)`

func TestGroundEqualityPremisePolarity(t *testing.T) {
	for _, tc := range []struct {
		name, facts string
		proved      bool
	}{
		{"transitive", "(= a b) (= c b) (p a) (not (p c))", true},
		{"nested", "(= a b) (= (f a) v) (not (= v (f b)))", true},
		{"two arguments", "(= c a) (r a b) (not (r c b))", true},
		{"ordered arguments", "(= c a) (r a b) (not (r b c))", false},
		{"distinct declarations", "(= a b) (p a) (not (q b))", false},
		{"equal branches", "(= a b) (= c (ite guard a b)) (p c) (not (p b))", true},
		{"unequal branches", "(= c (ite guard a b)) (p c) (not (p b))", false},
		{"negative implication", "(not (=> (= a b) guard)) (p a) (not (p b))", true},
		{"positive implication", "(=> guard (= a b)) (p a) (not (p b))", false},
		{"negative disjunction", "(not (or guard (not (= a b)))) (p a) (not (p b))", true},
		{"positive disjunction", "(or guard (= a b)) (p a) (not (p b))", false},
		{"negative conjunction", "(not (and guard (= a b))) (p a) (not (p b))", false},
		{"boolean equality", "(= (p a) (q a)) (r (ite (p a) a b) a) (not (r (ite (q a) a b) a))", true},
		{"class cycle", "(= (f a) a) (p (f (f a))) (not (p a))", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, groundEqualityDecls+"(assert (and "+tc.facts+"))")
			m, err := c.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			bits, err := order.ProjectGroundEqualities(m, roots, 80_000)
			if bits == nil || err != nil {
				t.Fatalf("projection failed: %v", err)
			}
			state := orderProofState{source: s}
			t.Cleanup(func() { astTestMust(t, state.release()) })
			yes, err := state.checkBooleanInput(m, bits, 80_000, roots)
			if err != nil || yes != tc.proved {
				t.Fatalf("proved=%v, want %v, error=%v", yes, tc.proved, err)
			}
		})
	}
}

// Give each generated problem a concrete two-element model. Ground equalities,
// functions and conditionals must admit this model for every assignment and
// predicate table. A forced contradiction at equal arguments must refute it.
func TestGroundEqualityFiniteModels(t *testing.T) {
	var script strings.Builder
	script.WriteString(groundEqualityDecls)
	for mask := 0; mask < 64; mask++ {
		elems := []string{"a", "b"}
		x, y := mask&1, mask>>1&1
		function := [2]int{mask >> 2 & 1, mask >> 3 & 1}
		predicate := [2]bool{mask&16 != 0, mask&32 != 0}
		atom := func(term string, truth bool) string {
			if truth {
				return "(p " + term + ")"
			}
			return "(not (p " + term + "))"
		}
		fmt.Fprintf(&script, "(assert (and (distinct a b) (= c %s) (= v %s) (= (f a) %s) (= (f b) %s) %s %s))\n",
			elems[x], elems[y], elems[function[0]], elems[function[1]], atom("a", predicate[0]), atom("b", predicate[1]))
		fmt.Fprintf(&script, "(assert %s)\n", atom("(f c)", !predicate[function[x]]))
	}
	c, s := orderSolver(t, script.String())
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if len(roots) != 128 {
		t.Fatalf("expected 128 source assertions, got %d", len(roots))
	}
	for i := 0; i < len(roots); i += 2 {
		for n := 1; n <= 2; n++ {
			input := roots[i : i+n]
			state := orderProofState{source: s}
			bits, err := order.ProjectGroundEqualities(m, input, 80_000)
			if bits == nil || err != nil {
				t.Fatalf("mask %d, projection %v", i/2, err)
			}
			yes, err := state.checkBooleanInput(m, bits, 80_000, input)
			astTestMust(t, state.release())
			if err != nil || yes != (n == 2) {
				t.Fatalf("mask %d, %d inputs, proved=%v error=%v", i/2, n, yes, err)
			}
		}
	}
}

func TestGroundEqualityValidatesOriginalSourceAndBudget(t *testing.T) {
	c, s := orderSolver(t, groundEqualityDecls+"(assert (= a b))(assert (p a))(assert (not (p b)))")
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	state := orderProofState{source: s}
	t.Cleanup(func() { astTestMust(t, state.release()) })
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	free, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(free))
	defer func() { astTestMust(t, m.DecRef(free)) }()
	source := append([]ast.ASTHandle{free}, roots...)
	for _, tc := range []struct {
		source []ast.ASTHandle
		steps  uint32
		proved bool
	}{
		{source, 80_000, false}, {roots, 1, false}, {roots, 80_000, true},
	} {
		bits, err := order.ProjectGroundEqualities(m, roots, 80_000)
		if bits == nil || err != nil {
			t.Fatalf("projection error %v", err)
		}
		yes, err := state.checkBooleanInput(m, bits, tc.steps, tc.source)
		if yes != tc.proved || err != nil || len(m.ResourceLimit().Children) != 0 {
			t.Fatalf("proved=%v error=%v", yes, err)
		}
	}
	if bits, err := order.ProjectGroundEqualities(m, roots, 1); bits != nil || err != nil {
		t.Fatalf("tiny projection budget: %v", err)
	}
	m.ResourceLimit().Cancel()
	defer m.ResourceLimit().ResetCancel()
	if bits, err := order.ProjectGroundEqualities(m, roots, 80_000); bits != nil || err != nil {
		t.Fatalf("canceled projection: %v", err)
	}
}
