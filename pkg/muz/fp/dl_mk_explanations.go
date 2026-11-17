package fp

// Source: src/muz/rel/dl_mk_explanations.cpp mk_explanations (lines
// 589-880): the rule transformer that rel_context::query(expr*) applies
// with datalog.generate_explanations. Every predicate p gets an explained
// predicate p_expl with an extra rule-sort column; each rule becomes a rule
// over the explained predicates whose head explanation is the rule's term
// applied to the explanations of its positive tails; output predicates get
// a rule copying the explained relation back. The relations of the
// explained predicates are the explanation plugin's (through
// finite_product_relation for fact-level explanations, a product of sieves
// for relation-level ones); the relation side is in muz/rel
// (dl_mk_explanations_relations.go).

import (
	"bytes"
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// datalogExplanationsEnabled is context::generate_explanations() as read by
// rel_context::query(expr*). The relation query rel_context::query(num_rels,
// rels), which serves (query r), never applies mk_explanations, so only the
// expression-query clone sees the option.
func (e *PropositionalEngine) datalogExplanationsEnabled() (bool, error) {
	if e.backend != "datalog" || e.params == nil || e.globals == nil || !e.expressionClone {
		return false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, err
	}
	enabled, err := view.Bool("datalog.generate_explanations", false)
	return enabled, errors.Join(err, view.Close())
}

// mkExplanations is mk_explanations.
type mkExplanations struct {
	engine        *PropositionalEngine
	m             *ast.ASTManager
	relationLevel bool
	// pinned is m_pinned.
	pinned []ast.ASTRef
	er     *rel.ExplanationRelationPlugin
	eSort  ast.ASTHandle
	// eFact is m_e_fact_relation.
	eFact rel.RelationBase
	// eDecls is m_e_decl_map.
	eDecls map[ast.ASTHandle]ast.ASTHandle
	closed bool
}

// newMkExplanations is the mk_explanations constructor.
func (e *PropositionalEngine) newMkExplanations() (_ *mkExplanations, resultErr error) {
	view, err := e.fixedpointParamView()
	if err != nil {
		return nil, err
	}
	relationLevel, err := view.Bool("datalog.explanations_on_relation_level", false)
	if err = errors.Join(err, view.Close()); err != nil {
		return nil, err
	}
	x := &mkExplanations{engine: e, m: e.manager, relationLevel: relationLevel, eDecls: make(map[ast.ASTHandle]ast.ASTHandle)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, x.Close())
		}
	}()
	if x.eSort, err = e.manager.MkDatalogRuleSort(); err != nil {
		return nil, err
	}
	if err = x.pin(x.eSort); err != nil {
		return nil, err
	}
	if x.er, err = e.relations.RegisterExplanationPlugins(relationLevel); err != nil {
		return nil, err
	}
	return x, nil
}

func (x *mkExplanations) pin(h ast.ASTHandle) error {
	ref, err := x.m.NewRef(h)
	if err != nil {
		return err
	}
	x.pinned = append(x.pinned, ref)
	return nil
}

func (x *mkExplanations) Priority() uint32            { return 50000 }
func (x *mkExplanations) CanDestratifyNegation() bool { return false }
func (x *mkExplanations) Cancel()                     {}

// SourceTypeName is typeid(mk_explanations).name() for the transformer's
// verbose output.
func (x *mkExplanations) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_explanations")
}

// Close is ~mk_explanations: m_e_fact_relation, then m_pinned.
func (x *mkExplanations) Close() (result error) {
	if x.closed {
		return nil
	}
	x.closed = true
	if x.eFact != nil {
		// scoped_rel<explanation_relation>: universal_delete is the
		// template's dealloc, so the relation is destroyed, not recycled.
		result = rel.DeleteExplanationRelation(x.eFact)
		x.eFact = nil
	}
	for i := len(x.pinned) - 1; i >= 0; i-- {
		result = errors.Join(result, x.m.DelRef(&x.pinned[i]))
	}
	x.pinned = nil
	return result
}

// getEDecl is get_e_decl(orig_decl).
func (x *mkExplanations) getEDecl(orig ast.ASTHandle) (ast.ASTHandle, error) {
	if d, found := x.eDecls[orig]; found {
		return d, nil
	}
	arity, err := x.m.GetArity(orig)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0, arity+1)
	for i := uint32(0); i < arity; i++ {
		s, err := x.m.GetDomain(orig, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		domain = append(domain, s)
	}
	domain = append(domain, x.eSort)
	name, err := x.m.GetASTName(orig)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	suffix, err := x.m.Symbols.Intern("expl")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	newDecl, err := x.engine.mkFreshHeadPredicate(name, suffix, domain, orig)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = x.pin(newDecl); err != nil {
		return ast.ASTHandle{}, err
	}
	x.eDecls[orig] = newDecl
	if x.relationLevel {
		if err = x.engine.relations.AssignRelLevelKind(x.er, newDecl, orig); err != nil {
			return ast.ASTHandle{}, err
		}
		if err = x.engine.replaceKindPlaceholder(newDecl); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return newDecl, nil
}

// getELit is get_e_lit(lit, e_var_idx); the caller owns the result.
func (x *mkExplanations) getELit(lit ast.ASTHandle, eVarIdx uint32) (ast.ASTRef, error) {
	decl, err := x.m.GetDecl(lit)
	if err != nil {
		return ast.ASTRef{}, err
	}
	eDecl, err := x.getEDecl(decl)
	if err != nil {
		return ast.ASTRef{}, err
	}
	n, err := x.m.GetNumArgs(lit)
	if err != nil {
		return ast.ASTRef{}, err
	}
	args := make([]ast.ASTHandle, 0, n+1)
	for i := uint32(0); i < n; i++ {
		a, err := x.m.GetArg(lit, i)
		if err != nil {
			return ast.ASTRef{}, err
		}
		args = append(args, a)
	}
	v, err := x.m.MkVar(eVarIdx, x.eSort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	vref, err := x.m.NewRef(v)
	if err != nil {
		return ast.ASTRef{}, err
	}
	args = append(args, v)
	app, err := x.m.MkApp(eDecl, args)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, x.m.DelRef(&vref))
	}
	res, err := x.m.NewRef(app)
	return res, errors.Join(err, x.m.DelRef(&vref))
}

// getRuleSymbol is get_rule_symbol(r): the rule's name, or its display
// without the trailing newlines.
func (x *mkExplanations) getRuleSymbol(r *base.Rule) (usymbol.Symbol, error) {
	name, err := r.NameSymbol()
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if !name.IsNull() {
		return name, nil
	}
	var b bytes.Buffer
	if err = x.engine.rules.WriteRuleDisplay(&b, r); err != nil {
		return usymbol.Symbol{}, err
	}
	return x.m.Symbols.Intern(strings.TrimRight(b.String(), "\n"))
}

// getERule is get_e_rule(r); the caller owns the result.
func (x *mkExplanations) getERule(r *base.Rule) (_ *base.Rule, resultErr error) {
	var held []ast.ASTRef
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, x.m.DelRef(&held[i]))
		}
	}()
	// rule_counter ctr; ctr.count_rule_vars(r); get_max_positive.
	used, err := r.UsedVariables()
	if err != nil {
		return nil, err
	}
	nextVar := used.MaxFoundVarIndexPlusOne()
	headVar := nextVar
	nextVar++
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	eHead, err := x.getELit(head, headVar)
	if err != nil {
		return nil, err
	}
	held = append(held, eHead)
	posTailSz, err := r.PositiveTailSize()
	if err != nil {
		return nil, err
	}
	tailSz, err := r.TailSize()
	if err != nil {
		return nil, err
	}
	eTail := make([]ast.ASTHandle, 0, tailSz+1)
	negFlags := make([]bool, 0, tailSz+1)
	for i := 0; i < posTailSz; i++ {
		eVar := nextVar
		nextVar++
		tail, _, err := r.Tail(i)
		if err != nil {
			return nil, err
		}
		lit, err := x.getELit(tail, eVar)
		if err != nil {
			return nil, err
		}
		held = append(held, lit)
		h, err := x.m.RefGet(lit)
		if err != nil {
			return nil, err
		}
		eTail = append(eTail, h)
		negFlags = append(negFlags, false)
	}
	for i := posTailSz; i < tailSz; i++ {
		tail, negated, err := r.Tail(i)
		if err != nil {
			return nil, err
		}
		eTail = append(eTail, tail)
		negFlags = append(negFlags, negated)
	}
	ruleRepr, err := x.getRuleSymbol(r)
	if err != nil {
		return nil, err
	}
	ruleExprArgs := make([]ast.ASTHandle, 0, posTailSz)
	for i := 0; i < posTailSz; i++ {
		// this adds the explanation term of the tail
		n, err := x.m.GetNumArgs(eTail[i])
		if err != nil {
			return nil, err
		}
		a, err := x.m.GetArg(eTail[i], n-1)
		if err != nil {
			return nil, err
		}
		ruleExprArgs = append(ruleExprArgs, a)
	}
	ruleExpr, err := x.m.MkDatalogRule(ruleRepr, ruleExprArgs)
	if err != nil {
		return nil, err
	}
	ruleExprRef, err := x.m.NewRef(ruleExpr)
	if err != nil {
		return nil, err
	}
	held = append(held, ruleExprRef)
	hv, err := x.m.MkVar(headVar, x.eSort)
	if err != nil {
		return nil, err
	}
	record, err := x.m.MkEq(hv, ruleExpr)
	if err != nil {
		return nil, err
	}
	recordRef, err := x.m.NewRef(record)
	if err != nil {
		return nil, err
	}
	held = append(held, recordRef)
	eTail = append(eTail, record)
	negFlags = append(negFlags, false)
	eHeadH, err := x.m.RefGet(eHead)
	if err != nil {
		return nil, err
	}
	return x.engine.rules.NewRuleWithSymbolUnchecked(eHeadH, eTail, negFlags, usymbol.Symbol{})
}

// transformRules is transform_rules(src, dst).
func (x *mkExplanations) transformRules(src, dst *base.RuleSet) (resultErr error) {
	count, err := src.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		r, err := src.Rule(i)
		if err != nil {
			return err
		}
		eRule, err := x.getERule(r)
		err = errors.Join(err, r.Close())
		if err != nil {
			return err
		}
		err = dst.Add(eRule)
		if err = errors.Join(err, eRule.Close()); err != nil {
			return err
		}
	}
	// add rules that will (for output predicates) copy facts from explained
	// relations back to the original ones
	outputs, err := x.declSetOrder(src.OutputPredicates())
	if err != nil {
		return err
	}
	for _, orig := range outputs {
		if err = x.addCopyRule(orig, dst); err != nil {
			return err
		}
	}
	return nil
}

// addCopyRule is transform_rules' orig(x) :- orig_expl(x, e).
func (x *mkExplanations) addCopyRule(orig ast.ASTHandle, dst *base.RuleSet) (resultErr error) {
	var held []ast.ASTRef
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, x.m.DelRef(&held[i]))
		}
	}()
	arity, err := x.m.GetArity(orig)
	if err != nil {
		return err
	}
	litArgs := make([]ast.ASTHandle, 0, arity)
	for i := uint32(0); i < arity; i++ {
		s, err := x.m.GetDomain(orig, i)
		if err != nil {
			return err
		}
		v, err := x.m.MkVar(i, s)
		if err != nil {
			return err
		}
		ref, err := x.m.NewRef(v)
		if err != nil {
			return err
		}
		held = append(held, ref)
		litArgs = append(litArgs, v)
	}
	origLit, err := x.m.MkApp(orig, litArgs)
	if err != nil {
		return err
	}
	origRef, err := x.m.NewRef(origLit)
	if err != nil {
		return err
	}
	held = append(held, origRef)
	eLit, err := x.getELit(origLit, arity)
	if err != nil {
		return err
	}
	held = append(held, eLit)
	eLitH, err := x.m.RefGet(eLit)
	if err != nil {
		return err
	}
	r, err := x.engine.rules.NewRuleWithSymbolUnchecked(origLit, []ast.ASTHandle{eLitH}, nil, usymbol.Symbol{})
	if err != nil {
		return err
	}
	err = dst.Add(r)
	return errors.Join(err, r.Close())
}

// declSetOrder is the iteration order of a func_decl_set (obj_hashtable)
// built by inserting decls in order.
func (x *mkExplanations) declSetOrder(decls []ast.ASTHandle, err error) ([]ast.ASTHandle, error) {
	if err != nil {
		return nil, err
	}
	set := ast.NewObjMap(x.m)
	for _, d := range decls {
		if _, err := set.InsertIfNotThere(d); err != nil {
			return nil, err
		}
	}
	entries := set.Entries()
	res := make([]ast.ASTHandle, 0, len(entries))
	for _, entry := range entries {
		res = append(res, entry.Key)
	}
	return res, nil
}

// transformFacts is transform_facts(rmgr, src, dst).
func (x *mkExplanations) transformFacts(src, dst *base.RuleSet) error {
	e := x.engine
	rmgr := e.relations
	if x.eFact == nil {
		f, err := rmgr.MkExplanationFactRelation(x.er, x.eSort)
		if err != nil {
			return err
		}
		x.eFact = f
	}
	// func_decl_set predicates(m_context.get_predicates())
	predicates, err := e.rules.ContextPredicatesCopy()
	if err != nil {
		return err
	}
	for _, orig := range predicates {
		eDecl, err := x.getEDecl(orig)
		if err != nil {
			return err
		}
		_, exists, err := rmgr.TryGetFiniteRelation(orig)
		if err != nil {
			return err
		}
		exists = exists && !rmgr.IsPlaceholderRelation(orig)
		if !exists {
			contains, err := src.Contains(orig)
			if err != nil {
				return err
			}
			if !contains {
				// there are no facts or rules for this predicate
				continue
			}
		}
		if err = dst.InheritPredicate(src, orig, eDecl); err != nil {
			return err
		}
		origTable, err := rmgr.GetFiniteRelation(orig)
		if err != nil {
			return err
		}
		eTable, err := rmgr.GetFiniteRelation(eDecl)
		if err != nil {
			return err
		}
		origRel := rmgr.RelationView(origTable, e.predicates[orig].sorts)
		eRel := rmgr.RelationView(eTable, e.predicates[eDecl].sorts)
		if err = rmgr.ExplainRelation(x.er, origRel, eRel, x.eFact); err != nil {
			return err
		}
	}
	return nil
}

// Transform is mk_explanations::operator()(source).
func (x *mkExplanations) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	n, err := source.Len()
	if err != nil || n == 0 {
		return nil, err
	}
	res, err := base.NewRuleSet(x.engine.rules)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, res.Close())
		}
	}()
	if err = x.transformFacts(source, res); err != nil {
		return nil, err
	}
	if err = x.transformRules(source, res); err != nil {
		return nil, err
	}
	return res, nil
}

// mkFreshHeadPredicate is context::mk_fresh_head_predicate(prefix, suffix,
// arity, domain, orig_pred): a fresh registered predicate that inherits
// orig's requested relation kind.
func (e *PropositionalEngine) mkFreshHeadPredicate(prefix, suffix usymbol.Symbol, domain []ast.ASTHandle, orig ast.ASTHandle) (ast.ASTHandle, error) {
	b, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := e.manager.MkFreshFuncDecl(prefix, suffix, domain, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.RegisterPredicate(d); err != nil {
		return ast.ASTHandle{}, err
	}
	e.auxiliary[d] = true
	if e.relations.CanHandleTableSorts(domain) {
		if err = e.inheritDatalogFiniteRepresentation(d, orig); err != nil {
			return ast.ASTHandle{}, err
		}
		return d, nil
	}
	// rel_context::inherit_predicate_kind: the origin's requested kind; a
	// table kind that cannot handle the signature leaves
	// mk_empty_relation's choice to the favourite and the plugin list.
	if err = e.relations.InheritPredicateKind(d, orig); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.replaceKindPlaceholder(d); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}

// replaceKindPlaceholder makes a placeholder created before the predicate's
// requested kind was set the kind's relation (get_relation creates it with
// the kind).
func (e *PropositionalEngine) replaceKindPlaceholder(d ast.ASTHandle) error {
	state := e.predicates[d]
	if state == nil || state.table == nil || !e.relations.IsPlaceholderRelation(d) {
		return nil
	}
	table, err := e.relations.RecreatePlaceholderRelation(d)
	if err != nil {
		return err
	}
	state.table = table
	return nil
}

// applyQueryExplanations is rel_context::query's
// m_context.transform_rules(alloc(mk_explanations, m_context)) followed by
// query_pred = get_rules().get_pred(query_pred).
func (e *PropositionalEngine) applyQueryExplanations(goal ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	if err := e.flushPendingRules(); err != nil {
		return goal, err
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return goal, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
		if err != nil {
			return goal, err
		}
		if !valid {
			return goal, ErrUnstratifiedNegation
		}
	}
	plugin, err := e.newMkExplanations()
	if err != nil {
		return goal, err
	}
	transformer := base.NewRuleTransformer()
	if err = transformer.Register(plugin); err != nil {
		return goal, errors.Join(err, plugin.Close(), transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil {
		return goal, err
	}
	if modified {
		e.revision++
	}
	return e.set.DerivedPredicate(goal)
}
