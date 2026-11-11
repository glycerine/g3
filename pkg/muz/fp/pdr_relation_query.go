package fp

// Source: src/muz/base/dl_engine_base.h engine_base::query(num_rels, rels),
// src/muz/pdr/pdr_dl_interface.cpp dl_interface::query and
// src/muz/base/dl_rule.cpp rule_manager::mk_query.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// queryPDRRelation is (query R) under PDR: engine_base wraps R as the
// expression R(x0, ..., xn), and dl_interface::query runs mk_query, which
// adds a fresh query!N predicate and the rule R(...) => query!N to the
// context's own rules, then solves query!N. dl_interface::query restores the
// rules it copied before mk_query (replace_rules(old_rules)), so the query
// rule leaves the persistent program again; query!N stays registered.
func (e *PropositionalEngine) queryPDRRelation(declaration ast.ASTHandle) (status lbool.LBool, resultErr error) {
	// rel_query does not install a model converter (see contextModelConverter).
	e.pdrNullModelConverter = !e.contextModelConverter
	return e.queryRelationMkQuery(declaration, func(head ast.ASTHandle) (lbool.LBool, error) {
		e.preprocessNullModelConverter = e.pdrNullModelConverter
		defer func() { e.preprocessNullModelConverter = false }()
		return e.queryPDR(head)
	})
}

// querySpacerRelation is (query R) under Spacer: spacer's dl_interface::query
// runs the same mk_query and restores the old rules.
func (e *PropositionalEngine) querySpacerRelation(declaration ast.ASTHandle) (lbool.LBool, error) {
	// rel_query leaves m_mc null: the transformations record no converters.
	e.spacerNullModelConverter = !e.contextModelConverter
	return e.queryRelationMkQuery(declaration, func(head ast.ASTHandle) (lbool.LBool, error) {
		e.preprocessNullModelConverter = !e.contextModelConverter
		defer func() { e.preprocessNullModelConverter = false }()
		return e.querySpacer(head)
	})
}

// queryRelationMkQuery is engine_base::query(num_rels, rels) followed by the
// interface's mk_query, solve, and replace_rules(old_rules).
func (e *PropositionalEngine) queryRelationMkQuery(declaration ast.ASTHandle, solve func(ast.ASTHandle) (lbool.LBool, error)) (status lbool.LBool, resultErr error) {
	ref, err := e.makeRelationQueryExpression(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	// engine_base's q (and the args vector, here pinned by q) lives for the
	// whole query.
	queryRoots := []ast.ASTRef{ref}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, queryRoots)) }()
	query, err := e.manager.RefGet(ref)
	if err != nil {
		return lbool.Undef, err
	}
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	if arity != 0 {
		// q = mk_exists(sorts, names, R(#0..)) with sorts and the numeric
		// names symbol(i) both reversed.
		sorts := make([]ast.ASTHandle, arity)
		names := make([]symbol.Symbol, arity)
		for i := uint32(0); i < arity; i++ {
			sorts[arity-1-i], err = e.manager.GetDomain(declaration, i)
			if err != nil {
				return lbool.Undef, err
			}
			names[arity-1-i] = symbol.Numerical(i)
		}
		exists, err := e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: query})
		if err != nil {
			return lbool.Undef, err
		}
		existsRef, err := e.manager.NewRef(exists)
		if err != nil {
			return lbool.Undef, err
		}
		queryRoots = append(queryRoots, existsRef)
		query = exists
	}
	// dl_interface::query traces its query parameter (mk_pp(query)), not
	// the query!N head mk_query adds.
	e.pdrTraceQuery = query
	defer func() { e.pdrTraceQuery = ast.ASTHandle{} }()
	// dl_interface::query reads m_ctx.get_rules(), which flushes the queued
	// rules after engine_base built q.
	if err = e.flushPendingRules(); err != nil {
		return lbool.Undef, err
	}
	// mk_query: pull the existential prefix, then compact the free variables.
	roots := make([]ast.ASTRef, 0, 4)
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, roots)) }()
	hoisted, names, err := e.hoistQueryExistentialsNamed(query)
	if err != nil {
		return lbool.Undef, err
	}
	roots = append(roots, hoisted)
	if query, err = e.manager.RefGet(hoisted); err != nil {
		return lbool.Undef, err
	}
	compacted, err := e.compactQueryVariables(query)
	if err != nil {
		return lbool.Undef, err
	}
	roots = append(roots, compacted)
	if query, err = e.manager.RefGet(compacted); err != nil {
		return lbool.Undef, err
	}
	head, headRef, err := e.makeQueryHead(query)
	if err != nil {
		return lbool.Undef, err
	}
	roots = append(roots, headRef)
	headApp, err := e.manager.RefGet(headRef)
	if err != nil {
		return lbool.Undef, err
	}
	// mk_query registers query!N, but the synthetic predicate and its rule
	// are not persistent program changes: check_reset decides the reset from
	// the rules themselves, so keep the revision PDR's reset test compares.
	revision := e.revision
	if err = e.RegisterPredicate(head); err != nil {
		return lbool.Undef, err
	}
	e.revision = revision
	e.auxiliary[head] = true
	if e.pdrQueryAlias == nil {
		e.pdrQueryAlias = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	e.pdrQueryAlias[declaration] = head
	rule, err := e.manager.MkImplies(query, headApp)
	if err != nil {
		return lbool.Undef, err
	}
	ruleRef, err := e.manager.NewRef(rule)
	if err != nil {
		return lbool.Undef, err
	}
	roots = append(roots, ruleRef)
	// rule_expr = forall(vars, names, impl) over the query's free variables:
	// pad names with symbol(k), then reverse both so the outermost declared
	// variable comes first.
	used, err := ast.NewASTUsedVars(e.manager)
	if err != nil {
		return lbool.Undef, err
	}
	if err = used.Process(query); err != nil {
		return lbool.Undef, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if count != 0 {
		sorts := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			if sorts[count-1-i], err = used.Get(i); err != nil {
				return lbool.Undef, err
			}
		}
		for uint32(len(names)) < count {
			names = append(names, symbol.Numerical(uint32(len(names))))
		}
		reversed := make([]symbol.Symbol, len(names))
		for i, name := range names {
			reversed[len(names)-1-i] = name
		}
		rule, err = e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: reversed, Body: rule})
		if err != nil {
			return lbool.Undef, err
		}
		forallRef, err := e.manager.NewRef(rule)
		if err != nil {
			return lbool.Undef, err
		}
		roots = append(roots, forallRef)
	}
	// rules.add_rule(mk_rule(...)): the rule joins the rule set directly,
	// without the raw add_rule queue (m_rule_fmls) that keeps formulas.
	if err = e.addQueryRuleNow(rule); err != nil {
		return lbool.Undef, err
	}
	queryRule, err := e.lastRuleWithHead(head)
	if err != nil {
		return lbool.Undef, err
	}
	// queryRule is a retained owner of the set's rule: release it once the
	// rule leaves the set.
	defer func() { resultErr = errors.Join(resultErr, e.removeQueryRule(queryRule), queryRule.Close()) }()
	// mk_query's locals die when it returns; engine_base's q stays alive
	// for the whole query.
	err = closeASTRefs(e.manager, roots)
	roots = nil
	if err != nil {
		return lbool.Undef, err
	}
	return solve(head)
}

// addQueryRuleNow binds formula and adds it to the rule set.
func (e *PropositionalEngine) addQueryRuleNow(formula ast.ASTHandle) (resultErr error) {
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	bound, err := e.binder.Bind(formula, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&bound)) }()
	boundFormula, err := e.manager.RefGet(bound)
	if err != nil {
		return err
	}
	// mk_query: under scoped_proof_mode, pr = m.mk_asserted(rule_expr) when
	// proofs are traced, then mk_rule(rule_expr, pr, rules).
	generateProof, err := e.sourceRuleProofTrace()
	if err != nil {
		return err
	}
	var proof ast.ASTHandle
	if generateProof {
		scope, err := ast.NewScopedProof(e.manager)
		if err != nil {
			return err
		}
		proof, err = e.manager.MkProofAsserted(boundFormula)
		if err != nil {
			return errors.Join(err, scope.Close())
		}
		proofRef, err := e.manager.NewRef(proof)
		if err = errors.Join(err, scope.Close()); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&proofRef)) }()
	}
	// The rule is removed again after the query (replace_rules(old_rules)),
	// so the persistent program revision is unchanged.
	return e.addFormulaWithProof(e.set, boundFormula, proof, symbol.Symbol{})
}

// lastRuleWithHead returns the set's final rule, which must have the given
// head declaration.
func (e *PropositionalEngine) lastRuleWithHead(head ast.ASTHandle) (*base.Rule, error) {
	count, err := e.set.Len()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ast.ErrASTDomain
	}
	rule, err := e.set.Rule(count - 1)
	if err != nil {
		return nil, err
	}
	declaration, err := rule.HeadDeclaration()
	if err != nil {
		return nil, errors.Join(err, rule.Close())
	}
	if declaration != head {
		return nil, errors.Join(ast.ErrASTDomain, rule.Close())
	}
	return rule, nil
}

// removeQueryRule is replace_rules(old_rules) for the mk_query rule. The rule
// is the set's last, so removal keeps the remaining rule order.
func (e *PropositionalEngine) removeQueryRule(rule *base.Rule) error {
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = e.set.Reopen(); err != nil {
			return err
		}
	}
	if err = e.set.Remove(rule); err != nil {
		return err
	}
	if finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return err
		}
		if !valid {
			return ErrUnstratifiedNegation
		}
	}
	return nil
}

// pdrModelConversions is the model converter chain the PDR context applies:
// none when the relation query ran with a null m_mc. With a converter
// installed by an earlier context::query the source concatenates that older
// chain too; g3 applies only the current query's conversions.
func (e *PropositionalEngine) pdrModelConversions() []hornModelConversion {
	if e.pdrNullModelConverter {
		return nil
	}
	return e.pdrConversions
}

// pdrQueryTarget maps a relation queried through queryPDRRelation to the
// query!N predicate that PDR actually solved.
func (e *PropositionalEngine) pdrQueryTarget(declaration ast.ASTHandle) ast.ASTHandle {
	if e.backend != "pdr" && e.backend != "qpdr" && e.backend != "spacer" {
		return declaration
	}
	if target, ok := e.pdrQueryAlias[declaration]; ok {
		return target
	}
	return declaration
}
