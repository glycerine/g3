package tab

// Source: src/muz/tab/tab_context.cpp imp::get_answer and imp::get_proof.
// The proof is replayed from the engine's clause store only when the answer
// is requested; the clauses stay alive until then.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// PendingAnswer owns the successful search state of one query.
type PendingAnswer struct {
	attempt *tabQueryAttempt
	// session holds the search state instead when the engine keeps one.
	session *Session
}

// Proof is imp::get_proof; the caller owns the returned reference.
func (p *PendingAnswer) Proof() (ast.ASTRef, error) {
	if p != nil && p.session != nil && p.session.search != nil {
		return p.session.search.Proof()
	}
	if p == nil || p.attempt == nil || p.attempt.search == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	return p.attempt.search.Proof()
}

func (p *PendingAnswer) Close() error {
	if p == nil || p.attempt == nil {
		return nil
	}
	if err := p.attempt.closeResources(); err != nil {
		return err
	}
	p.attempt = nil
	return nil
}

// QueryPredicateWithSessionPending is QueryPredicateWithSession with a lazy
// proof: a reachable query returns its search state in pending instead of an
// answer reference.
func QueryPredicateWithSessionPending(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, stats *Statistics, pending **PendingAnswer, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return queryPredicatePending(rules, query, nil, nil, rw, lite, querySolver, subsumptionSolver, "", session, stats, pending, evaluator...)
}

// QueryExpressionWithSessionStatisticsPending is the lazy-proof form of
// QueryExpressionWithSessionStatistics.
func QueryExpressionWithSessionStatisticsPending(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, stats *Statistics, pending **PendingAnswer, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, resultErr error) {
	return queryExpressionPending(rules, query, rw, lite, querySolver, subsumptionSolver, "", session, stats, pending, evaluator...)
}
