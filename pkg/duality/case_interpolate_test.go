package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func setCaseInterpolationFaultSchedule(t *testing.T, count, limit int) {
	t.Helper()
	previous := caseInterpolationFaultSchedule
	caseInterpolationFaultSchedule = struct{ count, limit int }{count, limit}
	t.Cleanup(func() { caseInterpolationFaultSchedule = previous })
}

func TestDualityCaseInterpolationNativeLearningFaultAndRetry(t *testing.T) {
	setCaseInterpolationFaultSchedule(t, 0, 1)
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
	auxiliary, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer auxiliary.Close()
	services := derivationEvaluatorFixture(t, ctx, globals)
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	quantifiers, err := qe.NewLite(m, nil, true, qe.Config{Context: ctx.config.params, Globals: globals, Cooperation: services.Cooperation})
	must(err)
	defer quantifiers.Close()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(main))
	must(g.BindModelEvaluator(services))
	p, err := ctx.BoolConst("case-learn-p")
	must(err)
	closeExpr(t, p)
	pd, err := p.Decl()
	must(err)
	must(g.DeclareConstant(pd))
	must(pd.Close())
	parentTerm, err := ctx.BoolConst("case-learn-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("case-learn-child")
	must(err)
	closeExpr(t, childTerm)
	parent, err := g.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := g.CreateNode(childTerm)
	must(err)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	body, err := negative.And(childTerm)
	must(err)
	closeExpr(t, body)
	childDecl, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDecl)
	parentRule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, body)
	must(err)
	defer parentRule.Close()
	childRule, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer childRule.Close()
	parentEdge, err := g.CreateEdge(parent, parentRule, []*Node{child})
	must(err)
	childEdge, err := g.CreateEdge(child, childRule, nil)
	must(err)
	must(g.Push())
	must(g.AssertNode(parent))
	must(g.AssertEdge(parentEdge, 0, false, false))
	must(g.AssertEdge(childEdge, 0, false, false))
	// Source SimplifyNode enters case interpolation after an UNSAT check.
	warmStatus, err := g.Check(parent, nil, nil)
	must(err)
	if warmStatus != lbool.False {
		t.Fatal("case context was not UNSAT", warmStatus)
	}
	baseline, err := main.NumAssertions()
	must(err)
	original, err := NewRPFPWithVocabulary(g.vocabulary)
	must(err)
	defer original.Close()
	base, err := NewDerivationTree(original, g)
	must(err)
	base.Top = parent
	search, err := NewBacktrackingDerivation(base)
	must(err)
	for attempt := 0; attempt < 3; attempt++ {
		runner := &CaseInterpolator{Graph: g, Auxiliary: auxiliary, Extensional: true, Simplifier: simplifier, Quantifiers: quantifiers}
		err := search.SimplifyNode(child, runner, nil)
		if attempt == 1 {
			if !errors.Is(err, ErrCaseInterpolationBad) {
				t.Fatal("source TEST_BAD schedule did not fail second case", err)
			}
		} else {
			must(err)
		}
		depth, err := main.ScopeLevel()
		must(err)
		auxDepth, err := auxiliary.ScopeLevel()
		must(err)
		assertions, err := main.NumAssertions()
		must(err)
		if depth != 1 || auxDepth != 0 || assertions != baseline || len(childEdge.constraints) != 0 {
			t.Fatal("case iteration leaked solver scope/constraints", depth, auxDepth, assertions, "baseline", baseline, "constraints", len(childEdge.constraints), "attempt", attempt)
		}
		forward, err := child.Annotation.SubsetEq(childRule, auxiliary)
		must(err)
		backward, err := childRule.SubsetEq(child.Annotation, auxiliary)
		must(err)
		if !forward || !backward {
			t.Fatal("case interpolation did not learn p")
		}
		if attempt != 1 && len(g.proofCore) == 0 {
			t.Fatal("case proof assumptions not installed")
		}
	}
	if caseInterpolationFaultSchedule.limit != 2 || caseInterpolationFaultSchedule.count != 1 {
		t.Fatal("fault schedule did not survive new runner", caseInterpolationFaultSchedule)
	}
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("case learning changed surrounding node annotation")
	}
	must(g.Pop(1))
	status, err := main.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("enclosing graph scope did not restore")
	}
}

func TestDualityCaseInterpolationEmptyEdgeAndConsistentContext(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "consistent-context"
		if empty {
			name = "empty-edge"
		}
		t.Run(name, func(t *testing.T) {
			setCaseInterpolationFaultSchedule(t, 0, 1)
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
			auxiliary, err := factory.New(m, nil, false, true, false, "")
			must(err)
			defer auxiliary.Close()
			simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer simplifier.Close()
			g, err := NewRPFP(ctx)
			must(err)
			defer func() { must(g.Close()) }()
			must(g.BindSolver(main))
			must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			p, err := ctx.BoolConst("case-terminal-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			must(g.DeclareConstant(pd))
			must(pd.Close())
			relation, err := ctx.BoolConst("case-terminal-R")
			must(err)
			closeExpr(t, relation)
			node, err := g.CreateNode(relation)
			must(err)
			must(node.Bound.SetEmpty())
			rule, err := NewTransformer(ctx, nil, nil, p)
			must(err)
			defer rule.Close()
			if empty {
				must(rule.SetEmpty())
			}
			edge, err := g.CreateEdge(node, rule, nil)
			must(err)
			must(g.Push())
			must(g.AssertNode(node))
			must(g.AssertEdge(edge, 0, false, false))
			_, err = g.Check(node, nil, nil)
			must(err)
			baseline, err := main.NumAssertions()
			must(err)
			runner := &CaseInterpolator{Graph: g, Auxiliary: auxiliary, Extensional: true, Simplifier: simplifier}
			err = runner.Interpolate(node, node, nil)
			if empty {
				must(err)
				if len(g.proofCore) == 0 || caseInterpolationFaultSchedule.count != 0 {
					t.Fatal("empty edge ran a learning case or lost proof seed")
				}
			} else if !errors.Is(err, ErrCaseInterpolationReallyBad) {
				t.Fatal("consistent main context not rejected", err)
			}
			depth, err := main.ScopeLevel()
			must(err)
			auxDepth, err := auxiliary.ScopeLevel()
			must(err)
			count, err := main.NumAssertions()
			must(err)
			if depth != 1 || auxDepth != 0 || count != baseline || len(edge.constraints) != 0 {
				t.Fatal("terminal case did not restore scopes")
			}
			must(g.Pop(1))
		})
	}
}
