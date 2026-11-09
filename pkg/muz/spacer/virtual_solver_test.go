package spacer_test

// Source: src/muz/spacer/spacer_virtual_solver.cpp guarded sharing, delayed
// scopes, auxiliary-core filtering, and proof cleanup.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestVirtualSolverSharesBackendAndIsolatesGuardedViews(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	spacerMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	factory, err := spacer.NewVirtualSolverFactory(backend)
	spacerMust(t, err)
	first, err := factory.Fresh()
	spacerMust(t, err)
	second, err := factory.Fresh()
	spacerMust(t, err)

	p := spacerBool(t, manager, "virtual-p")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	spacerMust(t, first.Assert(p))
	spacerMust(t, second.Assert(notP))
	if status, err := first.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("first isolated view", status, err)
	}
	if status, err := second.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("second isolated view", status, err)
	}

	spacerMust(t, first.Push())
	spacerMust(t, first.Assert(notP))
	if status, err := first.CheckSat(nil); err != nil || status != lbool.False {
		t.Fatal("materialized delayed scope", status, err)
	}
	spacerMust(t, first.Pop(1))
	if status, err := first.CheckSat([]ast.ASTHandle{notP}); err != nil || status != lbool.False {
		t.Fatal("base assertion and explicit assumption", status, err)
	}
	core, err := first.UnsatCore()
	spacerMust(t, err)
	if len(core) != 1 || core[0] != notP {
		t.Fatal("virtual predicate leaked into core", core)
	}
	proof, err := first.Proof()
	spacerMust(t, err)
	proofAgain, err := first.Proof()
	spacerMust(t, err)
	if proofAgain != proof {
		t.Fatal("virtual proof cache changed identity", proof, proofAgain)
	}
	var printed bytes.Buffer
	spacerMust(t, ast.DisplayLegacySMT(&printed, manager, proof, 0, nil))
	if strings.Contains(printed.String(), "vsolver#") {
		t.Fatal("virtual predicate leaked into proof", printed.String())
	}
	if assumptions, err := first.NumAssumptions(); err != nil || assumptions != 1 {
		t.Fatal("private guard counted as public assumption", assumptions, err)
	}
	stats := statistics.New()
	spacerMust(t, factory.CollectStatistics(stats))
	counts := map[statistics.Key]uint32{}
	for index := 0; index < stats.Size(); index++ {
		if stats.IsUint(index) {
			counts[stats.Key(index)] += stats.UintValue(index)
		}
	}
	if counts["virtual_solver.checks"] != 4 || counts["virtual_solver.checks.sat"] != 2 {
		t.Fatal("virtual solver source statistics", counts)
	}
	spacerMust(t, factory.ResetStatistics())
	stats.Reset()
	spacerMust(t, factory.CollectStatistics(stats))
	for index := 0; index < stats.Size(); index++ {
		if stats.Key(index) == "virtual_solver.checks" || stats.Key(index) == "virtual_solver.checks.sat" || stats.Key(index) == "virtual_solver.checks.undef" {
			t.Fatal("virtual solver reset retained counters", stats.Key(index))
		}
	}

	spacerMust(t, first.Close())
	if status, err := second.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("retiring first view damaged second", status, err)
	}
	spacerMust(t, second.Close())
	spacerMust(t, factory.Close())
	spacerMust(t, command.Close())
}

func TestVirtualSolverCloseAfterResourceExhaustion(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	factory, err := spacer.NewVirtualSolverFactory(backend)
	spacerMust(t, err)
	first, err := factory.Fresh()
	spacerMust(t, err)
	second, err := factory.Fresh()
	spacerMust(t, err)
	limit := manager.ResourceLimit()
	spacerMust(t, limit.Push(1))
	limit.IncOffset(2)
	limit.IncCancel()
	spacerMust(t, first.Close())
	if limit.CancelCount() != 1 {
		t.Fatal("solver cleanup changed independent cancellation", limit.CancelCount())
	}
	limit.DecCancel()
	limit.Pop()
	if status, e := second.CheckSat(nil); e != nil || status != lbool.True {
		t.Fatal("surviving virtual solver", status, e)
	}
	spacerMust(t, second.Close())
	spacerMust(t, factory.Close())
	spacerMust(t, command.Close())
}
