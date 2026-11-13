package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, bmc::linear naming, rule-variable
// substitution and compile.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (c *Compiler) linearName(predicate ast.ASTHandle, suffix string, sort ast.ASTHandle) (ast.ASTRef, error) {
	m := c.manager
	name, err := m.GetASTName(predicate)
	if err != nil {
		return ast.ASTRef{}, err
	}
	text, err := m.Symbols.Display(name)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sym, err := m.Symbols.Intern(text + suffix)
	if err != nil {
		return ast.ASTRef{}, err
	}
	value, err := m.MkConst(sym, sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(value)
}

// LinearQuery constructs the nullary Boolean predicate used to query a level.
func (c *Compiler) LinearQuery(predicate ast.ASTHandle, level uint32) (ast.ASTRef, error) {
	if c == nil || c.manager == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.linearName(predicate, fmt.Sprintf("#%d", level), boolean)
}

func (c *Compiler) linearArgument(predicate ast.ASTHandle, column, level uint32) (ast.ASTRef, error) {
	sort, err := c.manager.GetDomain(predicate, column)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.linearName(predicate, fmt.Sprintf("#%d_%d", level, column), sort)
}

func (c *Compiler) linearRule(predicate ast.ASTHandle, index, level uint32) (ast.ASTRef, error) {
	m := c.manager
	name, err := m.GetASTName(predicate)
	if err != nil {
		return ast.ASTRef{}, err
	}
	text, err := m.Symbols.Display(name)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sym, err := m.Symbols.Intern(fmt.Sprintf("rule:%s#%d_%d", text, level, index))
	if err != nil {
		return ast.ASTRef{}, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTRef{}, err
	}
	value, err := m.MkConst(sym, boolean)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(value)
}

// CompileLinear returns owning references to the source linear encoding at a
// single level. It rejects rules that require a different BMC compiler.
func (c *Compiler) CompileLinear(rules *base.RuleSet, level uint32) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil {
		return nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil {
			resultErr = disposeASTRefs(c.manager, result, resultErr)
			result = nil
		}
	}()
	resultErr = c.compileLinearEach(rules, level, func(value ast.ASTHandle) error {
		ref, err := c.manager.NewRef(value)
		if err == nil {
			result = append(result, ref)
		}
		return err
	})
	return result, resultErr
}

// compileLinearEach is linear::compile: it walks the grouped rules in the
// rule set's predicate-table order and hands each formula to emit (the
// source b.assert_expr) as soon as it is built. emit borrows the formula.
func (c *Compiler) compileLinearEach(rules *base.RuleSet, level uint32, emit func(ast.ASTHandle) error) (resultErr error) {
	if c == nil || c.manager == nil || rules == nil {
		return ast.ErrASTHandle
	}
	linear, err := IsLinear(rules)
	if err != nil {
		return err
	}
	if !linear {
		return ast.ErrASTDomain
	}
	predicates, err := rules.GroupedPredicates()
	if err != nil {
		return err
	}
	for _, predicate := range predicates {
		if err := c.compileLinearGroup(rules, predicate, level, emit); err != nil {
			return err
		}
	}
	return nil
}

// linearGroupLocals are linear::compile's per-group locals, released in
// reverse declaration order: tmp, rule_body, conjs, sub, rules, level_pred.
type linearGroupLocals struct {
	manager           *ast.ASTManager
	levelPred         ast.ASTRef
	rules, sub, conjs []ast.ASTRef
	ruleBody, tmp     ast.ASTRef
}

// assign is expr_ref assignment: the new value is retained before the old
// one is released.
func (l *linearGroupLocals) assign(ref *ast.ASTRef, value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if *ref == (ast.ASTRef{}) {
		*ref, err = l.manager.NewRef(value)
		return value, err
	}
	return value, l.manager.SetRef(*ref, value)
}

// push is expr_ref_vector::push_back.
func (l *linearGroupLocals) push(vector *[]ast.ASTRef, value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := l.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*vector = append(*vector, ref)
	return value, nil
}

// reset is expr_ref_vector::reset, releasing front to back.
func (l *linearGroupLocals) reset(vector *[]ast.ASTRef) error {
	for len(*vector) != 0 {
		if (*vector)[0] != (ast.ASTRef{}) {
			if err := l.manager.DelRef(&(*vector)[0]); err != nil {
				return err
			}
		}
		*vector = (*vector)[1:]
	}
	*vector = nil
	return nil
}

func (l *linearGroupLocals) release(ref *ast.ASTRef) error {
	if *ref == (ast.ASTRef{}) {
		return nil
	}
	return l.manager.DelRef(ref)
}

func (l *linearGroupLocals) close() error {
	if err := l.release(&l.tmp); err != nil {
		return err
	}
	if err := l.release(&l.ruleBody); err != nil {
		return err
	}
	for _, vector := range []*[]ast.ASTRef{&l.conjs, &l.sub, &l.rules} {
		if err := l.reset(vector); err != nil {
			return err
		}
	}
	return l.release(&l.levelPred)
}

// compileLinearGroup is one iteration of linear::compile over a predicate
// and its rules. emit is b.assert_expr and borrows its argument.
func (c *Compiler) compileLinearGroup(rules *base.RuleSet, predicate ast.ASTHandle, level uint32, emit func(ast.ASTHandle) error) (resultErr error) {
	m := c.manager
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = disposeRuleOwners(m, group, resultErr)
	}()
	l := &linearGroupLocals{manager: m}
	defer func() {
		if cause := l.close(); cause != nil {
			resultErr = errors.Join(resultErr, cause)
		}
	}()
	l.levelPred, err = c.LinearQuery(predicate, level)
	if err != nil {
		return err
	}
	levelPred, err := m.RefGet(l.levelPred)
	if err != nil {
		return err
	}
	for i, rule := range group {
		if err = l.reset(&l.sub); err != nil {
			return err
		}
		if err = l.reset(&l.conjs); err != nil {
			return err
		}
		ruleRef, err := c.linearRule(predicate, uint32(i), level)
		if err != nil {
			return err
		}
		l.rules = append(l.rules, ruleRef)
		ruleAtom, err := m.RefGet(ruleRef)
		if err != nil {
			return err
		}
		tailCount, err := rule.UninterpretedTailSize()
		if err != nil {
			return err
		}
		if level == 0 && tailCount > 0 {
			not, err := m.MkNot(ruleAtom)
			value, err := l.assign(&l.tmp, not, err)
			if err == nil {
				err = emit(value)
			}
			if err != nil {
				return err
			}
			continue
		}
		if err = c.compileLinearRuleBody(l, rule, predicate, uint32(i), level); err != nil {
			return err
		}
		body, err := m.RefGet(l.ruleBody)
		if err != nil {
			return err
		}
		implication, err := m.MkImplies(ruleAtom, body)
		value, err := l.assign(&l.ruleBody, implication, err)
		if err == nil {
			err = emit(value)
		}
		if err != nil {
			return err
		}
	}
	alternatives := make([]ast.ASTHandle, len(l.rules))
	for i := range l.rules {
		if alternatives[i], err = m.RefGet(l.rules[i]); err != nil {
			return err
		}
	}
	orRef, err := c.simplifyBoolean(false, alternatives)
	if err != nil {
		return err
	}
	or, err := m.RefGet(orRef)
	if err == nil {
		_, err = l.assign(&l.tmp, or, nil)
	}
	if err = errors.Join(err, m.DelRef(&orRef)); err != nil {
		return err
	}
	or, err = m.RefGet(l.tmp)
	if err != nil {
		return err
	}
	implication, err := m.MkImplies(levelPred, or)
	value, err := l.assign(&l.tmp, implication, err)
	if err != nil {
		return err
	}
	return emit(value)
}

// compileLinearRuleBody fills sub and conjs for one rule and leaves the
// bool_rewriter conjunction in rule_body; tmp holds the last var_subst result.
func (c *Compiler) compileLinearRuleBody(l *linearGroupLocals, rule *base.Rule, predicate ast.ASTHandle, ruleIndex, level uint32) (resultErr error) {
	m := c.manager
	owner := &linearCompileRuleOwner{manager: m}
	defer func() { resultErr = finishLinearCompileRule(owner, resultErr) }()
	bindingRefs, err := c.linearRuleBindings(rule, predicate, ruleIndex, level)
	if err != nil {
		return err
	}
	l.sub = bindingRefs
	bindings := make([]ast.ASTHandle, len(bindingRefs))
	for i, ref := range bindingRefs {
		if ref == (ast.ASTRef{}) {
			continue
		}
		if bindings[i], err = m.RefGet(ref); err != nil {
			return err
		}
	}
	head, err := rule.Head()
	if err != nil {
		return err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	// var_subst vs(m, false) lives for this rule.
	if err = owner.newSubstitution(false); err != nil {
		return err
	}
	substitute := func(expression ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		err = owner.sub.Substitute(expression, bindings, ref)
		var value ast.ASTHandle
		if err == nil {
			value, err = m.RefGet(ref)
		}
		if err == nil {
			_, err = l.assign(&l.tmp, value, nil)
		}
		return value, errors.Join(err, m.DelRef(&ref))
	}
	addEqualities := func(application, declaration ast.ASTHandle, atLevel uint32) error {
		arity, err := m.GetArity(declaration)
		if err != nil {
			return err
		}
		for i := uint32(0); i < arity; i++ {
			arg, err := m.GetArg(application, i)
			if err != nil {
				return err
			}
			value, err := substitute(arg)
			if err != nil {
				return err
			}
			levelArg, err := c.linearArgument(declaration, i, atLevel)
			if err != nil {
				return err
			}
			levelValue, err := m.RefGet(levelArg)
			if err == nil {
				var eq ast.ASTHandle
				eq, err = m.MkEq(value, levelValue)
				_, err = l.push(&l.conjs, eq, err)
			}
			if err = errors.Join(err, m.DelRef(&levelArg)); err != nil {
				return err
			}
		}
		return nil
	}
	if err = addEqualities(head, predicate, level); err != nil {
		return err
	}
	for i := 0; i < uninterpreted; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		decl, err := rule.TailDeclaration(i)
		if err != nil {
			return err
		}
		if err = addEqualities(tail, decl, level-1); err != nil {
			return err
		}
		levelRef, err := c.LinearQuery(decl, level-1)
		if err != nil {
			return err
		}
		levelValue, err := m.RefGet(levelRef)
		if err == nil {
			_, err = l.push(&l.conjs, levelValue, nil)
		}
		if err = errors.Join(err, m.DelRef(&levelRef)); err != nil {
			return err
		}
	}
	total, err := rule.TailSize()
	if err != nil {
		return err
	}
	for i := uninterpreted; i < total; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return err
		}
		value, err := substitute(tail)
		if err != nil {
			return err
		}
		if _, err = l.push(&l.conjs, value, nil); err != nil {
			return err
		}
	}
	conditions := make([]ast.ASTHandle, len(l.conjs))
	for i := range l.conjs {
		if conditions[i], err = m.RefGet(l.conjs[i]); err != nil {
			return err
		}
	}
	bodyRef, err := c.simplifyBoolean(true, conditions)
	if err != nil {
		return err
	}
	body, err := m.RefGet(bodyRef)
	if err == nil {
		_, err = l.assign(&l.ruleBody, body, nil)
	}
	return errors.Join(err, m.DelRef(&bodyRef))
}
