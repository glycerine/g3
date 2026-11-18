package solver

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type musScriptSolver struct {
	Solver
	check   func([]ast.ASTHandle) (lbool.LBool, error)
	core    []ast.ASTHandle
	model   *model.ModelCore
	queries [][]ast.ASTHandle
}

func (s *musScriptSolver) CheckSat(values []ast.ASTHandle) (lbool.LBool, error) {
	s.queries = append(s.queries, append([]ast.ASTHandle(nil), values...))
	return s.check(values)
}
func (s *musScriptSolver) UnsatCore() ([]ast.ASTHandle, error) {
	return append([]ast.ASTHandle(nil), s.core...), nil
}
func (s *musScriptSolver) AcquireModel() (*model.ModelCore, error) {
	if s.model != nil {
		if err := s.model.IncRef(); err != nil {
			return nil, err
		}
	}
	return s.model, nil
}

func newMUSScriptSolver(t *testing.T, m *ast.ASTManager) *musScriptSolver {
	t.Helper()
	params := solverTestParams(t, m)
	base, err := NewTacticSolver(m, tactic.NewSkipTactic(), nil, false, false, false, "", TacticSolverConfig{Params: params})
	solverMust(t, err)
	s := &musScriptSolver{Solver: base}
	t.Cleanup(func() { solverMust(t, base.Close()) })
	return s
}

func TestMUSDeletionExtractionCoreReductionAndAssumptions(t *testing.T) {
	m := solverTestManager(t)
	a := solverTacticBool(t, m, "mus-a")
	b := solverTacticBool(t, m, "mus-b")
	c := solverTacticBool(t, m, "mus-c")
	extra := solverTacticBool(t, m, "mus-extra")
	s := newMUSScriptSolver(t, m)
	step := 0
	s.check = func(query []ast.ASTHandle) (lbool.LBool, error) {
		if !containsAST(query, extra) {
			t.Fatal("additional assumption omitted", query)
		}
		step++
		switch step {
		case 1:
			s.core = []ast.ASTHandle{a, b}
			return lbool.False, nil
		case 2:
			return lbool.True, nil
		case 3:
			notA, err := m.MkNotSimplified(a)
			solverMust(t, err)
			s.core = []ast.ASTHandle{notA, b}
			return lbool.False, nil
		default:
			t.Fatal("unexpected MUS query", step, query)
			return lbool.Undef, nil
		}
	}
	u, err := NewMUS(s, MUSConfig{})
	solverMust(t, err)
	defer func() { solverMust(t, u.Close()) }()
	if index, e := u.AddSoft(a); e != nil || index != 0 {
		t.Fatal(index, e)
	}
	if index, e := u.AddSoft(b); e != nil || index != 1 {
		t.Fatal(index, e)
	}
	if index, e := u.AddSoft(c); e != nil || index != 2 {
		t.Fatal(index, e)
	}
	solverMust(t, u.AddAssumption(extra))
	status, core, err := u.GetMUS()
	solverMust(t, err)
	if status != lbool.True || len(core) != 1 || core[0] != b || len(s.queries) != 3 {
		t.Fatal("MUS result", status, core, len(s.queries))
	}
	solverMust(t, u.Reset())
	if len(u.literals) != 0 || len(u.assumptions) != 0 {
		t.Fatal("reset retained extraction inputs")
	}
}

func TestMUSSingleUnknownAndLiteralGuards(t *testing.T) {
	m := solverTestManager(t)
	a := solverTacticBool(t, m, "mus-single")
	s := newMUSScriptSolver(t, m)
	s.check = func([]ast.ASTHandle) (lbool.LBool, error) { return lbool.Undef, nil }
	u, err := NewMUS(s, MUSConfig{})
	solverMust(t, err)
	defer func() { solverMust(t, u.Close()) }()
	solverMust(t, u.AddSoftAll([]ast.ASTHandle{a}))
	status, core, err := u.GetMUS()
	solverMust(t, err)
	if status != lbool.True || len(core) != 1 || core[0] != a || len(s.queries) != 0 {
		t.Fatal("single-literal shortcut", status, core, s.queries)
	}
	compound, err := m.MkOr(a, a)
	solverMust(t, err)
	if _, err = u.AddSoft(compound); err == nil {
		t.Fatal("accepted nonliteral soft constraint")
	}
	solverMust(t, u.Reset())
	solverMust(t, u.AddSoftAll([]ast.ASTHandle{a, solverTacticBool(t, m, "mus-unknown")}))
	status, core, err = u.GetMUS()
	solverMust(t, err)
	if status != lbool.Undef || core != nil {
		t.Fatal("unknown result", status, core)
	}
}

func TestMUSWeightedBestModel(t *testing.T) {
	m := solverTestManager(t)
	a := solverTacticBool(t, m, "weighted-a")
	b := solverTacticBool(t, m, "weighted-b")
	s := newMUSScriptSolver(t, m)
	md, err := model.NewModel(m)
	solverMust(t, err)
	defer func() { solverMust(t, md.Close()) }()
	s.model = md.Core()
	s.check = func([]ast.ASTHandle) (lbool.LBool, error) { return lbool.True, nil }
	evaluations := 0
	u, err := NewMUS(s, MUSConfig{EvaluateTruth: func(got *model.ModelCore, value ast.ASTHandle) (bool, error) {
		if got != md.Core() {
			t.Fatal("wrong model")
		}
		round := evaluations / 2
		evaluations++
		if round == 0 {
			return value == a, nil
		}
		return value == b, nil
	}})
	solverMust(t, err)
	defer func() { solverMust(t, u.Close()) }()
	solverMust(t, u.AddSoftAll([]ast.ASTHandle{a, b}))
	q := u.q()
	weights := []numeral.Rational{numeral.NewRational(), numeral.NewRational()}
	solverMust(t, q.SetInt64(&weights[0], 3))
	solverMust(t, q.SetInt64(&weights[1], 5))
	solverMust(t, u.SetSoft([]ast.ASTHandle{a, b}, weights))
	status, _, err := u.GetMUS()
	solverMust(t, err)
	if status != lbool.True || evaluations != 4 {
		t.Fatal(status, evaluations)
	}
	weight := numeral.NewRational()
	best, err := u.BestModel(&weight)
	solverMust(t, err)
	if text, _ := q.String(&weight); best != md.Core() || text != "3" {
		t.Fatal("best weighted model", best, weight)
	}
	solverMust(t, best.DecRef())
}
