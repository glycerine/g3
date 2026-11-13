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
	return queryPredicate(rules, query, nil, rw, lite, querySolver, subsumptionSolver, strategy, nil, stats, evaluator...)
}

// QueryPredicateWithSession runs a query using source state that persists
// across calls, including selection decay/cache and clause sequence IDs.
func QueryPredicateWithSession(rules *base.RuleSet, query ast.ASTHandle, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return queryPredicate(rules, query, nil, rw, lite, querySolver, subsumptionSolver, "", session, stats, evaluator...)
}

func queryPredicate(rules *base.RuleSet, query ast.ASTHandle, queryRule *base.Rule, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, session *Session, stats *Statistics, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	return queryPredicatePending(rules, query, queryRule, nil, rw, lite, querySolver, subsumptionSolver, strategy, session, stats, nil, evaluator...)
}

// queryPredicatePending takes ownership of prebuilt, the rule index that
// imp::query initialized (m_rules.init) before mk_query.
func queryPredicatePending(rules *base.RuleSet, query ast.ASTHandle, queryRule *base.Rule, prebuilt *RuleIndex, rw *rewriter.TheoryRewriter, lite *qe.Lite, querySolver, subsumptionSolver solver.Solver, strategy string, session *Session, stats *Statistics, pending **PendingAnswer, evaluator ...model.ModelEvaluatorConfig) (status lbool.LBool, answer ast.ASTRef, err error) {
	if rw == nil || rw.Manager() == nil {
		return lbool.Undef, answer, errors.Join(ast.ErrASTHandle, prebuilt.Close())
	}
	m := rw.Manager()
	attempt := &tabQueryAttempt{manager: m, index: prebuilt}
	if rules == nil || lite == nil || querySolver == nil || subsumptionSolver == nil {
		return lbool.Undef, answer, errors.Join(ast.ErrASTHandle, attempt.closeResources())
	}
	defer func() { answer, err = attempt.finish(answer, err) }()
	defer func() {
		// The engine keeps this query's rules and clauses until the next
		// query resets them.
		if err == nil && session != nil && attempt.search != nil {
			session.retain(attempt.index, attempt.search)
			attempt.index, attempt.search = nil, nil
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
	attempt.properties = properties
	if err = properties.Collect(rules); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckExistentialTail(); err != nil {
		return lbool.Undef, answer, err
	}
	if err = properties.CheckNegativePredicates(); err != nil {
		return lbool.Undef, answer, err
	}
	index := prebuilt
	if index == nil {
		if err = session.releaseRules(); err != nil {
			return lbool.Undef, answer, err
		}
		index, err = NewRuleIndex(rules, rw)
		if err != nil {
			return lbool.Undef, answer, err
		}
		attempt.index = index
	}
	falsehood, err := m.MkFalse()
	if err != nil {
		return lbool.Undef, answer, err
	}
	truth, err := m.MkTrue()
	if err != nil {
		return lbool.Undef, answer, err
	}
	var goal *Clause
	if queryRule == nil {
		goal, err = NewClause(m, rw, falsehood, []ast.ASTHandle{query}, truth)
	} else {
		// Source imp::query initializes the last query rule, then replaces its
		// head with false. The query rule never enters the program index.
		goal, err = NewClauseFromRule(queryRule, rw)
	}
	attempt.goal = goal
	if err != nil {
		return lbool.Undef, answer, err
	}
	if queryRule != nil {
		if err = m.SetRef(goal.head, falsehood); err != nil {
			return lbool.Undef, answer, err
		}
	}
	// init_clause(g): m_clauses.reset().
	if err = session.releaseSearch(); err != nil {
		return lbool.Undef, answer, err
	}
	var modelConfig *model.ModelEvaluatorConfig
	if len(evaluator) > 1 {
		return lbool.Undef, answer, ast.ErrASTHandle
	}
	if len(evaluator) == 1 {
		modelConfig = &evaluator[0]
	}
	search, err := newTabSearchWithSession(goal, index, rw, lite, querySolver, subsumptionSolver, strategy, session, modelConfig)
	if err != nil {
		return lbool.Undef, answer, err
	}
	attempt.goal = nil
	attempt.search = search
	defer func() {
		if stats != nil {
			stats.Unfold += search.unfold
			stats.UnfoldFail += search.unfoldFail
			stats.Subsumed += search.subsumed
		}
	}()
	if err = search.verboseInitialClause(); err != nil {
		return lbool.Undef, answer, err
	}
	if queryRule != nil && len(goal.predicates) == 0 {
		// C++ select_predicate skips the initial clause's constraint check.
		// Preserve the existing Go false/theory-only query behavior: before
		// direct initialization, the synthetic unfold ran this tautology check.
		canceled, cause := m.Canceled()
		if cause != nil {
			return lbool.Undef, answer, cause
		}
		if !canceled {
			tautology, cause := search.isTautology(goal)
			if cause != nil {
				return lbool.Undef, answer, cause
			}
			if tautology {
				answer, err = m.NewRef(truth)
				return lbool.False, answer, err
			}
		}
	}
	status, err = search.Run()
	if err != nil {
		return status, answer, err
	}
	switch status {
	case lbool.True:
		if pending != nil && session != nil {
			// imp::query returns l_true; get_answer replays the proof later
			// from the clauses the session retains.
			*pending = &PendingAnswer{session: session}
			return status, answer, nil
		}
		if pending != nil {
			*pending = &PendingAnswer{attempt: &tabQueryAttempt{manager: m, index: attempt.index, search: attempt.search}}
			attempt.index, attempt.search = nil, nil
			return status, answer, nil
		}
		answer, err = search.Proof()
	case lbool.False:
		answer, err = m.NewRef(truth)
	}
	return status, answer, err
}
