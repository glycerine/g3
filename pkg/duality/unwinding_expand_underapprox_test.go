package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityExpandUnderapproxNodesNativeRecursiveAndFailurePaths(t *testing.T) {
	for _, mode := range []string{"sat", "nested", "refuted", "unresolved"} {
		t.Run(mode, func(t *testing.T) {
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
			term, err := ctx.BoolConst("expand-under-R")
			must(err)
			closeExpr(t, term)
			sourceNode, err := source.CreateNode(term)
			must(err)
			truth, err := ctx.BoolVal(mode != "refuted")
			must(err)
			closeExpr(t, truth)
			rule, err := NewTransformer(ctx, nil, nil, truth)
			must(err)
			defer rule.Close()
			sourceEdge, err := source.CreateEdge(sourceNode, rule, nil)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			outer, err := NewUnwinding(source, graph, covering)
			must(err)
			original, err := outer.CreateNodeInstance(sourceNode, 0)
			must(err)
			if mode != "unresolved" {
				_, err = outer.CreateEdgeInstance(sourceEdge, original, nil)
				must(err)
			}
			under, err := outer.CreateNodeInstance(sourceNode, 0)
			must(err)
			_, err = graph.CreateLowerBoundEdge(under)
			must(err)
			must(covering.MapUnderapproximation(under, original))
			if mode == "nested" {
				// The first rederivation reaches a second hidden underapproximation leaf.
				second, err := outer.CreateNodeInstance(sourceNode, 0)
				must(err)
				_, err = outer.CreateEdgeInstance(sourceEdge, second, nil)
				must(err)
				hidden, err := outer.CreateNodeInstance(sourceNode, 0)
				must(err)
				_, err = graph.CreateLowerBoundEdge(hidden)
				must(err)
				must(covering.MapUnderapproximation(hidden, second))
				declaration, err := term.Decl()
				must(err)
				defer declaration.Close()
				parentRule, err := NewTransformer(ctx, []*FuncDecl{declaration}, nil, term)
				must(err)
				defer parentRule.Close()
				parentSource, err := source.CreateEdge(sourceNode, parentRule, []*Node{sourceNode})
				must(err)
				_, err = outer.CreateEdgeInstance(parentSource, original, []*Node{hidden})
				must(err)
			}
			tree, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer tree.Close()
			must(tree.BindSolver(native))
			must(tree.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			root, err := tree.CloneNode(under)
			must(err)
			_, err = tree.CreateLowerBoundEdge(root)
			must(err)
			base, err := NewDerivationTree(graph, tree)
			must(err)
			bookkeeping, err := NewBacktrackingDerivation(base)
			must(err)
			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			services := &DerivationSearch{Search: bookkeeping, Checker: checker, Auxiliary: aux, Simplifier: manager}
			must(native.Push())
			caller, err := ctx.BoolConst("expand-under-caller")
			must(err)
			closeExpr(t, caller)
			callerRaw, err := caller.Raw()
			must(err)
			must(native.Assert(callerRaw))
			status, err := tree.Check(nil, nil, nil)
			must(err)
			if status != lbool.True {
				t.Fatal(status)
			}
			baseline, err := native.NumAssertions()
			must(err)
			oldOutgoing := root.Outgoing
			err = outer.ExpandUnderapproxNodes(tree, root, services, true)
			switch mode {
			case "refuted":
				if !errors.Is(err, ErrBogusUnderapproximation) {
					t.Fatal("missing source bogus-underapproximation error", err)
				}
				empty, err := original.Annotation.SubsetEq(rule, aux)
				must(err)
				if !empty || base.Heuristic.updates[original] == 0 {
					t.Fatal("refutation did not publish learned upper fact/heuristic update")
				}
			case "unresolved":
				if !errors.Is(err, ErrWrapperOwner) {
					t.Fatal("unresolved expansion failure missing", err)
				}
			default:
				must(err)
				expected := 2
				if mode == "nested" {
					expected = 4
				}
				if len(tree.nodes) != expected || tree.dualModel == nil {
					t.Fatal("recursive rederivation nodes/model missing", len(tree.nodes), expected)
				}
			}
			if tree.closed || root.closed || root.Outgoing != oldOutgoing || base.Top != nil || len(bookkeeping.stack) != 1 {
				t.Fatal("recursive search consumed graph/root or template bookkeeping")
			}
			// Source installs the complemented model-state bound and never restores it.
			bound, err := original.Bound.Formula()
			must(err)
			defer bound.Close()
			raw, err := bound.Raw()
			must(err)
			yes, err := m.MkTrue()
			must(err)
			want, err := m.MkNot(yes)
			must(err)
			if raw != want {
				t.Fatal("source model-state bound not left installed")
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline || len(tree.scopes) != 1 {
				t.Fatal("recursive query leaked caller scopes/assertions", depth, assertions, baseline)
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityExpandUnderapproxNodesUnmappedLeafNeedsNoServices(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	source, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	graph, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := NewUnwinding(source, graph, covering)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	term, err := ctx.BoolConst("expand-under-unmapped")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, term)
	root, err := tree.CreateNode(term)
	if err != nil {
		t.Fatal(err)
	}
	if err = outer.ExpandUnderapproxNodes(tree, root, nil, false); err != nil {
		t.Fatal(err)
	}
	if len(tree.nodes) != 1 || len(tree.edges) != 0 {
		t.Fatal("unmapped leaf mutated")
	}
}
