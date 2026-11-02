package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"
)

// Only statistics are controlled: assertions, proofs, models and scopes still
// execute in the real native solver. This makes the 500/501 policy deterministic.
type conjectureDecisionSolver struct {
	solver.Solver
	reads, delta int
}

func (s *conjectureDecisionSolver) CollectStatistics(dst *statistics.Statistics) error {
	s.reads++
	if err := s.Solver.CollectStatistics(dst); err != nil {
		return err
	}
	if s.reads >= 2 {
		dst.UpdateUint("decisions", uint32(s.delta))
	}
	return nil
}

func TestDualityProveConjectureNativeBoundsOwnershipAndDecisionThreshold(t *testing.T) {
	for _, mode := range []string{"proved", "disproved", "output", "threshold500", "threshold501", "unresolved"} {
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
			p, err := ctx.BoolConst("conjecture-p")
			must(err)
			closeExpr(t, p)
			q, err := ctx.BoolConst("conjecture-q")
			must(err)
			closeExpr(t, q)
			for _, term := range []*Expr{p, q} {
				decl, err := term.Decl()
				must(err)
				must(source.DeclareConstant(decl))
				must(decl.Close())
			}
			relationTerm, err := ctx.BoolConst("conjecture-R")
			must(err)
			closeExpr(t, relationTerm)
			leafTerm, err := ctx.BoolConst("conjecture-leaf")
			must(err)
			closeExpr(t, leafTerm)
			leafDecl, err := leafTerm.Decl()
			must(err)
			defer leafDecl.Close()
			relation, err := source.CreateNode(relationTerm)
			must(err)
			leafSource, err := source.CreateNode(leafTerm)
			must(err)
			rule, err := NewTransformer(ctx, []*FuncDecl{leafDecl}, nil, leafTerm)
			must(err)
			defer rule.Close()
			edge, err := source.CreateEdge(relation, rule, []*Node{leafSource})
			must(err)
			leafRule, err := NewTransformer(ctx, nil, nil, p)
			must(err)
			defer leafRule.Close()
			leafEdge, err := source.CreateEdge(leafSource, leafRule, nil)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			must(u.CreateEdgesByChildMap())
			leaf, err := u.CreateNodeInstance(leafSource, 0)
			must(err)
			_, err = u.CreateEdgeInstance(leafEdge, leaf, nil)
			must(err)
			other, err := u.CreateNodeInstance(relation, 0)
			must(err)
			_, err = u.CreateEdgeInstance(edge, other, []*Node{leaf})
			must(err)
			must(u.ActivateInstance(other))
			node, err := u.CreateNodeInstance(relation, 0)
			must(err)
			if mode != "unresolved" {
				_, err = u.CreateEdgeInstance(edge, node, []*Node{leaf})
				must(err)
			}
			covering.Update(node)
			saved, err := m.RefGet(node.Bound.formula)
			must(err)
			working, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer working.Close()
			var boundSolver solver.Solver = native
			threshold := mode == "threshold500" || mode == "threshold501"
			if threshold {
				delta := 500
				if mode == "threshold501" {
					delta = 501
				}
				boundSolver = &conjectureDecisionSolver{Solver: native, delta: delta}
			}
			must(working.BindSolver(boundSolver))
			must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			base, err := NewDerivationTree(graph, working)
			must(err)
			bookkeeping, err := NewBacktrackingDerivation(base)
			must(err)
			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			search := &DerivationSearch{Search: bookkeeping, Checker: checker, Auxiliary: aux, Simplifier: manager}
			cex := NewCounterexample()
			defer cex.Close()
			prior, err := NewRPFP(ctx)
			must(err)
			priorRoot, err := prior.CreateNode(relationTerm)
			must(err)
			must(cex.Set(prior, priorRoot))
			var output *Counterexample
			var oldOutput *RPFP
			if mode == "output" {
				output = NewCounterexample()
				defer output.Close()
				oldOutput, err = NewRPFP(ctx)
				must(err)
				oldRoot, err := oldOutput.CreateNode(relationTerm)
				must(err)
				must(output.Set(oldOutput, oldRoot))
			}
			conjectureTerm := q
			if mode == "proved" {
				conjectureTerm = p
			}
			conjecture, err := NewTransformer(ctx, nil, nil, conjectureTerm)
			must(err)
			defer conjecture.Close()
			must(native.Push())
			caller, err := ctx.BoolConst("conjecture-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			proved, err := u.ProveConjecture(node, conjecture, search, cex, output, threshold)
			currentBound, readErr := m.RefGet(node.Bound.formula)
			must(readErr)
			switch mode {
			case "proved":
				must(err)
				if !proved || currentBound == saved || !working.closed || prior.closed || cex.Tree() != prior {
					t.Fatal("proved conjecture bound/ownership")
				}
				entails, err := node.Annotation.SubsetEq(conjecture, aux)
				must(err)
				if !entails {
					t.Fatal("proved upper fact not published")
				}
			case "unresolved":
				if !errors.Is(err, ErrWrapperOwner) || proved || currentBound == saved || !working.closed || cex.Tree() != prior || prior.closed {
					t.Fatal("error did not preserve source partially strengthened state", err)
				}
			default:
				must(err)
				if proved || currentBound != saved || cex.Tree() != nil || !prior.closed {
					t.Fatal("disproved conjecture did not restore bound/clear owner")
				}
				if output != nil {
					if output.Tree() != working || working.closed || !oldOutput.closed {
						t.Fatal("counterexample swap did not transfer result/release previous output")
					}
				} else if !working.closed {
					t.Fatal("discarded counterexample leaked")
				}
			}
			if threshold {
				want := 500
				if mode == "threshold501" {
					want = 501
				}
				if u.lastDecisions != want {
					t.Fatal("controlled decision count", u.lastDecisions, want)
				}
				if mode == "threshold501" {
					under := covering.coveredBy[node]
					if under == nil || covering.underapprox[under] != node {
						t.Fatal("expensive failure did not build covering underapproximation")
					}
				} else if len(covering.underapprox) != 0 {
					t.Fatal("500 decisions incorrectly triggered strict threshold")
				}
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("conjecture disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}
