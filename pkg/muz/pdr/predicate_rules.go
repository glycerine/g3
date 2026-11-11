package pdr

// Source: pred_transformer::initialize, init_rules, init_rule,
// ground_free_vars, and init_atom in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (p *PredicateTransformer) selectRule(modelCore *model.ModelCore) (*normalizedRule, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if modelCore == nil {
		return nil, ast.ErrASTDomain
	}
	live := make([]*normalizedRule, 0, len(p.rules))
	for _, rule := range p.rules {
		if !rule.tag.IsNull() {
			live = append(live, rule)
		}
	}
	if len(live) == 1 {
		return live[0], nil
	}
	out, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer p.manager.DelRef(&out)
	for _, rule := range live {
		declaration, x := p.manager.GetDecl(rule.tag)
		if x != nil {
			return nil, x
		}
		found, x := modelCore.EvalConstant(declaration, out)
		if x != nil {
			return nil, x
		}
		if !found {
			continue
		}
		value, x := p.manager.RefGet(out)
		if x != nil {
			return nil, x
		}
		if p.manager.IsTrue(value) {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("muz/pdr: model did not select a transition rule")
}

func (p *PredicateTransformer) selectInitialRule(modelCore *model.ModelCore) (*normalizedRule, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	initial := make([]*normalizedRule, 0, len(p.rules))
	for _, rule := range p.rules {
		if len(rule.predecessors) == 0 {
			initial = append(initial, rule)
		}
	}
	if len(initial) == 1 {
		return initial[0], nil
	}
	if modelCore == nil {
		return nil, ast.ErrASTDomain
	}
	out, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer p.manager.DelRef(&out)
	for _, rule := range initial {
		declaration, x := p.manager.GetDecl(rule.tag)
		if x != nil {
			return nil, x
		}
		found, x := modelCore.EvalConstant(declaration, out)
		if x != nil {
			return nil, x
		}
		if !found {
			continue
		}
		value, x := p.manager.RefGet(out)
		if x != nil {
			return nil, x
		}
		if p.manager.IsTrue(value) {
			return rule, nil
		}
	}
	return nil, fmt.Errorf("muz/pdr: model did not select an initial rule")
}

func (p *PredicateTransformer) modelConstant(modelCore *model.ModelCore, application ast.ASTHandle, out ast.ASTRef) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if modelCore == nil {
		return false, ast.ErrASTDomain
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return false, err
	}
	if count != 0 {
		return false, ast.ErrASTDomain
	}
	declaration, err := p.manager.GetDecl(application)
	if err != nil {
		return false, err
	}
	found, err := modelCore.EvalConstant(declaration, out)
	if err != nil || found {
		return found, err
	}
	sort, err := p.manager.GetSort(application)
	if err != nil {
		return false, err
	}
	value, err := modelCore.GetSomeValue(sort)
	if err != nil {
		return false, err
	}
	if value.IsNull() {
		return false, ast.ErrASTDomain
	}
	if err = p.manager.SetRef(out, value); err != nil {
		return false, err
	}
	return true, nil
}

type normalizedRule struct {
	rule         *base.Rule
	transition   ast.ASTHandle
	tag          ast.ASTHandle
	instances    []ast.ASTHandle
	auxiliaries  []ast.ASTHandle
	predecessors []*PredicateTransformer
}

type predicateRuleBuilder struct {
	transformer *PredicateTransformer
	all         map[ast.ASTHandle]*PredicateTransformer
	theory      *rewriter.TheoryRewriter
	references  []ast.ASTRef
	rules       []*normalizedRule
	// initialRef is the m_initial_state expr_ref slot (also in references).
	initialRef ast.ASTRef
	// ruleTheory is init_rule's local th_rewriter, live for one rule.
	ruleTheory *rewriter.TheoryRewriter
	// ruleConj is init_rule's local expr_ref_vector conj.
	ruleConj []ast.ASTRef
	// finishRule releases init_rule's locals (set by normalizeRule).
	finishRule func(failed error) error
}

// pinConj is conj.push_back(e).
func (b *predicateRuleBuilder) pinConj(handle ast.ASTHandle) error {
	reference, err := b.transformer.manager.NewRef(handle)
	if err == nil {
		b.ruleConj = append(b.ruleConj, reference)
	}
	return err
}

func (b *predicateRuleBuilder) pin(handle ast.ASTHandle) error {
	reference, err := b.transformer.manager.NewRef(handle)
	if err == nil {
		b.references = append(b.references, reference)
	}
	return err
}

// mkAnd is pm.mk_and: the bool rewriter's and, held in the builder pins.
func (b *predicateRuleBuilder) mkAnd(conjuncts []ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := b.transformer.owner.AndRef(conjuncts)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b.references = append(b.references, ref)
	return b.transformer.manager.RefGet(ref)
}

func (b *predicateRuleBuilder) fresh(prefix string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	value, err := b.transformer.manager.MkFreshConst(prefix, sort)
	if err == nil {
		err = b.pin(value)
	}
	return value, err
}

func (b *predicateRuleBuilder) rewrite(formula ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := b.transformer.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	theory := b.theory
	if b.ruleTheory != nil {
		theory = b.ruleTheory
	}
	if err = theory.Rewrite(formula, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	result, err := b.transformer.manager.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	b.references = append(b.references, out)
	return result, nil
}

// rewriteInPlace is th_rewriter::operator()(expr_ref& term): the result
// replaces term's value (releasing the old one) and the temporary dies.
func (b *predicateRuleBuilder) rewriteInPlace(term ast.ASTRef, formula ast.ASTHandle) (result ast.ASTHandle, err error) {
	manager := b.transformer.manager
	out, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, manager.DelRef(&out)) }()
	theory := b.theory
	if b.ruleTheory != nil {
		theory = b.ruleTheory
	}
	if err = theory.Rewrite(formula, out); err != nil {
		return ast.ASTHandle{}, err
	}
	if result, err = manager.RefGet(out); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, manager.SetRef(term, result)
}

func (b *predicateRuleBuilder) blastTermITE(formula ast.ASTHandle) (ast.ASTHandle, error) {
	manager := b.transformer.manager
	output, err := manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source rewrites an expr_ref in place; keep its root owned by the rule
	// builder before the standalone traversal can publish a new expression.
	b.references = append(b.references, output)
	if err = tacticcore.BlastTermITEInto(manager, formula, output); err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.RefGet(output)
}

func (b *predicateRuleBuilder) stateApplication(transformer *PredicateTransformer, argument uint32, old *uint32) (ast.ASTHandle, error) {
	if argument >= uint32(len(transformer.signature)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	declaration := transformer.signature[argument]
	var err error
	if old == nil {
		declaration, err = b.transformer.owner.OldToNew(declaration, 0)
	} else {
		declaration, err = b.transformer.owner.OldToOld(declaration, 0, *old)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := b.transformer.manager.MkApp(declaration, nil)
	if err == nil {
		err = b.pin(application)
	}
	return application, err
}

func (b *predicateRuleBuilder) initializeAtom(atom ast.ASTHandle, variables *[]ast.ASTHandle, conjunction *[]ast.ASTHandle, old *uint32) (*PredicateTransformer, error) {
	declaration, err := b.transformer.manager.GetDecl(atom)
	if err != nil {
		return nil, err
	}
	owner := b.all[declaration]
	if owner == nil || owner.owner != b.transformer.owner {
		return nil, fmt.Errorf("%w: missing predicate transformer", ast.ErrASTDomain)
	}
	count, err := b.transformer.manager.GetNumArgs(atom)
	if err != nil || count != uint32(len(owner.signature)) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	for index := uint32(0); index < count; index++ {
		representative, x := b.stateApplication(owner, index, old)
		if x != nil {
			return nil, x
		}
		argument, x := b.transformer.manager.GetArg(atom, index)
		if x != nil {
			return nil, x
		}
		kind, x := b.transformer.manager.GetKind(argument)
		if x != nil {
			return nil, x
		}
		if kind == ast.ASTVar {
			variable, x := b.transformer.manager.GetVarIndex(argument)
			if x != nil {
				return nil, x
			}
			for uint32(len(*variables)) <= variable {
				*variables = append(*variables, ast.ASTHandle{})
			}
			if prior := (*variables)[variable]; prior.IsNull() {
				(*variables)[variable] = representative
			} else {
				equality, y := b.transformer.manager.MkEq(representative, prior)
				if y != nil {
					return nil, y
				}
				if y = b.pinConj(equality); y != nil {
					return nil, y
				}
				*conjunction = append(*conjunction, equality)
			}
			continue
		}
		equality, x := b.transformer.manager.MkEq(representative, argument)
		if x != nil {
			return nil, x
		}
		if x = b.pinConj(equality); x != nil {
			return nil, x
		}
		*conjunction = append(*conjunction, equality)
	}
	return owner, nil
}

func (b *predicateRuleBuilder) groundVariables(expression ast.ASTHandle, variables *[]ast.ASTHandle, auxiliaries *[]ast.ASTHandle) error {
	used, err := ast.NewASTUsedVars(b.transformer.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(expression); err != nil {
		return err
	}
	for uint32(len(*variables)) < used.MaxFoundVarIndexPlusOne() {
		*variables = append(*variables, ast.ASTHandle{})
	}
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		sort := used.Contains(index)
		if sort.IsNull() || !(*variables)[index].IsNull() {
			continue
		}
		value, x := b.fresh("aux", sort)
		if x != nil {
			return x
		}
		(*variables)[index] = value
		*auxiliaries = append(*auxiliaries, value)
	}
	return nil
}

func (b *predicateRuleBuilder) substitute(expression ast.ASTHandle, variables []ast.ASTHandle) (ast.ASTHandle, error) {
	substitution, err := rewriter.NewVarSubstitution(b.transformer.manager, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer substitution.Close()
	out, err := b.transformer.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = substitution.Substitute(expression, variables, out); err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	result, err := b.transformer.manager.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, b.transformer.manager.DelRef(&out))
	}
	// tmp is pushed on conj.
	b.ruleConj = append(b.ruleConj, out)
	return result, nil
}

// releaseForward is an expr_ref_vector destructor: front to back.
func (b *predicateRuleBuilder) releaseForward(refs []ast.ASTRef) (result error) {
	for i := range refs {
		if refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, b.transformer.manager.DelRef(&refs[i]))
		}
	}
	return result
}

func (b *predicateRuleBuilder) normalizeRule(source *base.Rule) (result *normalizedRule, resultErr error) {
	owned, err := source.Retain()
	if err != nil {
		return nil, err
	}
	result = &normalizedRule{rule: owned}
	// init_rule's locals die at its return in reverse declaration order:
	// fml and the flattened tail, then var_reprs, then conj. What the rule
	// keeps (the transition and the var_reprs copy) gets its own refs.
	mark := len(b.references)
	end := -1 // len(b.references) when normalizeRule returns
	defer func() { end = len(b.references) }()
	b.ruleConj = nil
	var tailRefs []ast.ASTRef
	// The caller runs finishRule after init = mk_or(init, fml) and after
	// the local th_rewriter dies.
	b.finishRule = func(failed error) (resultErr error) {
		var kept []ast.ASTRef
		if failed == nil {
			ref, err := b.transformer.manager.NewRef(result.transition)
			if err != nil {
				resultErr = err
			} else {
				kept = append(kept, ref)
			}
			for _, v := range result.instances {
				if resultErr != nil || v.IsNull() {
					continue
				}
				ref, err := b.transformer.manager.NewRef(v)
				if err != nil {
					resultErr = err
					continue
				}
				kept = append(kept, ref)
			}
		}
		if end < 0 {
			end = len(b.references)
		}
		locals := append([]ast.ASTRef(nil), b.references[mark:end]...)
		later := append([]ast.ASTRef(nil), b.references[end:]...)
		b.references = append(append(b.references[:mark], kept...), later...)
		for i := range tailRefs {
			resultErr = errors.Join(resultErr, b.transformer.owner.releaseNotAndRef(&tailRefs[i]))
		}
		resultErr = errors.Join(resultErr, b.releaseForward(locals), b.releaseForward(b.ruleConj))
		b.ruleConj = nil
		b.finishRule = nil
		return resultErr
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, b.finishRule(resultErr), owned.Close())
			result = nil
		}
	}()
	headDeclaration, err := owned.HeadDeclaration()
	if err != nil || headDeclaration != b.transformer.head {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	head, err := owned.Head()
	if err != nil {
		return nil, err
	}
	variables := make([]ast.ASTHandle, 0)
	conjunction := make([]ast.ASTHandle, 0)
	if _, err = b.initializeAtom(head, &variables, &conjunction, nil); err != nil {
		return nil, err
	}
	uninterpreted, err := owned.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	tailSize, err := owned.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < uninterpreted; index++ {
		tail, negated, x := owned.Tail(index)
		if x != nil {
			return nil, x
		}
		if negated {
			return nil, fmt.Errorf("muz/pdr: negated predicate tails are unsupported")
		}
		old := uint32(index)
		predecessor, x := b.initializeAtom(tail, &variables, &conjunction, &old)
		if x != nil {
			return nil, x
		}
		result.predecessors = append(result.predecessors, predecessor)
	}
	interpreted := make([]ast.ASTHandle, 0, tailSize-uninterpreted)
	for index := uninterpreted; index < tailSize; index++ {
		tail, _, x := owned.Tail(index)
		if x != nil {
			return nil, x
		}
		if x = b.groundVariables(tail, &variables, &result.auxiliaries); x != nil {
			return nil, x
		}
		interpreted = append(interpreted, tail)
	}
	interpreted, err = b.transformer.manager.FlattenAnd(interpreted)
	if err != nil {
		return nil, err
	}
	// Source flatten_and owns its vector, including synthesized negations.
	for _, tail := range interpreted {
		ref, x := b.transformer.manager.NewRef(tail)
		if x != nil {
			return nil, x
		}
		tailRefs = append(tailRefs, ref)
	}
	for _, formula := range interpreted {
		ground, x := b.substitute(formula, variables)
		if x != nil {
			return nil, x
		}
		conjunction = append(conjunction, ground)
		if x = b.traceRuleExpressions(602, formula, ground); x != nil {
			return nil, x
		}
		// Source checks each substituted tail before the manager rewriter can
		// fold away a quantified or otherwise non-ground expression.
		isGround, x := b.transformer.manager.IsGround(ground)
		if x != nil {
			return nil, x
		}
		if !isGround {
			return nil, b.nonGroundTailError(ground)
		}
	}
	// fml = pm.mk_and(conj); rw(fml) rewrites the expr_ref in place, so
	// the conjunction dies as soon as the rewrite returns.
	formula, err := b.mkAnd(conjunction)
	if err != nil {
		return nil, err
	}
	fml := b.references[len(b.references)-1]
	result.transition, err = b.rewriteInPlace(fml, formula)
	if err != nil {
		return nil, err
	}
	mode := b.transformer.owner.contexts.arithmeticMode()
	if mode == smtcore.SMTArithmeticDifferenceLogic || mode == smtcore.SMTArithmeticUTVPI {
		result.transition, err = b.blastTermITE(result.transition)
		if err != nil {
			return nil, err
		}
	}
	if err = b.traceRuleExpressions(616, result.transition); err != nil {
		return nil, err
	}
	ground, err := b.transformer.manager.IsGround(result.transition)
	if err != nil || !ground {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("muz/pdr: normalized rule transition is not ground")
	}
	result.instances = append(result.instances, variables...)
	return result, nil
}

func (b *predicateRuleBuilder) tagRule(rule *normalizedRule) error {
	boolean, err := b.transformer.manager.MkBoolSort()
	if err != nil {
		return err
	}
	name, err := b.transformer.headName()
	if err != nil {
		return err
	}
	rule.tag, err = b.fresh(name, boolean)
	return err
}

// extendInitial is init_rule's `init = m.mk_or(init, fml)` for a fact rule
// with a non-false transition, made right after that rule.
func (b *predicateRuleBuilder) extendInitial(initial ast.ASTHandle, rule *normalizedRule) (ast.ASTHandle, error) {
	if b.transformer.manager.IsFalse(rule.transition) || len(rule.predecessors) != 0 {
		return initial, nil
	}
	extended, err := b.transformer.manager.MkOr(initial, rule.transition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return extended, b.setInitial(extended)
}

// setInitial assigns the m_initial_state expr_ref: the previous value is
// released when replaced, as `init = ...` and `rw(m_initial_state)` do.
func (b *predicateRuleBuilder) setInitial(initial ast.ASTHandle) error {
	if b.initialRef == (ast.ASTRef{}) {
		ref, err := b.transformer.manager.NewRef(initial)
		if err != nil {
			return err
		}
		b.initialRef = ref
		b.references = append(b.references, ref)
		return nil
	}
	return b.transformer.manager.SetRef(b.initialRef, initial)
}

// buildSystem is the rest of init_rules: the transition over the live rules.
// The initial state was accumulated per rule; initialize rewrites the
// transition first, then the initial state.
func (b *predicateRuleBuilder) buildSystem(initial ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	live := make([]*normalizedRule, 0, len(b.rules))
	for _, rule := range b.rules {
		if b.transformer.manager.IsFalse(rule.transition) {
			continue
		}
		live = append(live, rule)
	}
	if len(live) == 0 {
		falsity, err := b.transformer.manager.MkFalse()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if falsity, err = b.rewrite(falsity); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if initial, err = b.rewrite(initial); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		return initial, falsity, b.setInitial(initial)
	}
	var err error
	// init_rules' transitions/disj vectors are locals that die before
	// initialize rewrites m_transition.
	var locals []ast.ASTRef
	releaseLocals := func() (result error) {
		for i := range locals {
			result = errors.Join(result, b.transformer.manager.DelRef(&locals[i]))
		}
		locals = nil
		return result
	}
	defer func() { _ = releaseLocals() }()
	pinLocal := func(h ast.ASTHandle) error {
		ref, err := b.transformer.manager.NewRef(h)
		if err == nil {
			locals = append(locals, ref)
		}
		return err
	}
	guarded := make([]ast.ASTHandle, 0, len(live)+1)
	tags := make([]ast.ASTHandle, 0, len(live))
	for _, rule := range live {
		if err = b.tagRule(rule); err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		tags = append(tags, rule.tag)
		if len(live) == 1 {
			guarded = append(guarded, rule.transition)
			continue
		}
		implication, x := b.transformer.manager.MkImplies(rule.tag, rule.transition)
		if x != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, x
		}
		if x = pinLocal(implication); x != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, x
		}
		guarded = append(guarded, implication)
	}
	selection, err := b.transformer.manager.MkOrArray(tags)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = pinLocal(selection); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	guarded = append(guarded, selection)
	transition, err := b.mkAnd(guarded)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	// The m_transition slot is mk_and's pinned result.
	transitionSlot := b.references[len(b.references)-1]
	if err = releaseLocals(); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if transition, err = b.rewrite(transition); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	// rw(m_transition) replaces the expr_ref's value.
	if err = b.transformer.manager.SetRef(transitionSlot, transition); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if initial, err = b.rewrite(initial); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	return initial, transition, b.setInitial(initial)
}

// InitializeRules converts rules with this predicate as their head into the
// ground transition system consumed by the frame engine. Every predicate used
// in a rule atom must have an entry in transformers.
func (p *PredicateTransformer) InitializeRules(transformers map[ast.ASTHandle]*PredicateTransformer, rules []*base.Rule) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if p.initialized || p.rulesReady || transformers[p.head] != p {
		return ast.ErrASTDomain
	}
	theory, err := rewriter.NewTheoryRewriter(p.manager, p.owner.contexts.config.Params, p.owner.contexts.config.Globals, nil)
	if err != nil {
		if theory != nil {
			err = errors.Join(err, theory.Close())
		}
		return err
	}
	defer func() {
		if theory != nil {
			resultErr = errors.Join(resultErr, theory.Close())
		}
	}()
	builder := &predicateRuleBuilder{transformer: p, all: transformers, theory: theory}
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(builder.rules) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, builder.rules[index].rule.Close())
		}
		for index := len(builder.references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&builder.references[index]))
		}
	}()
	// initialize: m_initial_state = m.mk_false(), extended by each fact rule.
	initial, err := p.manager.MkFalse()
	if err != nil {
		return err
	}
	for _, rule := range rules {
		// init_rule's th_rewriter rw(m) lives until the rule is recorded.
		ruleTheory, x := rewriter.NewTheoryRewriter(p.manager, p.owner.contexts.config.Params, p.owner.contexts.config.Globals, nil)
		if x != nil {
			if ruleTheory != nil {
				x = errors.Join(x, ruleTheory.Close())
			}
			return x
		}
		builder.ruleTheory = ruleTheory
		normalized, x := builder.normalizeRule(rule)
		if x == nil {
			builder.rules = append(builder.rules, normalized)
			initial, x = builder.extendInitial(initial, normalized)
		}
		builder.ruleTheory = nil
		x = errors.Join(x, ruleTheory.Close())
		if builder.finishRule != nil {
			x = errors.Join(x, builder.finishRule(x))
		}
		if x != nil {
			return x
		}
	}
	initial, transition, err := builder.buildSystem(initial)
	if err != nil {
		return err
	}
	// initialize's local th_rewriter lives until initialize returns, after
	// the transition and initial state are asserted; the deferred Close
	// releases it (and its m_pr) then.
	if err = p.InitializeSystem(initial, transition); err != nil {
		return err
	}
	p.references = append(p.references, builder.references...)
	p.rules = builder.rules
	p.rulesReady = true
	for _, rule := range p.rules {
		for _, predecessor := range rule.predecessors {
			predecessor.addUse(p)
		}
	}
	committed = true
	return nil
}

func (p *PredicateTransformer) addUse(user *PredicateTransformer) {
	for _, prior := range p.uses {
		if prior == user {
			return
		}
	}
	p.uses = append(p.uses, user)
}

func (p *PredicateTransformer) makeAssumptions(head ast.ASTHandle, formula ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, 0)
	for _, rule := range p.rules {
		if rule.tag.IsNull() {
			continue
		}
		for index, predecessor := range rule.predecessors {
			if predecessor.head != head {
				continue
			}
			implication, err := p.manager.MkImplies(rule.tag, formula)
			if err != nil {
				return nil, err
			}
			converted, err := p.owner.FormulaNewToOld(implication, uint32(index), false)
			if err != nil {
				return nil, err
			}
			result = append(result, converted)
		}
	}
	return result, nil
}

func (p *PredicateTransformer) addChildProperty(child *PredicateTransformer, lemma ast.ASTHandle, level uint32) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if level != InfinityLevel {
		if err := p.EnsureLevel(level); err != nil {
			return err
		}
	}
	// add_child_property's local fmls vector owns the assumptions; the
	// solver keeps what it needs and the vector releases forward on return.
	formulas, err := p.makeAssumptionRefs(child.head, lemma)
	defer func() {
		for i := range formulas {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&formulas[i]))
		}
	}()
	if err != nil {
		return err
	}
	for _, ref := range formulas {
		formula, x := p.manager.RefGet(ref)
		if x != nil {
			return x
		}
		if level == InfinityLevel {
			err = p.solver.AddFormula(formula)
		} else {
			err = p.solver.AddLevelFormula(formula, level)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// makeAssumptionRefs is mk_assumptions into a caller-owned vector. Its
// expr_ref tmp1 holds each implication until the next one or the return.
func (p *PredicateTransformer) makeAssumptionRefs(head ast.ASTHandle, formula ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	result := make([]ast.ASTRef, 0)
	tmp1, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&tmp1)) }()
	for _, rule := range p.rules {
		if rule.tag.IsNull() {
			continue
		}
		for index, predecessor := range rule.predecessors {
			if predecessor.head != head {
				continue
			}
			implication, err := p.manager.MkImplies(rule.tag, formula)
			if err != nil {
				return result, err
			}
			if err = p.manager.SetRef(tmp1, implication); err != nil {
				return result, err
			}
			converted, err := p.owner.FormulaNewToOldRef(implication, uint32(index), false)
			if err != nil {
				return result, err
			}
			result = append(result, converted)
		}
	}
	return result, nil
}

// Premises returns this predicate's system formulas plus every predecessor's
// frame formulas converted into the rule-tail state index.
func (p *PredicateTransformer) Premises(level uint32) ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if !p.rulesReady || !p.initialized || level == InfinityLevel {
		return nil, ast.ErrASTDomain
	}
	background, err := p.owner.Background()
	if err != nil {
		return nil, err
	}
	result := []ast.ASTHandle{background}
	if level == 0 {
		result = append(result, p.initialState)
	} else {
		result = append(result, p.transition)
	}
	for _, rule := range p.rules {
		for index, predecessor := range rule.predecessors {
			formula, skip, x := func() (converted ast.ASTHandle, skip bool, resultErr error) {
				ref, err := predecessor.FormulasRef(level, false)
				if err != nil {
					return converted, false, err
				}
				defer func() { resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&ref)) }()
				formula, err := p.manager.RefGet(ref)
				if err != nil {
					return converted, false, err
				}
				if p.manager.IsTrue(formula) {
					return converted, true, nil
				}
				converted, err = p.owner.FormulaNewToOld(formula, uint32(index), true)
				return converted, false, err
			}()
			if x != nil {
				return nil, x
			}
			if !skip {
				result = append(result, formula)
			}
		}
	}
	return result, nil
}

// PropagationFormula builds the source PDR premise used to validate frame
// lemmas, including tag-guarded predecessor frames.
func (p *PredicateTransformer) PropagationFormula(level uint32) (result ast.ASTHandle, resultErr error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if !p.rulesReady || level == InfinityLevel {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	conjunction := make([]ast.ASTHandle, 0)
	var formulaRefs []ast.ASTRef
	defer func() {
		for i := len(formulaRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&formulaRefs[i]))
		}
	}()
	if level == 0 {
		conjunction = append(conjunction, p.initialState)
	} else {
		conjunction = append(conjunction, p.transition)
	}
	ref, err := p.FormulasRef(level, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formulaRefs = append(formulaRefs, ref)
	formulas, err := p.manager.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction = append(conjunction, formulas)
	if level > 0 {
		for _, rule := range p.rules {
			if rule.tag.IsNull() {
				continue
			}
			for index, predecessor := range rule.predecessors {
				ref, x := predecessor.FormulasRef(level-1, false)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				formulaRefs = append(formulaRefs, ref)
				formula, x := p.manager.RefGet(ref)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				formula, x = p.owner.FormulaNewToOld(formula, uint32(index), false)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				guarded, x := p.manager.MkImplies(rule.tag, formula)
				if x != nil {
					return ast.ASTHandle{}, x
				}
				conjunction = append(conjunction, guarded)
			}
		}
	}
	result, err = p.manager.MkAndArray(conjunction)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	resultRef, err := p.manager.NewRef(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	p.owner.notAndBorrowed = append(p.owner.notAndBorrowed, resultRef)
	return result, nil
}

// inductiveAssumptionsRef builds the source check_inductive formula. Its
// output owns the conjunction after temporary state roots are released.
func (p *PredicateTransformer) inductiveAssumptionsRef(literals []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&roots[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&result))
		}
	}()
	cube, err := p.owner.AndRef(literals)
	if err != nil {
		return result, err
	}
	roots = append(roots, cube)
	states, err := p.manager.RefGet(cube)
	if err != nil {
		return result, err
	}
	states, err = p.manager.MkNot(states)
	if err != nil {
		return result, err
	}
	negated, err := p.manager.NewRef(states)
	if err != nil {
		return result, err
	}
	roots = append(roots, negated)
	assumptions, err := p.makeAssumptions(p.head, states)
	if err != nil {
		return result, err
	}
	return p.owner.AndRef(assumptions)
}

// CheckInductive tests whether the conjunction of literals is inductive at a
// frame. On success it returns the subset core selected by the SMT solver.
// Returned handles remain valid until the next check on this transformer.
func (p *PredicateTransformer) CheckInductive(level uint32, literals []ast.ASTHandle) (inductive bool, assumesLevel bool, core []ast.ASTHandle, resultErr error) {
	if err := p.open(); err != nil {
		return false, false, nil, err
	}
	if level == InfinityLevel {
		return false, false, nil, ast.ErrASTDomain
	}
	// The native expr_ref_vector keeps every input literal alive throughout
	// formula construction and the subsequent solver check.
	var literalRefs []ast.ASTRef
	defer func() {
		for i := len(literalRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&literalRefs[i]))
		}
	}()
	for _, literal := range literals {
		ref, err := p.manager.NewRef(literal)
		if err != nil {
			return false, false, nil, err
		}
		literalRefs = append(literalRefs, ref)
	}
	ref, err := p.inductiveAssumptionsRef(literals)
	if err != nil {
		return false, false, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.owner.releaseNotAndRef(&ref)) }()
	formula, err := p.manager.RefGet(ref)
	if err != nil {
		return false, false, nil, err
	}
	if err = p.solver.SetSubsetBasedCore(true); err != nil {
		return false, false, nil, err
	}
	result, err := p.solver.CheckAtLevelAndFormula(level, literals, formula, true, false)
	if err == nil && result.Status == lbool.False {
		// The solver's core borrows input terms. Keep them alive until the
		// next check clears coreReferences, as this method promises callers.
		p.solver.coreReferences = append(p.solver.coreReferences, literalRefs...)
		literalRefs = nil
	}
	return result.Status == lbool.False, result.AssumesLevel, result.Core, err
}

func (p *PredicateTransformer) Rules() ([]*base.Rule, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := make([]*base.Rule, 0, len(p.rules))
	for _, rule := range p.rules {
		owned, err := rule.rule.Retain()
		if err != nil {
			for _, prior := range result {
				_ = prior.Close()
			}
			return nil, err
		}
		result = append(result, owned)
	}
	return result, nil
}
