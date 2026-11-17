package fp

// Source: rel_context.cpp scoped_query (58-91) around
// rel_context::query(expr*) (331-380).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// datalogScopedQuery is rel_context::scoped_query of an expression query
// on the context: m_rules (a copy of the context's rule set), m_preds (the
// context's predicates) and m_was_closed.
type datalogScopedQuery struct {
	rules     *base.RuleSet
	preds     []ast.ASTHandle
	wasClosed bool
}

// beginDatalogScopedQuery is scoped_query(ctx): m_rules(ctx.get_rules()),
// m_preds(ctx.get_predicates()), m_was_closed(ctx.is_closed()). The
// relation manager's predicates before mk_query (datalogQueryPreds) are
// the ones context::replace_rules' restrict_predicates keeps.
func (e *PropositionalEngine) beginDatalogScopedQuery() (_ *datalogScopedQuery, resultErr error) {
	rules, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, err
	}
	scoped := &datalogScopedQuery{rules: rules}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rules.Close())
		}
	}()
	if err = rules.AddRules(e.set); err != nil {
		return nil, err
	}
	if scoped.preds, err = e.rules.ContextPredicatesCopy(); err != nil {
		return nil, err
	}
	if scoped.wasClosed, err = e.set.IsFinalized(); err != nil {
		return nil, err
	}
	e.datalogQueryPreds = make(map[ast.ASTHandle]bool, len(e.predicates))
	for d := range e.predicates {
		e.datalogQueryPreds[d] = true
	}
	return scoped, nil
}

// endDatalogScopedQuery is ~scoped_query, run when the query returns or
// throws: reopen, restrict_predicates(m_preds), replace_rules(m_rules)
// (context::replace_rules restricts the relations to the predicates as
// well: the query's auxiliary relations go), close when the context was
// closed; then m_rules dies.
func (e *PropositionalEngine) endDatalogScopedQuery(scoped *datalogScopedQuery) error {
	err := e.set.Reopen()
	err = errors.Join(err, e.rules.RestrictContextPredicates(scoped.preds))
	if err == nil {
		err = e.set.ReplaceRules(scoped.rules)
	}
	if err == nil {
		err = e.restrictDatalogQueryPredicates()
	}
	if err == nil && scoped.wasClosed {
		_, err = e.set.Finalize()
	}
	return errors.Join(err, scoped.rules.Close())
}
