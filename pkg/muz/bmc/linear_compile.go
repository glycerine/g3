package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, bmc::linear naming, rule-variable
// substitution and compile.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
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
	if c == nil || c.manager == nil || rules == nil {
		return nil, ast.ErrASTHandle
	}
	linear, err := IsLinear(rules)
	if err != nil {
		return nil, err
	}
	if !linear {
		return nil, ast.ErrASTDomain
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			for i := len(result) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, m.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	count, err := rules.Len()
	if err != nil {
		return result, err
	}
	seen := make(map[ast.ASTHandle]struct{})
	var predicates []ast.ASTHandle
	for i := 0; i < count; i++ {
		rule, err := rules.Rule(i)
		if err != nil {
			return result, err
		}
		decl, e := rule.HeadDeclaration()
		e = errors.Join(e, rule.Close())
		if e != nil {
			return result, e
		}
		if _, found := seen[decl]; !found {
			seen[decl] = struct{}{}
			predicates = append(predicates, decl)
		}
	}
	for _, predicate := range predicates {
		if err := c.compileLinearGroup(rules, predicate, level, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *Compiler) compileLinearGroup(rules *base.RuleSet, predicate ast.ASTHandle, level uint32, result *[]ast.ASTRef) (resultErr error) {
	m := c.manager
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(group) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, group[i].Close())
		}
	}()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	holdRef := func(ref ast.ASTRef, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		return holdRef(ref, err)
	}
	levelPred, err := holdRef(c.LinearQuery(predicate, level))
	if err != nil {
		return err
	}
	var alternatives []ast.ASTHandle
	for i, rule := range group {
		ruleAtom, err := holdRef(c.linearRule(predicate, uint32(i), level))
		if err != nil {
			return err
		}
		alternatives = append(alternatives, ruleAtom)
		tailCount, err := rule.UninterpretedTailSize()
		if err != nil {
			return err
		}
		if level == 0 && tailCount > 0 {
			not, err := hold(m.MkNot(ruleAtom))
			if err != nil {
				return err
			}
			ref, err := m.NewRef(not)
			if err != nil {
				return err
			}
			*result = append(*result, ref)
			continue
		}
		body, err := c.compileLinearRule(rule, predicate, uint32(i), level)
		if err != nil {
			return err
		}
		refs = append(refs, body)
		bodyValue, err := m.RefGet(body)
		if err != nil {
			return err
		}
		implication, err := m.MkImplies(ruleAtom, bodyValue)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(implication)
		if err != nil {
			return err
		}
		*result = append(*result, ref)
	}
	orRef, err := c.simplifyBoolean(false, alternatives)
	if err != nil {
		return err
	}
	refs = append(refs, orRef)
	or, err := m.RefGet(orRef)
	if err != nil {
		return err
	}
	implication, err := m.MkImplies(levelPred, or)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(implication)
	if err != nil {
		return err
	}
	*result = append(*result, ref)
	return nil
}

func (c *Compiler) compileLinearRule(rule *base.Rule, predicate ast.ASTHandle, ruleIndex, level uint32) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	holdRef := func(ref ast.ASTRef, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		return holdRef(ref, err)
	}
	sorts, err := rule.VariableSorts()
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, len(sorts))
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	bindArguments := func(application, declaration ast.ASTHandle, atLevel uint32) error {
		arity, err := m.GetArity(declaration)
		if err != nil {
			return err
		}
		for i := uint32(0); i < arity; i++ {
			arg, err := m.GetArg(application, i)
			if err != nil {
				return err
			}
			kind, err := m.GetKind(arg)
			if err != nil {
				return err
			}
			if kind != ast.ASTVar {
				continue
			}
			index, err := m.GetVarIndex(arg)
			if err != nil || int(index) >= len(bindings) {
				return errors.Join(ast.ErrASTDomain, err)
			}
			if bindings[index].IsNull() {
				bindings[index], err = holdRef(c.linearArgument(declaration, i, atLevel))
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = bindArguments(head, predicate, level); err != nil {
		return result, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	for i := 0; i < uninterpreted; i++ {
		if level == 0 {
			return result, ast.ErrASTDomain
		}
		tail, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		decl, err := rule.TailDeclaration(i)
		if err != nil {
			return result, err
		}
		if err = bindArguments(tail, decl, level-1); err != nil {
			return result, err
		}
	}
	var fresh uint32
	for i, sort := range sorts {
		if sort.IsNull() || !bindings[i].IsNull() {
			continue
		}
		bindings[i], err = holdRef(c.linearName(predicate, fmt.Sprintf("#%d_%d_%d", level, ruleIndex, fresh), sort))
		if err != nil {
			return result, err
		}
		fresh++
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	substitute := func(expression ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		if err = sub.Substitute(expression, bindings, ref); err != nil {
			return ast.ASTHandle{}, err
		}
		return m.RefGet(ref)
	}
	var conditions []ast.ASTHandle
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
			levelArg, err := holdRef(c.linearArgument(declaration, i, atLevel))
			if err != nil {
				return err
			}
			eq, err := hold(m.MkEq(value, levelArg))
			if err != nil {
				return err
			}
			conditions = append(conditions, eq)
		}
		return nil
	}
	if err = addEqualities(head, predicate, level); err != nil {
		return result, err
	}
	for i := 0; i < uninterpreted; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		decl, err := rule.TailDeclaration(i)
		if err != nil {
			return result, err
		}
		if err = addEqualities(tail, decl, level-1); err != nil {
			return result, err
		}
		value, err := holdRef(c.LinearQuery(decl, level-1))
		if err != nil {
			return result, err
		}
		conditions = append(conditions, value)
	}
	total, err := rule.TailSize()
	if err != nil {
		return result, err
	}
	for i := uninterpreted; i < total; i++ {
		tail, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		value, err := substitute(tail)
		if err != nil {
			return result, err
		}
		conditions = append(conditions, value)
	}
	return c.simplifyBoolean(true, conditions)
}
