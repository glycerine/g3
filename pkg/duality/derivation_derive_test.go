package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityDeriveRestoresCallerScopeAfterUnresolvedExpansion(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	aux, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer aux.Close()
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	term, err := ctx.BoolConst("derive-unresolved-R")
	must(err)
	closeExpr(t, term)
	root, err := original.CreateNode(term)
	must(err)
	must(root.Bound.SetEmpty())
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(native))
	must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	base, err := NewDerivationTree(original, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	checker, err := NewLocalEdgeChecker(original, factory, nil)
	must(err)
	defer checker.Close()
	controller := &DerivationSearch{Search: search, Checker: checker, Auxiliary: aux, Simplifier: manager}
	must(native.Push())
	caller, err := ctx.BoolConst("derive-error-caller")
	must(err)
	closeExpr(t, caller)
	callerRaw, err := caller.Raw()
	must(err)
	must(native.Assert(callerRaw))
	_, err = native.CheckSat(nil)
	must(err)
	baseline, err := native.NumAssertions()
	must(err)
	if _, err := controller.Derive(root, false, false); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("unresolved expansion error missing", err)
	}
	depth, err := native.ScopeLevel()
	must(err)
	assertions, err := native.NumAssertions()
	must(err)
	if depth != 1 || assertions != baseline || len(working.scopes) != 1 || len(search.stack) != 1 {
		t.Fatal("partial expansion leaked caller scope/assertions", depth, assertions)
	}
	if base.Top == nil || base.Top.closed || len(working.nodes) != 1 {
		t.Fatal("caller-owned partial graph was lost")
	}
	if len(search.nodeMap) != 0 || len(search.updatedNodes) != 0 {
		t.Fatal("failed search kept expansion/update tracking")
	}
	// A Derive object represents one query; the partial graph remains available
	// to its owner, while reuse is rejected before another solver scope is pushed.
	if _, err := controller.Derive(root, false, false); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("used derivation object accepted a second query", err)
	}
	depth, err = native.ScopeLevel()
	must(err)
	if depth != 1 {
		t.Fatal("reuse rejection changed scope depth")
	}
	must(native.Pop(1))
}
