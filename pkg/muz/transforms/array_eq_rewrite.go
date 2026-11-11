package transforms

// Source: src/muz/transforms/dl_mk_array_eq_rewrite.cpp/.h array-equivalence
// representative selection and select-over-store rewriting.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type ArrayEqRewritePlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	priority uint32
	mkRule   ArrayInstantiationRuleMaker
	closed   bool
}

func NewArrayEqRewritePlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *ArrayEqRewritePlugin, resultErr error) {
	if manager == nil || rules == nil || context == nil || globals == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	return &ArrayEqRewritePlugin{manager: manager, rules: rules, context: context, globals: globals, priority: priority}, nil
}

func (p *ArrayEqRewritePlugin) Priority() uint32            { return p.priority }
func (p *ArrayEqRewritePlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayEqRewritePlugin) Cancel()                     {}

// arrayEqClasses is expr_equiv_class (obj_equiv_class over a
// basic_union_find): classes iterate by root id, members from the root along
// the next links.
type arrayEqClasses struct {
	toInt map[ast.ASTHandle]uint32
	toObj []ast.ASTHandle
	find  []uint32
	size  []uint32
	next  []uint32
}

func newArrayEqClasses() *arrayEqClasses {
	return &arrayEqClasses{toInt: make(map[ast.ASTHandle]uint32)}
}

func (c *arrayEqClasses) addIfNotThere(o ast.ASTHandle) uint32 {
	if id, found := c.toInt[o]; found {
		return id
	}
	id := uint32(len(c.toObj))
	c.toInt[o] = id
	c.toObj = append(c.toObj, o)
	return id
}

func (c *arrayEqClasses) findRoot(v uint32) uint32 {
	if int(v) >= len(c.find) {
		return v
	}
	for c.find[v] != v {
		v = c.find[v]
	}
	return v
}

func (c *arrayEqClasses) ensureSize(v uint32) {
	for int(v) >= len(c.find) {
		r := uint32(len(c.find))
		c.find = append(c.find, r)
		c.size = append(c.size, 1)
		c.next = append(c.next, r)
	}
}

func (c *arrayEqClasses) merge(a, b ast.ASTHandle) {
	v1, v2 := c.addIfNotThere(a), c.addIfNotThere(b)
	r1, r2 := c.findRoot(v1), c.findRoot(v2)
	if r1 == r2 {
		return
	}
	c.ensureSize(v1)
	c.ensureSize(v2)
	if c.size[r1] > c.size[r2] {
		r1, r2 = r2, r1
	}
	c.find[r1] = r2
	c.size[r2] += c.size[r1]
	c.next[r1], c.next[r2] = c.next[r2], c.next[r1]
}

func (c *arrayEqClasses) nextOf(v uint32) uint32 {
	if int(v) >= len(c.next) {
		return v
	}
	return c.next[v]
}

func (c *arrayEqClasses) isRoot(v uint32) bool {
	return int(v) >= len(c.find) || c.find[v] == v
}

// classes lists each class's members in source iteration order.
func (c *arrayEqClasses) classes() [][]ast.ASTHandle {
	var out [][]ast.ASTHandle
	for root := uint32(0); int(root) < len(c.toObj); root++ {
		if !c.isRoot(root) {
			continue
		}
		var members []ast.ASTHandle
		for current := root; ; {
			members = append(members, c.toObj[current])
			current = c.nextOf(current)
			if current == root {
				break
			}
		}
		out = append(out, members)
	}
	return out
}

func (p *ArrayEqRewritePlugin) arrayEquality(expression ast.ASTHandle) (left, right ast.ASTHandle, yes bool, err error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpEq {
		return left, right, false, err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = p.manager.GetArg(expression, 0)
	if err != nil {
		return left, right, false, err
	}
	right, err = p.manager.GetArg(expression, 1)
	if err != nil {
		return left, right, false, err
	}
	sort, err := p.manager.GetSort(left)
	if err != nil {
		return left, right, false, err
	}
	family, err = p.manager.GetFamilyID(sort)
	if err != nil {
		return left, right, false, err
	}
	arrayFamily, err := p.manager.ArrayFamilyID()
	return left, right, family == arrayFamily, err
}

// replaceApp follows the source helper: it recursively rebuilds applications
// but deliberately does not descend through variables or quantifiers.
func (p *ArrayEqRewritePlugin) replaceApp(expression, oldValue, newValue ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if expression == oldValue {
		return p.manager.NewRef(newValue)
	}
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		if err != nil {
			return ast.ASTRef{}, err
		}
		return p.manager.NewRef(expression)
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	args := make([]ast.ASTHandle, count)
	refs := make([]ast.ASTRef, 0, count)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&refs[i]))
		}
	}()
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(expression, i)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		ref, getErr := p.replaceApp(argument, oldValue, newValue)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		refs = append(refs, ref)
		args[i], getErr = p.manager.RefGet(ref)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
	}
	declaration, err := p.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	rebuilt, err := p.manager.MkApp(declaration, args)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(rebuilt)
}

func (p *ArrayEqRewritePlugin) transformRule(rule *base.Rule, destination *base.RuleSet) (_ *base.Rule, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for i := 0; i < uninterpreted; i++ {
		tail, negative, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		tails, negated = append(tails, tail), append(negated, negative)
	}
	equivalences := newArrayEqClasses()
	for i := uninterpreted; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		left, right, equality, getErr := p.arrayEquality(tail)
		if getErr != nil {
			return nil, getErr
		}
		if equality {
			equivalences.merge(left, right)
		} else {
			tails, negated = append(tails, tail), append(negated, false)
		}
	}
	// instantiate_rule's locals die in reverse declaration order: pr,
	// res_conjs (front to back), the th_rewriter t, array_eq_classes, and
	// new_tail (front to back). A new_tail element assignment holds the new
	// value before releasing the replaced one.
	releaseForward := func(refs []ast.ASTRef) (result error) {
		for i := range refs {
			if refs[i] != (ast.ASTRef{}) {
				result = errors.Join(result, p.manager.DelRef(&refs[i]))
			}
		}
		return result
	}
	newTail := make([]ast.ASTRef, 0, len(tails))
	defer func() { resultErr = errors.Join(resultErr, releaseForward(newTail)) }()
	for _, tail := range tails {
		ref, makeErr := p.manager.NewRef(tail)
		if makeErr != nil {
			return nil, makeErr
		}
		newTail = append(newTail, ref)
	}
	for _, members := range equivalences.classes() {
		representative := members[0]
		for _, member := range members {
			kind, getErr := p.manager.GetKind(member)
			if getErr != nil {
				return nil, getErr
			}
			if kind != ast.ASTVar {
				representative = member
				break
			}
		}
		for _, member := range members {
			for i := range tails {
				ref, replaceErr := p.replaceApp(tails[i], member, representative)
				if replaceErr != nil {
					return nil, replaceErr
				}
				previous := newTail[i]
				newTail[i] = ref
				if replaceErr = p.manager.DelRef(&previous); replaceErr != nil {
					return nil, replaceErr
				}
				tails[i], replaceErr = p.manager.RefGet(ref)
				if replaceErr != nil {
					return nil, replaceErr
				}
			}
		}
		for _, member := range members {
			equality, makeErr := p.manager.MkEq(member, representative)
			if makeErr != nil {
				return nil, makeErr
			}
			held, makeErr := p.manager.NewRef(equality)
			if makeErr != nil {
				return nil, makeErr
			}
			newTail = append(newTail, held)
			tails, negated = append(tails, equality), append(negated, false)
		}
	}
	local, err := p.context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.context.Del(&local)) }()
	optionName := "expand_select_store"
	if err = p.context.SetBoolName(&local, &optionName, true); err != nil {
		return nil, err
	}
	rewriter, err := astrewriter.NewTheoryRewriter(p.manager, p.context, p.globals, &local)
	if err != nil {
		if rewriter != nil {
			err = errors.Join(err, rewriter.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rewriter.Close()) }()
	var resConjs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseForward(resConjs)) }()
	// The C++ transform runs th_rewriter over every retained tail, including
	// predicate applications whose arguments contain interpreted array terms.
	for i := 0; i < len(tails); i++ {
		ref, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return nil, makeErr
		}
		resConjs = append(resConjs, ref)
		if makeErr = rewriter.Rewrite(tails[i], ref); makeErr != nil {
			return nil, makeErr
		}
		tails[i], makeErr = p.manager.RefGet(ref)
		if makeErr != nil {
			return nil, makeErr
		}
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	// mk_rule(m.mk_implies(m.mk_and(res_conjs), r.get_head()), ...): the raw
	// formula is released as soon as mk_rule returns.
	conjunction, err := p.manager.MkFamilyApp(0, ast.ASTOpAnd, nil, tails)
	if err != nil {
		return nil, err
	}
	formula, err := p.manager.MkImplies(conjunction, head)
	if err != nil {
		return nil, err
	}
	held, err := p.manager.NewRef(formula)
	if err != nil {
		return nil, err
	}
	if p.mkRule == nil {
		made, makeErr := p.rules.NewRuleWithSymbol(head, tails, negated, name)
		return made, errors.Join(makeErr, p.manager.DelRef(&held))
	}
	err = p.mkRule(destination, formula, name)
	return nil, errors.Join(err, p.manager.DelRef(&held))
}

// SetRuleMaker supplies rule_manager::mk_rule.
func (p *ArrayEqRewritePlugin) SetRuleMaker(mk ArrayInstantiationRuleMaker) { p.mkRule = mk }

func (p *ArrayEqRewritePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.context == nil || p.globals == nil {
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
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		replacement, transformErr := p.transformRule(rule, result)
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr == nil && replacement != nil {
			transformErr = result.Add(replacement)
		}
		if replacement != nil {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	keep = true
	return result, nil
}

func (p *ArrayEqRewritePlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}
