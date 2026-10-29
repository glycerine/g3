package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
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

func TestSolverTemporaryLiteralAssumptionsReturnCoreAndCollectClauses(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	positive, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	negative, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	// Retain both atoms independently of the temporary unit clauses.
	assignmentMust(t, s.AddClause([]sat.Literal{positive, negative}, Assumption{}))
	permanent := len(s.clauses.originals)

	assumptions := []sat.Literal{positive, negative}
	result, err := s.CheckAssumptions(&assumptions)
	assignmentMust(t, err)
	if result != lbool.False || len(assumptions) != 2 {
		t.Fatal("temporary assumption core", result, assumptions)
	}
	seen := map[sat.Literal]bool{}
	for _, literal := range assumptions {
		seen[literal] = true
	}
	if !seen[positive] || !seen[negative] {
		t.Fatal("temporary assumption core lost a literal", assumptions)
	}
	if len(s.clauses.originals) != permanent {
		t.Fatal("temporary input clauses survived", len(s.clauses.originals), permanent)
	}
	for _, clause := range s.clauses.learned {
		found, cause := s.clauseUsesInputLiteral(clause, map[*sat.Literal]struct{}{&assumptions[0]: {}, &assumptions[1]: {}})
		assignmentMust(t, cause)
		if found {
			t.Fatal("temporary learned clause survived", clause.ID())
		}
	}

	assumptions = []sat.Literal{positive}
	result, err = s.CheckAssumptions(&assumptions)
	assignmentMust(t, err)
	if result != lbool.True || len(assumptions) != 0 {
		t.Fatal("satisfiable assumptions were not cleared", result, assumptions)
	}
}

func solverUnit(t *testing.T, s *Solver, kind AtomKind, p polynomial.Polynomial) sat.Literal {
	t.Helper()
	literal, err := s.MkInequality(kind, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	return literal
}

func solverPolynomial(t *testing.T, s *Solver, coefficients []int32, powers [][]polynomial.MonomialPower) polynomial.Polynomial {
	t.Helper()
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	monomials, err := core.MonomialManager()
	assignmentMust(t, err)
	as := make([]unumeral.Int, len(coefficients))
	ms := make([]polynomial.Monomial, len(coefficients))
	for i := range coefficients {
		as[i] = unumeral.NewInt(coefficients[i])
		ms[i], err = monomials.MkMonomial(powers[i])
		assignmentMust(t, err)
	}
	p, err := core.MkTerms(as, ms)
	assignmentMust(t, err)
	assignmentMust(t, core.IncRef(p))
	t.Cleanup(func() { assignmentMust(t, core.DecRef(p)) })
	return p
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

func TestSolverInlineVariablesPatchesModelAndConflict(t *testing.T) {
	t.Run("model-patch", func(t *testing.T) {
		s := solverFixtureConfig(t, SolverConfig{InlineVars: true, Reorder: true})
		for i := 0; i < 2; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		xPlusYMinusThree := evaluatorPolynomial(t, s.evaluator,
			evaluatorTerm{1, 1, 0}, evaluatorTerm{1, 0, 1}, evaluatorTerm{-3, 0, 0})
		yMinusOne := evaluatorPolynomial(t, s.evaluator,
			evaluatorTerm{1, 0, 1}, evaluatorTerm{-1, 0, 0})
		solverUnit(t, s, AtomEQ, xPlusYMinusThree)
		solverUnit(t, s, AtomEQ, yMinusOne)
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.True || len(s.patches) != 2 {
			t.Fatal("inline model", result, len(s.patches))
		}
		for _, constraint := range []polynomial.Polynomial{xPlusYMinusThree, yMinusOne} {
			sign, err := s.config.Algebraic.EvalSign(s.config.Polynomial, constraint, s.assignment)
			assignmentMust(t, err)
			if sign != 0 {
				t.Fatal("patched model violates eliminated equality", sign)
			}
		}
	})

	t.Run("substituted-conflict", func(t *testing.T) {
		s := solverFixtureConfig(t, SolverConfig{InlineVars: true})
		_, err := s.MkVar(false)
		assignmentMust(t, err)
		xMinusOne := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-1, 0, 0})
		xMinusTwo := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-2, 0, 0})
		solverUnit(t, s, AtomEQ, xMinusOne)
		solverUnit(t, s, AtomGT, xMinusTwo)
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.False {
			t.Fatal("inline substituted conflict", result)
		}
	})

	t.Run("eliminated-equality-remains-guarded", func(t *testing.T) {
		s := solverFixtureConfig(t, SolverConfig{InlineVars: true})
		_, err := s.MkVar(false)
		assignmentMust(t, err)
		xMinusOne := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-1, 0, 0})
		equality := solverUnit(t, s, AtomEQ, xMinusOne)
		guard, err := s.MkBoolVar()
		assignmentMust(t, err)
		guardLiteral := sat.MakeLiteral(guard, false)
		assignmentMust(t, s.AddClause([]sat.Literal{equality, guardLiteral}, Assumption{}))
		assignmentMust(t, s.AddClause([]sat.Literal{guardLiteral.Negated()}, Assumption{}))
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.True {
			t.Fatal("eliminated true equality falsified guarded clause", result)
		}
	})

	t.Run("reorder-suppressed-with-patch", func(t *testing.T) {
		s := solverFixtureConfig(t, SolverConfig{InlineVars: true, Reorder: true})
		for i := 0; i < 2; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		xMinusY := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{-2, 0, 1})
		y := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
		xTimesYMinusTwo := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 1}, evaluatorTerm{-2, 0, 0})
		solverUnit(t, s, AtomEQ, xMinusY)
		solverUnit(t, s, AtomGT, y)
		solverUnit(t, s, AtomEQ, xTimesYMinusTwo)
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.True {
			t.Fatal("inline model with reorder option", result)
		}
		if s.patches[0].variable != 0 {
			t.Fatal("restored inline model patch", s.patches[0].variable)
		}
		if err = s.ReorderVariables([]uint32{1, 0}); err == nil {
			t.Fatal("manual reorder accepted live inline model patches")
		}
	})

	t.Run("three-variable-substitution", func(t *testing.T) {
		s := solverFixtureConfig(t, SolverConfig{InlineVars: true})
		for i := 0; i < 3; i++ {
			_, err := s.MkVar(false)
			assignmentMust(t, err)
		}
		xMinusTwoY := solverPolynomial(t, s, []int32{1, -2}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}}, {{Var: 1, Degree: 1}}})
		xTimesZPlusTwo := solverPolynomial(t, s, []int32{1, 2}, [][]polynomial.MonomialPower{{{Var: 0, Degree: 1}, {Var: 2, Degree: 1}}, nil})
		yMinusOne := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 1, Degree: 1}}, nil})
		zPlusOne := solverPolynomial(t, s, []int32{1, 1}, [][]polynomial.MonomialPower{{{Var: 2, Degree: 1}}, nil})
		zMinusOne := solverPolynomial(t, s, []int32{1, -1}, [][]polynomial.MonomialPower{{{Var: 2, Degree: 1}}, nil})
		solverUnit(t, s, AtomEQ, xMinusTwoY)
		solverUnit(t, s, AtomEQ, xTimesZPlusTwo)
		for _, bound := range []struct {
			kind     AtomKind
			value    polynomial.Polynomial
			negative bool
		}{{AtomLT, yMinusOne, true}, {AtomLT, zPlusOne, true}, {AtomGT, zMinusOne, true}} {
			literal, err := s.MkInequality(bound.kind, []polynomial.Polynomial{bound.value}, []bool{false}, bound.negative)
			assignmentMust(t, err)
			assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
		}
		result, err := s.Check()
		assignmentMust(t, err)
		if result != lbool.True {
			t.Fatal("three-variable inline result", result)
		}
	})
}
