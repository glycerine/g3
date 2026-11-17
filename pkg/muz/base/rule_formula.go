package base

// Source: src/muz/base/dl_rule.cpp extract_horn, hoist_compound_predicates,
// hoist_compound, mk_negations, ensure_app, and the already-Horn fast path of
// mk_rule/mk_horn_rule.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (m *RuleManager) pinTemporary(temporaries *[]ast.ASTRef, handle ast.ASTHandle) error {
	reference, err := m.manager.NewRef(handle)
	if err != nil {
		return err
	}
	*temporaries = append(*temporaries, reference)
	return nil
}

func (m *RuleManager) ensureBooleanApplication(expression ast.ASTHandle, temporaries *[]ast.ASTRef) (ast.ASTHandle, error) {
	kind, err := m.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTApp {
		return expression, nil
	}
	sort, err := m.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := m.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort != boolean {
		return ast.ASTHandle{}, fmt.Errorf("muz/base: expected Boolean rule expression")
	}
	truth, err := m.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	root, err := m.manager.NewRef(ast.ASTHandle{})
	if root != (ast.ASTRef{}) {
		*temporaries = append(*temporaries, root)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	application, err := m.manager.MkEq(expression, truth)
	if !application.IsNull() {
		err = errors.Join(err, m.manager.SetRef(root, application))
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return application, nil
}

func (m *RuleManager) containsRegisteredPredicate(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := m.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			declaration, e := m.manager.GetDecl(current)
			if e != nil {
				return false, e
			}
			if _, found := m.predicates[declaration]; found {
				return true, nil
			}
			count, e := m.manager.GetNumArgs(current)
			if e != nil {
				return false, e
			}
			for i := count; i > 0; i-- {
				argument, x := m.manager.GetArg(current, i-1)
				if x != nil {
					return false, x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, e := m.manager.GetQuantifierBody(current)
			if e != nil {
				return false, e
			}
			stack = append(stack, body)
		}
	}
	return false, nil
}

// ensureApp is ensure_app(e): the returned app_ref is a temporary the
// caller releases.
func (m *RuleManager) ensureApp(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	var temporaries []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(m.manager, temporaries)) }()
	application, err := m.ensureBooleanApplication(expression, &temporaries)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(application)
}

// assignHorn is an app_ref assignment: the new term is held before the
// previous one is released.
func (m *RuleManager) assignHorn(slot *ast.ASTRef, h ast.ASTHandle) error {
	fresh, err := m.manager.NewRef(h)
	if err != nil {
		return err
	}
	previous := *slot
	*slot = fresh
	if previous != (ast.ASTRef{}) {
		return m.manager.DelRef(&previous)
	}
	return nil
}

// assignHornRef is fml = <temporary app_ref>: the assignment, then the
// temporary's release.
func (m *RuleManager) assignHornRef(slot *ast.ASTRef, temporary ast.ASTRef) error {
	h, err := m.manager.RefGet(temporary)
	if err == nil {
		err = m.assignHorn(slot, h)
	}
	return errors.Join(err, m.manager.DelRef(&temporary))
}

// hoistCompound is rule_manager::hoist_compound(num_bound, fml, body): fml
// is m_head or hoist_compound_predicates' local b, body is m_body, and the
// rebuilt arguments go through the member m_args.
func (m *RuleManager) hoistCompound(next *uint32, fml *ast.ASTRef) error {
	expression, err := m.manager.RefGet(*fml)
	if err != nil {
		return err
	}
	operator, err := m.manager.GetDeclKind(expression)
	if err != nil {
		return err
	}
	if operator == ast.ASTOpNot {
		argument, e := m.manager.GetArg(expression, 0)
		if e != nil {
			return e
		}
		// fml = ensure_app(not_fml), including Boolean variables produced
		// by bit blasting.
		application, e := m.ensureApp(argument)
		if e != nil {
			return e
		}
		if e = m.assignHornRef(fml, application); e != nil {
			return e
		}
		if e = m.hoistCompound(next, fml); e != nil {
			return e
		}
		// fml = m.mk_not(fml)
		inner, e := m.manager.RefGet(*fml)
		if e != nil {
			return e
		}
		if inner == argument {
			// mk_not finds the original negation: no new node.
			return m.assignHorn(fml, expression)
		}
		root, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		negation, e := m.manager.MkNot(inner)
		if e = errors.Join(e, m.manager.SetRef(root, negation)); e != nil {
			return errors.Join(e, m.manager.DelRef(&root))
		}
		return m.assignHornRef(fml, root)
	}
	predicate, err := m.isPredicate(expression)
	if err != nil || !predicate {
		return err
	}
	// m_args.reset()
	if err = m.releaseHornArgs(); err != nil {
		return err
	}
	count, err := m.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	arguments := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		argument, e := m.manager.GetArg(expression, i)
		if e != nil {
			return e
		}
		arguments[i] = argument
		kind, e := m.manager.GetKind(argument)
		if e != nil {
			return e
		}
		value := false
		if kind == ast.ASTApp {
			if value, e = m.manager.IsValue(argument); e != nil {
				return e
			}
		}
		if kind != ast.ASTApp || value {
			if e = m.pinTemporary(&m.hornArgs, argument); e != nil {
				return e
			}
			continue
		}
		sort, e := m.manager.GetSort(argument)
		if e != nil {
			return e
		}
		// var* v = m.mk_var(num_bound++, ...); m_args.push_back(v);
		// body.push_back(m.mk_eq(v, b));
		variableRoot, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		m.hornArgs = append(m.hornArgs, variableRoot)
		variable, e := m.manager.MkVar(*next, sort)
		if e = errors.Join(e, m.manager.SetRef(variableRoot, variable)); e != nil {
			return e
		}
		(*next)++
		arguments[i] = variable
		equalityRoot, e := m.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		m.hornBody = append(m.hornBody, equalityRoot)
		equality, e := m.manager.MkEq(variable, argument)
		if e = errors.Join(e, m.manager.SetRef(equalityRoot, equality)); e != nil {
			return e
		}
	}
	// fml = m.mk_app(fml->get_decl(), m_args.size(), m_args.c_ptr()): with
	// no argument replaced, mk_app finds fml itself and the assignment
	// changes nothing, so the lookup is skipped.
	changed := false
	for i := uint32(0); i < count && !changed; i++ {
		argument, e := m.manager.GetArg(expression, i)
		if e != nil {
			return e
		}
		changed = argument != arguments[i]
	}
	if !changed {
		return nil
	}
	declaration, err := m.manager.GetDecl(expression)
	if err != nil {
		return err
	}
	root, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	result, err := m.manager.MkApp(declaration, arguments)
	if err = errors.Join(err, m.manager.SetRef(root, result)); err != nil {
		return errors.Join(err, m.manager.DelRef(&root))
	}
	return m.assignHornRef(fml, root)
}

// NewRuleFromFormula converts an already-Horn formula into one owning Rule.
// General formulas that require HNF fresh-predicate definitions are rejected
// explicitly until the HNF transformer supplies those additional rules.
func (m *RuleManager) NewRuleFromFormula(formula ast.ASTHandle, name string) (_ *Rule, resultErr error) {
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.NewRuleFromFormulaWithSymbol(formula, label)
}

// NewRuleFromFormulaWithSymbol constructs an already-Horn source rule without
// collapsing null, empty-string, or numerical labels.
func (m *RuleManager) NewRuleFromFormulaWithSymbol(formula ast.ASTHandle, name symbol.Symbol) (_ *Rule, resultErr error) {
	return m.newRuleFromFormulaWithSymbol(formula, name, false)
}

// HNF output may still contain predicates inside interpreted tails. Source
// mk_horn_rule retains those tails for engine-specific rule-property checks.
func (m *RuleManager) newRuleFromFormulaWithSymbol(formula ast.ASTHandle, name symbol.Symbol, fromHNF bool) (_ *Rule, resultErr error) {
	if err := m.checkRuleName(name); err != nil {
		return nil, err
	}
	// mk_horn_rule: m_body.reset(), then extract_horn(fml, m_body, m_head).
	if err := m.releaseHornBody(); err != nil {
		return nil, err
	}
	kind, err := m.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTQuantifier {
		forall, e := m.manager.IsForall(formula)
		if e != nil {
			return nil, e
		}
		// extract_horn removes only a universal prefix. Other Boolean
		// quantifiers pass through ensure_app as (= quantifier true).
		if forall {
			formula, err = m.manager.GetQuantifierBody(formula)
			if err != nil {
				return nil, err
			}
		}
	}
	used, err := ast.NewASTUsedVars(m.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Collect(formula); err != nil {
		return nil, err
	}
	next := used.MaxFoundVarIndexPlusOne()
	operator := int32(-1)
	kind, err = m.manager.GetKind(formula)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTApp {
		operator, err = m.manager.GetDeclKind(formula)
		if err != nil {
			return nil, err
		}
	}
	if operator == ast.ASTOpImplies {
		antecedent, e := m.manager.GetArg(formula, 0)
		if e != nil {
			return nil, e
		}
		consequent, e := m.manager.GetArg(formula, 1)
		if e != nil {
			return nil, e
		}
		// m_args.reset(); head = ensure_app(e2); flatten_and(e1, m_args);
		// body.push_back(ensure_app(m_args[i])).
		if err = m.releaseHornArgs(); err != nil {
			return nil, err
		}
		head, e := m.ensureApp(consequent)
		if e != nil {
			return nil, e
		}
		if err = m.assignHornRef(&m.hornHead, head); err != nil {
			return nil, err
		}
		conjunction, e := m.manager.NewRef(antecedent)
		if e != nil {
			return nil, e
		}
		if m.hornArgs, err = m.manager.FlattenAndOwned([]ast.ASTRef{conjunction}); err != nil {
			return nil, err
		}
		for i := range m.hornArgs {
			conjunct, e := m.manager.RefGet(m.hornArgs[i])
			if e != nil {
				return nil, e
			}
			application, e := m.ensureApp(conjunct)
			if e != nil {
				return nil, e
			}
			m.hornBody = append(m.hornBody, application)
		}
	} else {
		head, e := m.ensureApp(formula)
		if e != nil {
			return nil, e
		}
		if err = m.assignHornRef(&m.hornHead, head); err != nil {
			return nil, err
		}
	}
	// hoist_compound_predicates(index, m_head, m_body): the head in place,
	// then each original body element through a local b that replaces
	// body[i] (releasing the old element) when it is done.
	if err = m.hoistCompound(&next, &m.hornHead); err != nil {
		return nil, err
	}
	for i, size := 0, len(m.hornBody); i < size; i++ {
		element, e := m.manager.RefGet(m.hornBody[i])
		if e != nil {
			return nil, e
		}
		b, e := m.manager.NewRef(element)
		if e != nil {
			return nil, e
		}
		e = m.hoistCompound(&next, &b)
		if e == nil {
			var h ast.ASTHandle
			if h, e = m.manager.RefGet(b); e == nil {
				e = m.assignHorn(&m.hornBody[i], h)
			}
		}
		if e = errors.Join(e, m.manager.DelRef(&b)); e != nil {
			return nil, e
		}
	}
	// mk_negations(m_body, m_neg): body[i] = e1 for a negated predicate.
	negated := make([]bool, len(m.hornBody))
	for i := range m.hornBody {
		expression, e := m.manager.RefGet(m.hornBody[i])
		if e != nil {
			return nil, e
		}
		operator, e := m.manager.GetDeclKind(expression)
		if e != nil {
			return nil, e
		}
		if operator == ast.ASTOpNot {
			argument, x := m.manager.GetArg(expression, 0)
			if x != nil {
				return nil, x
			}
			predicate, x := m.isPredicate(argument)
			if x != nil {
				return nil, x
			}
			if predicate {
				if x = m.assignHorn(&m.hornBody[i], argument); x != nil {
					return nil, x
				}
				negated[i] = true
				continue
			}
		}
		predicate, e := m.isPredicate(expression)
		if e != nil {
			return nil, e
		}
		if !predicate && !fromHNF {
			contains, x := m.containsRegisteredPredicate(expression)
			if x != nil {
				return nil, x
			}
			if contains {
				return nil, fmt.Errorf("muz/base: formula requires Horn normal-form definitions")
			}
		}
	}
	head, err := m.manager.RefGet(m.hornHead)
	if err != nil {
		return nil, err
	}
	body := make([]ast.ASTHandle, len(m.hornBody))
	for i := range m.hornBody {
		if body[i], err = m.manager.RefGet(m.hornBody[i]); err != nil {
			return nil, err
		}
	}
	return m.NewRuleWithSymbol(head, body, negated, name)
}

// ReleaseHornTemporaries drops the terms the last rule conversion keeps in
// m_body/m_head; the next conversion would release them anyway.
func (m *RuleManager) ReleaseHornTemporaries() error { return m.releaseHornTemporaries() }

// releaseHornTemporaries releases m_args, m_head, then m_body (destruction
// order).
func (m *RuleManager) releaseHornTemporaries() (result error) {
	result = m.releaseHornArgs()
	if m.hornHead != (ast.ASTRef{}) {
		result = errors.Join(result, m.manager.DelRef(&m.hornHead))
	}
	return errors.Join(result, m.releaseHornBody())
}

// releaseHornBody is m_body.reset(): forward release.
func (m *RuleManager) releaseHornBody() (result error) {
	for i := range m.hornBody {
		if m.hornBody[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.manager.DelRef(&m.hornBody[i]))
		}
	}
	m.hornBody = nil
	return result
}

// releaseHornArgs is m_args.reset(): forward release.
func (m *RuleManager) releaseHornArgs() (result error) {
	for i := range m.hornArgs {
		if m.hornArgs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.manager.DelRef(&m.hornArgs[i]))
		}
	}
	m.hornArgs = nil
	return result
}
