package duality

// Source: src/muz/duality/duality_dl_interface.cpp, query clause and bounds preparation.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
)

// PreparedQuery owns the clauses passed to Duality's RPFP construction.
type PreparedQuery struct {
	self       *PreparedQuery
	manager    *ast.ASTManager
	background []ast.ASTRef
	// run and theory: Backend.PrepareQuery's started query, whose
	// background axioms are asserted; Query takes them over.
	owner   *QueryEngine
	run     *queryRun
	theory  []*graph.Expr
	clauses []ast.ASTRef
	bounds  []uint32
	closing bool
}

// PrepareQuery adds the closed query and missing predicate heads to rule
// formulas, then replaces unbounded sentinels with the configured bound.
func PrepareQuery(m *ast.ASTManager, rules []ast.ASTHandle, bounds []uint32, query ast.ASTHandle, predicates []ast.ASTHandle, recursionBound uint32) (result *PreparedQuery, err error) {
	return prepareQuery(m, nil, rules, bounds, query, nil, predicates, recursionBound)
}

// prepareQuery follows dl_interface::query: with an engine, the query
// starts (a new duality_data, under scoped_proof); then the rule clauses,
// the query clause, the background axioms (RPFP::AssertAxiom: the
// assert_axiom conversion, with its fresh @distinct declaration, and the
// assertion in the interpolating solver), and the head-completion clauses
// with their fresh X constants. FromClauses (and its @Fail) comes later,
// in BuildGraph.
func prepareQuery(m *ast.ASTManager, engine *QueryEngine, rules []ast.ASTHandle, bounds []uint32, query ast.ASTHandle, background []ast.ASTHandle, predicates []ast.ASTHandle, recursionBound uint32) (result *PreparedQuery, err error) {
	if m == nil || len(rules) != len(bounds) || (engine == nil && len(background) > 0) {
		return nil, ast.ErrASTDomain
	}
	result = &PreparedQuery{manager: m}
	result.self = result
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	if engine != nil {
		if err := engine.startQuery(); err != nil {
			return result, err
		}
		proofScope, err := ast.NewScopedProof(m)
		if err != nil {
			return result, err
		}
		defer func() { err = errors.Join(err, proofScope.Close()) }()
		// The run is the engine's from here on: a query that fails before
		// Query leaves it to the next query's reap, or to Close.
		result.owner = engine
		result.run = &queryRun{}
		engine.runs = append(engine.runs, result.run)
	}
	for i, rule := range rules {
		ref, e := m.NewRef(rule)
		if e != nil {
			return result, e
		}
		result.clauses = append(result.clauses, ref)
		result.bounds = append(result.bounds, bounds[i])
	}
	queryRef, err := QueryClause(m, query)
	if err != nil {
		return result, err
	}
	result.clauses = append(result.clauses, queryRef)
	result.bounds = append(result.bounds, math.MaxUint32)
	if engine != nil {
		for _, axiom := range engine.config.Theory {
			if err := engine.assertAxiom(result.run, &result.theory, axiom); err != nil {
				return result, err
			}
		}
		for _, formula := range background {
			if err := result.keepBackground(formula); err != nil {
				return result, err
			}
			axiom, err := graph.NewExpr(engine.context, formula)
			if err != nil {
				return result, err
			}
			result.run.background = append(result.run.background, axiom)
			if err := engine.assertAxiom(result.run, &result.theory, axiom); err != nil {
				return result, err
			}
		}
	}
	clauses, err := result.Clauses()
	if err != nil {
		return result, err
	}
	missing, err := MissingHeadClauses(m, clauses, predicates)
	if err != nil {
		return result, err
	}
	result.clauses = append(result.clauses, missing...)
	for range missing {
		result.bounds = append(result.bounds, math.MaxUint32)
	}
	for i := range result.bounds {
		if result.bounds[i] == math.MaxUint32 {
			result.bounds[i] = recursionBound
		}
	}
	return result, nil
}

// Clauses returns borrowed handles, valid until Close.
func (q *PreparedQuery) Clauses() ([]ast.ASTHandle, error) {
	if q == nil || q.manager == nil {
		return nil, ast.ErrASTHandle
	}
	if q.self != q || q.closing {
		return nil, graph.ErrWrapperOwner
	}
	values := make([]ast.ASTHandle, len(q.clauses))
	for i, ref := range q.clauses {
		h, err := q.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
		values[i] = h
	}
	return values, nil
}
func (q *PreparedQuery) Bounds() ([]uint32, error) {
	if q == nil || q.manager == nil {
		return nil, ast.ErrASTHandle
	}
	if q.self != q || q.closing {
		return nil, graph.ErrWrapperOwner
	}
	return append([]uint32(nil), q.bounds...), nil
}
func (q *PreparedQuery) Close() error {
	if q == nil || q.manager == nil {
		return nil
	}
	if q.self != q {
		return graph.ErrWrapperOwner
	}
	q.closing = true
	q.run, q.theory = nil, nil
	for len(q.background) > 0 {
		i := len(q.background) - 1
		err := q.manager.DelRef(&q.background[i])
		if q.background[i] == (ast.ASTRef{}) {
			q.background = q.background[:i]
		}
		if err != nil {
			return err
		}
	}
	for len(q.clauses) > 0 {
		i := len(q.clauses) - 1
		err := q.manager.DelRef(&q.clauses[i])
		if q.clauses[i] == (ast.ASTRef{}) {
			q.clauses = q.clauses[:i]
		}
		if err != nil {
			return err
		}
	}
	q.bounds = nil
	q.manager = nil
	return nil
}
