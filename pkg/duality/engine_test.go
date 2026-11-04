package duality

import (
	"bytes"
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"strings"
	"testing"
)

func TestDualityEngineNativeLifecycleRepeatedSolveAndHistoryTransfer(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		name := "safe"
		if unsafe {
			name = "unsafe"
		}
		t.Run(name, func(t *testing.T) {
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
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			rootTerm, err := ctx.BoolConst("engine-root")
			must(err)
			closeExpr(t, rootTerm)
			childTerm, err := ctx.BoolConst("engine-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			defer childDecl.Close()
			root, err := source.CreateNode(rootTerm)
			must(err)
			must(root.Bound.SetEmpty())
			child, err := source.CreateNode(childTerm)
			must(err)
			rule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
			must(err)
			defer rule.Close()
			_, err = source.CreateEdge(root, rule, []*Node{child})
			must(err)
			value, err := ctx.BoolVal(unsafe)
			must(err)
			closeExpr(t, value)
			leaf, err := NewTransformer(ctx, nil, nil, value)
			must(err)
			defer leaf.Close()
			_, err = source.CreateEdge(child, leaf, nil)
			must(err)
			previousProfile := NewProfiler(nil)
			ctx.activeProfiler = previousProfile
			profile := NewProfiler(nil)
			config := EngineConfig{Profiler: profile, Native: native, Auxiliary: aux, Factory: factory, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Simplifier: manager}
			engine, err := NewEngine(source, config)
			must(err)
			defer engine.Close()
			if engine.config.Interpolation.Profiler != profile.interpolation {
				t.Fatal("engine did not use its profiler's interpolation owner")
			}
			other := interp.NewProfiler()
			config.Interpolation = &interp.RunConfig{Profiler: other}
			alternate, err := NewEngine(source, config)
			must(err)
			if alternate.config.Interpolation.Profiler != other || config.Interpolation.SolverFactory != nil {
				t.Fatal("explicit interpolation configuration was changed")
			}
			must(alternate.Close())
			config.Interpolation = nil
			copied := *engine
			if err = copied.Close(); !errors.Is(err, ErrWrapperOwner) {
				t.Fatal("copied owning engine accepted", err)
			}
			if engine.SetOption("not-an-option", "1") {
				t.Fatal("unknown option accepted")
			}
			must(native.Push())
			caller, err := ctx.BoolConst("engine-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			// Exercise the actual interpolation service through the engine's
			// normalized config; this fixture's solve can bypass interpolation.
			p, err := ctx.BoolConst("engine-profile-interpolation")
			must(err)
			closeExpr(t, p)
			notP, err := p.Not()
			must(err)
			closeExpr(t, notP)
			truth, err := ctx.BoolVal(true)
			must(err)
			closeExpr(t, truth)
			left, err := NewTermTree(p, nil)
			must(err)
			right, err := NewTermTree(notP, nil)
			must(err)
			tree, err := NewTermTree(truth, []*TermTree{left, right})
			must(err)
			defer tree.Close()
			interpolatingGraph, err := NewRPFP(ctx)
			must(err)
			defer interpolatingGraph.Close()
			must(interpolatingGraph.BindSolver(native))
			_, interpolant, err := interpolatingGraph.InterpolateTree(manager, tree, nil, false, engine.config.Interpolation)
			must(err)
			if interpolant == nil {
				t.Fatal("fixture did not interpolate")
			}
			must(interpolant.Close())
			var report bytes.Buffer
			must(profile.interpolation.Print(&report))
			if !strings.Contains(report.String(), "Proof interpolation") {
				t.Fatal("interpolation missing from the reported profiler", report.String())
			}
			baseline, err := native.NumAssertions()
			must(err)
			var previousGraph, previousCex *RPFP
			for round := 0; round < 2; round++ {
				solved, err := engine.Solve()
				if ctx.activeProfiler != previousProfile || profile.current != profile.top {
					t.Fatal("solve timer scope not restored")
				}
				must(err)
				if solved == unsafe || engine.graph == nil || engine.graph.closed || engine.local != nil || engine.global != nil || engine.heuristic != nil || engine.cases.Graph != nil {
					t.Fatal("engine result/per-run ownership", round, solved)
				}
				if round > 0 && (!previousGraph.closed || (previousCex != nil && !previousCex.closed)) {
					t.Fatal("repeated solve did not release old unwinding/replay graph")
				}
				if unsafe {
					if engine.GetCounterexample().Tree() == nil || engine.GetCounterexample().Tree().closed {
						t.Fatal("missing owning engine counterexample")
					}
				} else if engine.GetCounterexample().Tree() != nil {
					t.Fatal("safe engine kept counterexample")
				}
				previousGraph = engine.graph
				previousCex = engine.GetCounterexample().Tree()
			}
			// LearnFrom owns cached history facts and takes the old counterexample;
			// the old engine can then release its unwinding before the new solve.
			next, err := NewEngine(source, config)
			must(err)
			defer next.Close()
			must(next.LearnFrom(engine))
			if len(next.proposers) != 1 || engine.GetCounterexample().Tree() != nil || next.GetCounterexample().Tree() != previousCex {
				t.Fatal("LearnFrom history/cex ownership")
			}
			must(engine.Close())
			if engine.GetCounterexample() != nil || !previousGraph.closed {
				t.Fatal("closed engine exposed owned results")
			}
			solved, err := next.Solve()
			must(err)
			if solved == unsafe || len(next.proposers) != 0 || (previousCex != nil && !previousCex.closed) {
				t.Fatal("learned/replayed engine lifecycle")
			}
			lastGraph := next.UnwindingGraph()
			lastCex := next.GetCounterexample().Tree()
			must(next.Close())
			must(next.Close())
			if !lastGraph.closed || (lastCex != nil && !lastCex.closed) {
				t.Fatal("engine Close leaked results")
			}
			if _, err = next.Solve(); !errors.Is(err, ErrWrapperOwner) {
				t.Fatal("closed engine solved", err)
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("engine lifecycle disturbed caller solver scopes/assertions")
			}
			// Configured services stay borrowed after both owning engines close.
			_, err = aux.CheckSat(nil)
			must(err)
			_, err = manager.ASTManager()
			must(err)
			must(native.Pop(1))
		})
	}
}

func TestDualityEngineNativeFailureCleanupAndRecovery(t *testing.T) {
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
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("engine-recovery-R")
	must(err)
	closeExpr(t, term)
	root, err := source.CreateNode(term)
	must(err)
	must(root.Bound.SetEmpty())
	root.RecursionBound = 3
	previousProfile := NewProfiler(nil)
	ctx.activeProfiler = previousProfile
	profile := NewProfiler(nil)
	engine, err := NewEngine(source, EngineConfig{Profiler: profile, Native: native, Auxiliary: aux, Factory: factory, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Simplifier: manager})
	must(err)
	defer engine.Close()
	if !engine.SetOption("stratified_inlining", "1") {
		t.Fatal("option rejected")
	}
	if _, err = engine.Solve(); !errors.Is(err, ErrLevelizedUnwinding) {
		t.Fatal("open-source failure missing", err)
	}
	if ctx.activeProfiler != previousProfile || profile.current != profile.top {
		t.Fatal("failed solve timer scope not restored")
	}
	old := engine.UnwindingGraph()
	if old == nil || old.closed || engine.local != nil || engine.global != nil || engine.heuristic != nil || engine.running {
		t.Fatal("failed engine leaked run services/state")
	}
	value, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, value)
	rule, err := NewTransformer(ctx, nil, nil, value)
	must(err)
	defer rule.Close()
	_, err = source.CreateEdge(root, rule, nil)
	must(err)
	solved, err := engine.Solve()
	must(err)
	if !solved || !old.closed || engine.unwinding.RecursionBound != 0 || engine.IsResultRecursionBounded() {
		t.Fatal("recovered engine or automatic finite-node recursion mode")
	}
	must(engine.Close())
	full, err := root.Annotation.IsEmpty()
	must(err)
	if !full {
		t.Fatal("engine consumed borrowed source graph")
	}
}
