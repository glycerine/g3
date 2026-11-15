package spacer_test

// Source: src/muz/spacer/spacer_itp_solver.cpp assumption proxying,
// background filtering, and scoped proxy-definition ownership.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func spacerBool(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	boolean, err := manager.MkBoolSort()
	spacerMust(t, err)
	symbol, err := manager.Symbols.Intern(name)
	spacerMust(t, err)
	result, err := manager.MkConst(symbol, boolean)
	spacerMust(t, err)
	return result
}

func TestItpSolverExpandsProxyCoreAndScopesDefinitions(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	defaultCore, err := spacer.NewItpSolver(backend, spacer.ItpSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal("native default optimized core mode", err)
	}
	spacerMust(t, defaultCore.Close())
	if _, err = spacer.NewItpSolver(backend, spacer.ItpSolverConfig{Params: globals.Context, Globals: globals, CoreMode: spacer.ItpCoreMode(255)}); !errors.Is(err, spacer.ErrITPCoreMode) {
		t.Fatal("unsupported core mode was not rejected", err)
	}
	wrapped, err := spacer.NewItpSolver(backend, spacer.ItpSolverConfig{Params: globals.Context, Globals: globals, CoreMode: spacer.ItpCoreLegacyFarkas})
	spacerMust(t, err)

	p := spacerBool(t, manager, "itp-proxy-p")
	q := spacerBool(t, manager, "itp-proxy-q")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	notQ, err := manager.MkNot(q)
	spacerMust(t, err)
	spacerMust(t, wrapped.Assert(notP))
	spacerMust(t, wrapped.Assert(notQ))
	disjunction, err := manager.MkOr(p, q)
	spacerMust(t, err)
	status, err := wrapped.CheckSat([]ast.ASTHandle{disjunction})
	spacerMust(t, err)
	if status != lbool.False {
		t.Fatal("proxied assumption status", status)
	}
	core, err := wrapped.UnsatCore()
	spacerMust(t, err)
	if len(core) != 1 || core[0] != disjunction {
		t.Fatal("proxy was not expanded in the public core", core, disjunction)
	}

	spacerMust(t, wrapped.Push())
	r := spacerBool(t, manager, "itp-proxy-r")
	s := spacerBool(t, manager, "itp-proxy-s")
	rOrS, err := manager.MkOr(r, s)
	spacerMust(t, err)
	status, err = wrapped.CheckSat([]ast.ASTHandle{rOrS})
	spacerMust(t, err)
	if status != lbool.True {
		t.Fatal("scoped proxy assumption status", status)
	}
	spacerMust(t, wrapped.Pop(1))

	tAtom := spacerBool(t, manager, "itp-background-t")
	uAtom := spacerBool(t, manager, "itp-background-u")
	both, err := manager.MkAnd(tAtom, uAtom)
	spacerMust(t, err)
	notBoth, err := manager.MkNot(both)
	spacerMust(t, err)
	spacerMust(t, wrapped.Assert(notBoth))
	spacerMust(t, wrapped.PushBackground(tAtom))
	if count, err := wrapped.NumBackground(); err != nil || count != 1 {
		t.Fatal("background count", count, err)
	}
	status, err = wrapped.CheckSat([]ast.ASTHandle{uAtom})
	spacerMust(t, err)
	if status != lbool.False {
		t.Fatal("background contradiction status", status)
	}
	core, err = wrapped.UnsatCore()
	spacerMust(t, err)
	if len(core) != 1 || core[0] != uAtom {
		t.Fatal("background leaked into public core", core, uAtom)
	}
	spacerMust(t, wrapped.PopBackground(1))

	spacerMust(t, wrapped.Close())
	if _, err := wrapped.CheckSat(nil); !errors.Is(err, spacer.ErrITPSolverOwner) {
		t.Fatal("closed wrapper accepted check", err)
	}
	spacerMust(t, backend.Close())
	spacerMust(t, command.Close())
}

func TestItpSolverExtractsFarkasCore(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_LRA", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	wrapped, err := spacer.NewItpSolver(backend, spacer.ItpSolverConfig{Params: globals.Context, Globals: globals, SplitLiterals: true, CoreMode: spacer.ItpCoreLegacyFarkas})
	spacerMust(t, err)

	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	spacerMust(t, err)
	x, err := manager.MkFreshConst("spacer-itp-x", realSort)
	spacerMust(t, err)
	y, err := manager.MkFreshConst("spacer-itp-y", realSort)
	spacerMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	spacerMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	spacerMust(t, err)
	sum, err := manager.MkArithmeticAdd(x, y)
	spacerMust(t, err)
	upper, err := manager.MkArithmeticLE(sum, minusOne)
	spacerMust(t, err)
	xLower, err := manager.MkArithmeticGE(x, zero)
	spacerMust(t, err)
	yLower, err := manager.MkArithmeticGE(y, zero)
	spacerMust(t, err)
	spacerMust(t, wrapped.Assert(upper))
	status, err := wrapped.CheckSat([]ast.ASTHandle{xLower, yLower})
	spacerMust(t, err)
	if status != lbool.False {
		t.Fatal("arithmetic contradiction status", status)
	}
	core, err := wrapped.ItpCore()
	spacerMust(t, err)
	if len(core) == 0 {
		t.Fatal("Farkas proof produced no interpolating core")
	}
	for i := range core {
		handle, err := manager.RefGet(core[i])
		spacerMust(t, err)
		if _, err = manager.GetKind(handle); err != nil {
			t.Fatal("invalid interpolating-core result", err)
		}
		spacerMust(t, manager.DelRef(&core[i]))
	}

	spacerMust(t, wrapped.Close())
	spacerMust(t, backend.Close())
	spacerMust(t, command.Close())
}

// AUDIT.md muz_spacer_01-F3: itp_solver::collect_statistics always reports
// time.itp_solver.itp_core (m_itp_watch, timing get_itp_core), and
// reset_statistics resets the watch.
func TestItpSolverCollectsItpCoreTime(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_LRA", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	wrapped, err := spacer.NewItpSolver(backend, spacer.ItpSolverConfig{Params: globals.Context, Globals: globals, SplitLiterals: true, CoreMode: spacer.ItpCoreLegacyFarkas})
	spacerMust(t, err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	spacerMust(t, err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	spacerMust(t, err)
	x, err := manager.MkFreshConst("spacer-itp-watch-x", realSort)
	spacerMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	spacerMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	spacerMust(t, err)
	upper, err := manager.MkArithmeticLE(x, minusOne)
	spacerMust(t, err)
	lower, err := manager.MkArithmeticGE(x, zero)
	spacerMust(t, err)
	spacerMust(t, wrapped.Assert(upper))
	status, err := wrapped.CheckSat([]ast.ASTHandle{lower})
	spacerMust(t, err)
	if status != lbool.False {
		t.Fatal("arithmetic contradiction status", status)
	}
	core, err := wrapped.ItpCore()
	spacerMust(t, err)
	for i := range core {
		spacerMust(t, manager.DelRef(&core[i]))
	}
	itpCoreTime := func() (float64, bool) {
		st := statistics.New()
		spacerMust(t, wrapped.CollectStatistics(st))
		for i := 0; i < st.Size(); i++ {
			if st.Key(i) == "time.itp_solver.itp_core" && !st.IsUint(i) {
				return st.DoubleValue(i), true
			}
		}
		return 0, false
	}
	if _, found := itpCoreTime(); !found {
		t.Fatal("missing time.itp_solver.itp_core")
	}
	spacerMust(t, wrapped.ResetStatistics())
	// statistics::update drops a zero increment, so after the reset the
	// key is absent (or zero).
	if seconds, found := itpCoreTime(); found && seconds != 0 {
		t.Fatal("reset_statistics kept the itp_core time", seconds)
	}
	spacerMust(t, wrapped.Close())
	spacerMust(t, backend.Close())
	spacerMust(t, command.Close())
}
