package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

var errCaseGraphPopRefused = errors.New("case interpolation graph pop refused")

type caseGraphPopRefusalSolver struct {
	solver.Solver
	failure error
	pops    int
}

func (s *caseGraphPopRefusalSolver) Pop(count uint32) error {
	s.pops++
	if s.failure != nil {
		err := s.failure
		s.failure = nil
		return err
	}
	return s.Solver.Pop(count)
}

func TestDualityCaseInterpolationGraphScopePopRetriesOwnership(t *testing.T) {
	ctx, manager, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	wrapped := &caseGraphPopRefusalSolver{Solver: native}
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	must(graph.BindSolver(wrapped))
	term, err := ctx.BoolConst("case-scope-retry-node")
	must(err)
	closeExpr(t, term)
	node, err := graph.CreateNode(term)
	must(err)
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	rule, err := NewTransformer(ctx, nil, nil, truth)
	must(err)
	defer func() { must(rule.Close()) }()
	edge, err := graph.CreateEdge(node, rule, nil)
	must(err)
	must(graph.Push())
	constraint, err := ctx.BoolConst("case-scope-retry-constraint")
	must(err)
	closeExpr(t, constraint)
	must(graph.ConstrainEdgeLocalized(edge, constraint))
	active := true

	wrapped.failure = errCaseGraphPopRefused
	if err = popCaseGraphScope(graph, &active); !errors.Is(err, errCaseGraphPopRefused) {
		t.Fatal("graph scope pop did not preserve refusal", err)
	}
	depth, err := native.ScopeLevel()
	must(err)
	assertions, err := native.NumAssertions()
	must(err)
	if !active || depth != 1 || len(graph.scopes) != 2 || len(edge.constraints) != 1 || assertions != 1 || wrapped.pops != 1 {
		t.Fatal("refused graph pop surrendered scope ownership", active, depth, len(graph.scopes), len(edge.constraints), assertions, wrapped.pops)
	}

	must(popCaseGraphScope(graph, &active))
	depth, err = native.ScopeLevel()
	must(err)
	assertions, err = native.NumAssertions()
	must(err)
	if active || depth != 0 || len(graph.scopes) != 1 || len(edge.constraints) != 0 || assertions != 0 || wrapped.pops != 2 {
		t.Fatal("retried graph pop did not release exactly one scope", active, depth, len(graph.scopes), len(edge.constraints), assertions, wrapped.pops)
	}
	must(popCaseGraphScope(graph, &active))
	if wrapped.pops != 2 {
		t.Fatal("released graph scope popped again", wrapped.pops)
	}

	must(graph.Push())
	cleanupConstraint, err := ctx.BoolConst("case-scope-retry-cleanup")
	must(err)
	must(graph.ConstrainEdgeLocalized(edge, cleanupConstraint))
	cleanupHandle, err := cleanupConstraint.Raw()
	must(err)
	recorded := &graph.scopes[1].constraints[0].ref
	stale := *recorded
	must(manager.DelRef(recorded))
	*recorded = stale
	active = true
	cleanupFailure := popCaseGraphScope(graph, &active)
	if !errors.Is(cleanupFailure, ast.ErrASTHandle) {
		t.Fatal("AST cleanup refusal was not preserved", cleanupFailure)
	}
	depth, err = native.ScopeLevel()
	must(err)
	if !active || depth != 0 || len(graph.scopes) != 2 || !graph.scopes[1].solverPopped || wrapped.pops != 3 {
		t.Fatal("successful native pop lost pending graph cleanup receipt", active, depth, len(graph.scopes), graph.scopes[1].solverPopped, wrapped.pops, cleanupFailure)
	}
	*recorded, err = manager.NewRef(cleanupHandle)
	must(err)
	must(cleanupConstraint.Close())
	must(popCaseGraphScope(graph, &active))
	if active || len(graph.scopes) != 1 || len(edge.constraints) != 0 || wrapped.pops != 3 {
		t.Fatal("graph cleanup retry popped native scope twice", active, len(graph.scopes), len(edge.constraints), wrapped.pops)
	}

	must(graph.Push())
	lowerConstraint, err := ctx.BoolConst("case-scope-retry-lower")
	must(err)
	must(graph.ConstrainEdgeLocalized(edge, lowerConstraint))
	lowerHandle, err := lowerConstraint.Raw()
	must(err)
	lowerRecorded := &graph.scopes[1].constraints[0].ref
	lowerStale := *lowerRecorded
	must(manager.DelRef(lowerRecorded))
	*lowerRecorded = lowerStale
	must(graph.Push())
	upperConstraint, err := ctx.BoolConst("case-scope-retry-upper")
	must(err)
	must(graph.ConstrainEdgeLocalized(edge, upperConstraint))
	must(upperConstraint.Close())
	if err = graph.Pop(2); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("multi-scope graph cleanup refusal was not preserved", err)
	}
	depth, err = native.ScopeLevel()
	must(err)
	if depth != 0 || len(graph.scopes) != 3 || !graph.scopes[1].solverPopped || !graph.scopes[2].solverPopped || wrapped.pops != 4 {
		t.Fatal("partial multi-scope cleanup lost its original frontier", depth, len(graph.scopes), wrapped.pops)
	}
	*lowerRecorded, err = manager.NewRef(lowerHandle)
	must(err)
	must(lowerConstraint.Close())
	must(graph.Pop(2))
	if len(graph.scopes) != 1 || len(edge.constraints) != 0 || wrapped.pops != 4 {
		t.Fatal("multi-scope retry popped native scopes twice", len(graph.scopes), len(edge.constraints), wrapped.pops)
	}
}
