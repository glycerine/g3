package spacer_test

// Source: src/muz/spacer/spacer_prop_solver.cpp poor-man's MaxSAT,
// level-guard activation, delta levels, cores, models, and background scopes.

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type spacerPropFixture struct {
	command  *cmd_context.Context
	manager  *ast.ASTManager
	backends [2]*smtcore.SMTSolver
	solver   *spacer.PropSolver
	coop     *cooperate.Cooperation
}

func newSpacerPropFixture(t *testing.T, keepProxy bool) *spacerPropFixture {
	return newSpacerPropFixtureWithMode(t, keepProxy, false, spacer.ItpCoreLegacyFarkas)
}

func newSpacerPropFixtureWithProofs(t *testing.T, keepProxy, proofs bool) *spacerPropFixture {
	return newSpacerPropFixtureWithMode(t, keepProxy, proofs, spacer.ItpCoreLegacyFarkas)
}

func newSpacerPropFixtureWithMode(t *testing.T, keepProxy, proofs bool, mode spacer.ItpCoreMode) *spacerPropFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	if proofs {
		spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	}
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	fixture := &spacerPropFixture{command: command, manager: manager}
	fixture.coop, err = cooperate.NewCooperation(manager.Budget())
	spacerMust(t, err)
	evaluator := model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: manager.NumeralService().RuntimeOwner(), Cooperation: fixture.coop}
	var generic [2]solver.Solver
	for i := range fixture.backends {
		fixture.backends[i], err = smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals, ModelEvaluator: evaluator})
		spacerMust(t, err)
		generic[i] = fixture.backends[i]
	}
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	fixture.solver, err = spacer.NewPropSolver(manager, generic, truth, "spacer-test", spacer.PropSolverConfig{
		Itp:       spacer.ItpSolverConfig{Params: globals.Context, Globals: globals, CoreMode: mode, FarkasAConst: true},
		KeepProxy: keepProxy,
	})
	spacerMust(t, err)
	return fixture
}

func TestPropSolverPublishesNewPluginCore(t *testing.T) {
	for _, mode := range []spacer.ItpCoreMode{spacer.ItpCoreNewFarkas, spacer.ItpCoreNewOptimized} {
		fixture := newSpacerPropFixtureWithMode(t, true, true, mode)
		p := spacerBool(t, fixture.manager, "spacer-new-core-p")
		notP, err := fixture.manager.MkNot(p)
		spacerMust(t, err)
		spacerMust(t, fixture.solver.Assert(notP))
		result, err := fixture.solver.CheckAssumptions([]ast.ASTHandle{p}, nil, spacer.PropCheckOptions{NeedCore: true})
		spacerMust(t, err)
		if result.Status != lbool.False || !result.FarkasCore || len(result.Core) != 1 {
			t.Fatal("new plugin PropSolver core", mode, result.Status, result.FarkasCore, len(result.Core))
		}
		handle, err := fixture.manager.RefGet(result.Core[0])
		spacerMust(t, err)
		if handle != p {
			t.Fatal("new plugin core selected wrong literal", mode, handle, p)
		}
		spacerMust(t, result.Close(fixture.manager))
		fixture.close(t)
	}
}

func TestPropSolverUsesPooledNativeDefaultBackends(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	var pools [2]*spacer.SMTContextManager
	var backends [2]*spacer.VirtualSolver
	var generic [2]solver.Solver
	for index := range pools {
		pools[index], err = spacer.NewSMTContextManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
		spacerMust(t, err)
		backends[index], err = pools[index].Fresh()
		spacerMust(t, err)
		generic[index] = backends[index]
	}
	truth, err := manager.MkTrue()
	spacerMust(t, err)
	prop, err := spacer.NewPropSolver(manager, generic, truth, "pooled-default", spacer.PropSolverConfig{
		Itp: spacer.ItpSolverConfig{Params: globals.Context, Globals: globals},
	})
	spacerMust(t, err)
	p := spacerBool(t, manager, "pooled-default-p")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	spacerMust(t, prop.Assert(notP))
	result, err := prop.CheckAssumptions([]ast.ASTHandle{p}, nil, spacer.PropCheckOptions{NeedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || !result.FarkasCore || len(result.Core) != 1 {
		t.Fatal("pooled native-default core", result.Status, result.FarkasCore, len(result.Core))
	}
	core, err := manager.RefGet(result.Core[0])
	spacerMust(t, err)
	if core != p {
		t.Fatal("pooled native-default core literal", core, p)
	}
	spacerMust(t, result.Close(manager))
	spacerMust(t, prop.Close())
	for index := len(backends) - 1; index >= 0; index-- {
		spacerMust(t, backends[index].Close())
		spacerMust(t, pools[index].Close())
	}
	spacerMust(t, command.Close())
}

func TestPropSolverPublishesFarkasCore(t *testing.T) {
	fixture := newSpacerPropFixtureWithProofs(t, true, true)
	p := spacerBool(t, fixture.manager, "spacer-farkas-p")
	q := spacerBool(t, fixture.manager, "spacer-farkas-q")
	notP, err := fixture.manager.MkNot(p)
	spacerMust(t, err)
	notQ, err := fixture.manager.MkNot(q)
	spacerMust(t, err)
	disjunction, err := fixture.manager.MkOr(p, q)
	spacerMust(t, err)
	spacerMust(t, fixture.solver.Assert(notP))
	spacerMust(t, fixture.solver.Assert(notQ))
	result, err := fixture.solver.CheckAssumptions([]ast.ASTHandle{disjunction}, nil, spacer.PropCheckOptions{NeedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || !result.FarkasCore || len(result.Core) != 1 {
		t.Fatal("Farkas PropSolver core", result.Status, result.FarkasCore, len(result.Core))
	}
	core, err := fixture.manager.RefGet(result.Core[0])
	spacerMust(t, err)
	var printed bytes.Buffer
	spacerMust(t, ast.DisplayLegacySMT(&printed, fixture.manager, core, 0, nil))
	if strings.Contains(printed.String(), "spacer_proxy!") {
		t.Fatal("proxy was not eliminated from Farkas core", printed.String())
	}
	spacerMust(t, result.Close(fixture.manager))
	fixture.close(t)
}

func (f *spacerPropFixture) close(t *testing.T) {
	t.Helper()
	spacerMust(t, f.solver.Close())
	for i := len(f.backends) - 1; i >= 0; i-- {
		spacerMust(t, f.backends[i].Close())
	}
	spacerMust(t, f.coop.Close())
	spacerMust(t, f.command.Close())
}

func TestPropSolverReducesSoftAssumptionsAndPublishesSubsetCore(t *testing.T) {
	fixture := newSpacerPropFixture(t, false)
	p := spacerBool(t, fixture.manager, "spacer-soft-p")
	q := spacerBool(t, fixture.manager, "spacer-soft-q")
	notP, err := fixture.manager.MkNot(p)
	spacerMust(t, err)
	spacerMust(t, fixture.solver.Assert(notP))
	result, err := fixture.solver.CheckAssumptions(nil, []ast.ASTHandle{p, q}, spacer.PropCheckOptions{NeedModel: true})
	spacerMust(t, err)
	if result.Status != lbool.True || len(result.Soft) != 1 || result.Soft[0] != q || result.Model == nil {
		t.Fatal("soft reduction result", result.Status, result.Soft, result.Model)
	}
	spacerMust(t, result.Close(fixture.manager))

	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{p}, nil, spacer.PropCheckOptions{NeedCore: true, SubsetBasedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || result.FarkasCore || len(result.Core) != 1 {
		t.Fatal("subset core result", result.Status, result.FarkasCore, len(result.Core))
	}
	core, err := fixture.manager.RefGet(result.Core[0])
	spacerMust(t, err)
	if core != p {
		t.Fatal("wrong subset core", core, p)
	}
	spacerMust(t, result.Close(fixture.manager))
	fixture.close(t)
}

func TestPropSolverScopesWeakAbstractionParameters(t *testing.T) {
	fixture := newSpacerPropFixture(t, true)
	globals, err := fixture.command.GlobalParameters()
	spacerMust(t, err)
	baseline, err := globals.Context.NewValue()
	spacerMust(t, err)
	ignoreName := "arith.ignore_int"
	arrayWeakName := "array.weak"
	spacerMust(t, globals.Context.SetBoolName(&baseline, &ignoreName, true))
	spacerMust(t, globals.Context.SetBoolName(&baseline, &arrayWeakName, true))
	spacerMust(t, fixture.backends[0].UpdateParams(&baseline))
	spacerMust(t, globals.Context.Del(&baseline))
	truth, err := fixture.manager.MkTrue()
	spacerMust(t, err)
	weakness := uint32(2)
	result, err := fixture.solver.CheckAssumptions([]ast.ASTHandle{truth}, nil, spacer.PropCheckOptions{Weakness: &weakness})
	spacerMust(t, err)
	if result.Status != lbool.True {
		t.Fatal("weak-abstraction scoped check", result.Status)
	}
	spacerMust(t, result.Close(fixture.manager))
	restored, err := fixture.backends[0].Params()
	spacerMust(t, err)
	ignore, err := globals.Context.GetBoolName(restored, &ignoreName, false)
	spacerMust(t, err)
	arrayWeak, err := globals.Context.GetBoolName(restored, &arrayWeakName, false)
	spacerMust(t, err)
	if !ignore || !arrayWeak {
		t.Fatal("weak-abstraction parameters were not restored", ignore, arrayWeak)
	}
	fixture.close(t)
}

// prop_solver::internal_check_assumptions reads m_ctx->get_model after
// l_undef; after NUM_CONFLICTS mk_proto_model built nothing, so there is no
// candidate model (AUDIT.md smtcore_07-F1).
func TestPropSolverConflictLimitHasNoCandidateModel(t *testing.T) {
	fixture := newSpacerPropFixture(t, true)
	globals, err := fixture.command.GlobalParameters()
	spacerMust(t, err)
	limited, err := globals.Context.NewValue()
	spacerMust(t, err)
	limitName := "max_conflicts"
	spacerMust(t, globals.Context.SetUIntName(&limited, &limitName, 0))
	spacerMust(t, fixture.backends[0].UpdateParams(&limited))
	spacerMust(t, globals.Context.Del(&limited))
	a := spacerBool(t, fixture.manager, "spacer-candidate-a")
	b := spacerBool(t, fixture.manager, "spacer-candidate-b")
	notA, err := fixture.manager.MkNot(a)
	spacerMust(t, err)
	notB, err := fixture.manager.MkNot(b)
	spacerMust(t, err)
	var clauses []ast.ASTHandle
	for _, pair := range [][2]ast.ASTHandle{{a, b}, {notA, b}, {a, notB}, {notA, notB}} {
		clause, makeErr := fixture.manager.MkOr(pair[0], pair[1])
		spacerMust(t, makeErr)
		clauses = append(clauses, clause)
	}
	formula, err := fixture.manager.MkAndArgs(clauses)
	spacerMust(t, err)
	result, err := fixture.solver.CheckAssumptions([]ast.ASTHandle{formula}, nil, spacer.PropCheckOptions{NeedModel: true})
	spacerMust(t, err)
	if result.Status != lbool.Undef || result.Model != nil {
		t.Fatal("conflict-limit PropSolver candidate", result.Status, result.Model)
	}
	spacerMust(t, result.Close(fixture.manager))
	fixture.close(t)
}

func TestPropSolverNormalAndDeltaLevelActivation(t *testing.T) {
	fixture := newSpacerPropFixture(t, true)
	p := spacerBool(t, fixture.manager, "spacer-level-p")
	q := spacerBool(t, fixture.manager, "spacer-level-q")
	notP, err := fixture.manager.MkNot(p)
	spacerMust(t, err)
	notQ, err := fixture.manager.MkNot(q)
	spacerMust(t, err)
	spacerMust(t, fixture.solver.AssertAtLevel(p, 0))
	spacerMust(t, fixture.solver.AssertAtLevel(q, 1))
	if levels, err := fixture.solver.LevelCount(); err != nil || levels != 2 {
		t.Fatal("level count", levels, err)
	}

	level0 := uint32(0)
	result, err := fixture.solver.CheckAssumptions([]ast.ASTHandle{notP}, nil, spacer.PropCheckOptions{Level: &level0, NeedCore: true, SubsetBasedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || !result.AssumesLevel || result.UsesLevel != 0 {
		t.Fatal("normal level zero result", result.Status, result.AssumesLevel, result.UsesLevel)
	}
	spacerMust(t, result.Close(fixture.manager))

	level1 := uint32(1)
	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{notP}, nil, spacer.PropCheckOptions{Level: &level1, NeedModel: true})
	spacerMust(t, err)
	if result.Status != lbool.True || result.Model == nil || result.AssumesLevel || result.UsesLevel != math.MaxUint32 {
		t.Fatal("inactive earlier level", result.Status, result.Model, result.AssumesLevel, result.UsesLevel)
	}
	spacerMust(t, result.Close(fixture.manager))

	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{notQ}, nil, spacer.PropCheckOptions{Level: &level1, DeltaLevel: true, NeedCore: true, SubsetBasedCore: true})
	spacerMust(t, err)
	if result.Status != lbool.False || !result.AssumesLevel || result.UsesLevel != 1 {
		t.Fatal("delta level one result", result.Status, result.AssumesLevel, result.UsesLevel)
	}
	spacerMust(t, result.Close(fixture.manager))

	infinity := spacer.InfinityLevel
	notPAtInfinity, err := fixture.manager.MkNot(p)
	spacerMust(t, err)
	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{notPAtInfinity}, nil, spacer.PropCheckOptions{Level: &infinity, NeedModel: true})
	spacerMust(t, err)
	if result.Status != lbool.True || result.Model == nil {
		t.Fatal("infinite level did not deactivate finite frames", result.Status, result.Model)
	}
	spacerMust(t, result.Close(fixture.manager))
	if levels, err := fixture.solver.LevelCount(); err != nil || levels != 2 {
		t.Fatal("infinite level allocated finite guards", levels, err)
	}

	background := spacerBool(t, fixture.manager, "spacer-background")
	notBackground, err := fixture.manager.MkNot(background)
	spacerMust(t, err)
	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{notBackground}, nil, spacer.PropCheckOptions{Background: []ast.ASTHandle{background}})
	spacerMust(t, err)
	if result.Status != lbool.False {
		t.Fatal("scoped background result", result.Status)
	}
	spacerMust(t, result.Close(fixture.manager))
	result, err = fixture.solver.CheckAssumptions([]ast.ASTHandle{notBackground}, nil, spacer.PropCheckOptions{})
	spacerMust(t, err)
	if result.Status != lbool.True {
		t.Fatal("scoped background survived check", result.Status)
	}
	spacerMust(t, result.Close(fixture.manager))

	spacerMust(t, fixture.solver.Close())
	if _, err := fixture.solver.LevelCount(); !errors.Is(err, spacer.ErrPropSolverOwner) {
		t.Fatal("closed PropSolver accepted operation", err)
	}
	for i := len(fixture.backends) - 1; i >= 0; i-- {
		spacerMust(t, fixture.backends[i].Close())
	}
	spacerMust(t, fixture.coop.Close())
	spacerMust(t, fixture.command.Close())
}
