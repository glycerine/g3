package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"strings"
	"testing"
)

func TestDualityOuterSolveNativeOrdinaryStratifiedAndUnderapprox(t *testing.T) {
	for _, mode := range []string{"ordinary-safe", "ordinary-unsafe", "stratified-safe", "stratified-unsafe", "ordinary-unsafe-underapprox", "ordinary-safe-all-edges", "ordinary-unsafe-all-edges"} {
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
			rootTerm, err := ctx.BoolConst("outer-root")
			must(err)
			closeExpr(t, rootTerm)
			childTerm, err := ctx.BoolConst("outer-child")
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
			unsafe := strings.Contains(mode, "unsafe")
			value, err := ctx.BoolVal(unsafe)
			must(err)
			closeExpr(t, value)
			leafRule, err := NewTransformer(ctx, nil, nil, value)
			must(err)
			defer leafRule.Close()
			_, err = source.CreateEdge(child, leafRule, nil)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			u.StratifiedInlining = strings.HasPrefix(mode, "stratified")
			u.FeasibleEdges = !strings.HasSuffix(mode, "all-edges")
			must(u.CreateEdgesByChildMap())
			local, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer local.Close()
			global, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer global.Close()
			cex := NewCounterexample()
			defer cex.Close()
			evaluator := derivationEvaluatorFixture(t, ctx, globals)
			s := &UnwindingSearch{Unwinding: u, Native: native, Evaluator: evaluator, Local: local, Global: global, Auxiliary: aux, Simplifier: manager, Counterexample: cex, UseUnderapprox: strings.HasSuffix(mode, "underapprox")}
			must(native.Push())
			caller, err := ctx.BoolConst("outer-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			must(s.CreateInitialUnwinding())
			if u.StratifiedInlining && len(graph.nodes) != 0 {
				t.Fatal("active stratified initialization should defer graph construction")
			}
			if !u.FeasibleEdges {
				if len(graph.nodes) != 4 || len(u.leaves) != 2 || u.topological == nil {
					t.Fatal("all-edge initial instances missing")
				}
				initialRoot := graph.nodes[2]
				if initialRoot.Map != root || u.backEdges[initialRoot][child] != 1 {
					t.Fatal("source pre-sort zero-rank back-edge accounting changed")
				}
				for _, leaf := range u.leaves {
					count := 0
					for _, active := range covering.instances[leaf.Map] {
						if active == leaf {
							count++
						}
					}
					if count != 1 {
						t.Fatal("source temporary leaf duplication/removal ordering", count)
					}
				}
			}

			solved, err := s.SolveMain()
			must(err)
			if solved == unsafe {
				t.Fatal("outer solve result", solved, mode)
			}
			if unsafe {
				if cex.Tree() == nil || cex.Root() == nil || cex.Root().Map.Map != root || len(cex.Tree().edges) != 2 {
					t.Fatal("outer counterexample missing full mapped derivation")
				}
			} else {
				empty, err := root.Annotation.IsEmpty()
				must(err)
				if !empty || cex.Tree() != nil || u.IsResultRecursionBounded() {
					t.Fatal("outer solution/publication status")
				}
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("outer loop disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityRecursionResultUsesOnlyUncoveredActiveInstances(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("outer-recursion-result")
	must(err)
	closeExpr(t, term)
	original, err := source.CreateNode(term)
	must(err)
	original.RecursionBound = 2
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	node, err := u.CreateNodeInstance(original, 0)
	must(err)
	_, err = graph.CreateLowerBoundEdge(node)
	must(err)
	u.RecursionBound = 0
	u.backEdges[node] = map[*Node]int{original: 3}
	must(u.TestRecursionBounded())
	if u.IsResultRecursionBounded() {
		t.Fatal("pending instance affected result")
	}
	must(u.ActivateInstance(node))
	must(u.TestRecursionBounded())
	if !u.IsResultRecursionBounded() {
		t.Fatal("active over-bound instance omitted")
	}
	covering.coveredBy[node] = node
	must(u.TestRecursionBounded())
	if u.IsResultRecursionBounded() {
		t.Fatal("covered instance affected result")
	}
	delete(covering.coveredBy, node)
	u.backEdges[node][original] = 2
	must(u.TestRecursionBounded())
	if u.IsResultRecursionBounded() {
		t.Fatal("inclusive recursion limit cut off")
	}
	u.backEdges[node][original] = 3
	u.RecursionBound = -1
	must(u.TestRecursionBounded())
	if u.IsResultRecursionBounded() {
		t.Fatal("disabled recursion bound left stale result")
	}
}
