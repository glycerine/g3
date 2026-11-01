package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationUnsatSkipsIrrelevantExpansionLevels(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	aux, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer aux.Close()
	original, err := NewRPFP(ctx)
	must(err)
	defer func() { must(original.Close()) }()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer func() { must(working.Close()) }()
	must(working.BindSolver(s))
	term, err := ctx.BoolConst("unsat-skip-R")
	must(err)
	closeExpr(t, term)
	base, err := NewDerivationTree(original, working)
	must(err)
	base.FullExpand = true
	search, err := NewBacktrackingDerivation(base)
	must(err)
	var originals, instances []*Node
	for i := 0; i < 3; i++ {
		node, err := original.CreateNode(term)
		must(err)
		edge, err := original.CreateLowerBoundEdge(node)
		must(err)
		must(edge.F.SetEmpty())
		instance, err := base.CreateApproximatedInstance(node)
		must(err)
		originals = append(originals, node)
		instances = append(instances, instance)
	}
	base.Top = instances[0]
	checker, err := NewLocalEdgeChecker(original, factory, nil)
	must(err)
	defer checker.Close()
	must(working.Push())
	for _, node := range instances {
		base.RemoveLeaves(map[*Node]bool{node: true})
		must(working.Push())
		must(search.ExpandMaterializedNode(node))
	}
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	must(working.ConstrainEdgeLocalized(instances[2].Outgoing, falsity))
	status, err := working.Check(base.Top, nil, nil)
	must(err)
	if status != lbool.False {
		t.Fatal(status)
	}
	done, err := search.BacktrackUnsatisfiable(checker, aux)
	must(err)
	if !done || len(search.stack) != 1 || len(working.edges) != 0 || len(working.nodes) != 3 {
		t.Fatal("irrelevant expansions did not unwind to base")
	}
	for i, node := range originals {
		want := 1
		if i == 2 {
			want = 0
		}
		if search.Heuristic.updates[node] != want {
			t.Fatal("wrong unused expansion penalty", i, search.Heuristic.updates[node])
		}
	}
	depth, err := s.ScopeLevel()
	must(err)
	if depth != 1 || len(base.leaves) != 3 {
		t.Fatal("base scope/frontier not preserved", depth, len(base.leaves))
	}
	status, err = working.Check(base.Top, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("popped contradiction survived", status)
	}
	must(working.Pop(1))
}

func TestDerivationUnsatPropagatesRelevantExpansion(t *testing.T) {
	for _, name := range []string{"no-new-fact", "learned-fact", "propagation-stops-skipping"} {
		success := name != "no-new-fact"
		guard := name == "propagation-stops-skipping"
		t.Run(name, func(t *testing.T) {
			ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
			must(err)
			defer factory.Close()
			aux, err := factory.New(m, nil, false, false, false, "")
			must(err)
			defer aux.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer func() { must(source.Close()) }()
			r, err := ctx.BoolConst("backtrack-propagate-R")
			must(err)
			closeExpr(t, r)
			p, err := ctx.BoolConst("backtrack-propagate-p")
			must(err)
			closeExpr(t, p)
			q, err := ctx.BoolConst("backtrack-propagate-q")
			must(err)
			closeExpr(t, q)
			for _, term := range []*Expr{p, q} {
				decl, err := term.Decl()
				must(err)
				must(source.DeclareConstant(decl))
				must(decl.Close())
			}
			chosen := q
			if success {
				chosen = p
			}
			rule, err := NewTransformer(ctx, nil, nil, chosen)
			must(err)
			defer rule.Close()
			fact, err := NewTransformer(ctx, nil, nil, p)
			must(err)
			defer fact.Close()
			relation, err := source.CreateNode(r)
			must(err)
			sourceEdge, err := source.CreateEdge(relation, rule, []*Node{relation})
			must(err)
			unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(unwinding.Close()) }()
			parent, err := unwinding.CloneNode(relation)
			must(err)
			must(parent.Bound.SetEmpty())
			child, err := unwinding.CloneNode(relation)
			must(err)
			must(replaceNodeAnnotation(child, fact))
			mapped, err := unwinding.CreateEdge(parent, rule, []*Node{child})
			must(err)
			mapped.Map = sourceEdge
			_, err = unwinding.CreateLowerBoundEdge(child)
			must(err)
			working, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer func() { must(working.Close()) }()
			must(working.BindSolver(s))
			base, err := NewDerivationTree(unwinding, working)
			must(err)
			base.FullExpand = true
			search, err := NewBacktrackingDerivation(base)
			must(err)
			top, err := base.CreateApproximatedInstance(parent)
			must(err)
			base.Top = top
			checker, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer checker.Close()
			must(working.Push())
			must(working.AssertNode(top))
			var older *Node
			if guard {
				originalOlder, err := unwinding.CloneNode(relation)
				must(err)
				oldEdge, err := unwinding.CreateLowerBoundEdge(originalOlder)
				must(err)
				must(oldEdge.F.SetEmpty())
				older, err = base.CreateApproximatedInstance(originalOlder)
				must(err)
				base.RemoveLeaves(map[*Node]bool{older: true})
				must(working.Push())
				must(search.ExpandMaterializedNode(older))
			}

			base.RemoveLeaves(map[*Node]bool{top: true})
			must(working.Push())
			must(search.ExpandMaterializedNode(top))
			descendant := top.Outgoing.Children[0]
			base.RemoveLeaves(map[*Node]bool{descendant: true})
			must(working.Push())
			must(search.ExpandMaterializedNode(descendant))
			opposite, err := chosen.Not()
			must(err)
			closeExpr(t, opposite)
			must(working.ConstrainEdgeLocalized(descendant.Outgoing, opposite))
			status, err := working.Check(top, nil, nil)
			must(err)
			if status != lbool.False {
				t.Fatal(status)
			}
			used, err := search.LevelUsedInProof(len(search.stack) - 2)
			must(err)
			if !used {
				t.Fatal("preceding edge missing from native proof")
			}
			done, err := search.BacktrackUnsatisfiable(checker, aux)
			must(err)
			if done != (success && !guard) {
				t.Fatal("unexpected propagation/backtrack result", done, success)
			}
			if success {
				full, err := parent.Annotation.IsFull()
				must(err)
				if full {
					t.Fatal("propagated fact not published to unwinding")
				}
				redundant, err := parent.Annotation.SubsetEq(fact, aux)
				must(err)
				expectedDepth := 1
				if guard {
					expectedDepth = 2
				}
				if !redundant || len(search.stack) != expectedDepth || !descendant.closed || top.Outgoing != nil {
					t.Fatal("successful propagation did not learn and finish unwinding")
				}
			} else {
				full, err := parent.Annotation.IsFull()
				must(err)
				if !full || len(search.stack) != 2 || top.Outgoing == nil || descendant.closed || descendant.Outgoing != nil {
					t.Fatal("failed propagation did not stop at relevant expansion")
				}
				must(search.PopLevel())
			}
			if guard {
				if older.Outgoing == nil || search.Heuristic.updates[older.Map] != 0 {
					t.Fatal("stale proof core skipped/penalized an older level after propagation")
				}
				must(search.PopLevel())
			}
			if search.Heuristic.updates[parent] != 0 {
				t.Fatal("relevant expansion penalized as unused")
			}
			depth, err := s.ScopeLevel()
			must(err)
			if depth != 1 {
				t.Fatal("enclosing scope changed", depth)
			}
			must(working.Pop(1))
		})
	}
}
