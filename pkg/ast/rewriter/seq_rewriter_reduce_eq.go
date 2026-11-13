package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp:1590-2110: reduce_eq (vector and
// expression forms), add_seqs, concat_non_empty, set_empty, min_length,
// is_string, solve_itos, length_constrained, is_subsequence.
//
// The vectors are the caller's expr_ref_vectors as plain slices: reduce_eq
// edits ls and rs in place (including swapping them) and appends to lhs and
// rhs. Terms it creates are returned in those slices; the caller holds them,
// as its expr_ref_vectors do.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const seqOpItos int32 = 27 // OP_STRING_ITOS

func (r *SequenceRewriter) isUnitArg(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	return r.manager.SequenceUnitArgument(e)
}

func (r *SequenceRewriter) isItos(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	return r.manager.SequenceUnaryOperationArgument(e, seqOpItos)
}

func (r *SequenceRewriter) mkChar(s string, idx int) (ast.ASTHandle, error) {
	return r.manager.BitVectorNumeralUint64(uint64(s[idx]), 8)
}

// mkConcatN is str.mk_concat(n, es), right associative.
func (r *SequenceRewriter) mkConcatN(es []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(es) == 1 {
		return es[0], nil
	}
	j := len(es) - 1
	acc, err := r.manager.MkSequenceConcat(es[j-1], es[j])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for j--; j > 0; {
		j--
		if acc, err = r.manager.MkSequenceConcat(es[j], acc); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return acc, nil
}

// The vectors are expr_ref_vectors: each element of ls, rs, lhs and rhs holds
// one reference, which pop, assignment and shrink release where the source
// does, and push acquires.
func (r *SequenceRewriter) vPush(v *[]ast.ASTHandle, h ast.ASTHandle) error {
	if err := r.manager.IncRef(h); err != nil {
		return err
	}
	*v = append(*v, h)
	return nil
}

func (r *SequenceRewriter) vPop(v *[]ast.ASTHandle) error {
	h := (*v)[len(*v)-1]
	*v = (*v)[:len(*v)-1]
	return r.manager.DecRef(h)
}

func (r *SequenceRewriter) vSet(v *[]ast.ASTHandle, i int, h ast.ASTHandle) error {
	if err := r.manager.IncRef(h); err != nil {
		return err
	}
	old := (*v)[i]
	(*v)[i] = h
	return r.manager.DecRef(old)
}

// vShrink releases the dropped suffix front to back.
func (r *SequenceRewriter) vShrink(v *[]ast.ASTHandle, n int) error {
	// DecRef touches only the AST manager, never v, so the dropped tail is
	// read in place from the old slice.
	dropped := (*v)[n:]
	*v = (*v)[:n]
	for _, h := range dropped {
		if err := r.manager.DecRef(h); err != nil {
			return err
		}
	}
	return nil
}

// ReleaseVec releases a vector filled by ReduceEqVec or AddSeqs.
func (r *SequenceRewriter) ReleaseVec(v *[]ast.ASTHandle) error { return r.vShrink(v, 0) }

// ReduceEqVec is reduce_eq(ls, rs, lhs, rhs, change). It returns false when
// the equation is inconsistent.
func (r *SequenceRewriter) ReduceEqVec(ls, rs, lhs, rhs *[]ast.ASTHandle, change *bool) (bool, error) {
	m := r.manager
	lchange := false
	isEmpty := func(e ast.ASTHandle) (bool, error) { return m.IsSequenceEmpty(e) }
	// solve from back
	for {
		for len(*rs) > 0 {
			e, err := isEmpty((*rs)[len(*rs)-1])
			if err != nil {
				return false, err
			}
			if !e {
				break
			}
			if err := r.vPop(rs); err != nil {
				return false, err
			}
		}
		for len(*ls) > 0 {
			e, err := isEmpty((*ls)[len(*ls)-1])
			if err != nil {
				return false, err
			}
			if !e {
				break
			}
			if err := r.vPop(ls); err != nil {
				return false, err
			}
		}
		if len(*ls) == 0 || len(*rs) == 0 {
			break
		}
		l := (*ls)[len(*ls)-1]
		rr := (*rs)[len(*rs)-1]
		_, rUnit, err := r.isUnitArg(rr)
		if err != nil {
			return false, err
		}
		_, lString, err := m.SequenceStringValue(l)
		if err != nil {
			return false, err
		}
		if rUnit && lString {
			l, rr = rr, l
			*ls, *rs = *rs, *ls
		}
		a, lu, err := r.isUnitArg(l)
		if err != nil {
			return false, err
		}
		b, ru, err := r.isUnitArg(rr)
		if err != nil {
			return false, err
		}
		s, rs1, err := m.SequenceStringValue(rr)
		if err != nil {
			return false, err
		}
		if l == rr {
			if err := r.vPop(ls); err != nil {
				return false, err
			}
			if err := r.vPop(rs); err != nil {
				return false, err
			}
		} else if lu && ru {
			d, err := m.AreDistinct(a, b)
			if err != nil {
				return false, err
			}
			if d {
				return false, nil
			}
			if err := r.vPush(lhs, a); err != nil {
				return false, err
			}
			if err := r.vPush(rhs, b); err != nil {
				return false, err
			}
			if err := r.vPop(ls); err != nil {
				return false, err
			}
			if err := r.vPop(rs); err != nil {
				return false, err
			}
		} else if lu && rs1 {
			ch, err := r.mkChar(s, len(s)-1)
			if err != nil {
				return false, err
			}
			if err := r.vPush(lhs, ch); err != nil {
				return false, err
			}
			if err := r.vPush(rhs, a); err != nil {
				return false, err
			}
			if err := r.vPop(ls); err != nil {
				return false, err
			}
			if len(s) == 1 {
				if err := r.vPop(rs); err != nil {
					return false, err
				}
			} else {
				s2, err := m.MkString(s[:len(s)-1])
				if err != nil {
					return false, err
				}
				if err := r.vSet(rs, len(*rs)-1, s2); err != nil {
					return false, err
				}
			}
		} else {
			break
		}
		*change = true
		lchange = true
	}

	// solve from front
	head1, head2 := 0, 0
	for {
		for head1 < len(*ls) {
			e, err := isEmpty((*ls)[head1])
			if err != nil {
				return false, err
			}
			if !e {
				break
			}
			head1++
		}
		for head2 < len(*rs) {
			e, err := isEmpty((*rs)[head2])
			if err != nil {
				return false, err
			}
			if !e {
				break
			}
			head2++
		}
		if head1 == len(*ls) || head2 == len(*rs) {
			break
		}
		l := (*ls)[head1]
		rr := (*rs)[head2]
		_, rUnit, err := r.isUnitArg(rr)
		if err != nil {
			return false, err
		}
		_, lString, err := m.SequenceStringValue(l)
		if err != nil {
			return false, err
		}
		if rUnit && lString {
			l, rr = rr, l
			*ls, *rs = *rs, *ls
			head1, head2 = head2, head1
		}
		a, lu, err := r.isUnitArg(l)
		if err != nil {
			return false, err
		}
		b, ru, err := r.isUnitArg(rr)
		if err != nil {
			return false, err
		}
		s, rs1, err := m.SequenceStringValue(rr)
		if err != nil {
			return false, err
		}
		if l == rr {
			head1++
			head2++
		} else if lu && ru {
			d, err := m.AreDistinct(a, b)
			if err != nil {
				return false, err
			}
			if d {
				return false, nil
			}
			if err := r.vPush(lhs, a); err != nil {
				return false, err
			}
			if err := r.vPush(rhs, b); err != nil {
				return false, err
			}
			head1++
			head2++
		} else if lu && rs1 {
			ch, err := r.mkChar(s, 0)
			if err != nil {
				return false, err
			}
			if err := r.vPush(lhs, ch); err != nil {
				return false, err
			}
			if err := r.vPush(rhs, a); err != nil {
				return false, err
			}
			head1++
			if len(s) == 1 {
				head2++
			} else {
				s2, err := m.MkString(s[1:])
				if err != nil {
					return false, err
				}
				if err := r.vSet(rs, head2, s2); err != nil {
					return false, err
				}
			}
		} else {
			break
		}
		*change = true
		lchange = true
	}
	// reduce strings
	for head1 < len(*ls) && head2 < len(*rs) {
		s1, ok1, err := m.SequenceStringValue((*ls)[head1])
		if err != nil {
			return false, err
		}
		if !ok1 {
			break
		}
		s2, ok2, err := m.SequenceStringValue((*rs)[head2])
		if err != nil {
			return false, err
		}
		if !ok2 {
			break
		}
		l := min(len(s1), len(s2))
		for i := 0; i < l; i++ {
			if s1[i] != s2[i] {
				return false, nil
			}
		}
		if l == len(s1) {
			head1++
		} else {
			h, err := m.MkString(s1[l:])
			if err != nil {
				return false, err
			}
			if err = r.vSet(ls, head1, h); err != nil {
				return false, err
			}
		}
		if l == len(s2) {
			head2++
		} else {
			h, err := m.MkString(s2[l:])
			if err != nil {
				return false, err
			}
			if err = r.vSet(rs, head2, h); err != nil {
				return false, err
			}
		}
		*change = true
		lchange = true
	}
	for head1 < len(*ls) && head2 < len(*rs) {
		s1, ok1, err := m.SequenceStringValue((*ls)[len(*ls)-1])
		if err != nil {
			return false, err
		}
		if !ok1 {
			break
		}
		s2, ok2, err := m.SequenceStringValue((*rs)[len(*rs)-1])
		if err != nil {
			return false, err
		}
		if !ok2 {
			break
		}
		l := min(len(s1), len(s2))
		for i := 0; i < l; i++ {
			if s1[len(s1)-i-1] != s2[len(s2)-i-1] {
				return false, nil
			}
		}
		if err := r.vPop(ls); err != nil {
			return false, err
		}
		if err := r.vPop(rs); err != nil {
			return false, err
		}
		if l < len(s1) {
			h, err := m.MkString(s1[:len(s1)-l])
			if err != nil {
				return false, err
			}
			if err := r.vPush(ls, h); err != nil {
				return false, err
			}
		}
		if l < len(s2) {
			h, err := m.MkString(s2[:len(s2)-l])
			if err != nil {
				return false, err
			}
			if err := r.vPush(rs, h); err != nil {
				return false, err
			}
		}
		*change = true
		lchange = true
	}

	isSat := true
	_ls, _rs := (*ls)[head1:], (*rs)[head2:]
	szl, szr := len(_ls), len(_rs)

	if ok, err := r.solveItos(_ls, _rs, lhs, rhs, &isSat); err != nil {
		return false, err
	} else if ok {
		if err := r.vShrink(ls, 0); err != nil {
			return false, err
		}
		if err := r.vShrink(rs, 0); err != nil {
			return false, err
		}
		*change = true
		return isSat, nil
	}
	if ok, err := r.lengthConstrained(_ls, _rs, lhs, rhs, &isSat); err != nil {
		return false, err
	} else if ok {
		if err := r.vShrink(ls, 0); err != nil {
			return false, err
		}
		if err := r.vShrink(rs, 0); err != nil {
			return false, err
		}
		*change = true
		return isSat, nil
	}
	if szr == 0 && szl == 0 {
		if err := r.vShrink(ls, 0); err != nil {
			return false, err
		}
		if err := r.vShrink(rs, 0); err != nil {
			return false, err
		}
		return true, nil
	}
	if szr == 0 && szl > 0 {
		szr, szl = szl, szr
		_ls, _rs = _rs, _ls
	}
	if szl == 0 && szr > 0 {
		ok, err := r.setEmpty(_rs, true, lhs, rhs)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		lchange = lchange || szr > 1
		*change = *change || szr > 1
		if szr == 1 && !lchange {
			if err := r.vShrink(lhs, 0); err != nil {
				return false, err
			}
			if err := r.vShrink(rhs, 0); err != nil {
				return false, err
			}
		} else {
			if err := r.vShrink(ls, 0); err != nil {
				return false, err
			}
			if err := r.vShrink(rs, 0); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if ok, err := r.isSubsequence(_ls, _rs, lhs, rhs, &isSat); err != nil {
		return false, err
	} else if ok {
		if err := r.vShrink(ls, 0); err != nil {
			return false, err
		}
		if err := r.vShrink(rs, 0); err != nil {
			return false, err
		}
		*change = true
		return isSat, nil
	}
	if lchange {
		if head1 > 0 {
			for i := 0; i < szl; i++ {
				if err := r.vSet(ls, i, (*ls)[i+head1]); err != nil {
					return false, err
				}
			}
		}
		if err := r.vShrink(ls, szl); err != nil {
			return false, err
		}
		if head2 > 0 {
			for i := 0; i < szr; i++ {
				if err := r.vSet(rs, i, (*rs)[i+head2]); err != nil {
					return false, err
				}
			}
		}
		if err := r.vShrink(rs, szr); err != nil {
			return false, err
		}
	}
	*change = *change || lchange
	return true, nil
}

// AddSeqs is add_seqs(ls, rs, lhs, rhs).
func (r *SequenceRewriter) AddSeqs(ls, rs []ast.ASTHandle, lhs, rhs *[]ast.ASTHandle) error {
	m := r.manager
	switch {
	case len(ls) == 0 && len(rs) != 0:
		c, err := r.mkConcatN(rs)
		if err != nil {
			return err
		}
		if err := r.vPush(rhs, c); err != nil {
			return err
		}
		s, err := m.GetSort(c)
		if err != nil {
			return err
		}
		e, err := m.MkSequenceEmpty(s)
		if err != nil {
			return err
		}
		if err := r.vPush(lhs, e); err != nil {
			return err
		}
	case len(rs) == 0 && len(ls) != 0:
		c, err := r.mkConcatN(ls)
		if err != nil {
			return err
		}
		if err := r.vPush(lhs, c); err != nil {
			return err
		}
		s, err := m.GetSort(c)
		if err != nil {
			return err
		}
		e, err := m.MkSequenceEmpty(s)
		if err != nil {
			return err
		}
		if err := r.vPush(rhs, e); err != nil {
			return err
		}
	case len(rs) != 0 && len(ls) != 0:
		c, err := r.mkConcatN(ls)
		if err != nil {
			return err
		}
		if err := r.vPush(lhs, c); err != nil {
			return err
		}
		if c, err = r.mkConcatN(rs); err != nil {
			return err
		}
		if err := r.vPush(rhs, c); err != nil {
			return err
		}
	}
	return nil
}

// ReduceEqExpr is reduce_eq(l, r, lhs, rhs, changed). The flattened words
// live in the m_lhs and m_rhs members, which keep them (and any remainder
// reduce_eq created) referenced after the call.
func (r *SequenceRewriter) ReduceEqExpr(l, rr ast.ASTHandle, lhs, rhs *[]ast.ASTHandle, changed *bool) (bool, error) {
	m := r.manager
	if err := r.lhsMember.reset(m); err != nil {
		return false, err
	}
	if err := r.rhsMember.reset(m); err != nil {
		return false, err
	}
	ls, err := m.SequenceFlattenConcat(l)
	if err != nil {
		return false, err
	}
	for _, h := range ls {
		if err = r.lhsMember.pushBack(m, h); err != nil {
			return false, err
		}
	}
	rs, err := m.SequenceFlattenConcat(rr)
	if err != nil {
		return false, err
	}
	for _, h := range rs {
		if err = r.rhsMember.pushBack(m, h); err != nil {
			return false, err
		}
	}
	change := false
	ok, err := r.ReduceEqVec(&r.lhsMember.items, &r.rhsMember.items, lhs, rhs, &change)
	if err != nil || !ok {
		return false, err
	}
	if !change {
		if err = r.vPush(lhs, l); err != nil {
			return false, err
		}
		if err = r.vPush(rhs, rr); err != nil {
			return false, err
		}
	} else if err = r.AddSeqs(r.lhsMember.items, r.rhsMember.items, lhs, rhs); err != nil {
		return false, err
	}
	*changed = *changed || change
	return true, nil
}

func (r *SequenceRewriter) concatNonEmptyN(as []ast.ASTHandle) (ast.ASTHandle, error) {
	var bs []ast.ASTHandle
	for _, a := range as {
		_, u, err := r.isUnitArg(a)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		_, s, err := r.manager.SequenceStringValue(a)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if u || s {
			bs = append(bs, a)
		}
	}
	if len(bs) == 0 {
		s, err := r.manager.GetSort(as[0])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return r.manager.MkSequenceEmpty(s)
	}
	return r.mkConcatN(bs)
}

func (r *SequenceRewriter) setEmpty(es []ast.ASTHandle, all bool, lhs, rhs *[]ast.ASTHandle) (bool, error) {
	m := r.manager
	var emp ast.ASTHandle
	for _, e := range es {
		_, u, err := r.isUnitArg(e)
		if err != nil {
			return false, err
		}
		if u {
			if all {
				return false, nil
			}
			continue
		}
		empty, err := m.IsSequenceEmpty(e)
		if err != nil {
			return false, err
		}
		if empty {
			continue
		}
		_, str, err := m.SequenceStringValue(e)
		if err != nil {
			return false, err
		}
		if str {
			if all {
				return false, nil
			}
			continue
		}
		if emp.IsNull() {
			s, err := m.GetSort(e)
			if err != nil {
				return false, err
			}
			if emp, err = m.MkSequenceEmpty(s); err != nil {
				return false, err
			}
		}
		if err := r.vPush(lhs, emp); err != nil {
			return false, err
		}
		if err := r.vPush(rhs, e); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *SequenceRewriter) minLength(es []ast.ASTHandle) (int, bool, error) {
	m := r.manager
	bounded, n := true, 0
	for _, e := range es {
		_, u, err := r.isUnitArg(e)
		if err != nil {
			return 0, false, err
		}
		if u {
			n++
			continue
		}
		empty, err := m.IsSequenceEmpty(e)
		if err != nil {
			return 0, false, err
		}
		if empty {
			continue
		}
		s, str, err := m.SequenceStringValue(e)
		if err != nil {
			return 0, false, err
		}
		if str {
			n += len(s)
		} else {
			bounded = false
		}
	}
	return n, bounded, nil
}

// isStringN is is_string(n, es, s): concatenated string constants and
// numeral units.
func (r *SequenceRewriter) isStringN(es []ast.ASTHandle) (string, bool, error) {
	var out []byte
	for _, e := range es {
		s1, ok, err := r.manager.SequenceStringValue(e)
		if err != nil {
			return "", false, err
		}
		if ok {
			out = append(out, s1...)
			continue
		}
		// is_unit(es[i], e) && bv.is_numeral(e, val, sz)
		element, unit, err := r.manager.SequenceUnitArgument(e)
		if err != nil {
			return "", false, err
		}
		if !unit {
			return "", false, nil
		}
		b, ok, err := r.manager.SequenceUnitByteElementValue(element)
		if err != nil {
			return "", false, err
		}
		if !ok {
			return "", false, nil
		}
		out = append(out, b)
	}
	return string(out), true, nil
}

func (r *SequenceRewriter) solveItos(ls, rs []ast.ASTHandle, lhs, rhs *[]ast.ASTHandle, isSat *bool) (bool, error) {
	*isSat = true
	if len(ls) == 1 {
		l, ok, err := r.isItos(ls[0])
		if err != nil {
			return false, err
		}
		if ok {
			if len(rs) == 1 {
				rr, ok2, err := r.isItos(rs[0])
				if err != nil {
					return false, err
				}
				if ok2 {
					if err := r.vPush(lhs, l); err != nil {
						return false, err
					}
					if err := r.vPush(rhs, rr); err != nil {
						return false, err
					}
					return true, nil
				}
			}
			s, ok3, err := r.isStringN(rs)
			if err != nil {
				return false, err
			}
			if ok3 {
				if v, parsed := new(big.Int).SetString(s, 10); parsed && v.String() == s {
					n, err := r.manager.ArithmeticIntegerNumeralText(v.String())
					if err != nil {
						return false, err
					}
					if err := r.vPush(lhs, l); err != nil {
						return false, err
					}
					if err := r.vPush(rhs, n); err != nil {
						return false, err
					}
					return true, nil
				}
			}
		}
	}
	if len(rs) == 1 && len(ls) >= 1 {
		_, rIt, err := r.isItos(rs[0])
		if err != nil {
			return false, err
		}
		_, lIt, err := r.isItos(ls[0])
		if err != nil {
			return false, err
		}
		if rIt && !lIt {
			return r.solveItos(rs, ls, rhs, lhs, isSat)
		}
	}
	return false, nil
}

func (r *SequenceRewriter) lengthConstrained(l, rr []ast.ASTHandle, lhs, rhs *[]ast.ASTHandle, isSat *bool) (bool, error) {
	*isSat = true
	len1, bounded1, err := r.minLength(l)
	if err != nil {
		return false, err
	}
	len2, bounded2, err := r.minLength(rr)
	if err != nil {
		return false, err
	}
	if bounded1 && len1 < len2 {
		*isSat = false
		return true, nil
	}
	if bounded2 && len2 < len1 {
		*isSat = false
		return true, nil
	}
	emptySide := []ast.ASTHandle(nil)
	if bounded1 && len1 == len2 && len1 > 0 {
		emptySide = rr
	} else if bounded2 && len1 == len2 && len1 > 0 {
		emptySide = l
	} else {
		return false, nil
	}
	ok, err := r.setEmpty(emptySide, false, lhs, rhs)
	if err != nil {
		return false, err
	}
	*isSat = ok
	if ok {
		a, err := r.concatNonEmptyN(l)
		if err != nil {
			return false, err
		}
		if err := r.vPush(lhs, a); err != nil {
			return false, err
		}
		b, err := r.concatNonEmptyN(rr)
		if err != nil {
			return false, err
		}
		if err := r.vPush(rhs, b); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *SequenceRewriter) isSubsequence(l, rr []ast.ASTHandle, lhs, rhs *[]ast.ASTHandle, isSat *bool) (bool, error) {
	*isSat = true
	if len(l) == len(rr) {
		return false, nil
	}
	if len(rr) < len(l) {
		l, rr = rr, l
	}
	rpos := make(map[int]bool)
	for i := range l {
		found := false
		j := 0
		_, isUnit, err := r.isUnitArg(l[i])
		if err != nil {
			return false, err
		}
		for ; !found && j < len(rr); j++ {
			if rpos[j] {
				continue
			}
			if l[i] == rr[j] {
				found = true
				continue
			}
			if isUnit {
				_, u, err := r.isUnitArg(rr[j])
				if err != nil {
					return false, err
				}
				found = u
			}
		}
		if !found {
			return false, nil
		}
		rpos[j-1] = true
	}
	change := false
	var rs []ast.ASTHandle
	for j := range rr {
		if rpos[j] {
			rs = append(rs, rr[j])
			continue
		}
		ok, err := r.setEmpty(rr[j:j+1], true, lhs, rhs)
		if err != nil {
			return false, err
		}
		if !ok {
			*isSat = false
			return true, nil
		}
		change = true
	}
	if !change {
		return false, nil
	}
	if len(l) > 0 {
		a, err := r.mkConcatN(l)
		if err != nil {
			return false, err
		}
		if err := r.vPush(lhs, a); err != nil {
			return false, err
		}
		b, err := r.mkConcatN(rs)
		if err != nil {
			return false, err
		}
		if err := r.vPush(rhs, b); err != nil {
			return false, err
		}
	}
	return true, nil
}
