package api

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func enumerationProofInput() (string, int) {
	var script strings.Builder
	script.WriteString(`(declare-datatypes () ((A a0 a1 a2) (B b0 b1 b2) (C c0 c1) (D d0 d1 d2 d3 d4)))
(declare-sort U 0)
(declare-fun x0 (A B) C)
(declare-const ef1_1 A)
(declare-fun choose (B) A)
(declare-fun extra (U) D)
(declare-const u U)
(assert (= (x0 a0 b0) c0))
(assert (= (choose b1) a2))
`)
	const padding = 7
	for i := range padding {
		fmt.Fprintf(&script, "(declare-fun rel%d (A B C) Bool)\n", i)
		fmt.Fprintf(&script, "(assert (forall ((a A)(b B)(c C)) (rel%d a b c)))\n", i)
	}
	return script.String(), 2 + padding
}

func TestEnumerationProjectionMatchesConcreteModels(t *testing.T) {
	base, n := enumerationProofInput()
	cases := []struct {
		goal string
		want lbool.LBool
	}{
		{"(= a0 a2)", lbool.False},
		{"(distinct a0 a1 a2)", lbool.True},
		{"(distinct ef1_1 a0 a1 a2)", lbool.False},
		{"(= (x0 a0 b0) c1)", lbool.False},
		{"(xor (= (choose b1) a2) (= (x0 a0 b0) c1))", lbool.True},
		{"(= (ite (= ef1_1 a2) (x0 a0 b0) c0) c1)", lbool.False},
		{"(forall ((x0 A)) (exists ((ea B)) (= (x0 x0 ea) c0)))", lbool.True},
		{"(exists ((v A)) (forall ((w B)) (distinct v (choose w))) )", lbool.True},
		// The only occurrence of D and extra in this query is outside the base.
		{"(distinct (extra u) d0 d1 d2 d3 d4)", lbool.False},
		{"(= (extra u) d3)", lbool.True},
	}
	var script strings.Builder
	script.WriteString(base)
	for _, tc := range cases {
		fmt.Fprintf(&script, "(assert %s)\n", tc.goal)
	}
	c, donor := orderSolver(t, script.String())
	m, err := c.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	if len(forms) != n+len(cases) {
		t.Fatal("unexpected assertion count", len(forms))
	}
	batch := enumerationRefutation{}
	t.Cleanup(func() { astTestMust(t, batch.release()) })
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			input := append(slices.Clone(forms[:n]), forms[n+i])
			problem, err := batch.premises.Project(m, input)
			astTestMust(t, err)
			if problem == nil {
				t.Fatal("finite enumeration input declined", tc.goal)
			}
			if strings.Contains(problem.Text, "declare-datatypes") ||
				strings.Count(problem.Premises(), "(forall ((ea ") != 3 {
				t.Fatal("incorrect finite-domain premises")
			}
			if i >= 8 && !strings.Contains(problem.Assertions()[len(problem.Premises()):], "(forall ((ea ") {
				t.Fatal("missing coverage for goal-local sort")
			}
			concrete, err := c.NewSolver()
			astTestMust(t, err)
			defer func() { astTestMust(t, concrete.Close()) }()
			for _, f := range input {
				astTestMust(t, concrete.Assert(f))
			}
			answer, err := concrete.Check()
			astTestMust(t, err)
			if answer != tc.want {
				t.Fatal("concrete answer", tc.goal, answer)
			}
			yes, err := batch.check(m, input)
			astTestMust(t, err)
			if yes != (tc.want == lbool.False) {
				t.Fatal("finite refutation", tc.goal, yes)
			}
			if !yes && batch.kernel != nil {
				t.Fatal("inconclusive query kept its private search state")
			}
			if tc.want == lbool.True {
				model, err := concrete.Model()
				astTestMust(t, err)
				astTestMust(t, model.Close())
			}
		})
	}
}

func TestEnumerationProofBatchScopesAndUnsupportedSiblings(t *testing.T) {
	base, n := enumerationProofInput()
	c, donor := orderSolver(t, base+`
(assert (exists ((v A)) (distinct v a0 a1 a2)))
(assert (= (extra u) d4))
(assert (> 3 1))
(assert (distinct (extra u) d0 d1 d2 d3 d4))`)
	m, err := c.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	s, err := c.NewSolverForIncrementalChecks()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	for _, f := range forms[:n] {
		astTestMust(t, s.Assert(f))
	}
	batch := s.BeginOrderProofBatch()
	t.Cleanup(func() { astTestMust(t, batch.Close()) })
	for i, want := range []bool{true, false, false, true} {
		got, err := batch.ProveWithGoal(forms[n+i])
		astTestMust(t, err)
		if got != want {
			t.Fatalf("temporary goal %d proved=%v, want %v", i, got, want)
		}
		level, err := s.ScopeLevel()
		astTestMust(t, err)
		active, err := solver.Assertions(s.solver)
		astTestMust(t, err)
		if level != 0 || !slices.Equal(active, forms[:n]) {
			t.Fatal("private proof altered source assertions or scope")
		}
	}
	var projected order.EnumerationPremises
	query := append(slices.Clone(forms[:n]), forms[n+2])
	problem, err := projected.Project(m, query)
	astTestMust(t, err)
	if problem != nil {
		t.Fatal("arithmetic accepted as a finite enumeration")
	}
	a, err := m.MkAndArgs([]ast.ASTHandle{forms[n+1]})
	astTestMust(t, err)
	query[n] = a
	problem, err = projected.Project(m, query)
	astTestMust(t, err)
	if problem == nil {
		t.Fatal("singleton Boolean connective or supported sibling rejected")
	}
	before := m.Limit.CancelFlag()
	m.Limit.Cancel()
	defer m.Limit.ResetCancel()
	if before {
		t.Fatal("fixture started cancelled")
	}
	problem, err = projected.Project(m, query)
	astTestMust(t, err)
	if problem != nil {
		t.Fatal("cancelled private projection")
	}
}

func TestEnumerationSelectionAcrossNarrowAndWideSiblingGoals(t *testing.T) {
	var script strings.Builder
	script.WriteString(`(declare-datatypes () ((A a0 a1)(B b0 b1)(C c0 c1)))
(declare-sort U 0)
(declare-const u U)
(declare-fun f (U) A)
(declare-fun g (U) B)
(declare-fun h (U) C)
(assert (= (f u) a0))
(assert (= (g u) b0))
(assert (= (h u) c0))
`)
	// Each premise quantifies two non-enumeration arguments. The three
	// enumerations appear as values of functions in the original problem.
	for i := range 11 {
		fmt.Fprintf(&script, "(declare-fun r%d (U U) Bool)\n(assert (forall ((x U)(y U)) (r%d x y)))\n", i, i)
	}
	cases := []struct {
		goal            string
		selected, unsat bool
	}{
		{"(not (r0 u u))", false, true},
		{"(exists ((x A)) (distinct x a0 a1))", true, true},
		{"(exists ((x U)(y U)) (not (r0 x y)))", false, true},
		{"(exists ((x U)) (exists ((y U)(z U)) (and (r1 x y) (not (r0 z x)))))", true, true},
		{"(exists ((x A)) (= x a0))", true, false},
		{"(r0 u u)", false, false},
	}
	for _, tc := range cases {
		fmt.Fprintf(&script, "(assert %s)\n", tc.goal)
	}
	c, donor := orderSolver(t, script.String())
	m, err := c.Manager()
	astTestMust(t, err)
	forms, err := solver.Assertions(donor.solver)
	astTestMust(t, err)
	base := forms[:len(forms)-len(cases)]
	var batch enumerationRefutation
	t.Cleanup(func() { astTestMust(t, batch.release()) })
	for i, tc := range cases {
		input := append(slices.Clone(base), forms[len(base)+i])
		projection, err := batch.premises.Project(m, input)
		astTestMust(t, err)
		if (projection != nil) != tc.selected {
			t.Fatalf("goal %d selection = %t, want %t", i, projection != nil, tc.selected)
		}
		proved, err := batch.check(m, input)
		astTestMust(t, err)
		if proved != (tc.selected && tc.unsat) {
			t.Fatalf("goal %d private proof = %t", i, proved)
		}
		if !tc.selected {
			concrete, err := c.NewSolver()
			astTestMust(t, err)
			for _, a := range input {
				astTestMust(t, concrete.Assert(a))
			}
			result, err := concrete.Check()
			astTestMust(t, err)
			astTestMust(t, concrete.Close())
			want := lbool.True
			if tc.unsat {
				want = lbool.False
			}
			if result != want {
				t.Fatalf("goal %d concrete result = %v, want %v", i, result, want)
			}
		}
	}
}
