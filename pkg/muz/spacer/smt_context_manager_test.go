package spacer_test

// Source: src/muz/spacer/spacer_smt_context_manager.cpp factory allocation,
// bounded round-robin reuse, statistics, and ownership.

import (
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSpacerSMTContextManagerPoolsGuardedSolvers(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	spacerMust(t, err)
	manager, err := command.Manager()
	spacerMust(t, err)
	globals, err := command.GlobalParameters()
	spacerMust(t, err)
	contexts, err := spacer.NewSMTContextManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	spacerMust(t, err)
	first, err := contexts.Fresh()
	spacerMust(t, err)
	second, err := contexts.Fresh()
	spacerMust(t, err)
	p := spacerBool(t, manager, "spacer-context-p")
	notP, err := manager.MkNot(p)
	spacerMust(t, err)
	spacerMust(t, first.Assert(p))
	spacerMust(t, second.Assert(notP))
	if status, err := first.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("first pooled context", status, err)
	}
	if status, err := second.CheckSat(nil); err != nil || status != lbool.True {
		t.Fatal("second pooled context", status, err)
	}
	collected := statistics.New()
	spacerMust(t, contexts.CollectStatistics(collected))
	if collected.Size() == 0 {
		t.Fatal("pooled context statistics are empty")
	}
	spacerMust(t, contexts.ResetStatistics())
	// ~smt_context_manager deletes the factories, which destroy the live
	// virtual solvers they own.
	spacerMust(t, contexts.Close())
	spacerMust(t, second.Close())
	spacerMust(t, first.Close())
	spacerMust(t, command.Close())
}
