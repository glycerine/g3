package transforms

// Source: src/muz/transforms/dl_mk_interp_tail_simplifier.cpp/.h retained
// interpreted-tail rewriting and variable-equivalence propagation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type InterpTailSimplifierPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	rewriter *astrewriter.TheoryRewriter
	// borrowed: the rewriter is datalog::context's m_rewriter (m_simp is a
	// reference to ctx.get_rewriter()), owned by the context.
	borrowed bool
	// heldSimplified is simp_res awaiting the end of the propagation step.
	heldSimplified []ast.ASTRef
	params         *uparams.ParamsContext
	globals        *uparams.GlobalParamsService
	normalization  *RuleNormalizationParams
	priority       uint32
	// rule_substitution m_rule_subst: one per plugin, reset for each rule
	// (m_subst keeps its apply results until then); m_head, m_tail, m_neg
	// hold the last get_result.
	ruleSubst   *substitution.Substitution
	ruleUnifier *substitution.Unifier
	ruleHead    ast.ASTRef
	ruleTails   []ast.ASTRef
	ruleNeg     []bool
	closed      bool
	closing     bool
}

func NewInterpTailSimplifierPlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (*InterpTailSimplifierPlugin, error) {
	if priority == 0 {
		priority = 40000
	}
	rewriter, err := astrewriter.NewTheoryRewriter(manager, context, globals, nil)
	if err != nil {
		if rewriter != nil {
			err = errors.Join(err, rewriter.Close())
		}
		return nil, err
	}
	p := &InterpTailSimplifierPlugin{manager: manager, rules: rules, rewriter: rewriter, params: context, globals: globals, priority: priority}
	if err = p.initRuleSubstitution(); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

// NewInterpTailSimplifierPluginShared borrows the context rewriter, whose
// caches outlive the plugin as in the source.
// newNestedInterpTailSimplifier is a plugin-member mk_interp_tail_simplifier:
// the source's simplifier always uses ctx.get_rewriter(); a nil context
// rewriter (standalone use) gives it its own.
func newNestedInterpTailSimplifier(manager *ast.ASTManager, rules *base.RuleManager, contextRewriter *astrewriter.TheoryRewriter, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (*InterpTailSimplifierPlugin, error) {
	if contextRewriter != nil {
		return NewInterpTailSimplifierPluginShared(manager, rules, contextRewriter, context, globals, priority)
	}
	return NewInterpTailSimplifierPlugin(manager, rules, context, globals, priority)
}

func NewInterpTailSimplifierPluginShared(manager *ast.ASTManager, rules *base.RuleManager, rewriter *astrewriter.TheoryRewriter, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (*InterpTailSimplifierPlugin, error) {
	if rewriter == nil {
		return NewInterpTailSimplifierPlugin(manager, rules, context, globals, priority)
	}
	if priority == 0 {
		priority = 40000
	}
	p := &InterpTailSimplifierPlugin{manager: manager, rules: rules, rewriter: rewriter, borrowed: true, params: context, globals: globals, priority: priority}
	if err := p.initRuleSubstitution(); err != nil {
		return nil, errors.Join(err, p.Close())
	}
	return p, nil
}

func (p *InterpTailSimplifierPlugin) Priority() uint32            { return p.priority }
func (p *InterpTailSimplifierPlugin) CanDestratifyNegation() bool { return false }
func (p *InterpTailSimplifierPlugin) Cancel()                     {}

// initRuleSubstitution constructs rule_substitution(ctx).
func (p *InterpTailSimplifierPlugin) initRuleSubstitution() (err error) {
	if p.ruleSubst, err = substitution.New(p.manager); err != nil {
		return err
	}
	if p.ruleUnifier, err = substitution.NewUnifier(p.manager); err != nil {
		return err
	}
	p.ruleHead, err = p.manager.NewRef(ast.ASTHandle{})
	return err
}

// releaseRuleTails is m_tail.reset() (front to back) and m_neg.reset().
func (p *InterpTailSimplifierPlugin) releaseRuleTails() (result error) {
	for len(p.ruleTails) != 0 {
		if err := p.manager.DelRef(&p.ruleTails[0]); err != nil {
			return err
		}
		p.ruleTails = p.ruleTails[1:]
	}
	p.ruleNeg = p.ruleNeg[:0]
	return nil
}

// closeRuleSubstitution is ~rule_substitution: members in reverse order.
func (p *InterpTailSimplifierPlugin) closeRuleSubstitution() (result error) {
	result = p.releaseRuleTails()
	if p.ruleHead != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&p.ruleHead))
	}
	if p.ruleUnifier != nil {
		result = errors.Join(result, p.ruleUnifier.Close())
		p.ruleUnifier = nil
	}
	if p.ruleSubst != nil {
		result = errors.Join(result, p.ruleSubst.Close())
		p.ruleSubst = nil
	}
	return result
}

func (p *InterpTailSimplifierPlugin) simplifyRule(rule *base.Rule) (_ *base.Rule, dropped, changed bool, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, false, err
	}
	total, err := rule.TailSize()
	if err != nil || uninterpreted == total {
		return nil, false, false, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < uninterpreted; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, false, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	interpreted := make([]ast.ASTHandle, 0, total-uninterpreted)
	for index := uninterpreted; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, false, getErr
		}
		if negative {
			return nil, false, false, fmt.Errorf("muz/transforms: interpreted rule tail retained negation metadata")
		}
		interpreted = append(interpreted, tail)
	}
	combined := interpreted[0]
	if len(interpreted) != 1 {
		combined, err = p.manager.MkAndArgs(interpreted)
		if err != nil {
			return nil, false, false, err
		}
		changed = true
	}
	// app_ref itail is destroyed with transform_rule's other locals, just
	// after simp_res.
	itail, err := p.manager.NewRef(combined)
	if err != nil {
		return nil, false, false, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&itail))
			return
		}
		p.heldSimplified = append(p.heldSimplified, itail)
	}()
	out, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&out)) }()
	if err = p.rewriter.Rewrite(combined, out); err != nil {
		return nil, false, false, err
	}
	simplified, err := p.manager.RefGet(out)
	if err != nil {
		return nil, false, false, err
	}
	normalized, err := normalizeBooleanExpression(p.manager, simplified)
	if err != nil {
		return nil, false, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&normalized)) }()
	normalizedHandle, err := p.manager.RefGet(normalized)
	if err != nil {
		return nil, false, false, err
	}
	final, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, false, err
	}
	// transform_rule's expr_ref simp_res stays alive through
	// propagate_variable_equivalences; the caller releases it then.
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&final))
			return
		}
		p.heldSimplified = append(p.heldSimplified, final)
	}()
	if err = p.rewriter.Rewrite(normalizedHandle, final); err != nil {
		return nil, false, false, err
	}
	simplified, err = p.manager.RefGet(final)
	if err != nil {
		return nil, false, false, err
	}
	changed = changed || simplified != combined
	if p.manager.IsFalse(simplified) {
		return nil, true, true, nil
	}
	if !changed {
		return nil, false, false, nil
	}
	conjuncts, err := p.manager.FlattenAnd([]ast.ASTHandle{simplified})
	if err != nil {
		return nil, false, false, err
	}
	for _, conjunct := range conjuncts {
		if p.manager.IsTrue(conjunct) {
			continue
		}
		kind, getErr := p.manager.GetKind(conjunct)
		if getErr != nil {
			return nil, false, false, getErr
		}
		if kind == ast.ASTVar {
			truth, getErr := p.manager.MkTrue()
			if getErr != nil {
				return nil, false, false, getErr
			}
			conjunct, getErr = p.manager.MkEq(conjunct, truth)
			if getErr != nil {
				return nil, false, false, getErr
			}
		}
		tails = append(tails, conjunct)
		negated = append(negated, false)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, false, err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, false, false, err
	}
	replacement, err := p.rules.NewRuleWithSymbol(head, tails, negated, name)
	if err != nil {
		return nil, false, false, err
	}
	if err = replacement.SetAccountingParentRule(rule); err != nil {
		return nil, false, false, errors.Join(err, replacement.Close())
	}
	return replacement, false, true, nil
}

func (p *InterpTailSimplifierPlugin) propagate(rule *base.Rule) (_ *base.Rule, changed bool, resultErr error) {
	enabled, err := p.variableEquivalencePropagation()
	if err != nil || !enabled {
		return nil, false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	total, err := rule.TailSize()
	if err != nil || uninterpreted == total {
		return nil, false, err
	}
	variables, err := ruleVariableCount(p.manager, rule)
	if err != nil {
		return nil, false, err
	}
	// m_rule_subst.reset(r): the previous rule's apply results die here.
	subst, unifier := p.ruleSubst, p.ruleUnifier
	if err = subst.Reset(); err != nil {
		return nil, false, err
	}
	if err = subst.Reserve(1, variables); err != nil {
		return nil, false, err
	}
	tryUnify := func(left, right ast.ASTHandle) (unified bool, resultErr error) {
		leftRef, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return false, makeErr
		}
		rightRef, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return false, errors.Join(makeErr, p.manager.DelRef(&leftRef))
		}
		defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&rightRef), p.manager.DelRef(&leftRef)) }()
		if makeErr = subst.ApplyExpression(left, leftRef); makeErr != nil {
			return false, makeErr
		}
		if makeErr = subst.ApplyExpression(right, rightRef); makeErr != nil {
			return false, makeErr
		}
		left, makeErr = p.manager.RefGet(leftRef)
		if makeErr != nil {
			return false, makeErr
		}
		right, makeErr = p.manager.RefGet(rightRef)
		if makeErr != nil {
			return false, makeErr
		}
		if makeErr = subst.ResetCache(); makeErr != nil {
			return false, makeErr
		}
		return unifier.UnifyPair(left, right, subst, false)
	}
	flexible := func(expression ast.ASTHandle) (bool, error) {
		kind, getErr := p.manager.GetKind(expression)
		if getErr != nil {
			return false, getErr
		}
		if kind == ast.ASTVar {
			return true, nil
		}
		return p.manager.IsValue(expression)
	}
	// expr_ref_vector trail and expr_ref tmp1, tmp2 are locals: at return
	// tmp2 dies, then tmp1, then trail releases forward.
	var trail []ast.ASTRef
	var tmp1, tmp2 ast.ASTRef
	defer func() {
		for _, ref := range []*ast.ASTRef{&tmp2, &tmp1} {
			if *ref != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, p.manager.DelRef(ref))
			}
		}
		for i := range trail {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&trail[i]))
		}
	}()
	assign := func(slot *ast.ASTRef, value ast.ASTHandle) error {
		if *slot == (ast.ASTRef{}) {
			ref, err := p.manager.NewRef(value)
			if err == nil {
				*slot = ref
			}
			return err
		}
		return p.manager.SetRef(*slot, value)
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	var todo []ast.ASTHandle
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		todo = append(todo, tail)
	}
	for len(todo) != 0 {
		last := len(todo) - 1
		term := todo[last]
		todo = todo[:last]
		negative, bare, getErr := p.manager.IsNot(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if negative {
			term = bare
		}
		kind, getErr := p.manager.GetKind(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if kind == ast.ASTVar {
			value, valueErr := p.manager.MkBoolVal(!negative)
			if valueErr != nil {
				return nil, false, valueErr
			}
			unified, valueErr := tryUnify(term, value)
			if valueErr != nil {
				return nil, false, valueErr
			}
			changed = changed || unified
			continue
		}
		// Declaration kinds are local to a family: arithmetic <=, for
		// example, has the same numeric kind as basic equality. Source
		// is_and/is_eq/is_iff require the basic family before propagating.
		family, getErr := p.manager.GetFamilyID(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if family == arithmetic && !negative {
			// a.is_le(t, a1, a2) || a.is_ge(t, a2, a1): the source builds
			// both differences (its unification branch is disabled) and
			// keeps tmp1 on the trail.
			operation, opErr := p.manager.GetDeclKind(term)
			if opErr != nil {
				return nil, false, opErr
			}
			if operation == 2 || operation == 3 {
				first, x := p.manager.GetArg(term, 0)
				if x != nil {
					return nil, false, x
				}
				second, x := p.manager.GetArg(term, 1)
				if x != nil {
					return nil, false, x
				}
				left, right := first, second
				if operation == 3 {
					left, right = second, first
				}
				difference, x := p.manager.MkArithmeticSub(left, right)
				if x == nil {
					x = assign(&tmp1, difference)
				}
				if x != nil {
					return nil, false, x
				}
				reverse, x := p.manager.MkArithmeticSub(right, left)
				if x == nil {
					x = assign(&tmp2, reverse)
				}
				if x != nil {
					return nil, false, x
				}
				held, x := p.manager.NewRef(difference)
				if x != nil {
					return nil, false, x
				}
				trail = append(trail, held)
			}
			continue
		}
		if family != p.manager.BasicFamilyID() {
			continue
		}
		operation, getErr := p.manager.GetDeclKind(term)
		if getErr != nil {
			return nil, false, getErr
		}
		if !negative && operation == ast.ASTOpAnd {
			count, countErr := p.manager.GetNumArgs(term)
			if countErr != nil {
				return nil, false, countErr
			}
			for index := uint32(0); index < count; index++ {
				argument, argErr := p.manager.GetArg(term, index)
				if argErr != nil {
					return nil, false, argErr
				}
				todo = append(todo, argument)
			}
			continue
		}
		if operation != ast.ASTOpEq && operation != ast.ASTOpIff {
			continue
		}
		left, getErr := p.manager.GetArg(term, 0)
		if getErr != nil {
			return nil, false, getErr
		}
		right, getErr := p.manager.GetArg(term, 1)
		if getErr != nil {
			return nil, false, getErr
		}
		if operation == ast.ASTOpIff {
			for {
				negated, bare, e := p.manager.IsNot(left)
				if e != nil {
					return nil, false, e
				}
				if !negated {
					break
				}
				left = bare
				negative = !negative
			}
			for {
				negated, bare, e := p.manager.IsNot(right)
				if e != nil {
					return nil, false, e
				}
				if !negated {
					break
				}
				right = bare
				negative = !negative
			}
			leftKind, e := p.manager.GetKind(left)
			if e != nil {
				return nil, false, e
			}
			if leftKind != ast.ASTVar {
				left, right = right, left
			}
		}
		leftFlex, getErr := flexible(left)
		if getErr != nil {
			return nil, false, getErr
		}
		rightFlex, getErr := flexible(right)
		if getErr != nil {
			return nil, false, getErr
		}
		if !leftFlex || !rightFlex {
			continue
		}
		leftKind, getErr := p.manager.GetKind(left)
		if getErr != nil {
			return nil, false, getErr
		}
		var target ast.ASTHandle
		switch {
		case operation == ast.ASTOpEq && !negative:
			target = right
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && !negative:
			target = right
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && negative && p.manager.IsTrue(right):
			target, getErr = p.manager.MkFalse()
		case operation == ast.ASTOpIff && leftKind == ast.ASTVar && negative && p.manager.IsFalse(right):
			target, getErr = p.manager.MkTrue()
		default:
			continue
		}
		if getErr != nil {
			return nil, false, getErr
		}
		unified, unifyErr := tryUnify(left, target)
		if unifyErr != nil {
			return nil, false, unifyErr
		}
		changed = changed || unified
	}
	if !changed {
		return nil, false, nil
	}
	// rule_substitution::get_result: apply(head, m_head); m_tail.reset();
	// each new tail is applied into a local app_ref and pushed on m_tail.
	apply := func(expression ast.ASTHandle, out ast.ASTRef) (ast.ASTHandle, error) {
		if applyErr := subst.ApplyExpression(expression, out); applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		return p.manager.RefGet(out)
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	head, err = apply(head, p.ruleHead)
	if err != nil {
		return nil, false, err
	}
	if err = p.releaseRuleTails(); err != nil {
		return nil, false, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < total; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, false, getErr
		}
		ref, getErr := p.manager.NewRef(ast.ASTHandle{})
		if getErr != nil {
			return nil, false, getErr
		}
		p.ruleTails = append(p.ruleTails, ref)
		p.ruleNeg = append(p.ruleNeg, negative)
		tail, getErr = apply(tail, ref)
		if getErr != nil {
			return nil, false, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	// remove_duplicate_tails drops later copies of an identical tail.
	tails, negated, err = base.RemoveDuplicateRuleTails(tails, negated)
	if err != nil {
		return nil, false, fmt.Errorf("muz/transforms: remove duplicate simplified tails: %w", err)
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, false, err
	}
	replacement, err := p.rules.NewRuleWithSymbol(head, tails, negated, name)
	if err != nil {
		return nil, false, err
	}
	if err = replacement.SetAccountingParentRule(rule); err != nil {
		return nil, false, errors.Join(err, replacement.Close())
	}
	return replacement, true, nil
}

func (p *InterpTailSimplifierPlugin) releaseHeldSimplified() (result error) {
	for i := range p.heldSimplified {
		result = errors.Join(result, p.manager.DelRef(&p.heldSimplified[i]))
	}
	p.heldSimplified = p.heldSimplified[:0]
	return result
}

func (p *InterpTailSimplifierPlugin) transformRule(source *base.Rule) (_ *base.Rule, dropped, changed bool, resultErr error) {
	if p == nil || p.closed || p.closing {
		return nil, false, false, base.ErrRuleTransformerOwner
	}
	head, err := source.Head()
	if err != nil {
		return nil, false, false, err
	}
	hasQuantifiers, err := p.manager.HasQuantifiers(head)
	if err != nil {
		return nil, false, false, err
	}
	total, err := source.TailSize()
	for index := 0; err == nil && !hasQuantifiers && index < total; index++ {
		var tail ast.ASTHandle
		tail, _, err = source.Tail(index)
		if err == nil {
			hasQuantifiers, err = p.manager.HasQuantifiers(tail)
		}
	}
	if err != nil || hasQuantifiers {
		return nil, false, false, err
	}
	current := source
	owned := false
	defer func() {
		if owned && resultErr != nil {
			resultErr = errors.Join(resultErr, current.Close())
		}
	}()
	for {
		simplified, drop, simplifiedChanged, err := p.simplifyRule(current)
		if err != nil {
			return nil, false, changed, errors.Join(err, p.releaseHeldSimplified())
		}
		if drop {
			if err = p.releaseHeldSimplified(); err != nil {
				return nil, false, changed, err
			}
			if owned {
				resultErr = current.Close()
				owned = false
			}
			return nil, true, true, resultErr
		}
		if simplifiedChanged {
			if owned {
				if err = current.Close(); err != nil {
					return nil, false, changed, err
				}
			}
			current, owned, changed = simplified, true, true
		}
		propagated, propagatedChanged, err := p.propagate(current)
		err = errors.Join(err, p.releaseHeldSimplified())
		if err != nil {
			return nil, false, changed, err
		}
		if !propagatedChanged {
			if !owned {
				return nil, false, changed, nil
			}
			if err = p.rules.MkRuleRewriteProof(source, current); err != nil {
				return nil, false, changed, err
			}
			owned = false
			return current, false, changed, nil
		}
		if owned {
			if err = current.Close(); err != nil {
				return nil, false, changed, errors.Join(err, propagated.Close())
			}
		}
		current, owned, changed = propagated, true, true
	}
}

func (p *InterpTailSimplifierPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.closing || p.manager == nil || p.rules == nil || p.rewriter == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	changed := false
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		replacement, dropped, ruleChanged, transformErr := p.transformRule(rule)
		changed = changed || ruleChanged
		if transformErr == nil && !dropped {
			if replacement == nil {
				replacement = rule
			}
			transformErr = result.Add(replacement)
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if replacement != nil && replacement != rule {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *InterpTailSimplifierPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closing = true
	if p.rewriter != nil && !p.borrowed {
		if err := p.rewriter.Close(); err != nil {
			return err
		}
	}
	p.rewriter = nil
	// m_rule_subst is the first data member, so it is destroyed last.
	if err := p.closeRuleSubstitution(); err != nil {
		return err
	}
	p.closed = true
	return nil
}
