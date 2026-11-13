package fp

// Source: src/muz/clp/clp_context.cpp and src/muz/fp/dl_register_engine.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) queryCLPPredicate(declaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, ErrUnstratifiedNegation
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	if e.predicates[declaration] == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	// CLP uses the shared default transformation pipeline, without Spacer's
	// additional slice, coalesce, and unfold stages.
	preprocessed, err := e.preprocessHornRulesWithQueryRule(declaration, nil, true, true, false, false)
	if err != nil {
		return lbool.Undef, err
	}
	attempt := &clpBackendAttempt{manager: e.manager, preprocessed: preprocessed}
	defer func() {
		resultErr = attempt.finish(resultErr)
		if resultErr != nil {
			result = lbool.Undef
		}
	}()
	// rules.get_output_predicates().empty() answers l_false; otherwise
	// get_output_predicate() is the first entry of the obj_hashtable.
	outputs, err := preprocessed.rules.OutputPredicates()
	if err != nil {
		return lbool.Undef, err
	}
	if len(outputs) == 0 {
		return lbool.False, nil
	}
	table := ast.NewObjMap(e.manager)
	for _, output := range outputs {
		if err = table.Insert(output, [2]ast.ASTHandle{}); err != nil {
			return lbool.Undef, err
		}
	}
	queryDeclaration := table.FirstKeys(1)[0]
	rules, err := preprocessed.rules.PredicateRules(queryDeclaration)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		for i := len(rules) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, rules[i].Close())
		}
	}()
	if len(rules) == 0 {
		return lbool.False, nil
	}
	query, err := rules[0].Head()
	if err != nil {
		return lbool.Undef, err
	}
	// clp::query's m_solver.reset() already ran; use the engine kernel.
	backend, err := e.bmcQuerySolver()
	if err != nil {
		return lbool.Undef, err
	}
	var engine *clp.Engine
	if e.bmcSlot != nil && e.bmcSlot.clp != nil {
		engine, err = clp.NewEngineWithState(e.manager, preprocessed.rules, backend, 20, e.bmcSlot.clp)
	} else {
		engine, err = clp.NewEngine(e.manager, preprocessed.rules, backend, 20)
	}
	if err != nil {
		return lbool.Undef, err
	}
	attempt.engine = engine
	return engine.Query(query)
}

// replaceCLPContextRules is the end of apply_default_transformation(m_ctx)
// under clp::imp::query: the rule transformer's
// m_rule_set.replace_rules(*new_rules) makes the transformed program (query
// rule included) the context's own rules and releases the originals, so a
// later query starts from these rules.
func (e *PropositionalEngine) replaceCLPContextRules(transformed *base.RuleSet) error {
	context := e.clpContext
	if context == nil || context.set == nil {
		return ErrPropositionalEngine
	}
	// The query clone shares the context's rule manager, where the
	// transforms registered their fresh predicates (context::
	// mk_fresh_head_predicate); the context's predicate states follow.
	register := func(declaration ast.ASTHandle) error {
		if context.predicates[declaration] != nil {
			return nil
		}
		if err := context.RegisterPredicate(declaration); err != nil {
			return err
		}
		if e.auxiliary[declaration] {
			context.auxiliary[declaration] = true
		}
		return nil
	}
	count, err := transformed.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, err := transformed.Rule(i)
		if err != nil {
			return err
		}
		head, err := rule.HeadDeclaration()
		if err == nil {
			err = register(head)
		}
		uninterpreted, uErr := rule.UninterpretedTailSize()
		err = errors.Join(err, uErr)
		for j := 0; err == nil && j < uninterpreted; j++ {
			var tail ast.ASTHandle
			if tail, err = rule.TailDeclaration(j); err == nil {
				err = register(tail)
			}
		}
		if err = errors.Join(err, rule.Close()); err != nil {
			return err
		}
	}
	outputs, err := transformed.OutputPredicates()
	if err != nil {
		return err
	}
	for _, declaration := range outputs {
		if err = register(declaration); err != nil {
			return err
		}
	}
	finalized, err := context.set.IsFinalized()
	if err == nil && finalized {
		err = context.set.Reopen()
	}
	if err == nil {
		err = context.set.ReplaceRules(transformed)
	}
	if err != nil {
		return err
	}
	context.advanceRuleRevision()
	// The query clone's copy of the original program goes with it.
	return e.set.ReplaceRules(transformed)
}
