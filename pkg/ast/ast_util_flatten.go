package ast

import "errors"

// Source: src/ast/ast_util.cpp:254-366; src/ast/ast_util.h:146-153.

func (m *ASTManager) astUtilRawNot(r astNodeRef) (astNodeRef, error) {
	args := [1]ASTHandle{m.external(r)}
	return m.mkFamilyApp(0, ASTOpNot, nil, args[:])
}

func (m *ASTManager) astUtilUnary(r astNodeRef, op int32) (astNodeRef, bool, error) {
	yes, e := m.astUtilOperator(r, op)
	if e != nil || !yes {
		return astNodeRef{}, false, e
	}
	a := m.appAt(r.slot)
	if a.count != 1 {
		return astNodeRef{}, false, ErrASTDomain
	}
	child, e := m.appArg(a, 0)
	return child, true, e
}

func (m *ASTManager) astUtilBinary(r astNodeRef, op int32) (astNodeRef, astNodeRef, bool, error) {
	yes, e := m.astUtilOperator(r, op)
	if e != nil || !yes {
		return astNodeRef{}, astNodeRef{}, false, e
	}
	a := m.appAt(r.slot)
	if a.count != 2 {
		return astNodeRef{}, astNodeRef{}, false, ErrASTDomain
	}
	left, e := m.appArg(a, 0)
	if e != nil {
		return astNodeRef{}, astNodeRef{}, false, e
	}
	right, e := m.appArg(a, 1)
	return left, right, true, e
}

// flattenOwned tracks the negations flatten_and creates. The source's
// result is an expr_ref_vector, so such a term is deleted as soon as the
// vector stops holding it and nothing else does; the caller's own terms
// are left to the caller.
type flattenOwned struct {
	m     *ASTManager
	count map[astNodeRef]int
	// initial counts the vector's own references to the input elements
	// (FlattenAndOwned); release drops one such reference when the element
	// is replaced.
	initial map[astNodeRef]int
	release func(ASTHandle) error
}

func (o *flattenOwned) hold(r astNodeRef) {
	if o.count == nil {
		o.count = make(map[astNodeRef]int)
	}
	o.count[r]++
}

func (o *flattenOwned) drop(h ASTHandle) error {
	r, e := o.m.requireExpr(h)
	if e != nil {
		return e
	}
	if n := o.initial[r]; n > 0 {
		if n == 1 {
			delete(o.initial, r)
		} else {
			o.initial[r] = n - 1
		}
		return o.release(h)
	}
	c, ok := o.count[r]
	if !ok {
		return nil
	}
	if c > 1 {
		o.count[r] = c - 1
		return nil
	}
	delete(o.count, r)
	if o.m.nodeAt(r.slot).rc != 0 {
		return nil
	}
	return o.m.deleteNode(r)
}

func (m *ASTManager) astUtilFlattenAnd(result []ASTHandle) ([]ASTHandle, error) {
	return m.astUtilFlattenAndWith(result, &flattenOwned{m: m})
}

func (m *ASTManager) astUtilFlattenAndWith(result []ASTHandle, owned *flattenOwned) ([]ASTHandle, error) {
	for i := 0; i < len(result); {
		r, e := m.requireExpr(result[i])
		if e != nil {
			return result, e
		}
		if yes, e := m.astUtilOperator(r, ASTOpAnd); e != nil {
			return result, e
		} else if yes {
			a := m.appAt(r.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.appArg(a, j)
				if e != nil {
					return result, e
				}
				result = append(result, m.external(child))
			}
			old := result[i]
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			if e = owned.drop(old); e != nil {
				return result, e
			}
			continue
		}
		inner, isNot, e := m.astUtilUnary(r, ASTOpNot)
		if e != nil {
			return result, e
		}
		if isNot {
			if twice, yes, e := m.astUtilUnary(inner, ASTOpNot); e != nil {
				return result, e
			} else if yes {
				old := result[i]
				result[i] = m.external(twice)
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if yes, e := m.astUtilOperator(inner, ASTOpOr); e != nil {
				return result, e
			} else if yes {
				a := m.appAt(inner.slot)
				for j := uint32(0); j < a.count; j++ {
					child, e := m.appArg(a, j)
					if e != nil {
						return result, e
					}
					negated, e := m.astUtilRawNot(child)
					if e != nil {
						return result, e
					}
					owned.hold(negated)
					result = append(result, m.external(negated))
				}
				old := result[i]
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if left, right, yes, e := m.astUtilBinary(inner, ASTOpImplies); e != nil {
				return result, e
			} else if yes {
				result = append(result, m.external(left))
				negated, e := m.astUtilRawNot(right)
				if e != nil {
					return result, e
				}
				owned.hold(negated)
				old := result[i]
				result[i] = m.external(negated)
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if inner == m.falseApp {
				old := result[i]
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if inner == m.trueApp {
				for _, h := range result {
					if e = owned.drop(h); e != nil {
						return result, e
					}
				}
				return []ASTHandle{m.external(m.falseApp)}, nil
			}
		}
		if r == m.trueApp {
			old := result[i]
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			if e = owned.drop(old); e != nil {
				return result, e
			}
			continue
		}
		if r == m.falseApp {
			for _, h := range result {
				if e = owned.drop(h); e != nil {
					return result, e
				}
			}
			return []ASTHandle{m.external(m.falseApp)}, nil
		}
		i++
	}
	return result, nil
}

func (m *ASTManager) astUtilFlattenOr(result []ASTHandle) ([]ASTHandle, error) {
	for i := 0; i < len(result); {
		r, e := m.requireExpr(result[i])
		if e != nil {
			return result, e
		}
		if yes, e := m.astUtilOperator(r, ASTOpOr); e != nil {
			return result, e
		} else if yes {
			a := m.appAt(r.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.appArg(a, j)
				if e != nil {
					return result, e
				}
				result = append(result, m.external(child))
			}
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		inner, isNot, e := m.astUtilUnary(r, ASTOpNot)
		if e != nil {
			return result, e
		}
		if isNot {
			if twice, yes, e := m.astUtilUnary(inner, ASTOpNot); e != nil {
				return result, e
			} else if yes {
				result[i] = m.external(twice)
				continue
			}
			if yes, e := m.astUtilOperator(inner, ASTOpAnd); e != nil {
				return result, e
			} else if yes {
				a := m.appAt(inner.slot)
				for j := uint32(0); j < a.count; j++ {
					child, e := m.appArg(a, j)
					if e != nil {
						return result, e
					}
					negated, e := m.astUtilRawNot(child)
					if e != nil {
						return result, e
					}
					result = append(result, m.external(negated))
				}
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if inner == m.trueApp {
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				continue
			}
			if inner == m.falseApp {
				return []ASTHandle{m.external(m.trueApp)}, nil
			}
		}
		if left, right, yes, e := m.astUtilBinary(r, ASTOpImplies); e != nil {
			return result, e
		} else if yes {
			result = append(result, m.external(right))
			negated, e := m.astUtilRawNot(left)
			if e != nil {
				return result, e
			}
			result[i] = m.external(negated)
			continue
		}
		if r == m.falseApp {
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			continue
		}
		if r == m.trueApp {
			return []ASTHandle{m.external(m.trueApp)}, nil
		}
		i++
	}
	return result, nil
}

// FlattenAnd rewrites a caller-owned conjunction vector in the source order.
func (m *ASTManager) FlattenAnd(result []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	return m.astUtilFlattenAnd(result)
}

// FlattenAndOwned is flatten_and(expr_ref_vector&) on an owning vector: refs
// are the vector's references, released one by one as flatten_and replaces
// their elements; the result's elements (surviving inputs and the terms
// flatten_and creates) come back as references the caller owns.
func (m *ASTManager) FlattenAndOwned(refs []ASTRef) (_ []ASTRef, resultErr error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	handles := make([]ASTHandle, len(refs))
	pool := make(map[ASTHandle][]ASTRef, len(refs))
	owned := &flattenOwned{m: m, initial: make(map[astNodeRef]int, len(refs))}
	for i := range refs {
		r, e := m.reference(refs[i])
		if e != nil {
			return nil, e
		}
		handles[i] = m.external(r.node)
		pool[handles[i]] = append(pool[handles[i]], refs[i])
		owned.initial[r.node]++
	}
	// poolOrder lists pool's keys in the order they were first added.
	poolOrder := append([]ASTHandle(nil), handles...)
	defer func() {
		if resultErr != nil {
			for _, h := range poolOrder {
				list := pool[h]
				delete(pool, h)
				for i := range list {
					resultErr = errors.Join(resultErr, m.delRef(&list[i]))
				}
			}
		}
	}()
	owned.release = func(h ASTHandle) error {
		list := pool[h]
		if len(list) == 0 {
			return nil
		}
		ref := list[len(list)-1]
		pool[h] = list[:len(list)-1]
		return m.delRef(&ref)
	}
	result, e := m.astUtilFlattenAndWith(handles, owned)
	if e != nil {
		return nil, e
	}
	out := make([]ASTRef, len(result))
	for i, h := range result {
		if list := pool[h]; len(list) != 0 {
			out[i] = list[len(list)-1]
			pool[h] = list[:len(list)-1]
			continue
		}
		ref, e := m.newRef(h)
		if e != nil {
			for j := 0; j < i; j++ {
				pool[result[j]] = append(pool[result[j]], out[j])
				poolOrder = append(poolOrder, result[j])
			}
			return nil, e
		}
		out[i] = ref
	}
	// References of inputs flatten_and removed without a replacement (true),
	// in input-vector order.
	for _, h := range handles {
		list := pool[h]
		delete(pool, h)
		for i := range list {
			if e := m.delRef(&list[i]); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}

// FlattenOr rewrites a caller-owned disjunction vector in the source order.
func (m *ASTManager) FlattenOr(result []ASTHandle) ([]ASTHandle, error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	return m.astUtilFlattenOr(result)
}

// FlattenOrHeld is flatten_or(fml, expr_ref_vector&) into a fresh owning
// vector: the returned references are the vector's, which the caller
// releases in order (expr_ref_vector destruction). A negation that
// flatten_or creates and later replaces is deleted at once, as the vector
// stops holding it.
func (m *ASTManager) FlattenOrHeld(formula ASTHandle) (_ []ASTRef, resultErr error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	result, e := m.astUtilFlattenOrWith([]ASTHandle{formula}, &flattenOwned{m: m})
	if e != nil {
		return nil, e
	}
	out := make([]ASTRef, 0, len(result))
	for _, h := range result {
		ref, e := m.newRef(h)
		if e != nil {
			for i := range out {
				e = errors.Join(e, m.delRef(&out[i]))
			}
			return nil, e
		}
		out = append(out, ref)
	}
	return out, nil
}

func (m *ASTManager) astUtilFlattenOrWith(result []ASTHandle, owned *flattenOwned) ([]ASTHandle, error) {
	for i := 0; i < len(result); {
		r, e := m.requireExpr(result[i])
		if e != nil {
			return result, e
		}
		if yes, e := m.astUtilOperator(r, ASTOpOr); e != nil {
			return result, e
		} else if yes {
			a := m.appAt(r.slot)
			for j := uint32(0); j < a.count; j++ {
				child, e := m.appArg(a, j)
				if e != nil {
					return result, e
				}
				result = append(result, m.external(child))
			}
			old := result[i]
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			if e = owned.drop(old); e != nil {
				return result, e
			}
			continue
		}
		inner, isNot, e := m.astUtilUnary(r, ASTOpNot)
		if e != nil {
			return result, e
		}
		if isNot {
			if twice, yes, e := m.astUtilUnary(inner, ASTOpNot); e != nil {
				return result, e
			} else if yes {
				old := result[i]
				result[i] = m.external(twice)
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if yes, e := m.astUtilOperator(inner, ASTOpAnd); e != nil {
				return result, e
			} else if yes {
				a := m.appAt(inner.slot)
				for j := uint32(0); j < a.count; j++ {
					child, e := m.appArg(a, j)
					if e != nil {
						return result, e
					}
					negated, e := m.astUtilRawNot(child)
					if e != nil {
						return result, e
					}
					owned.hold(negated)
					result = append(result, m.external(negated))
				}
				old := result[i]
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if inner == m.trueApp {
				old := result[i]
				result[i] = result[len(result)-1]
				result = result[:len(result)-1]
				if e = owned.drop(old); e != nil {
					return result, e
				}
				continue
			}
			if inner == m.falseApp {
				for _, h := range result {
					if e = owned.drop(h); e != nil {
						return result, e
					}
				}
				return []ASTHandle{m.external(m.trueApp)}, nil
			}
		}
		if left, right, yes, e := m.astUtilBinary(r, ASTOpImplies); e != nil {
			return result, e
		} else if yes {
			result = append(result, m.external(right))
			negated, e := m.astUtilRawNot(left)
			if e != nil {
				return result, e
			}
			owned.hold(negated)
			old := result[i]
			result[i] = m.external(negated)
			if e = owned.drop(old); e != nil {
				return result, e
			}
			continue
		}
		if r == m.falseApp {
			old := result[i]
			result[i] = result[len(result)-1]
			result = result[:len(result)-1]
			if e = owned.drop(old); e != nil {
				return result, e
			}
			continue
		}
		if r == m.trueApp {
			for _, h := range result {
				if e = owned.drop(h); e != nil {
					return result, e
				}
			}
			return []ASTHandle{m.external(m.trueApp)}, nil
		}
		i++
	}
	return result, nil
}
