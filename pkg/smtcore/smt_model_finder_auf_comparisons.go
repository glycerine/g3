package smtcore

// Source: src/smt/smt_model_finder.cpp quantifier_analyzer numeric atoms;
// x_leq_y, x_sleq_y and x_gle_t qinfo.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *smtModelFinderAUFGraph) normalizeGroundRef(term ast.ASTHandle) (ast.ASTRef, error) {
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	manager := g.finder.state.quantifierManager
	if manager == nil || manager.rewriter == nil {
		return ast.ASTRef{}, errors.Join(ast.ErrASTHandle, m.DelRef(&ref))
	}
	if err = manager.rewriter.Rewrite(term, ref); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&ref))
	}
	return ref, nil
}

// keep moves an owned result into g.refs (a qinfo's term) and returns it.
func (g *smtModelFinderAUFGraph) keep(ref ast.ASTRef) (ast.ASTHandle, error) {
	g.refs = append(g.refs, ref)
	return g.finder.manager.RefGet(ref)
}

// mutilArithRef is macro_util::mk_add/mk_sub(t1, t2, r): one step of the
// arithmetic rewriter (m_arith_rw), not rewritten further. Bit-vector
// operands keep the normalizing path. The caller owns the result.
func (g *smtModelFinderAUFGraph) mutilArithRef(t1, t2 ast.ASTHandle, plus bool) (ast.ASTRef, error) {
	m := g.finder.manager
	sort, err := m.GetSort(t1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	bv, err := m.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if bv {
		var raw ast.ASTHandle
		if plus {
			raw, err = g.finder.macroUtil.MkAdd(t1, t2)
		} else {
			raw, err = g.finder.macroUtil.MkSub(t1, t2)
		}
		if err != nil {
			return ast.ASTRef{}, err
		}
		return g.normalizeGroundRef(raw)
	}
	arith, err := g.localArithRewriter()
	if err != nil {
		return ast.ASTRef{}, err
	}
	var ref ast.ASTRef
	if plus {
		ref, err = arith.MkAdd2(t1, t2)
	} else {
		ref, err = arith.MkSub2(t1, t2)
	}
	if err = errors.Join(err, arith.Close()); err != nil {
		if ref != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&ref))
		}
		return ast.ASTRef{}, err
	}
	return ref, nil
}

// variableAndGround mirrors is_var_and_ground, including -x+k and
// occurrences on either side. The returned variable is a de Bruijn index;
// the caller owns boundRef (t), which is unset when bound is one of the
// atom's own arguments. The local tmp (the var-plus-ground offset) dies
// on return, as in the source.
func (g *smtModelFinderAUFGraph) variableAndGround(left, right ast.ASTHandle) (variable uint32, bound ast.ASTHandle, boundRef ast.ASTRef, inverse, matched bool, result error) {
	m := g.finder.manager
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		kind, err := m.GetKind(pair[0])
		if err != nil {
			return 0, bound, boundRef, false, false, err
		}
		ground, err := m.IsGround(pair[1])
		if err != nil {
			return 0, bound, boundRef, false, false, err
		}
		if kind == ast.ASTVar && ground {
			index, err := m.GetVarIndex(pair[0])
			return index, pair[1], boundRef, false, true, err
		}
	}
	// expr_ref tmp: is_var_plus_ground(lhs, inv, v, tmp) && is_ground(rhs),
	// then the same with the sides swapped.
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return 0, bound, boundRef, false, false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&tmp)) }()
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		inv, v, found, err := g.finder.macroUtil.IsVarPlusGround(pair[0], tmp)
		if err != nil {
			return 0, bound, boundRef, false, false, err
		}
		if !found {
			continue
		}
		ground, err := m.IsGround(pair[1])
		if err != nil {
			return 0, bound, boundRef, false, false, err
		}
		if !ground {
			continue
		}
		offset, err := m.RefGet(tmp)
		if err != nil {
			return 0, bound, boundRef, false, false, err
		}
		if inv {
			boundRef, err = g.mutilArithRef(offset, pair[1], false)
		} else {
			boundRef, err = g.mutilArithRef(pair[1], offset, false)
		}
		if err != nil {
			return 0, bound, ast.ASTRef{}, false, false, err
		}
		if bound, err = m.RefGet(boundRef); err != nil {
			return 0, bound, boundRef, false, false, err
		}
		index, err := m.GetVarIndex(v)
		return index, bound, boundRef, inv, true, err
	}
	return 0, bound, boundRef, false, false, nil
}

func (g *smtModelFinderAUFGraph) variableDifference(term ast.ASTHandle) (first, second uint32, matched bool, result error) {
	m := g.finder.manager
	add, err := g.finder.macroUtil.IsAdd(term)
	if err != nil || !add {
		return 0, 0, false, err
	}
	n, err := m.GetNumArgs(term)
	if err != nil || n < 2 {
		return 0, 0, false, err
	}
	left, err := m.GetArg(term, 0)
	if err != nil {
		return 0, 0, false, err
	}
	right, err := m.GetArg(term, 1)
	if err != nil {
		return 0, 0, false, err
	}
	kind, err := m.GetKind(left)
	if err != nil {
		return 0, 0, false, err
	}
	if kind != ast.ASTVar {
		left, right = right, left
		kind, err = m.GetKind(left)
		if err != nil {
			return 0, 0, false, err
		}
	}
	if kind != ast.ASTVar {
		return 0, 0, false, nil
	}
	right, negative, err := g.finder.macroUtil.IsTimesMinusOne(right)
	if err != nil || !negative {
		return 0, 0, false, err
	}
	kind, err = m.GetKind(right)
	if err != nil || kind != ast.ASTVar {
		return 0, 0, false, err
	}
	first, err = m.GetVarIndex(left)
	if err != nil {
		return 0, 0, false, err
	}
	second, err = m.GetVarIndex(right)
	return first, second, true, err
}

func (g *smtModelFinderAUFGraph) variableAndVariable(left, right ast.ASTHandle) (first, second uint32, matched bool, result error) {
	m := g.finder.manager
	lk, err := m.GetKind(left)
	if err != nil {
		return 0, 0, false, err
	}
	rk, err := m.GetKind(right)
	if err != nil {
		return 0, 0, false, err
	}
	if lk == ast.ASTVar && rk == ast.ASTVar {
		first, err := m.GetVarIndex(left)
		if err != nil {
			return 0, 0, false, err
		}
		second, err := m.GetVarIndex(right)
		return first, second, true, err
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		first, second, found, err := g.variableDifference(pair[0])
		if err != nil {
			return 0, 0, false, err
		}
		if found {
			zero, err := g.finder.macroUtil.IsZero(pair[1])
			if err != nil || zero {
				return first, second, zero, err
			}
		}
	}
	return 0, 0, false, nil
}

func (g *smtModelFinderAUFGraph) collectComparison(quantifier, atom ast.ASTHandle, negative bool) error {
	comparison, err := g.finder.macroUtil.IsLEGE(atom)
	if err != nil || !comparison {
		return err
	}
	m := g.finder.manager
	left, err := m.GetArg(atom, 0)
	if err != nil {
		return err
	}
	right, err := m.GetArg(atom, 1)
	if err != nil {
		return err
	}
	if negative {
		first, second, matched, err := g.variableAndVariable(left, right)
		if err != nil {
			return err
		}
		if matched {
			kind := aufOrderedVariables
			signed, err := m.IsBitVectorOperation(atom, 23)
			if err != nil {
				return err
			}
			if signed {
				kind = aufSignedOrderedVariables
			}
			g.appendLiteral(smtModelFinderAUFLiteral{kind: kind, quantifier: quantifier, first: first, second: second})
			return nil
		}
	}
	variable, bound, boundRef, inverse, matched, err := g.variableAndGround(left, right)
	if err != nil || !matched {
		return errors.Join(err, releaseOptionalRef(m, &boundRef))
	}
	if !negative {
		// is_x_gle_t_atom: t = tmp +/- 1; tmp dies on return.
		le, err := g.finder.macroUtil.IsLE(atom)
		if err != nil {
			return errors.Join(err, releaseOptionalRef(m, &boundRef))
		}
		if inverse {
			le = !le
		}
		sort, err := m.GetSort(bound)
		if err != nil {
			return errors.Join(err, releaseOptionalRef(m, &boundRef))
		}
		one, err := m.MkDatalogNumeral(1, sort)
		if err != nil {
			return errors.Join(err, releaseOptionalRef(m, &boundRef))
		}
		ref, err := g.mutilArithRef(bound, one, le)
		if err = errors.Join(err, releaseOptionalRef(m, &boundRef)); err != nil {
			return errors.Join(err, releaseOptionalRef(m, &ref))
		}
		if bound, err = g.keep(ref); err != nil {
			return err
		}
	} else if boundRef != (ast.ASTRef{}) {
		if bound, err = g.keep(boundRef); err != nil {
			return err
		}
	}
	g.appendLiteral(smtModelFinderAUFLiteral{kind: aufOrderedGround, quantifier: quantifier, first: variable, term: bound})
	return nil
}

func releaseOptionalRef(m *ast.ASTManager, ref *ast.ASTRef) error {
	if *ref == (ast.ASTRef{}) {
		return nil
	}
	return m.DelRef(ref)
}
