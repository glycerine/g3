package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func solverFixture(t *testing.T) *Solver {
	return solverFixtureConfig(t, SolverConfig{SimplifyCore: true})
}

func solverFixtureConfig(t *testing.T, config SolverConfig) *Solver {
	t.Helper()
	am, _, limit := assignmentFixtureWithLimit(t)
	q, err := am.RationalManager()
	assignmentMust(t, err)
	pm, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{Core: polynomial.MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Checkpoint: limit.Inc, Cooperation: func() error { return nil }}})
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, pm.CloseChecked()) })
	config.Algebraic, config.Polynomial, config.Limit = am, pm, limit
	s, err := NewSolver(config)
	assignmentMust(t, err)
	t.Cleanup(func() { assignmentMust(t, s.Close()) })
	return s
}

func TestSolverConflictMinimizationWithEquality(t *testing.T) {
	s := solverFixtureConfig(t, SolverConfig{SimplifyCore: true, MinimizeCore: true, Factor: true})
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	equality := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-1, 0, 0})
	consequence := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 2, 0}, evaluatorTerm{-1, 0, 0})
	solverUnit(t, s, AtomEQ, equality)
	solverUnit(t, s, AtomGT, consequence)
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("minimized equality conflict", result)
	}
	statistics, err := s.Statistics()
	assignmentMust(t, err)
	if statistics["nlsat conflicts"] == 0 || statistics["nlsat stages"] == 0 {
		t.Fatal("solver statistics", statistics)
	}
	assignmentMust(t, s.ResetStatistics())
	statistics, err = s.Statistics()
	assignmentMust(t, err)
	for name, value := range statistics {
		if value != 0 {
			t.Fatal("statistic reset", name, value)
		}
	}
}

func solverUnit(t *testing.T, s *Solver, kind AtomKind, p polynomial.Polynomial) sat.Literal {
	t.Helper()
	literal, err := s.MkInequality(kind, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	return literal
}

func TestSolverMixedPolynomialSearchAndRepeatedModel(t *testing.T) {
	s := solverFixture(t)
	for i := uint32(0); i < 2; i++ {
		x, err := s.MkVar(false)
		assignmentMust(t, err)
		if x != i {
			t.Fatal("solver variable order", x, i)
		}
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 2, 0}, evaluatorTerm{1, 0, 2}, evaluatorTerm{-2, 0, 0})
	literal := solverUnit(t, s, AtomEQ, p)
	for check := 0; check < 2; check++ {
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.True {
			t.Fatal("mixed quadratic search", check, result)
		}
		value, err := s.search.value(literal)
		assignmentMust(t, err)
		if value != lbool.True {
			t.Fatal("published assignment violates mixed polynomial", check, value)
		}
		for x := uint32(0); x < 2; x++ {
			var witness polynomial.Algebraic
			assignmentMust(t, s.Value(x, &witness))
			assignmentMust(t, s.config.Algebraic.Del(&witness))
		}
	}
}

func TestSolverMixedLinearConflict(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	inputs := []struct {
		kind  AtomKind
		terms []evaluatorTerm
	}{
		{AtomGT, []evaluatorTerm{{1, 1, 0}}},
		{AtomGT, []evaluatorTerm{{1, 0, 1}}},
		{AtomLT, []evaluatorTerm{{1, 1, 0}, {1, 0, 1}}},
	}
	attachments := make([]*AssumptionAttachment, len(inputs))
	for i, input := range inputs {
		attachments[i] = &AssumptionAttachment{}
		p := evaluatorPolynomial(t, s.evaluator, input.terms...)
		literal, err := s.MkInequality(input.kind, []polynomial.Polynomial{p}, []bool{false}, false)
		assignmentMust(t, err)
		assignmentMust(t, s.AddClause([]sat.Literal{literal}, AttachedAssumption(attachments[i])))
	}
	result, err := s.Check()
	assignmentMust(t, err)
	if result != lbool.False {
		t.Fatal("mixed linear contradiction", result)
	}
	var core []Assumption
	assignmentMust(t, s.Core(&core))
	if len(core) != len(attachments) {
		t.Fatal("mixed conflict core size", len(core), len(attachments))
	}
	seen := map[*AssumptionAttachment]bool{}
	for _, assumption := range core {
		seen[assumption.Attachment()] = true
	}
	for _, attachment := range attachments {
		if !seen[attachment] {
			t.Fatal("mixed conflict core lost input dependency", core)
		}
	}
}

func TestSolverIntegerBranchAndBound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		terms []evaluatorTerm
		want  lbool.LBool
	}{
		{"fractional-linear", []evaluatorTerm{{2, 1, 0}, {-1, 0, 0}}, lbool.False},
		{"integral-quadratic", []evaluatorTerm{{1, 2, 0}, {-4, 0, 0}}, lbool.True},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := solverFixture(t)
			x, err := s.MkVar(true)
			assignmentMust(t, err)
			if x != 0 {
				t.Fatal("integer variable order", x)
			}
			solverUnit(t, s, AtomEQ, evaluatorPolynomial(t, s.evaluator, tc.terms...))
			result, err := s.Check()
			assignmentMust(t, err)
			if result != tc.want {
				t.Fatal("integer branch-and-bound result", result, tc.want)
			}
			if tc.want == lbool.True {
				value, err := s.assignment.Value(0)
				assignmentMust(t, err)
				integer, err := s.config.Algebraic.IsInt(value)
				assignmentMust(t, err)
				if !integer {
					t.Fatal("integer SAT model remained fractional")
				}
			}
		})
	}
}

func TestSolverHeuristicVariablePermutation(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	x0 := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	x1Cubed := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 3})
	solverUnit(t, s, AtomGT, x0)
	repeated := solverUnit(t, s, AtomGT, x1Cubed)
	assignmentMust(t, s.AddClause([]sat.Literal{repeated}, Assumption{}))
	permutation, supported, err := s.HeuristicVariablePermutation()
	assignmentMust(t, err)
	if !supported {
		t.Fatal("inequality-only variable order was refused")
	}
	want := []uint32{1, 0}
	if len(permutation) != len(want) {
		t.Fatal("variable permutation length", permutation)
	}
	for i := range want {
		if permutation[i] != want[i] {
			t.Fatal("variable permutation", permutation, want)
		}
	}
	if err = s.ReorderVariables([]uint32{0}); err == nil {
		t.Fatal("short variable permutation accepted")
	}
	if err = s.ReorderVariables([]uint32{0, 0}); err == nil {
		t.Fatal("duplicate variable permutation accepted")
	}
	assignmentMust(t, s.ReorderVariables(permutation))
	if s.clauses.vars.perm[0] != 1 || s.clauses.vars.perm[1] != 0 {
		t.Fatal("composed variable map", s.clauses.vars.perm, s.clauses.vars.inv)
	}
	status, err := s.Check()
	assignmentMust(t, err)
	if status != lbool.True {
		t.Fatal("reordered solver result", status)
	}
	assignmentMust(t, s.RestoreVariableOrder())
	for variable := uint32(0); variable < 2; variable++ {
		if s.clauses.vars.perm[variable] != variable || s.clauses.vars.inv[variable] != variable {
			t.Fatal("restored variable map", s.clauses.vars.perm, s.clauses.vars.inv)
		}
		var value polynomial.Algebraic
		assignmentMust(t, s.Value(variable, &value))
		assignmentMust(t, s.config.Algebraic.Del(&value))
	}
	_, err = s.MkRoot(AtomRootEQ, 1, 1, x1Cubed, false)
	assignmentMust(t, err)
	permutation, supported, err = s.HeuristicVariablePermutation()
	assignmentMust(t, err)
	if supported || permutation != nil {
		t.Fatal("root-atom variable order was accepted", permutation)
	}
}

func TestSolverShuffledVariablePermutation(t *testing.T) {
	s := solverFixtureConfig(t, SolverConfig{Seed: 7})
	for i := 0; i < 4; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	for attempt, seed := range []uint32{8, 9} {
		got, supported, err := s.ShuffledVariablePermutation()
		assignmentMust(t, err)
		if !supported {
			t.Fatal("shuffle refused inequality-only solver")
		}
		order := []uint32{0, 1, 2, 3}
		z3rand.ShuffleUint32(order, z3rand.New(seed))
		want := make([]uint32, len(order))
		for position, variable := range order {
			want[variable] = uint32(position)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatal("shuffle permutation", attempt, got, want)
			}
		}
	}
}
