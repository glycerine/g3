package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityGlobalCandidatesNativeCachedFilteringFallbackAndMarkers(t *testing.T) {
	for _, mode := range []string{"updated", "fallback", "full-scan", "full-parent", "axioms", "repeated", "no-child"} {
		t.Run(mode, func(t *testing.T) {
			ctx, m, aux, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
			must(err)
			defer factory.Close()
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer manager.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			p, err := ctx.BoolConst("global-candidate-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			defer pd.Close()
			must(source.DeclareConstant(pd))
			parentTerm, err := ctx.BoolConst("global-candidate-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("global-candidate-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			defer childDecl.Close()
			parent, err := source.CreateNode(parentTerm)
			must(err)
			childSource, err := source.CreateNode(childTerm)
			must(err)
			relations := []*FuncDecl{childDecl}
			children := []*Node{childSource}
			formula := childTerm
			if mode == "repeated" {
				secondTerm, err := ctx.BoolConst("global-candidate-child-position2")
				must(err)
				closeExpr(t, secondTerm)
				secondDecl, err := secondTerm.Decl()
				must(err)
				defer secondDecl.Close()
				relations = append(relations, secondDecl)
				children = append(children, childSource)
				formula, err = childTerm.And(secondTerm)
				must(err)
				closeExpr(t, formula)
			}
			rule, err := NewTransformer(ctx, relations, nil, formula)
			must(err)
			defer rule.Close()
			edge, err := source.CreateEdge(parent, rule, children)
			must(err)
			no, err := ctx.BoolVal(false)
			must(err)
			closeExpr(t, no)
			falseRule, err := NewTransformer(ctx, nil, nil, no)
			must(err)
			defer falseRule.Close()
			childEdge, err := source.CreateEdge(childSource, falseRule, nil)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			instance, err := u.CreateNodeInstance(childSource, 0)
			must(err)
			ph, err := p.Raw()
			must(err)
			must(m.SetRef(instance.Annotation.formula, ph))
			_, err = u.CreateEdgeInstance(childEdge, instance, nil)
			must(err)
			if mode != "no-child" {
				must(u.ActivateInstance(instance))
			}
			if mode == "full-parent" {
				full, err := u.CreateNodeInstance(parent, 0)
				must(err)
				_, err = u.CreateEdgeInstance(edge, full, []*Node{instance})
				must(err)
				must(u.ActivateInstance(full))
			}
			var axioms []*Expr
			if mode == "axioms" {
				negative, err := p.Not()
				must(err)
				closeExpr(t, negative)
				axioms = []*Expr{negative}
			}
			checker, err := NewLocalEdgeChecker(source, factory, axioms)
			must(err)
			defer checker.Close()
			if mode != "fallback" && mode != "full-scan" {
				u.updatedNodes = map[*Node]bool{parent: true}
			}
			proofMode, err := m.ProofsEnabled()
			must(err)
			must(aux.Push())
			caller, err := ctx.BoolConst("global-candidate-caller")
			must(err)
			closeExpr(t, caller)
			callerRaw, err := caller.Raw()
			must(err)
			must(aux.Assert(callerRaw))
			_, err = aux.CheckSat(nil)
			must(err)
			baseline, err := aux.NumAssertions()
			must(err)
			evaluator := derivationEvaluatorFixture(t, ctx, globals)
			must(u.GenCandidatesFromInductionFailure(checker, evaluator, manager, mode == "full-scan"))
			wantCandidate := mode != "full-parent" && mode != "axioms" && mode != "no-child"
			candidate, ok := u.NextCandidate()
			if ok != wantCandidate {
				t.Fatal("global induction candidate availability", ok, mode)
			}
			if ok {
				if candidate.Edge != edge || len(candidate.Children) != len(children) {
					t.Fatal("candidate source/arity")
				}
				for _, selected := range candidate.Children {
					if selected != instance {
						t.Fatal("paired marker extraction leaked cloned child")
					}
				}
			}
			if _, ok = u.NextCandidate(); ok {
				t.Fatal("unexpected extra candidate")
			}
			if len(u.updatedNodes) != 0 {
				t.Fatal("updated-parent scan set not cleared")
			}
			if mode == "updated" && checker.pool.states[childEdge] != nil {
				t.Fatal("successful updated scan unnecessarily performed full scan")
			}
			if (mode == "fallback" || mode == "full-scan") && checker.pool.states[childEdge] == nil {
				t.Fatal("full-scan/fallback skipped source edge")
			}
			if mode == "full-parent" && checker.pool.states[edge] != nil {
				t.Fatal("full parent solution allocated a dedicated solver")
			}
			if mode != "no-child" {
				fact, err := NewTransformer(ctx, nil, nil, p)
				must(err)
				defer fact.Close()
				implies, err := childSource.Annotation.SubsetEq(fact, aux)
				must(err)
				reverse, err := fact.SubsetEq(childSource.Annotation, aux)
				must(err)
				if !implies || reverse {
					t.Fatal("published source solution omitted instance marker")
				}
			}
			if state := checker.pool.states[edge]; state != nil {
				guards, err := state.solver.NumAssertions()
				must(err)
				if u.updatedNodes == nil {
					u.updatedNodes = make(map[*Node]bool)
				}
				u.updatedNodes[parent] = true
				must(u.GenCandidatesFromInductionFailure(checker, evaluator, manager, false))
				after, err := state.solver.NumAssertions()
				must(err)
				if guards != after {
					t.Fatal("repeat global scan grew stable guard cache", guards, after)
				}
				for {
					_, ok := u.NextCandidate()
					if !ok {
						break
					}
				}
			}
			restored, err := m.ProofsEnabled()
			must(err)
			depth, err := aux.ScopeLevel()
			must(err)
			remaining, err := aux.NumAssertions()
			must(err)
			if restored != proofMode || checker.graph.solver != nil || len(checker.graph.scopes) != 1 || depth != 1 || remaining != baseline {
				t.Fatal("global scan leaked proof mode, solver binding or scopes/assertions")
			}
			must(aux.Pop(1))
		})
	}
}
