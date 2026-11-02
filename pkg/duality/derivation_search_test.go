package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualitySlowSearchResolvedGraphsSATUNSATAndRestart(t *testing.T) {
	for _, mode := range []string{"sat", "unsat", "restart", "constrained", "underapprox"} {
		t.Run(mode, func(t *testing.T) {
			ctx, m, main, globals := dualityNativeSolverServicesFixture(t)
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
			defer func() { must(source.Close()) }()
			p, err := ctx.BoolConst("search-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			must(source.DeclareConstant(pd))
			must(pd.Close())
			var terms []*Expr
			var declarations []*FuncDecl
			var original []*Node
			names := []string{"search-root", "search-mid", "search-leaf"}
			if mode == "restart" {
				names = []string{"search-root", "search-mid", "search-inner", "search-leaf"}
			}
			for _, name := range names {
				term, err := ctx.BoolConst(name)
				must(err)
				closeExpr(t, term)
				terms = append(terms, term)
				decl, err := term.Decl()
				must(err)
				closeDecl(t, decl)
				declarations = append(declarations, decl)
				node, err := source.CreateNode(term)
				must(err)
				original = append(original, node)
			}
			must(original[0].Bound.SetEmpty())
			wantSAT := mode == "sat" || mode == "constrained" || mode == "underapprox"
			rootFormula := terms[1]
			if !wantSAT {
				negative, err := p.Not()
				must(err)
				closeExpr(t, negative)
				rootFormula, err = negative.And(terms[1])
				must(err)
				closeExpr(t, rootFormula)
			}
			var rules []*Transformer
			for i := range original {
				var relations []*FuncDecl
				formula := p
				if i+1 < len(original) {
					relations = []*FuncDecl{declarations[i+1]}
					formula = terms[i+1]
				}
				if i == 0 {
					formula = rootFormula
				}
				rule, err := NewTransformer(ctx, relations, nil, formula)
				must(err)
				defer rule.Close()
				rules = append(rules, rule)
			}

			var sourceEdges []*Edge
			for i, node := range original {
				var children []*Node
				if i+1 < len(original) {
					children = []*Node{original[i+1]}
				}
				edge, err := source.CreateEdge(node, rules[i], children)
				must(err)
				sourceEdges = append(sourceEdges, edge)
			}
			unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(unwinding.Close()) }()
			var mapped []*Node
			for _, node := range original {
				copy, err := unwinding.CloneNode(node)
				must(err)
				mapped = append(mapped, copy)
			}
			for i, node := range mapped {
				var children []*Node
				if i+1 < len(mapped) {
					children = []*Node{mapped[i+1]}
				}
				edge, err := unwinding.CreateEdge(node, rules[i], children)
				must(err)
				edge.Map = sourceEdges[i]
			}
			working, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(working.Close()) }()
			must(working.BindSolver(main))
			must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			base, err := NewDerivationTree(unwinding, working)
			must(err)
			base.FalseApprox = false
			search, err := NewBacktrackingDerivation(base)
			must(err)

			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			// Preserve an unrelated native caller scope around Derive.
			must(main.Push())
			caller, err := ctx.BoolConst("derive-caller")
			must(err)
			closeExpr(t, caller)
			callerRaw, err := caller.Raw()
			must(err)
			must(main.Assert(callerRaw))
			_, err = main.CheckSat(nil)
			must(err)
			baseline, err := main.NumAssertions()
			must(err)
			controller := &DerivationSearch{Search: search, Checker: checker, Auxiliary: aux, Simplifier: manager, EnableRestarts: mode == "restart"}
			sat, err := controller.Derive(mapped[0], mode == "underapprox", mode == "constrained")
			top := base.Top
			must(err)
			if sat != wantSAT {
				t.Fatal("search result mismatch", sat, mode)
			}
			depth, err := main.ScopeLevel()
			must(err)
			if depth != 1 || len(search.stack) != 1 {
				t.Fatal("search did not restore base solver/expansion levels", depth, len(search.stack))
			}
			remaining, err := main.NumAssertions()
			must(err)
			if remaining != baseline || len(working.scopes) != 1 {
				t.Fatal("Derive disturbed enclosing assertion/graph scope", remaining, baseline)
			}
			if !base.FalseApprox || base.Underapprox != (mode == "underapprox") || base.Constrained != (mode == "constrained") {
				t.Fatal("source derive mode flags not initialized")
			}

			if sat {
				if len(working.edges) != 3 || len(working.nodes) != 3 || working.dualModel == nil {
					t.Fatal("SAT counterexample graph/model missing")
				}
			} else if top.Outgoing != nil || len(working.edges) != 0 {
				t.Fatal("refuted derivation graph not unwound")
			}
			if mode == "restart" {
				if controller.restartInterval != 4 {
					t.Fatal("update threshold did not trigger exactly one restart", controller.restartInterval)
				}
			} else if controller.restartInterval != 3 {
				t.Fatal("unexpected restart")
			}
			for _, node := range original {
				full, err := node.Annotation.IsFull()
				must(err)
				if !full {
					t.Fatal("search mutated source rule annotations")
				}
			}
			must(main.Pop(1))
		})
	}
}

func TestDualitySlowSearchRestartPolicyAndFailureThresholds(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mapped, err := NewRPFP(ctx)
	must(err)
	defer mapped.Close()
	term, err := ctx.BoolConst("restart-policy-R")
	must(err)
	closeExpr(t, term)
	parent, err := mapped.CreateNode(term)
	must(err)
	child, err := mapped.CreateNode(term)
	must(err)
	_, err = mapped.CreateEdge(parent, parent.Annotation, []*Node{child})
	must(err)
	working, err := NewRPFPWithVocabulary(mapped.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(native))
	base, err := NewDerivationTree(mapped, working)
	must(err)
	base.FalseApprox = false
	search, err := NewBacktrackingDerivation(base)
	must(err)
	top, err := base.CreateApproximatedInstance(parent)
	must(err)
	base.Top = top
	must(working.Push())
	must(working.Push())
	base.RemoveLeaves(map[*Node]bool{top: true})
	must(search.ExpandMaterializedNode(top))
	instanceChild := top.Outgoing.Children[0]
	controller := &DerivationSearch{Search: search, restartInterval: 3}
	must(parent.Annotation.SetEmpty())
	must(child.Annotation.SetEmpty())
	failure := controller.learningFailure(ErrCaseInterpolationReallyBad)
	if !errors.Is(failure, ErrDerivationRestart) || !errors.Is(failure, ErrCaseInterpolationReallyBad) {
		t.Fatal("ReallyBad did not request restart")
	}
	for _, node := range []*Node{parent, child} {
		full, err := node.Annotation.IsFull()
		must(err)
		if !full {
			t.Fatal("ReallyBad did not clear mapped parent/child fact")
		}
	}
	must(parent.Annotation.SetEmpty())
	must(child.Annotation.SetEmpty())
	failure = controller.learningFailure(ErrCaseInterpolationBad)
	if !errors.Is(failure, ErrDerivationRestart) {
		t.Fatal("Bad did not request restart")
	}
	for _, node := range []*Node{parent, child} {
		empty, err := node.Annotation.IsEmpty()
		must(err)
		if !empty {
			t.Fatal("Bad unexpectedly cleared mapped facts")
		}
	}
	if !errors.Is(controller.learningFailure(interp.NewError("test interpolation exception")), ErrDerivationRestart) {
		t.Fatal("interpolation exception did not request restart")
	}
	if !errors.Is(controller.learningFailure(ErrCoreNotUnsat), ErrDerivationRestart) {
		t.Fatal("source core verification exception did not request restart")
	}
	sentinel := errors.New("unrelated resource failure")
	if controller.learningFailure(sentinel) != sentinel {
		t.Fatal("unrelated failure was converted to restart")
	}
	// Consecutive UNSAT rounds do not count as failures after backtracking.
	controller.wasSAT = false
	for i := 0; i < 20; i++ {
		must(controller.learningProgress(0))
	}
	if controller.updateFailures != 0 {
		t.Fatal("non-SAT learning failures counted")
	}
	controller.wasSAT = true
	for i := 0; i < 10; i++ {
		must(controller.learningProgress(0))
	}
	if controller.updateFailures != 10 {
		t.Fatal("learning-failure boundary changed")
	}
	if !errors.Is(controller.learningProgress(0), ErrDerivationRestart) {
		t.Fatal("eleventh failure did not restart")
	}
	full, err := parent.Annotation.IsFull()
	must(err)
	empty, err := child.Annotation.IsEmpty()
	must(err)
	if !full || !empty {
		t.Fatal("failure fallback did not clear only the expansion's mapped fact")
	}
	must(controller.learningProgress(1))
	if controller.updateFailures != 0 {
		t.Fatal("learning did not reset failure count")
	}
	controller.EnableRestarts = true
	controller.totalUpdates = 2
	if !errors.Is(controller.learningProgress(1), ErrDerivationRestart) {
		t.Fatal("update-count restart boundary changed")
	}
	must(controller.restart())
	depth, err := native.ScopeLevel()
	must(err)
	if controller.restartInterval != 4 || len(search.stack) != 1 || depth != 1 || !instanceChild.closed || len(base.leaves) != 1 || base.leaves[0] != top {
		t.Fatal("restart did not restore base frontier/scope")
	}
	must(working.Pop(1))
}
