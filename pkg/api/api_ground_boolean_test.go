package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestGroundBooleanTruthTable(t *testing.T) {
	cases := []struct {
		expr  string
		value func(a, b, c, d bool) bool
	}{
		{"(and a (or b (not c)) d)", func(a, b, c, d bool) bool { return a && (b || !c) && d }},
		{"(=> (and a b) (or c d))", func(a, b, c, d bool) bool { return !(a && b) || c || d }},
		{"(xor (= a b) (ite c d a))", func(a, b, c, d bool) bool { return (a == b) != (c && d || !c && a) }},
		{"(= (or a b) (and c d))", func(a, b, c, d bool) bool { return (a || b) == (c && d) }},
		{"(or (not a) (and b a) (and c (not d)))", func(a, b, c, d bool) bool { return !a || b && a || c && !d }},
	}
	var input strings.Builder
	names := []string{"a", "b", "c", "d"}
	for _, name := range names {
		fmt.Fprintf(&input, "(declare-const %s Bool)\n", name)
	}
	for mask := 0; mask < 16; mask++ {
		input.WriteString("(assert (and")
		for j, name := range names {
			if mask&(1<<j) != 0 {
				fmt.Fprintf(&input, " %s", name)
			} else {
				fmt.Fprintf(&input, " (not %s)", name)
			}
		}
		input.WriteString("))\n")
	}
	for _, tc := range cases {
		fmt.Fprintf(&input, "(assert %s)\n", tc.expr)
	}
	c, s := orderSolver(t, input.String())
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if len(roots) != 16+len(cases) {
		t.Fatalf("got %d source assertions", len(roots))
	}
	p := orderProofState{source: s}
	t.Cleanup(func() { astTestMust(t, p.release()) })
	for i, tc := range cases {
		for mask := 0; mask < 16; mask++ {
			input := []ast.ASTHandle{roots[mask], roots[16+i]}
			q, err := order.ProjectGroundBooleans(m, input, 80_000)
			if err != nil || q == nil {
				t.Fatalf("project %s on %04b: %v", tc.expr, mask, err)
			}
			yes, err := p.checkBoolean(m, q, 80_000)
			want := !tc.value(mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0)
			if err != nil || yes != want {
				t.Fatalf("%s on %04b: refuted=%t, want %t, error %v", tc.expr, mask, yes, want, err)
			}
			if !yes {
				astTestMust(t, p.release())
			}
		}
	}
}

func TestGroundBooleanOpaqueTermsAndPremisePolarity(t *testing.T) {
	const decl = `(declare-sort D 0)
(declare-const x D) (declare-const y D) (declare-fun p (D) Bool)
(declare-const a Bool) (declare-const b Bool)`
	for _, tc := range []struct {
		name, base string
		proved     bool
	}{
		{"conjunct", "(and (forall ((v D)) (p v)) (=> (p x) a) (p x))", true},
		{"separate-atom", "(and (not (= x y)) (=> (p x) a) (p y))", false},
		{"data-equality", "(and (= x y) (=> (= x y) a))", true},
		{"guarded", "(or (forall ((v D)) (p v)) a)", false},
		{"negated-conjunction", "(not (and (forall ((v D)) (p v)) a))", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := orderSolver(t, decl+"(assert "+tc.base+") (assert (and (not a) b))")
			m, err := c.Manager()
			astTestMust(t, err)
			roots, err := solver.Assertions(s.solver)
			astTestMust(t, err)
			q, err := order.ProjectGroundBooleans(m, roots, 80_000)
			if err != nil || q == nil {
				t.Fatalf("projection returned %v, %v", q, err)
			}
			p := orderProofState{source: s}
			t.Cleanup(func() { astTestMust(t, p.release()) })
			yes, err := p.checkBoolean(m, q, 80_000)
			if err != nil || yes != tc.proved {
				t.Fatalf("refuted=%t, want %t, error %v", yes, tc.proved, err)
			}
			want := lbool.True
			if tc.proved {
				want = lbool.False
			}
			answer, err := s.Check()
			if err != nil || answer != want {
				t.Fatalf("concrete check %v, want %v, error %v", answer, want, err)
			}
			if answer == lbool.True {
				model, err := s.Model()
				astTestMust(t, err)
				if model == nil {
					t.Fatal("missing source model")
				}
				astTestMust(t, model.Close())
			}
		})
	}
}

func TestGroundBooleanLimits(t *testing.T) {
	c, s := orderSolver(t, "(declare-const a Bool) (declare-const b Bool) (assert a) (assert (and (not a) b))")
	m, err := c.Manager()
	astTestMust(t, err)
	roots, err := solver.Assertions(s.solver)
	astTestMust(t, err)
	if q, err := order.ProjectGroundBooleans(m, roots, 1); err != nil || q != nil {
		t.Fatalf("one-step projection returned %v, %v", q, err)
	}
	done := make(chan struct{})
	close(done)
	m.ResourceLimit().RunSignals = &control.RunSignals{Done: done}
	defer func() { m.ResourceLimit().RunSignals = nil }()
	if q, err := order.ProjectGroundBooleans(m, roots, 80_000); err != nil || q != nil {
		t.Fatalf("canceled projection returned %v, %v", q, err)
	}
}

// Check 43 has a large quantified background and a ground goal over Boolean
// state variables. Prepared measures projection and the private proof with the
// assertions already loaded, as they are at the GoIvy Check call.
func BenchmarkAppleGroundBooleans(b *testing.B) {
	input := appleBenchmarkInput(b, "cmpl_live-43")
	for _, mode := range []string{"Concrete", "Projected"} {
		b.Run("Fresh/"+mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := benchmarkGoalInstances(b, input, mode == "Projected"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("Prepared/Projected", func(b *testing.B) {
		c, err := NewSMTContext()
		if err != nil {
			b.Fatal(err)
		}
		defer c.Close()
		s, err := c.NewSolver()
		if err != nil {
			b.Fatal(err)
		}
		defer s.Close()
		if err = s.LoadSMT2String(input); err != nil {
			b.Fatal(err)
		}
		p := s.BeginOrderProofBatch()
		defer p.Close()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			yes, err := p.Prove()
			if err != nil || !yes {
				b.Fatalf("refuted=%t, error %v", yes, err)
			}
		}
		b.StopTimer()
	})
}
