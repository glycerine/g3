package transforms

// Source: src/muz/transforms/dl_mk_array_blast.cpp blast, ackermanize,
// insert_def and is_select_eq_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// SetRuleMaker supplies rule_manager::mk_rule.
func (p *ArrayBlastPlugin) SetRuleMaker(mk ArrayInstantiationRuleMaker) { p.mkRule = mk }

// selectEqVar is is_select_eq_var: (= (select ...) v) in either order.
func (p *ArrayBlastPlugin) selectEqVar(e ast.ASTHandle) (selectTerm, variable ast.ASTHandle, ok bool, err error) {
	x, y, equal, err := p.equality(e)
	if err != nil || !equal {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	ySelect, err := p.arrayApplication(y, ast.ASTArrayOpSelect)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if ySelect {
		x, y = y, x
	}
	xSelect, err := p.arrayApplication(x, ast.ASTArrayOpSelect)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	yKind, err := p.manager.GetKind(y)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if xSelect && yKind == ast.ASTVar {
		return x, y, true, nil
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

// arrayBlastAckermann holds ackermanize's state: m_defs (an obj_map from
// select terms to variables) and m_next_var.
type arrayBlastAckermann struct {
	p       *ArrayBlastPlugin
	rule    *base.Rule
	defs    *ast.ObjMap
	nextVar uint32
	// pinned keeps the m_defs keys and fresh variables valid until the end
	// of ackermanize; the source holds them as raw pointers.
	pinned []ast.ASTRef
}

func (k *arrayBlastAckermann) pin(h ast.ASTHandle) error {
	r, err := k.p.manager.NewRef(h)
	if err == nil {
		k.pinned = append(k.pinned, r)
	}
	return err
}

func (k *arrayBlastAckermann) find(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	value, found, err := k.defs.Find(e)
	return value[0], found, err
}

// insertDef is insert_def.
func (k *arrayBlastAckermann) insertDef(e, v ast.ASTHandle) (bool, error) {
	selectBase, err := k.p.selectBase(e)
	if err != nil {
		return false, err
	}
	kind, err := k.p.manager.GetKind(selectBase)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTVar {
		return false, nil
	}
	if err = k.pin(e); err != nil {
		return false, err
	}
	if v.IsNull() {
		if k.nextVar == 0 {
			count, e2 := k.p.maximumVariable(k.rule)
			if e2 != nil {
				return false, e2
			}
			k.nextVar = count + 1
		}
		sort, e2 := k.p.manager.GetSort(e)
		if e2 != nil {
			return false, e2
		}
		if v, e2 = k.p.manager.MkVar(k.nextVar, sort); e2 != nil {
			return false, e2
		}
		if e2 = k.pin(v); e2 != nil {
			return false, e2
		}
		k.nextVar++
	}
	return true, k.defs.Insert(e, [2]ast.ASTHandle{v})
}

func (p *ArrayBlastPlugin) holdAll(handles []ast.ASTHandle) ([]ast.ASTRef, error) {
	refs := make([]ast.ASTRef, 0, len(handles))
	for _, h := range handles {
		r, err := p.manager.NewRef(h)
		if err != nil {
			return refs, err
		}
		refs = append(refs, r)
	}
	return refs, nil
}

func (p *ArrayBlastPlugin) releaseAll(refs []ast.ASTRef) (result error) {
	for i := range refs {
		if refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, p.manager.DelRef(&refs[i]))
		}
	}
	return result
}

func (p *ArrayBlastPlugin) handles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	out := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		h, err := p.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		out[i] = h
	}
	return out, nil
}

// assignRef is expr_ref assignment: hold the new value, then release the
// old one.
func (p *ArrayBlastPlugin) assignRef(slot *ast.ASTRef, h ast.ASTHandle) error {
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

func (p *ArrayBlastPlugin) rewriteInPlace(ref ast.ASTRef) error {
	h, err := p.manager.RefGet(ref)
	if err != nil {
		return err
	}
	rewritten, err := p.rewrite(h)
	if err != nil {
		return err
	}
	result, err := p.manager.RefGet(rewritten)
	if err == nil {
		err = p.manager.SetRef(ref, result)
	}
	return errors.Join(err, p.manager.DelRef(&rewritten))
}

// ackermanize is mk_array_blast::ackermanize; bodyRef is rewritten in place.
func (p *ArrayBlastPlugin) ackermanize(rule *base.Rule, bodyRef ast.ASTRef, head ast.ASTHandle) (_ bool, resultErr error) {
	body, err := p.manager.RefGet(bodyRef)
	if err != nil {
		return false, err
	}
	flat, err := p.manager.FlattenAnd([]ast.ASTHandle{body})
	if err != nil {
		return false, err
	}
	k := &arrayBlastAckermann{p: p, rule: rule, defs: ast.NewObjMap(p.manager)}
	// Locals conjs and trail; at exit e1 is released first, then trail and
	// conjs, each forward.
	var conjs, trail []ast.ASTRef
	var e1 ast.ASTRef
	defer func() {
		if e1 != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&e1))
		}
		resultErr = errors.Join(resultErr, p.releaseAll(k.pinned), p.releaseAll(trail), p.releaseAll(conjs))
	}()
	if conjs, err = p.holdAll(flat); err != nil {
		return false, err
	}
	// Disable the reduction if the head has a non-variable, non-value
	// argument.
	headCount, err := p.manager.GetNumArgs(head)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < headCount; i++ {
		argument, x := p.manager.GetArg(head, i)
		if x != nil {
			return false, x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return false, x
		}
		value, x := p.manager.IsValue(argument)
		if x != nil {
			return false, x
		}
		if kind != ast.ASTVar && !value {
			return false, nil
		}
	}
	var todo []ast.ASTHandle
	for _, c := range flat {
		s, _, ok, x := p.selectEqVar(c)
		if x != nil {
			return false, x
		}
		if !ok {
			todo = append(todo, c)
			continue
		}
		n, x := p.manager.GetNumArgs(s)
		if x != nil {
			return false, x
		}
		for i := uint32(0); i < n; i++ {
			arg, x := p.manager.GetArg(s, i)
			if x != nil {
				return false, x
			}
			todo = append(todo, arg)
		}
	}
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	for len(todo) != 0 {
		e := todo[len(todo)-1]
		if _, found := cache[e]; found {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, x := p.manager.GetKind(e)
		if x != nil {
			return false, x
		}
		if kind == ast.ASTVar {
			cache[e] = e
			todo = todo[:len(todo)-1]
			continue
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		n, x := p.manager.GetNumArgs(e)
		if x != nil {
			return false, x
		}
		valid := true
		args := make([]ast.ASTHandle, 0, n)
		for i := uint32(0); i < n; i++ {
			arg, x := p.manager.GetArg(e, i)
			if x != nil {
				return false, x
			}
			if cached, found := cache[arg]; found {
				args = append(args, cached)
			} else {
				todo = append(todo, arg)
				valid = false
			}
		}
		if !valid {
			continue
		}
		todo = todo[:len(todo)-1]
		decl, x := p.manager.GetDecl(e)
		if x != nil {
			return false, x
		}
		built, x := p.manager.MkApp(decl, args)
		if x != nil {
			return false, x
		}
		if x = p.assignRef(&e1, built); x != nil {
			return false, x
		}
		held, x := p.manager.NewRef(built)
		if x != nil {
			return false, x
		}
		trail = append(trail, held)
		isSelect, x := p.arrayApplication(e, ast.ASTArrayOpSelect)
		if x != nil {
			return false, x
		}
		if !isSelect {
			cache[e] = built
			continue
		}
		v, found, x := k.find(built)
		if x != nil {
			return false, x
		}
		if !found {
			ok, x := k.insertDef(built, ast.ASTHandle{})
			if x != nil {
				return false, x
			}
			if !ok {
				return false, nil
			}
			if v, _, x = k.find(built); x != nil {
				return false, x
			}
		}
		cache[e] = v
	}
	for i, c := range flat {
		s, v, ok, x := p.selectEqVar(c)
		if x != nil {
			return false, x
		}
		replacement := cache[c]
		if ok {
			n, x := p.manager.GetNumArgs(s)
			if x != nil {
				return false, x
			}
			args := make([]ast.ASTHandle, n)
			for j := uint32(0); j < n; j++ {
				arg, x := p.manager.GetArg(s, j)
				if x != nil {
					return false, x
				}
				args[j] = cache[arg]
			}
			decl, x := p.manager.GetDecl(s)
			if x != nil {
				return false, x
			}
			built, x := p.manager.MkApp(decl, args)
			if x != nil {
				return false, x
			}
			if x = p.assignRef(&e1, built); x != nil {
				return false, x
			}
			_, found, x := k.find(built)
			if x != nil {
				return false, x
			}
			if !found {
				inserted, x := k.insertDef(built, v)
				if x != nil {
					return false, x
				}
				if !inserted {
					return false, nil
				}
			}
			definition, _, x := k.find(built)
			if x != nil {
				return false, x
			}
			if replacement, x = p.manager.MkEq(v, definition); x != nil {
				return false, x
			}
		}
		if x = p.manager.SetRef(conjs[i], replacement); x != nil {
			return false, x
		}
	}
	// The Ackermann reduction: i1 = i2 => v1 = v2 for selects on the same
	// array, over m_defs in hash-table order.
	entries := k.defs.Entries()
	for i := 0; i < len(entries); i++ {
		a1, v1 := entries[i].Key, entries[i].Value[0]
		base1, x := p.selectBase(a1)
		if x != nil {
			return false, x
		}
		for j := i + 1; j < len(entries); j++ {
			a2, v2 := entries[j].Key, entries[j].Value[0]
			base2, x := p.selectBase(a2)
			if x != nil {
				return false, x
			}
			if base1 != base2 {
				continue
			}
			args1, x := p.selectIndices(a1)
			if x != nil {
				return false, x
			}
			args2, x := p.selectIndices(a2)
			if x != nil {
				return false, x
			}
			var eqs []ast.ASTRef
			for q := range args1 {
				eq, x := p.manager.MkEq(args1[q], args2[q])
				var held ast.ASTRef
				if x == nil {
					held, x = p.manager.NewRef(eq)
				}
				if x != nil {
					return false, errors.Join(x, p.releaseAll(eqs))
				}
				eqs = append(eqs, held)
			}
			// mk_implies(mk_and(eqs), mk_eq(v1, v2)): GCC evaluates the
			// arguments right to left, so the value equality comes first.
			valuesEqual, x := p.manager.MkEq(v1, v2)
			var antecedent, implication ast.ASTHandle
			var eqHandles []ast.ASTHandle
			if x == nil {
				eqHandles, x = p.handles(eqs)
			}
			if x == nil {
				antecedent, x = p.manager.MkFamilyApp(0, ast.ASTOpAnd, nil, eqHandles)
			}
			if x == nil {
				implication, x = p.manager.MkImplies(antecedent, valuesEqual)
			}
			var held ast.ASTRef
			if x == nil {
				held, x = p.manager.NewRef(implication)
			}
			if x = errors.Join(x, p.releaseAll(eqs)); x != nil {
				return false, x
			}
			conjs = append(conjs, held)
		}
	}
	conjHandles, err := p.handles(conjs)
	if err != nil {
		return false, err
	}
	conjunction, err := p.manager.MkFamilyApp(0, ast.ASTOpAnd, nil, conjHandles)
	if err != nil {
		return false, err
	}
	if err = p.manager.SetRef(bodyRef, conjunction); err != nil {
		return false, err
	}
	return true, p.rewriteInPlace(bodyRef)
}

// blast is mk_array_blast::blast. It adds the rule, or its replacement, to
// rules and reports whether the rule changed.
func (p *ArrayBlastPlugin) blast(rule *base.Rule, rules *base.RuleSet) (_ bool, resultErr error) {
	if p == nil || p.closed || p.closing {
		return false, base.ErrRuleTransformerOwner
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	// Locals conjs and new_conjs die last (new_conjs, then conjs).
	var conjs, newConjs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, p.releaseAll(newConjs), p.releaseAll(conjs)) }()
	for i := 0; i < uninterpreted; i++ {
		tail, _, x := rule.Tail(i)
		var held ast.ASTRef
		if x == nil {
			held, x = p.manager.NewRef(tail)
		}
		if x != nil {
			return false, x
		}
		newConjs = append(newConjs, held)
	}
	interpreted := make([]ast.ASTHandle, 0, total-uninterpreted)
	for i := uninterpreted; i < total; i++ {
		tail, _, x := rule.Tail(i)
		if x != nil {
			return false, x
		}
		interpreted = append(interpreted, tail)
	}
	flat, err := p.manager.FlattenAnd(interpreted)
	if err != nil {
		return false, err
	}
	if conjs, err = p.holdAll(flat); err != nil {
		return false, err
	}
	sub, err := astrewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	inserted := false
	for _, e := range flat {
		x, y, definition, getErr := p.storeDefinition(e)
		if getErr != nil {
			return false, getErr
		}
		if definition {
			disjoint, getErr := p.disjoint(x, y)
			if getErr != nil {
				return false, getErr
			}
			if disjoint {
				if getErr = sub.Insert(x, y); getErr != nil {
					return false, getErr
				}
				inserted = true
				continue
			}
			held, getErr := p.manager.NewRef(e)
			if getErr != nil {
				return false, getErr
			}
			newConjs = append(newConjs, held)
			continue
		}
		tmp, getErr := p.rewrite(e)
		if getErr != nil {
			return false, getErr
		}
		newConjs = append(newConjs, tmp)
	}
	if !inserted {
		return false, rules.Add(rule)
	}
	// fml1, fml2, body and head die before sub (head, body, fml2 order).
	var fml2, body, head ast.ASTRef
	defer func() {
		for _, r := range []*ast.ASTRef{&head, &body, &fml2} {
			if *r != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, p.manager.DelRef(r))
			}
		}
	}()
	newHandles, err := p.handles(newConjs)
	if err != nil {
		return false, err
	}
	conjunction, err := p.manager.MkFamilyApp(0, ast.ASTOpAnd, nil, newHandles)
	if err != nil {
		return false, err
	}
	if err = p.assignRef(&body, conjunction); err != nil {
		return false, err
	}
	sourceHead, err := rule.Head()
	if err != nil {
		return false, err
	}
	if err = p.assignRef(&head, sourceHead); err != nil {
		return false, err
	}
	if err = sub.ApplyRef(body); err != nil {
		return false, err
	}
	if err = p.rewriteInPlace(body); err != nil {
		return false, err
	}
	if err = sub.ApplyRef(head); err != nil {
		return false, err
	}
	if err = p.rewriteInPlace(head); err != nil {
		return false, err
	}
	headHandle, err := p.manager.RefGet(head)
	if err != nil {
		return false, err
	}
	changed, err := p.ackermanize(rule, body, headHandle)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, rules.Add(rule)
	}
	bodyHandle, err := p.manager.RefGet(body)
	if err != nil {
		return false, err
	}
	formula, err := p.manager.MkImplies(bodyHandle, headHandle)
	if err != nil {
		return false, err
	}
	if err = p.assignRef(&fml2, formula); err != nil {
		return false, err
	}
	newRules, err := base.NewRuleSet(p.rules)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, newRules.Close()) }()
	name, err := rule.NameSymbol()
	if err != nil {
		return false, err
	}
	if p.mkRule != nil {
		err = p.mkRule(newRules, formula, name)
	} else {
		err = p.rules.AddFormulaWithSymbol(newRules, formula, name)
	}
	if err != nil {
		return false, err
	}
	count, err := newRules.Len()
	if err != nil || count == 0 {
		return true, err
	}
	last, err := newRules.Rule(count - 1)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, last.Close()) }()
	simplified, dropped, simplifiedChanged, err := p.simplifier.transformRule(last)
	if err != nil {
		return false, err
	}
	if dropped {
		return true, nil
	}
	newRule := last
	if simplifiedChanged {
		newRule = simplified
		defer func() { resultErr = errors.Join(resultErr, simplified.Close()) }()
	}
	sourceProof, hasProof, err := rule.Proof()
	if err != nil {
		return false, err
	}
	if hasProof {
		fml1, x := rule.ToFormula()
		if x != nil {
			return false, x
		}
		old, x := p.manager.RefGet(fml1)
		var proof ast.ASTHandle
		if x == nil {
			proof, x = p.manager.MkProofRewrite(old, formula)
		}
		if x == nil {
			proof, x = p.manager.MkProofModusPonens(sourceProof, proof)
		}
		if x == nil {
			x = newRule.SetProof(proof)
		}
		if x = errors.Join(x, p.manager.DelRef(&fml1)); x != nil {
			return false, x
		}
	}
	if err = rules.Add(newRule); err != nil {
		return false, err
	}
	return true, p.rules.MkRuleRewriteProof(rule, newRule)
}
