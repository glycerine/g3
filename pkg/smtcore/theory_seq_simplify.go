package smtcore

// Source: src/smt/theory_seq.cpp mk_sub and mk_simplified_literal, which
// pass arithmetic terms and comparisons through theory_seq::m_rewrite (a
// default th_rewriter) before they reach theory_arith. theory_arith only
// accepts simplified atoms t <= k / t >= k with a numeral k.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// seqRewrite is m_rewrite(e). The result stays referenced by the theory
// (released on Close), as the source's expr_ref values stay alive in the
// axioms that use them.
func (t *SMTSequenceTheory) seqRewrite(e ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m := state.manager
	rw, err := t.equalityRewriter() // theory_seq::m_rewrite
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = rw.Rewrite(e, ref); err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
	}
	t.rewriteRefs = append(t.rewriteRefs, ref)
	return m.RefGet(ref)
}

// seqSub is mk_sub(a, b): the rewritten a - b.
func (t *SMTSequenceTheory) seqSub(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	sub, err := m.MkArithmeticSub(a, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.seqRewrite(sub)
}

// seqCompareGE is mk_simplified_literal(mk_ge(mk_sub(a, b), 0)), the form
// in which the source states a >= b for non-numeral b.
func (t *SMTSequenceTheory) seqCompareGE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return t.seqCompare(true, a, b)
}

// seqCompareLE is mk_simplified_literal(mk_le(mk_sub(a, b), 0)).
func (t *SMTSequenceTheory) seqCompareLE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return t.seqCompare(false, a, b)
}

func (t *SMTSequenceTheory) seqCompare(ge bool, a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	diff, err := t.seqSub(a, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := t.integer(0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var atom ast.ASTHandle
	if ge {
		atom, err = m.MkArithmeticGE(diff, zero)
	} else {
		atom, err = m.MkArithmeticLE(diff, zero)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.seqRewrite(atom)
}

func (t *SMTSequenceTheory) releaseRewriteRefs() (result error) {
	for i := len(t.rewriteRefs) - 1; i >= 0; i-- {
		result = errors.Join(result, t.state.manager.DelRef(&t.rewriteRefs[i]))
	}
	t.rewriteRefs = nil
	return result
}
