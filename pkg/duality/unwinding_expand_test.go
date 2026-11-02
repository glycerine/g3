package duality

import (
	"bytes"
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"
)

func TestDualityExpandNodeNativeInductionCandidatesAndUsingCondition(t *testing.T) {
	for _, mode := range []string{"sat", "using-unsat", "duplicate-child", "edge-order", "full-parent"} {
		t.Run(mode, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer manager.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			p, err := ctx.BoolConst("expand-induction-p")
			must(err)
			closeExpr(t, p)
			pd, err := p.Decl()
			must(err)
			defer pd.Close()
			must(source.DeclareConstant(pd))
			parentTerm, err := ctx.BoolConst("expand-induction-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("expand-induction-child")
			must(err)
			closeExpr(t, childTerm)
			cd, err := childTerm.Decl()
			must(err)
			defer cd.Close()
			parent, err := source.CreateNode(parentTerm)
			must(err)
			child, err := source.CreateNode(childTerm)
			must(err)
			children := []*Node{child}
			relations := []*FuncDecl{cd}
			formula := childTerm
			if mode == "using-unsat" {
				formula, err = p.And(childTerm)
				must(err)
				closeExpr(t, formula)
			}
			if mode == "duplicate-child" {
				secondTerm, err := ctx.BoolConst("expand-induction-child-position2")
				must(err)
				closeExpr(t, secondTerm)
				secondDecl, err := secondTerm.Decl()
				must(err)
				defer secondDecl.Close()
				children = append(children, child)
				relations = append(relations, secondDecl)
				formula, err = childTerm.And(secondTerm)
				must(err)
				closeExpr(t, formula)
			}
			rule, err := NewTransformer(ctx, relations, nil, formula)
			must(err)
			defer rule.Close()
			edge, err := source.CreateEdge(parent, rule, children)
			must(err)
			edges := []*Edge{edge}
			if mode == "edge-order" {
				second, err := source.CreateEdge(parent, rule, children)
				must(err)
				edges = append(edges, second)
			}
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			var report bytes.Buffer
			graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: &report, Params: ctx.config.params, Globals: globals}, depth: -1}
			covering, err := NewCovering(graph, native)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			negative, err := p.Not()
			must(err)
			closeExpr(t, negative)
			var instances []*Node
			for _, fact := range []*Expr{p, negative} {
				instance, err := u.CreateNodeInstance(child, 0)
				must(err)
				h, err := fact.Raw()
				must(err)
				must(m.SetRef(instance.Annotation.formula, h))
				_, err = graph.CreateLowerBoundEdge(instance)
				must(err)
				instances = append(instances, instance)
			}
			must(u.ActivateInstance(instances[0]))
			if mode == "full-parent" {
				older, err := u.CreateNodeInstance(parent, 0)
				must(err)
				_, err = graph.CreateLowerBoundEdge(older)
				must(err)
				must(u.ActivateInstance(older))
			}
			must(u.CreateEdgesByChildMap())
			if len(u.edgesByChild[child]) != len(edges) {
				t.Fatal("reverse index duplicated repeated child positions")
			}
			// Rebuilding the index must replace it rather than append duplicate entries.
			must(u.CreateEdgesByChildMap())
			if len(u.edgesByChild[child]) != len(edges) {
				t.Fatal("reverse index rebuild accumulated entries")
			}
			must(native.Push())
			caller, err := ctx.BoolConst("expand-induction-caller")
			must(err)
			closeExpr(t, caller)
			h, err := caller.Raw()
			must(err)
			must(native.Assert(h))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			must(u.ExpandNode(instances[1], native, derivationEvaluatorFixture(t, ctx, globals), manager))
			if len(u.PendingNodes()) != 0 || len(covering.instances[child]) != 2 {
				t.Fatal("expansion did not activate instance")
			}
			if mode != "using-unsat" && mode != "full-parent" && !strings.Contains(report.String(), "]induction failure: ") {
				t.Fatal("missing live induction event", report.String())
			}
			if mode == "using-unsat" || mode == "full-parent" {
				if _, ok := u.NextCandidate(); ok {
					t.Fatal("generated candidate without new-instance induction failure")
				}
			} else {
				for _, want := range edges {
					candidate, ok := u.NextCandidate()
					if !ok || candidate.Edge != want || len(candidate.Children) != len(children) {
						t.Fatal("candidate source order/arity")
					}
					usesNew := false
					for _, selected := range candidate.Children {
						if selected == instances[1] {
							usesNew = true
						}
						if selected != instances[0] && selected != instances[1] {
							t.Fatal("candidate borrows temporary checker node")
						}
					}
					if !usesNew {
						t.Fatal("candidate does not use expanded instance")
					}
				}
				if _, ok := u.NextCandidate(); ok {
					t.Fatal("duplicate induction candidate")
				}
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("expansion disturbed caller solver scope/assertions")
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityCandidateExtractionInactiveAndPairedMarkers(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("extract-candidate-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	edge, err := source.CreateEdge(relation, relation.Annotation, []*Node{relation})
	must(err)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	var instances []*Node
	for i := 0; i < 2; i++ {
		node, err := u.CreateNodeInstance(relation, 0)
		must(err)
		_, err = graph.CreateLowerBoundEdge(node)
		must(err)
		must(u.ActivateInstance(node))
		instances = append(instances, node)
	}
	checker, err := NewRPFP(ctx)
	must(err)
	defer checker.Close()
	must(checker.BindSolver(native))
	must(checker.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	root, err := checker.CloneNode(relation)
	must(err)
	child, err := checker.CloneNode(relation)
	must(err)
	_, err = checker.CreateEdge(root, relation.Annotation, []*Node{child})
	must(err)
	must(checker.Push())
	activation, vars, err := checker.RedVars(child)
	must(err)
	defer activation.Close()
	for _, v := range vars {
		defer v.Close()
	}
	a, err := NodeMarker(instances[0], child)
	must(err)
	defer a.Close()
	b, err := NodeMarker(instances[1], child)
	must(err)
	defer b.Close()
	ah, err := a.Raw()
	must(err)
	bh, err := b.Raw()
	must(err)
	notA, err := m.MkNot(ah)
	must(err)
	must(native.Assert(notA))
	must(native.Assert(bh))
	active, err := activation.Raw()
	must(err)
	inactive, err := m.MkNot(active)
	must(err)
	must(native.Push())
	must(native.Assert(inactive))
	status, err := checker.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	candidate, err := u.ExtractCandidateFromCex(edge, checker, root)
	must(err)
	if candidate.Children[0] != instances[0] {
		t.Fatal("inactive child did not select first uncovered instance")
	}
	must(native.Pop(1))
	must(native.Assert(active))
	status, err = checker.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	candidate, err = u.ExtractCandidateFromCex(edge, checker, root)
	must(err)
	if candidate.Children[0] != instances[1] {
		t.Fatal("paired marker selection failed")
	}
	covering.coveredBy[instances[1]] = instances[1]
	if _, err = u.ExtractCandidateFromCex(edge, checker, root); !errors.Is(err, ErrNoInductionCandidate) {
		t.Fatal("missing no-candidate error", err)
	}
	must(checker.Pop(1))
}
