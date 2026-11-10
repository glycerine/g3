package smtcore

// Source: src/smt/smt_model_finder.cpp quantifier_analyzer numeric atoms;
// x_leq_y, x_sleq_y and x_gle_t qinfo.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *smtModelFinderAUFGraph) normalizeGround(term ast.ASTHandle) (ast.ASTHandle, error) {
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager := g.finder.state.quantifierManager
	if manager == nil || manager.rewriter == nil {
		return ast.ASTHandle{}, errors.Join(ast.ErrASTHandle, m.DelRef(&ref))
	}
	if err = manager.rewriter.Rewrite(term, ref); err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
	}
	g.refs = append(g.refs, ref)
	return m.RefGet(ref)
}

// mutilArith is macro_util::mk_add/mk_sub(t1, t2, r): one step of the
// arithmetic rewriter (m_arith_rw), not rewritten further. Bit-vector
// operands keep the normalizing path. The result is held in g.refs.
func (g *smtModelFinderAUFGraph) mutilArith(t1, t2 ast.ASTHandle, plus bool) (ast.ASTHandle, error) {
	m := g.finder.manager
	sort, err := m.GetSort(t1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bv, err := m.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bv {
		var raw ast.ASTHandle
		if plus {
			raw, err = g.finder.macroUtil.MkAdd(t1, t2)
		} else {
			raw, err = g.finder.macroUtil.MkSub(t1, t2)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return g.normalizeGround(raw)
	}
	arith, err := g.localArithRewriter()
	if err != nil {
		return ast.ASTHandle{}, err
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
		return ast.ASTHandle{}, err
	}
	g.refs = append(g.refs, ref)
	return m.RefGet(ref)
}

// variableAndGround mirrors is_var_and_ground, including -x+k and
// occurrences on either side. The returned variable is a de Bruijn index.
func (g *smtModelFinderAUFGraph) variableAndGround(left, right ast.ASTHandle) (variable uint32, bound ast.ASTHandle, inverse, matched bool, result error) {
	m := g.finder.manager
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		kind, err := m.GetKind(pair[0])
		if err != nil {
			return 0, bound, false, false, err
		}
		ground, err := m.IsGround(pair[1])
		if err != nil {
			return 0, bound, false, false, err
		}
		if kind == ast.ASTVar && ground {
			index, err := m.GetVarIndex(pair[0])
			return index, pair[1], false, true, err
		}
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		ground, err := m.IsGround(pair[1])
		if err != nil {
			return 0, bound, false, false, err
		}
		if !ground {
			continue
		}
		offsetRef, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return 0, bound, false, false, err
		}
		inv, v, found, err := g.finder.macroUtil.IsVarPlusGround(pair[0], offsetRef)
		if err != nil || !found {
			if err = errors.Join(err, m.DelRef(&offsetRef)); err != nil {
				return 0, bound, false, false, err
			}
			continue
		}
		g.refs = append(g.refs, offsetRef)
		offset, err := m.RefGet(offsetRef)
		if err != nil {
			return 0, bound, false, false, err
		}
		if inv {
			bound, err = g.mutilArith(offset, pair[1], false)
		} else {
			bound, err = g.mutilArith(pair[1], offset, false)
		}
		if err != nil {
			return 0, bound, false, false, err
		}
		index, err := m.GetVarIndex(v)
		return index, bound, inv, true, err
	}
	return 0, bound, false, false, nil
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
	variable, bound, inverse, matched, err := g.variableAndGround(left, right)
	if err != nil || !matched {
		return err
	}
	if !negative {
		le, err := g.finder.macroUtil.IsLE(atom)
		if err != nil {
			return err
		}
		if inverse {
			le = !le
		}
		sort, err := m.GetSort(bound)
		if err != nil {
			return err
		}
		one, err := m.MkDatalogNumeral(1, sort)
		if err != nil {
			return err
		}
		bound, err = g.mutilArith(bound, one, le)
		if err != nil {
			return err
		}
	}
	g.appendLiteral(smtModelFinderAUFLiteral{kind: aufOrderedGround, quantifier: quantifier, first: variable, term: bound})
	return nil
}
