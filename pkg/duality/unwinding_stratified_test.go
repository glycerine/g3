package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

// Observe bindings only; every check still executes in the native solver.
type stratifiedCaseBindingSolver struct {
	solver.Solver
	cases    *CaseInterpolator
	original *RPFP
	graphs   map[*RPFP]int
	bad      bool
}

func (s *stratifiedCaseBindingSolver) CheckSat(args []ast.ASTHandle) (lbool.LBool, error) {
	graph := s.cases.Graph
	if graph == nil || graph == s.original || graph.solver != s {
		s.bad = true
	} else {
		s.graphs[graph]++
	}
	return s.Solver.CheckSat(args)
}

func TestDualityStratifiedInliningNativeForwardCycleAndCounterexample(t *testing.T) {
	for _, mode := range []string{"forward", "cycle", "counterexample", "underapprox"} {
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
			rootTerm, err := ctx.BoolConst("stratified-root")
			must(err)
			closeExpr(t, rootTerm)
			rootDecl, err := rootTerm.Decl()
			must(err)
			defer rootDecl.Close()
			root, err := source.CreateNode(rootTerm)
			must(err)
			var leafSource *Node
			var rootEdge *Edge
			if mode == "cycle" {
				secondTerm, err := ctx.BoolConst("stratified-cycle-position2")
				must(err)
				closeExpr(t, secondTerm)
				secondDecl, err := secondTerm.Decl()
				must(err)
				defer secondDecl.Close()
				formula, err := rootTerm.And(secondTerm)
				must(err)
				closeExpr(t, formula)
				rule, err := NewTransformer(ctx, []*FuncDecl{rootDecl, secondDecl}, nil, formula)
				must(err)
				defer rule.Close()
				rootEdge, err = source.CreateEdge(root, rule, []*Node{root, root})
				must(err)
				must(root.Bound.SetEmpty())
			} else {
				leafTerm, err := ctx.BoolConst("stratified-leaf")
				must(err)
				closeExpr(t, leafTerm)
				leafDecl, err := leafTerm.Decl()
				must(err)
				defer leafDecl.Close()
				leafSource, err = source.CreateNode(leafTerm)
				must(err)
				_, err = source.CreateLowerBoundEdge(leafSource)
				must(err)
				rule, err := NewTransformer(ctx, []*FuncDecl{leafDecl}, nil, leafTerm)
				must(err)
				defer rule.Close()
				rootEdge, err = source.CreateEdge(root, rule, []*Node{leafSource})
				must(err)
				if mode != "forward" {
					must(root.Bound.SetEmpty())
				}
			}
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, aux)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			u.StratifiedInlining = true
			local, err := NewLocalEdgeChecker(source, factory, nil)
			must(err)
			defer local.Close()
			cex := NewCounterexample()
			defer cex.Close()
			cases := &CaseInterpolator{Graph: source, badCount: 6, badLimit: 16}
			observed := &stratifiedCaseBindingSolver{Solver: native, cases: cases, original: source, graphs: make(map[*RPFP]int)}
			s := &UnwindingSearch{Unwinding: u, Native: observed, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Local: local, Auxiliary: aux, Simplifier: manager, Cases: cases, Counterexample: cex, UseUnderapprox: mode == "underapprox"}
			must(native.Push())
			caller, err := ctx.BoolConst("stratified-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			profile := NewProfiler(nil)
			ctx.activeProfiler = profile
			defer func() { ctx.activeProfiler = nil }()
			must(profile.TimerStart("SolveMain"))
			outer := profile.current
			satisfied, err := s.DoStratifiedInlining()
			must(err)
			if profile.current != outer || outer.children["StratifiedInlining"] != nil || outer.children["Extend"] == nil {
				t.Fatal("active stratified branch timer scopes", outer.children)
			}
			must(profile.TimerStop("SolveMain"))
			wantSatisfied := mode == "forward" || mode == "cycle"
			if satisfied != wantSatisfied || !u.stratifiedDone {
				t.Fatal("stratified result/attempt flag", satisfied, mode)
			}
			if len(graph.nodes) != 2 || len(graph.edges) != 2 {
				t.Fatal("one-layer graph size", len(graph.nodes), len(graph.edges))
			}
			var instance *Node
			for _, node := range graph.nodes {
				if node.Outgoing.Map == rootEdge {
					instance = node
				}
			}
			if instance == nil {
				t.Fatal("missing root instance/source edge map")
			}
			active := false
			for _, node := range covering.instances[root] {
				if node == instance {
					active = true
				}
			}
			if !active {
				t.Fatal("layer instance not marked expanded, including counterexample exit")
			}
			if mode == "cycle" {
				children := instance.Outgoing.Children
				if children[0] != children[1] || children[0].Map != root || children[0].Outgoing.Map != nil || u.backEdges[instance][root] != 1 {
					t.Fatal("shared level-zero back-edge leaf/count")
				}
				empty, err := children[0].Annotation.IsEmpty()
				must(err)
				ruleEmpty, err := children[0].Outgoing.F.IsEmpty()
				must(err)
				if !empty || !ruleEmpty || len(u.PendingNodes()) != 1 {
					t.Fatal("level-zero leaf source state")
				}
			} else if instance.Outgoing.Children[0].Map != leafSource || len(u.PendingNodes()) != 0 {
				t.Fatal("forward dependency did not reuse completed level-one instance")
			}
			expectedQueries := 1
			if mode == "forward" {
				expectedQueries = 0
			}
			if mode == "underapprox" {
				expectedQueries = 2
			}
			if observed.bad || len(observed.graphs) != expectedQueries || cases.Graph != source || cases.badCount != 6 || cases.badLimit != 16 {
				t.Fatal("fresh query case bindings/failure schedule", observed.bad, len(observed.graphs), expectedQueries)
			}
			if !wantSatisfied && (cex.Tree() == nil || cex.Root() == nil || cex.Tree().closed) {
				t.Fatal("stratified counterexample ownership lost")
			}
			beforeNodes, beforeEdges := len(graph.nodes), len(graph.edges)
			repeated, err := s.DoStratifiedInlining()
			must(err)
			if !repeated || len(graph.nodes) != beforeNodes || len(graph.edges) != beforeEdges {
				t.Fatal("source one-shot attempt semantics changed")
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("stratified inlining disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityStratifiedInliningOpenSourceRejectsWithoutRepeating(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	source, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	term, err := ctx.BoolConst("stratified-open-source")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, term)
	_, err = source.CreateNode(term)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	if err != nil {
		t.Fatal(err)
	}
	u, err := NewUnwinding(source, graph, covering)
	if err != nil {
		t.Fatal(err)
	}
	s := &UnwindingSearch{Unwinding: u}
	if _, err = s.DoStratifiedInlining(); !errors.Is(err, ErrLevelizedUnwinding) {
		t.Fatal("missing levelized source diagnostic", err)
	}
	done, err := s.DoStratifiedInlining()
	if err != nil || !done || len(graph.nodes) != 0 {
		t.Fatal("source attempt flag not preserved on error", done, err)
	}
}
