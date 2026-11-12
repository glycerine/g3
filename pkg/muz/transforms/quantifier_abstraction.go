package transforms

// Source: src/muz/transforms/dl_mk_quantifier_abstraction.cpp/.h retained
// array-predicate abstraction into quantified index/value Horn clauses.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type QuantifierAbstractionPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	oldToNew map[ast.ASTHandle]ast.ASTHandle
	// order lists oldToNew's keys in creation order (the model converter's
	// m_old_funcs vector).
	order  []ast.ASTHandle
	mkRule ArrayInstantiationRuleMaker
	closed bool
}

// SetRuleMaker supplies rule_manager::mk_rule.
func (p *QuantifierAbstractionPlugin) SetRuleMaker(mk ArrayInstantiationRuleMaker) { p.mkRule = mk }

// declarationSubstitution builds declare_pred's sub vector (the qa model
// converter's argument terms). Without a model converter the source drops
// it at return: arg is released first, then sub forward.
func (p *QuantifierAbstractionPlugin) declarationSubstitution(old ast.ASTHandle, arity uint32) (resultErr error) {
	var sub []ast.ASTRef
	var arg ast.ASTRef
	defer func() {
		if arg != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&arg))
		}
		for i := range sub {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&sub[i]))
		}
	}()
	assign := func(h ast.ASTHandle) error {
		fresh, err := p.manager.NewRef(h)
		if err != nil {
			return err
		}
		if arg != (ast.ASTRef{}) {
			if err = p.manager.DelRef(&arg); err != nil {
				return errors.Join(err, p.manager.DelRef(&fresh))
			}
		}
		arg = fresh
		return nil
	}
	bound := uint32(0)
	for i := uint32(0); i < arity; i++ {
		s0, err := p.manager.GetDomain(old, i)
		if err != nil {
			return err
		}
		lookahead := uint32(0)
		for s := s0; ; {
			array, e := p.arraySort(s)
			if e != nil {
				return e
			}
			if !array {
				break
			}
			n, e := p.manager.ArrayArity(s)
			if e != nil {
				return e
			}
			lookahead += n
			if s, e = p.manager.ArrayRange(s); e != nil {
				return e
			}
		}
		variable, err := p.manager.MkVar(bound+lookahead, s0)
		if err != nil {
			return err
		}
		if err = assign(variable); err != nil {
			return err
		}
		for s := s0; ; {
			array, e := p.arraySort(s)
			if e != nil {
				return e
			}
			if !array {
				break
			}
			n, e := p.manager.ArrayArity(s)
			if e != nil {
				return e
			}
			indices := make([]ast.ASTHandle, 0, n)
			for j := uint32(0); j < n; j++ {
				s1, e := p.manager.ArrayDomain(s, j)
				if e != nil {
					return e
				}
				index, e := p.manager.MkVar(bound, s1)
				if e != nil {
					return e
				}
				indices = append(indices, index)
				bound++
			}
			current, e := p.manager.RefGet(arg)
			if e != nil {
				return e
			}
			selected, e := p.selectTerm(current, indices)
			if e != nil {
				return e
			}
			if e = assign(selected); e != nil {
				return e
			}
			if s, e = p.manager.ArrayRange(s); e != nil {
				return e
			}
		}
		bound++
		current, err := p.manager.RefGet(arg)
		if err != nil {
			return err
		}
		held, err := p.manager.NewRef(current)
		if err != nil {
			return err
		}
		sub = append(sub, held)
	}
	return nil
}

func NewQuantifierAbstractionPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *QuantifierAbstractionPlugin {
	return &QuantifierAbstractionPlugin{manager: manager, rules: rules, priority: priority, oldToNew: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *QuantifierAbstractionPlugin) Priority() uint32            { return p.priority }
func (p *QuantifierAbstractionPlugin) CanDestratifyNegation() bool { return false }
func (p *QuantifierAbstractionPlugin) Cancel()                     {}

func (p *QuantifierAbstractionPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.oldToNew))
	for old, fresh := range p.oldToNew {
		result[old] = fresh
	}
	return result
}

// FreshPredicateOrder lists FreshPredicates' keys in creation order, the
// order of qa_model_converter's m_old_funcs / m_new_funcs vectors.
func (p *QuantifierAbstractionPlugin) FreshPredicateOrder() []ast.ASTHandle {
	return append([]ast.ASTHandle(nil), p.order...)
}

func (p *QuantifierAbstractionPlugin) arraySort(sort ast.ASTHandle) (bool, error) {
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	array, err := p.manager.ArrayFamilyID()
	return family == array, err
}

func (p *QuantifierAbstractionPlugin) expandedDomain(sort ast.ASTHandle) ([]ast.ASTHandle, error) {
	var domain []ast.ASTHandle
	for {
		array, err := p.arraySort(sort)
		if err != nil {
			return nil, err
		}
		if !array {
			return append(domain, sort), nil
		}
		arity, err := p.manager.ArrayArity(sort)
		if err != nil {
			return nil, err
		}
		for i := uint32(0); i < arity; i++ {
			index, getErr := p.manager.ArrayDomain(sort, i)
			if getErr != nil {
				return nil, getErr
			}
			domain = append(domain, index)
		}
		sort, err = p.manager.ArrayRange(sort)
		if err != nil {
			return nil, err
		}
	}
}

func (p *QuantifierAbstractionPlugin) declarePredicate(source, destination *base.RuleSet, old ast.ASTHandle) (ast.ASTHandle, bool, error) {
	output, err := source.IsOutputPredicate(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if output {
		return old, false, destination.InheritPredicate(source, old, old)
	}
	if fresh := p.oldToNew[old]; !fresh.IsNull() {
		return fresh, true, nil
	}
	arity, err := p.manager.GetArity(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	domain := make([]ast.ASTHandle, 0, arity)
	hasArray := false
	for i := uint32(0); i < arity; i++ {
		sort, getErr := p.manager.GetDomain(old, i)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		expanded, getErr := p.expandedDomain(sort)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		hasArray = hasArray || len(expanded) != 1 || expanded[0] != sort
		domain = append(domain, expanded...)
	}
	if !hasArray {
		return old, false, destination.InheritPredicate(source, old, old)
	}
	if err = p.declarationSubstitution(old, arity); err != nil {
		return ast.ASTHandle{}, false, err
	}
	name, err := p.manager.GetASTName(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rangeSort, err := p.manager.GetRange(old)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	fresh, err := p.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = p.rules.RegisterPredicate(fresh); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = destination.InheritPredicate(source, old, fresh); err != nil {
		return ast.ASTHandle{}, false, err
	}
	p.oldToNew[old] = fresh
	p.order = append(p.order, old)
	return fresh, true, nil
}

func (p *QuantifierAbstractionPlugin) selectTerm(array ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 1, len(indices)+1)
	arguments[0] = array
	arguments = append(arguments, indices...)
	return p.manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, arguments)
}

func (p *QuantifierAbstractionPlugin) makeHead(source, destination *base.RuleSet, application ast.ASTHandle, next uint32) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, changed, err := p.declarePredicate(source, destination, old)
	if err != nil || !changed {
		return application, err
	}
	count, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(application, i)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		for {
			array, getErr := p.arraySort(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			if !array {
				break
			}
			arity, getErr := p.manager.ArrayArity(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, indexErr := p.manager.ArrayDomain(sort, j)
				if indexErr != nil {
					return ast.ASTHandle{}, indexErr
				}
				indices[j], indexErr = p.manager.MkVar(next, indexSort)
				if indexErr != nil {
					return ast.ASTHandle{}, indexErr
				}
				next++
				arguments = append(arguments, indices[j])
			}
			argument, getErr = p.selectTerm(argument, indices)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
			sort, getErr = p.manager.ArrayRange(sort)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
		}
		arguments = append(arguments, argument)
	}
	return p.manager.MkApp(fresh, arguments)
}

func (p *QuantifierAbstractionPlugin) makeTail(source, destination *base.RuleSet, application ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTRef{}, err
	}
	fresh, changed, err := p.declarePredicate(source, destination, old)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !changed {
		return p.manager.NewRef(application)
	}
	oldArity, err := p.manager.GetArity(old)
	if err != nil {
		return ast.ASTRef{}, err
	}
	newArity, err := p.manager.GetArity(fresh)
	if err != nil || newArity <= oldArity {
		return ast.ASTRef{}, errors.Join(err, ast.ErrASTDomain)
	}
	shift, err := astrewriter.NewVarShifter(p.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, shift.Close()) }()
	shiftedRef, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&shiftedRef)) }()
	if err = shift.ShiftBy(application, newArity-oldArity, shiftedRef); err != nil {
		return ast.ASTRef{}, err
	}
	shifted, err := p.manager.RefGet(shiftedRef)
	if err != nil {
		return ast.ASTRef{}, err
	}
	count, err := p.manager.GetNumArgs(shifted)
	if err != nil {
		return ast.ASTRef{}, err
	}
	arguments := make([]ast.ASTHandle, 0)
	var boundSorts []ast.ASTHandle
	var names []usymbol.Symbol
	var triggers []ast.ASTHandle
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(shifted, i)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		sort, getErr := p.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		arrayArgument := false
		for {
			array, getErr := p.arraySort(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			if !array {
				break
			}
			arrayArgument = true
			arity, getErr := p.manager.ArrayArity(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			indices := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				indexSort, indexErr := p.manager.ArrayDomain(sort, j)
				if indexErr != nil {
					return ast.ASTRef{}, indexErr
				}
				indices[j], indexErr = p.manager.MkVar(uint32(len(boundSorts)), indexSort)
				if indexErr != nil {
					return ast.ASTRef{}, indexErr
				}
				boundSorts = append(boundSorts, indexSort)
				names = append(names, usymbol.Numerical(uint32(len(names))))
				arguments = append(arguments, indices[j])
			}
			argument, getErr = p.selectTerm(argument, indices)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			sort, getErr = p.manager.ArrayRange(sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
		}
		if arrayArgument {
			triggers = append(triggers, argument)
		}
		arguments = append(arguments, argument)
	}
	// m.mk_pattern(pats) precedes m.mk_app(new_p, args).
	annotation, err := p.manager.MkPattern(triggers)
	if err != nil {
		return ast.ASTRef{}, err
	}
	body, err := p.manager.MkApp(fresh, arguments)
	if err != nil {
		return ast.ASTRef{}, err
	}
	quantified, err := p.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: boundSorts, Names: names, Body: body, Weight: 1,
		Patterns: []ast.ASTHandle{annotation},
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	wrapper, err := p.manager.MkEq(quantified, truth)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(wrapper)
}

// qaLoopLocals are operator()'s tail, head and fml, which live across the
// rule loop: each is released when reassigned, and at exit in reverse
// declaration order (fml, head, tail).
type qaLoopLocals struct {
	tail []ast.ASTRef
	head ast.ASTRef
	fml  ast.ASTRef
}

func (p *QuantifierAbstractionPlugin) resetTail(l *qaLoopLocals) (result error) {
	for i := range l.tail {
		result = errors.Join(result, p.manager.DelRef(&l.tail[i]))
	}
	l.tail = l.tail[:0]
	return result
}

func (p *QuantifierAbstractionPlugin) assign(slot *ast.ASTRef, h ast.ASTHandle) error {
	fresh, err := p.manager.NewRef(h)
	if err != nil {
		return err
	}
	if *slot != (ast.ASTRef{}) {
		if err = p.manager.DelRef(slot); err != nil {
			return errors.Join(err, p.manager.DelRef(&fresh))
		}
	}
	*slot = fresh
	return nil
}

func (p *QuantifierAbstractionPlugin) closeLoopLocals(l *qaLoopLocals) (result error) {
	if l.fml != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&l.fml))
	}
	if l.head != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&l.head))
	}
	return errors.Join(result, p.resetTail(l))
}

func (p *QuantifierAbstractionPlugin) transformRule(source, destination *base.RuleSet, rule *base.Rule, l *qaLoopLocals) error {
	if err := p.resetTail(l); err != nil {
		return err
	}
	variables, err := ruleVariableCount(p.manager, rule)
	if err != nil {
		return err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return err
	}
	total, err := rule.TailSize()
	if err != nil {
		return err
	}
	for i := 0; i < uninterpreted; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return getErr
		}
		transformed, getErr := p.makeTail(source, destination, tail)
		if getErr != nil {
			return getErr
		}
		l.tail = append(l.tail, transformed)
	}
	for i := uninterpreted; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return getErr
		}
		held, getErr := p.manager.NewRef(tail)
		if getErr != nil {
			return getErr
		}
		l.tail = append(l.tail, held)
	}
	head, err := rule.Head()
	if err != nil {
		return err
	}
	head, err = p.makeHead(source, destination, head, variables)
	if err != nil {
		return err
	}
	if err = p.assign(&l.head, head); err != nil {
		return err
	}
	tails := make([]ast.ASTHandle, len(l.tail))
	for i := range l.tail {
		if tails[i], err = p.manager.RefGet(l.tail[i]); err != nil {
			return err
		}
	}
	// fml = m.mk_implies(m.mk_and(tail), head); rm.mk_rule(fml, ...):
	// Horn normalization replaces non-variable head arguments.
	conjunction, err := p.manager.MkFamilyApp(0, ast.ASTOpAnd, nil, tails)
	if err != nil {
		return err
	}
	formula, err := p.manager.MkImplies(conjunction, head)
	if err != nil {
		return err
	}
	if err = p.assign(&l.fml, formula); err != nil {
		return err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return err
	}
	if p.mkRule != nil {
		return p.mkRule(destination, formula, name)
	}
	return p.rules.AddFormulaWithSymbol(destination, formula, name)
}

func (p *QuantifierAbstractionPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		positive, getErr := rule.PositiveTailSize()
		if getErr == nil {
			var uninterpreted int
			uninterpreted, getErr = rule.UninterpretedTailSize()
			if getErr == nil && positive != uninterpreted {
				if closeErr := rule.Close(); closeErr != nil {
					return nil, closeErr
				}
				return nil, nil
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	p.oldToNew = make(map[ast.ASTHandle]ast.ASTHandle)
	p.order = nil
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
	locals := &qaLoopLocals{}
	defer func() { resultErr = errors.Join(resultErr, p.closeLoopLocals(locals)) }()
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		transformErr := p.transformRule(source, result, rule, locals)
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if len(p.oldToNew) == 0 {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *QuantifierAbstractionPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.oldToNew, p.order = nil, nil
	return nil
}
