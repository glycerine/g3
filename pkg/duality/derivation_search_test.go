package duality

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"strings"
	"testing"
)

func TestDualitySlowSearchResolvedGraphsSATUNSATAndRestart(t *testing.T) {
	for _, mode := range []string{"sat", "unsat", "restart", "constrained", "underapprox", "bound-sat", "bound-unsat", "bound-underapprox", "full-cex", "full-cex-underapprox", "full-cex-refuted"} {
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
			wantSAT := mode == "full-cex" || mode == "full-cex-underapprox" || mode == "sat" || mode == "constrained" || mode == "underapprox" || mode == "bound-sat" || mode == "bound-underapprox"
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
			// Even when every lower approximation is already full, constrained
			// completion must expand every active node into a complete tree.
			if mode == "full-cex-underapprox" {
				for _, node := range mapped {
					must(node.Underapprox.SetFull())
				}
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
			under := mode == "underapprox" || mode == "bound-underapprox" || mode == "full-cex-underapprox"
			fullCex := mode == "full-cex" || mode == "full-cex-underapprox" || mode == "full-cex-refuted"
			var sat bool
			if fullCex || mode == "bound-sat" || mode == "bound-unsat" || mode == "bound-underapprox" {
				covering, err := NewCovering(unwinding, aux)
				must(err)
				outer, err := NewUnwinding(source, unwinding, covering)
				must(err)
				cex := NewCounterexample()
				defer cex.Close()
				if fullCex {
					prior, err := NewRPFPWithVocabulary(source.vocabulary)
					must(err)
					priorRoot, err := prior.CreateNode(terms[0])
					must(err)
					must(cex.Set(prior, priorRoot))
					err = outer.BuildFullCex(mapped[0], controller, cex, under)
					sat = wantSAT
					if wantSAT {
						must(err)
						if !prior.closed || cex.Tree() != working || cex.Root() != base.Top || working.closed {
							t.Fatal("full counterexample replacement lost ownership")
						}
					} else {
						if !errors.Is(err, ErrFullCounterexample) || !working.closed || prior.closed || cex.Tree() != prior || cex.Root() != priorRoot {
							t.Fatal("refuted completion lost old counterexample or new graph", err)
						}
					}
				} else {
					startDecisions, err := nativeSolverDecisions(main)
					must(err)
					satisfied, err := outer.SatisfyUpperBound(mapped[0], controller, cex, under)
					must(err)
					endDecisions, err := nativeSolverDecisions(main)
					must(err)
					if outer.lastDecisions != endDecisions-startDecisions {
						t.Fatal("bound decision delta differs from native counter", outer.lastDecisions, endDecisions-startDecisions)
					}
					sat = !satisfied
					if sat {
						if cex.Tree() != working || cex.Root() != base.Top || working.closed {
							t.Fatal("SAT graph was not transferred to counterexample")
						}
					} else if cex.Tree() != nil || !working.closed {
						t.Fatal("UNSAT graph ownership was not released")
					}
				}
			} else {
				sat, err = controller.Derive(mapped[0], under, mode == "constrained")
				must(err)
			}
			top := base.Top
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
			expectedScopes := 1
			if working.closed {
				expectedScopes = 0
			}
			if remaining != baseline || len(working.scopes) != expectedScopes {
				t.Fatal("Derive disturbed enclosing assertion/graph scope", remaining, baseline)
			}
			if !base.FalseApprox || base.Underapprox != under || base.Constrained != (mode == "constrained" || fullCex) {
				t.Fatal("source derive mode flags not initialized")
			}

			if sat {
				if len(working.edges) != 3 || len(working.nodes) != 3 || working.dualModel == nil {
					t.Fatal("SAT counterexample graph/model missing")
				}
				if mode == "full-cex-underapprox" {
					// Constrained completion keeps the partial counterexample's
					// cloned lower facts. The source's compiled
					// SKIP_UNDERAPPROX_NODES branch does not replace them with
					// facts extracted from the completion model.
					for _, node := range working.nodes {
						full, err := node.Underapprox.IsFull()
						must(err)
						if !full {
							t.Fatal("constrained completion replaced a cloned lower fact", node.Number)
						}
					}
				}
			} else if top.Outgoing != nil || len(working.edges) != 0 {
				t.Fatal("refuted derivation graph not unwound")
			}
			if mode == "restart" {
				if controller.restartInterval != 4 {
					t.Fatal("update threshold did not trigger exactly one restart", controller.restartInterval)
				}
			} else if mode == "constrained" || fullCex {
				if controller.restartInterval != 0 {
					t.Fatal("fast completion initialized slow-search restarts")
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
	var report bytes.Buffer
	mapped.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &report}}
	failure := controller.learningFailure(ErrCaseInterpolationReallyBad)
	if !strings.Contains(report.String(), "incompleteness: cleared annotation and child annotations") {
		t.Fatal("missing incompleteness reset diagnostic", report.String())
	}
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
	pointerFailure := interp.NewError("pointer interpolation exception")
	for _, failure := range []error{&pointerFailure, fmt.Errorf("wrapped: %w", &pointerFailure)} {
		if !errors.Is(controller.learningFailure(failure), ErrDerivationRestart) {
			t.Fatal("pointer interpolation failure did not restart", failure)
		}
	}
	if !strings.Contains(report.String(), "interpolation failure:test interpolation exception") {
		t.Fatal("missing interpolation failure diagnostic", report.String())
	}
	if !errors.Is(controller.learningFailure(ErrCoreNotUnsat), ErrDerivationRestart) {
		t.Fatal("source core verification exception did not request restart")
	}
	mapped.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: dualityRefusingWriter{}}}
	reportingFailure := controller.learningFailure(ErrCoreNotUnsat)
	if !errors.Is(reportingFailure, errDualityWriter) || !errors.Is(reportingFailure, ErrCoreNotUnsat) || errors.Is(reportingFailure, ErrDerivationRestart) {
		t.Fatal("diagnostic failure lost its causes or requested restart", reportingFailure)
	}
	mapped.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &report}}
	sentinel := errors.New("unrelated resource failure")
	if controller.learningFailure(sentinel) != sentinel {
		t.Fatal("unrelated failure was converted to restart")
	}
	for _, cause := range []error{ErrCaseInterpolationReallyBad, ErrCaseInterpolationBad, ErrCoreNotUnsat, interp.NewError("joined interpolation failure")} {
		joined := errors.Join(cause, sentinel)
		failure := controller.learningFailure(joined)
		if !errors.Is(failure, sentinel) || errors.Is(failure, ErrDerivationRestart) {
			t.Fatal("resource failure converted into restart", failure)
		}
	}
	// Consecutive UNSAT rounds do not count as failures after backtracking.
	controller.wasSAT = false
	report.Reset()
	for i := 0; i < 20; i++ {
		must(controller.learningProgress(0))
	}
	if strings.Count(report.String(), "backtracked without learning") != 20 {
		t.Fatal("missing zero-update round diagnostics", report.String())
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
	report.Reset()
	if !errors.Is(controller.learningProgress(0), ErrDerivationRestart) {
		t.Fatal("eleventh failure did not restart")
	}
	if strings.Contains(report.String(), "backtracked without learning") || !strings.Contains(report.String(), "incompleteness: cleared annotation") {
		t.Fatal("restart diagnostic order", report.String())
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
