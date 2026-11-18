package solver

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type unsatLastTactic struct{ tactic.Tactic }

func (u *unsatLastTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) error {
	if err := result.Prepare(goal.Manager()); err != nil {
		return err
	}
	n, err := goal.Size()
	if err != nil || n == 0 {
		return ast.ErrASTDomain
	}
	assumption, err := goal.Form(n - 1)
	if err != nil {
		return err
	}
	falsehood, err := goal.Manager().MkFalse()
	if err != nil {
		return err
	}
	proof, err := goal.Manager().MkProofAsserted(falsehood)
	if err != nil {
		return err
	}
	dependency, err := goal.Manager().DependencyLeaf(assumption)
	if err != nil {
		return err
	}
	if err = goal.Reset(); err != nil {
		return err
	}
	if err = goal.AssertWithProof(falsehood, proof, dependency); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
func (u *unsatLastTactic) Translate(*ast.ASTManager) (tactic.Tactic, error) { return u, nil }

func solverTacticBool(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := m.MkBoolSort()
	solverMust(t, err)
	symbol, err := m.Symbols.Intern(name)
	solverMust(t, err)
	value, err := m.MkConst(symbol, sort)
	solverMust(t, err)
	return value
}

func TestExtractClausesAndDependenciesNormalizesAndPins(t *testing.T) {
	m := solverTestManager(t)
	goal, err := tactic.NewGoal(m, false, true, true)
	solverMust(t, err)
	defer func() { solverMust(t, goal.Close()) }()
	formula0 := solverTacticBool(t, m, "clause-0")
	formula1 := solverTacticBool(t, m, "clause-1")
	booleanDependency := solverTacticBool(t, m, "boolean-dependency")
	nonBooleanDependency, err := m.MkNot(solverTacticBool(t, m, "compound-dependency"))
	solverMust(t, err)
	d0, err := m.DependencyLeaf(booleanDependency)
	solverMust(t, err)
	d1, err := m.DependencyLeaf(nonBooleanDependency)
	solverMust(t, err)
	solverMust(t, goal.AssertWithProof(formula0, ast.ASTHandle{}, d0))
	solverMust(t, goal.AssertWithProof(formula1, ast.ASTHandle{}, d1))

	extracted, err := ExtractClausesAndDependencies(goal)
	solverMust(t, err)
	if len(extracted.Clauses) != 2 || len(extracted.Assumptions) != 2 {
		t.Fatalf("clauses=%d assumptions=%d", len(extracted.Clauses), len(extracted.Assumptions))
	}
	if extracted.BoolToDependency[booleanDependency] != booleanDependency {
		t.Fatal("Boolean dependency was not used directly")
	}
	proxy := extracted.Assumptions[1]
	if proxy == nonBooleanDependency || extracted.BoolToDependency[proxy] != nonBooleanDependency || extracted.Filter == nil {
		t.Fatal("non-Boolean dependency was not normalized", proxy)
	}
	for _, clause := range extracted.Clauses {
		if n, e := m.GetNumArgs(clause); e != nil || n != 2 {
			t.Fatal("normalized clause", n, e)
		}
	}
	// Extraction retains all values used after the source goal is reset.
	solverMust(t, goal.Reset())
	for _, clause := range extracted.Clauses {
		if _, e := m.GetKind(clause); e != nil {
			t.Fatal("unpinned clause", e)
		}
	}
	solverMust(t, extracted.Close())
}

func newSkipSolverTactic(t *testing.T, m *ast.ASTManager, proofs, models, cores bool) *SolverTactic {
	t.Helper()
	params := solverTestParams(t, m)
	base, err := NewTacticSolver(m, tactic.NewSkipTactic(), nil, proofs, models, cores, "", TacticSolverConfig{Params: params})
	solverMust(t, err)
	converted, err := NewSolverTactic(base, SolverTacticConfig{Params: params})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, converted.Close()) })
	return converted
}

func TestSolverTacticSATProducesModelConverter(t *testing.T) {
	m := solverTestManager(t)
	converted := newSkipSolverTactic(t, m, false, true, false)
	goal, err := tactic.NewGoal(m, false, true, false)
	solverMust(t, err)
	defer func() { solverMust(t, goal.Close()) }()
	var result tactic.TacticResult
	solverMust(t, converted.Apply(goal, &result))
	defer func() { solverMust(t, result.Close()) }()
	if len(result.Goals) != 1 || result.Goals[0] != goal || result.ModelConverter == nil || !goal.IsDecidedSat() {
		t.Fatal("SAT solver tactic result")
	}
	var model tactic.ModelRef
	solverMust(t, result.ModelConverter.ApplyUnindexed(&model))
	defer func() { solverMust(t, model.Close()) }()
	if value, e := model.Get(); e != nil || value == nil {
		t.Fatal("model converter output", value, e)
	}
}

func TestSolverTacticUNSATRestoresCoreAndProof(t *testing.T) {
	m := solverTestManager(t)
	solverMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	params := solverTestParams(t, m)
	base, err := NewTacticSolver(m, &unsatLastTactic{Tactic: tactic.NewSkipTactic()}, nil, true, false, true, "", TacticSolverConfig{Params: params})
	solverMust(t, err)
	converted, err := NewSolverTactic(base, SolverTacticConfig{Params: params})
	solverMust(t, err)
	defer func() { solverMust(t, converted.Close()) }()
	goal, err := tactic.NewGoal(m, true, false, true)
	solverMust(t, err)
	defer func() { solverMust(t, goal.Close()) }()
	falsehood, err := m.MkFalse()
	solverMust(t, err)
	label := solverTacticBool(t, m, "unsat-label")
	proof, err := m.MkProofAsserted(falsehood)
	solverMust(t, err)
	dependency, err := m.DependencyLeaf(label)
	solverMust(t, err)
	solverMust(t, goal.AssertWithProof(falsehood, proof, dependency))

	var result tactic.TacticResult
	solverMust(t, converted.Apply(goal, &result))
	defer func() { solverMust(t, result.Close()) }()
	if len(result.Goals) != 1 || !goal.IsDecidedUnsat() || result.ProofConverter == nil || result.Core.IsNull() {
		t.Fatal("UNSAT solver tactic result")
	}
	core, err := m.DependencyLinearize(result.Core, nil)
	solverMust(t, err)
	if len(core) != 1 || core[0] != label {
		t.Fatal("restored core", core)
	}
	out, err := m.NewRef(ast.ASTHandle{})
	solverMust(t, err)
	defer func() { solverMust(t, m.DelRef(&out)) }()
	solverMust(t, result.ProofConverter.Convert(m, nil, out))
	convertedProof, err := m.RefGet(out)
	solverMust(t, err)
	if convertedProof.IsNull() {
		t.Fatal("missing converted proof")
	}
}

func TestSolverTacticUnknownBecomesTacticException(t *testing.T) {
	m := solverTestManager(t)
	converted := newSkipSolverTactic(t, m, false, false, false)
	goal, err := tactic.NewGoal(m, false, false, false)
	solverMust(t, err)
	defer func() { solverMust(t, goal.Close()) }()
	solverMust(t, goal.Assert(solverTacticBool(t, m, "unknown-formula")))
	var result tactic.TacticResult
	err = converted.Apply(goal, &result)
	var tacticError tactic.TacticException
	if !errors.As(err, &tacticError) || err.Error() != "incomplete" {
		t.Fatalf("unknown error = %v", err)
	}
	solverMust(t, result.Close())
}

// AUDIT.md solver_misc_01-F3: solver2tactic makes no sat_preserved /
// unsat_preserved checks (that is smt_tactic's policy): an approximated
// goal's sat or unsat result is returned.
func TestSolverTacticApproximationPolicy(t *testing.T) {
	m := solverTestManager(t)
	solverMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	params := solverTestParams(t, m)
	tests := []struct {
		name      string
		precision tactic.GoalPrecision
		child     tactic.Tactic
	}{
		{"sat", tactic.GoalOver, tactic.NewSkipTactic()},
		{"unsat", tactic.GoalUnder, &unsatLastTactic{Tactic: tactic.NewSkipTactic()}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proofs := tc.name == "unsat"
			base, err := NewTacticSolver(m, tc.child, nil, proofs, false, false, "", TacticSolverConfig{Params: params})
			solverMust(t, err)
			converted, err := NewSolverTactic(base, SolverTacticConfig{Params: params})
			solverMust(t, err)
			defer func() { solverMust(t, converted.Close()) }()
			goal, err := tactic.NewGoal(m, proofs, false, false)
			solverMust(t, err)
			defer func() { solverMust(t, goal.Close()) }()
			solverMust(t, goal.SetPrecision(tc.precision))
			if tc.name == "unsat" {
				solverMust(t, goal.Assert(solverTacticBool(t, m, "approx-unsat")))
			}
			var result tactic.TacticResult
			solverMust(t, converted.Apply(goal, &result))
			if len(result.Goals) != 1 {
				t.Fatal("approximated goal result", len(result.Goals))
			}
			solverMust(t, result.Close())
		})
	}
}

// solver2tactic::collect_param_descrs only forwards to the solver.
func TestSolverTacticParamDescrsHaveNoSMTTacticKeys(t *testing.T) {
	m := solverTestManager(t)
	converted := newSkipSolverTactic(t, m, false, false, false)
	descrs, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget(), Text: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}}})
	solverMust(t, err)
	defer func() { solverMust(t, descrs.Close()) }()
	solverMust(t, converted.CollectParamDescrs(descrs))
	for _, name := range []string{"candidate_models", "fail_if_inconclusive"} {
		if found, err := descrs.ContainsName(name); err != nil || found {
			t.Fatalf("parameter %s is smt_tactic's (%v)", name, err)
		}
	}
}

func TestSolverTacticCrossManagerTranslation(t *testing.T) {
	from := solverTestManager(t)
	to := solverTestManager(t)
	fromParams := solverTestParams(t, from)
	toParams := solverTestParams(t, to)
	baseConfig := TacticSolverConfig{Params: fromParams}
	baseConfig.TargetConfig = func(got *ast.ASTManager) (TacticSolverConfig, error) {
		if got != to {
			t.Fatal("unexpected solver target")
		}
		return TacticSolverConfig{Params: toParams}, nil
	}
	base, err := NewTacticSolver(from, tactic.NewSkipTactic(), nil, false, true, false, "", baseConfig)
	solverMust(t, err)
	config := SolverTacticConfig{Params: fromParams}
	config.TargetConfig = func(got *ast.ASTManager) (SolverTacticConfig, error) {
		if got != to {
			t.Fatal("unexpected tactic target")
		}
		return SolverTacticConfig{Params: toParams}, nil
	}
	converted, err := NewSolverTactic(base, config)
	solverMust(t, err)
	defer func() { solverMust(t, converted.Close()) }()
	translatedValue, err := converted.Translate(to)
	solverMust(t, err)
	translated := translatedValue.(*SolverTactic)
	defer func() { solverMust(t, translated.Close()) }()
	goal, err := tactic.NewGoal(to, false, true, false)
	solverMust(t, err)
	defer func() { solverMust(t, goal.Close()) }()
	var result tactic.TacticResult
	solverMust(t, translated.Apply(goal, &result))
	defer func() { solverMust(t, result.Close()) }()
	if len(result.Goals) != 1 || !goal.IsDecidedSat() || result.ModelConverter == nil {
		t.Fatal("translated solver tactic result")
	}
}
