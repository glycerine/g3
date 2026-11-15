package solver

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type errorTactic struct {
	tactic.Tactic
	cause error
}

func (t *errorTactic) Apply(*tactic.Goal, *tactic.TacticResult) error { return t.cause }

func solverTestParams(t *testing.T, m *ast.ASTManager) *uparams.ParamsContext {
	t.Helper()
	c, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, c.Close()) })
	return c
}

func TestTacticSolverSATUnknownScopesAndFailureReason(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	s, err := NewTacticSolver(m, tactic.NewSkipTactic(), nil, false, true, true, "QF_UF", TacticSolverConfig{Params: params})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, s.Close()) })
	status, err := s.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.True {
		t.Fatal("empty tactic solver", status)
	}
	md, err := s.AcquireModel()
	solverMust(t, err)
	if md == nil {
		t.Fatal("SAT tactic solver omitted model")
	}
	solverMust(t, md.DecRef())

	boolSort, err := m.MkBoolSort()
	solverMust(t, err)
	name, err := m.Symbols.Intern("tactic-solver-p")
	solverMust(t, err)
	p, err := m.MkConst(name, boolSort)
	solverMust(t, err)
	solverMust(t, s.Assert(p))
	status, err = s.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.Undef {
		t.Fatal("symbolic skip result", status)
	}
	if reason, err := s.ReasonUnknown(); err != nil || reason != "incomplete" {
		t.Fatal("incomplete reason", reason, err)
	}
	solverMust(t, s.Push())
	falsehood, err := m.MkFalse()
	solverMust(t, err)
	solverMust(t, s.Assert(falsehood))
	status, err = s.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.False {
		t.Fatal("false assertion", status)
	}
	solverMust(t, s.Pop(1))
	if n, _ := s.NumAssertions(); n != 1 {
		t.Fatal("pop did not restore assertions", n)
	}

	failing, err := NewTacticSolver(m, tactic.NewFailTactic(), nil, false, false, false, "", TacticSolverConfig{Params: params})
	solverMust(t, err)
	status, err = failing.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.Undef {
		t.Fatal("failing tactic status", status)
	}
	if reason, err := failing.ReasonUnknown(); err != nil || reason != "fail tactic" {
		t.Fatal("failing tactic reason", reason, err)
	}
	solverMust(t, failing.Close())
}

func TestTacticSolverTrackedCoreProofFactoriesAndTranslation(t *testing.T) {
	m := solverTestManager(t)
	solverMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	params := solverTestParams(t, m)
	factory, err := NewTacticSolverFactory(tactic.NewSkipTactic(), TacticSolverConfig{Params: params})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, factory.Close()) })
	created, err := factory.New(m, nil, true, false, true, "QF_UF")
	solverMust(t, err)
	s := created.(*TacticSolver)
	t.Cleanup(func() { solverMust(t, s.Close()) })
	boolSort, err := m.MkBoolSort()
	solverMust(t, err)
	name, err := m.Symbols.Intern("tracked")
	solverMust(t, err)
	tag, err := m.MkConst(name, boolSort)
	solverMust(t, err)
	falsehood, err := m.MkFalse()
	solverMust(t, err)
	status, err := s.CheckSat([]ast.ASTHandle{falsehood})
	solverMust(t, err)
	if status != lbool.False {
		t.Fatal("tracked false", status)
	}
	core, err := s.UnsatCore()
	solverMust(t, err)
	if len(core) != 1 || core[0] != falsehood {
		t.Fatal("assumption unsat core", core)
	}
	proof, err := s.Proof()
	solverMust(t, err)
	if proof.IsNull() {
		t.Fatal("assumption unsat proof missing")
	}
	second, err := factory.New(m, nil, false, false, false, "")
	solverMust(t, err)
	solverMust(t, second.Close())
	solverMust(t, s.AssertTracked(falsehood, tag))

	solverMust(t, s.Push())
	translated, err := s.Translate(m, nil)
	if err == nil || translated != nil {
		t.Fatal("scoped translation unexpectedly accepted", translated, err)
	}
	solverMust(t, s.Pop(1))
}

func TestTacticFactoryToSolverFactory(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	tf := tactic.TacticFactoryFunc(func(got *ast.ASTManager, _ *uparams.ParamsRef) (tactic.Tactic, error) {
		if got != m {
			t.Fatal("tactic factory manager")
		}
		return tactic.NewSkipTactic(), nil
	})
	factory, err := NewTacticFactorySolverFactory(tf, TacticSolverConfig{Params: params})
	solverMust(t, err)
	created, err := factory.New(m, nil, false, false, false, "")
	solverMust(t, err)
	status, err := created.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.True {
		t.Fatal("factory solver status", status)
	}
	solverMust(t, created.Close())
	solverMust(t, factory.Close())
}

func TestTacticSolverExceptionTransportAndTrace(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("tactic2solver")
	tacticValue := &errorTactic{Tactic: tactic.NewSkipTactic(), cause: z3exception.NewDefaultException("solver tactic stopped")}
	s, err := NewTacticSolver(m, tacticValue, nil, false, false, false, "", TacticSolverConfig{Params: params, Trace: tags})
	solverMust(t, err)
	status, err := s.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.Undef {
		t.Fatal("exception status", status)
	}
	if reason, err := s.ReasonUnknown(); err != nil || reason != "solver tactic stopped" {
		t.Fatal("exception reason", reason, err)
	}
	if tags.ShouldTrace("tactic2solver") {
		if !bytes.Contains(output.Bytes(), []byte("exception: solver tactic stopped\n")) {
			t.Fatal("exception trace", output.String())
		}
	} else if output.Len() != 0 {
		t.Fatal("ordinary build emitted exception trace", output.String())
	}
	solverMust(t, s.Close())
}

func TestTacticSolverBaseLevelTranslation(t *testing.T) {
	from := solverTestManager(t)
	to := solverTestManager(t)
	fromParams := solverTestParams(t, from)
	toParams := solverTestParams(t, to)
	config := TacticSolverConfig{Params: fromParams}
	config.TargetConfig = func(m *ast.ASTManager) (TacticSolverConfig, error) {
		if m != to {
			t.Fatal("unexpected translation manager")
		}
		return TacticSolverConfig{Params: toParams}, nil
	}
	s, err := NewTacticSolver(from, tactic.NewSkipTactic(), nil, false, false, false, "QF_UF", config)
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, s.Close()) })
	boolSort, err := from.MkBoolSort()
	solverMust(t, err)
	name, err := from.Symbols.Intern("translated-assertion")
	solverMust(t, err)
	p, err := from.MkConst(name, boolSort)
	solverMust(t, err)
	solverMust(t, s.Assert(p))
	translated, err := s.Translate(to, nil)
	solverMust(t, err)
	target := translated.(*TacticSolver)
	value, err := target.Assertion(0)
	solverMust(t, err)
	if _, err = to.GetKind(value); err != nil {
		t.Fatal("translated assertion is not owned by target", err)
	}
	solverMust(t, target.Close())
}

// labelTactic decides the goal like skip and installs a stored model
// converter that carries a label, as the smt tactic does.
type labelTactic struct {
	tactic.Tactic
	label usymbol.Symbol
}

func (t *labelTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) error {
	if err := t.Tactic.Apply(g, result); err != nil {
		return err
	}
	md, err := model.NewModel(g.Manager())
	if err != nil {
		return err
	}
	mc, err := tactic.NewStoredModelWithLabels(md.Core(), []usymbol.Symbol{t.label})
	if err != nil {
		return err
	}
	return result.SetModelConverter(mc)
}

// solver_misc_01-F1: tactic2solver::get_labels is empty, so the labels the
// tactic reports through ::check_sat never reach the solver's caller.
func TestTacticSolverLabelsAlwaysEmpty(t *testing.T) {
	m := solverTestManager(t)
	params := solverTestParams(t, m)
	label, err := m.Symbols.Intern("L1")
	solverMust(t, err)
	s, err := NewTacticSolver(m, &labelTactic{Tactic: tactic.NewSkipTactic(), label: label}, nil, false, true, true, "QF_UF", TacticSolverConfig{Params: params})
	solverMust(t, err)
	t.Cleanup(func() { solverMust(t, s.Close()) })
	status, err := s.CheckSat(nil)
	solverMust(t, err)
	if status != lbool.True {
		t.Fatal("label tactic status", status)
	}
	labels, err := s.Labels()
	solverMust(t, err)
	if labels == nil || len(labels) != 0 {
		t.Fatal("tactic solver labels must be empty", labels)
	}
}
