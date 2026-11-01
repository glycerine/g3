package tab

// Source: src/muz/tab/tab_context.cpp, tab::query and imp::get_answer.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Statistics contains the tabulation engine's cumulative source counters.
type Statistics struct {
	Unfold, UnfoldFail, Subsumed uint32
}

// QueryPredicate runs tabulation for a predicate application over rules.
// It returns an owning proof for a reachable query and true for an unreachable
// query. The caller owns a nonzero answer reference and all supplied services.
func QueryPredicate(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return QueryPredicateWithStatistics(rules, query, rw, lite, querySolver, subsumptionSolver, strategy, nil, evaluator...)
}

// QueryPredicateWithStatistics adds this query's search counters to stats.
func QueryPredicateWithStatistics(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	if rules == nil || rw == nil || rw.Manager() == nil || lite == nil || querySolver == nil || subsumptionSolver == nil {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	m := rw.Manager()
	defer func() {
		if err != nil && answer != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&answer))
			answer = ast.ASTRef{}
		}
	}()
	ruleManager, err := rules.RuleManager()
	if err != nil {
		return lbool.Undef, answer, err
	}
	properties, err := base.NewRuleProperties(ruleManager)
	if err != nil {
		return lbool.Undef, answer, err
	}
	defer func() { err = errors.Join(err, properties.Close()) }()
	if err = properties.Collect(rules); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckExistentialTail(); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckNegativePredicates(); err != nil {
		return lbool.Undef, answer, err
	}
	index, err := NewRuleIndex(rules, rw)
	if err != nil {
		return lbool.Undef, answer, err
	}
	defer func() { err = errors.Join(err, index.Close()) }()
	falsehood, err := m.MkFalse()
	if err != nil {
		return lbool.Undef, answer, err
	}
	truth, err := m.MkTrue()
	if err != nil {
		return lbool.Undef, answer, err
	}
	goal, err := NewClause(m, rw, falsehood, []ast.ASTHandle{query}, truth)
	if err != nil {
		return lbool.Undef, answer, err
	}
	var modelConfig *model.ModelEvaluatorConfig
	if len(evaluator) > 1 {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	if len(evaluator) == 1 {
		modelConfig = &evaluator[0]
	}
	search, err := newTabSearch(goal, index, rw, lite, querySolver, subsumptionSolver, strategy, modelConfig)
	if err != nil {
		return lbool.Undef, answer, errors.Join(err, goal.Close())
	}
	defer func() { err = errors.Join(err, search.Close()) }()
	defer func() {
		if stats != nil {
			stats.Unfold += search.unfold
			stats.UnfoldFail += search.unfoldFail
			stats.Subsumed += search.subsumed
		}
	}()
	status, err = search.Run()
	if err != nil {
		return status, answer, err
	}
	switch status {
	case lbool.True:
		answer, err = search.Proof()
	case lbool.False:
		answer, err = m.NewRef(truth)
	}
	return status, answer, err
}
