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
			*rs = (*rs)[:len(*rs)-1]
		}
		for len(*ls) > 0 {
			e, err := isEmpty((*ls)[len(*ls)-1])
			if err != nil {
				return false, err
			}
			if !e {
				break
			}
			*ls = (*ls)[:len(*ls)-1]
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
			*ls = (*ls)[:len(*ls)-1]
			*rs = (*rs)[:len(*rs)-1]
		} else if lu && ru {
			d, err := m.AreDistinct(a, b)
			if err != nil {
				return false, err
			}
			if d {
				return false, nil
			}
			*lhs = append(*lhs, a)
			*rhs = append(*rhs, b)
			*ls = (*ls)[:len(*ls)-1]
			*rs = (*rs)[:len(*rs)-1]
		} else if lu && rs1 {
			ch, err := r.mkChar(s, len(s)-1)
			if err != nil {
				return false, err
			}
			*lhs = append(*lhs, ch)
			*rhs = append(*rhs, a)
			*ls = (*ls)[:len(*ls)-1]
			if len(s) == 1 {
				*rs = (*rs)[:len(*rs)-1]
			} else {
				s2, err := m.MkString(s[:len(s)-1])
				if err != nil {
					return false, err
				}
				(*rs)[len(*rs)-1] = s2
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
			*lhs = append(*lhs, a)
			*rhs = append(*rhs, b)
			head1++
			head2++
		} else if lu && rs1 {
			ch, err := r.mkChar(s, 0)
			if err != nil {
				return false, err
			}
			*lhs = append(*lhs, ch)
			*rhs = append(*rhs, a)
			head1++
			if len(s) == 1 {
				head2++
			} else {
				s2, err := m.MkString(s[1:])
				if err != nil {
					return false, err
				}
				(*rs)[head2] = s2
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
		} else if (*ls)[head1], err = m.MkString(s1[l:]); err != nil {
			return false, err
		}
		if l == len(s2) {
			head2++
		} else if (*rs)[head2], err = m.MkString(s2[l:]); err != nil {
			return false, err
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
		*ls = (*ls)[:len(*ls)-1]
		*rs = (*rs)[:len(*rs)-1]
		if l < len(s1) {
			h, err := m.MkString(s1[:len(s1)-l])
			if err != nil {
				return false, err
			}
			*ls = append(*ls, h)
		}
		if l < len(s2) {
			h, err := m.MkString(s2[:len(s2)-l])
			if err != nil {
				return false, err
			}
			*rs = append(*rs, h)
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
		*ls, *rs = (*ls)[:0], (*rs)[:0]
		*change = true
		return isSat, nil
	}
	if ok, err := r.lengthConstrained(_ls, _rs, lhs, rhs, &isSat); err != nil {
		return false, err
	} else if ok {
		*ls, *rs = (*ls)[:0], (*rs)[:0]
		*change = true
		return isSat, nil
	}
	if szr == 0 && szl == 0 {
		*ls, *rs = (*ls)[:0], (*rs)[:0]
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
			*lhs, *rhs = (*lhs)[:0], (*rhs)[:0]
		} else {
			*ls, *rs = (*ls)[:0], (*rs)[:0]
		}
		return true, nil
	}
	if ok, err := r.isSubsequence(_ls, _rs, lhs, rhs, &isSat); err != nil {
		return false, err
	} else if ok {
		*ls, *rs = (*ls)[:0], (*rs)[:0]
		*change = true
		return isSat, nil
	}
	if lchange {
		if head1 > 0 {
			copy(*ls, (*ls)[head1:head1+szl])
		}
		*ls = (*ls)[:szl]
		if head2 > 0 {
			copy(*rs, (*rs)[head2:head2+szr])
		}
		*rs = (*rs)[:szr]
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
		*rhs = append(*rhs, c)
		s, err := m.GetSort(c)
		if err != nil {
			return err
		}
		e, err := m.MkSequenceEmpty(s)
		if err != nil {
			return err
		}
		*lhs = append(*lhs, e)
	case len(rs) == 0 && len(ls) != 0:
		c, err := r.mkConcatN(ls)
		if err != nil {
			return err
		}
		*lhs = append(*lhs, c)
		s, err := m.GetSort(c)
		if err != nil {
			return err
		}
		e, err := m.MkSequenceEmpty(s)
		if err != nil {
			return err
		}
		*rhs = append(*rhs, e)
	case len(rs) != 0 && len(ls) != 0:
		c, err := r.mkConcatN(ls)
		if err != nil {
			return err
		}
		*lhs = append(*lhs, c)
		if c, err = r.mkConcatN(rs); err != nil {
			return err
		}
		*rhs = append(*rhs, c)
	}
	return nil
}

// ReduceEqExpr is reduce_eq(l, r, lhs, rhs, changed).
func (r *SequenceRewriter) ReduceEqExpr(l, rr ast.ASTHandle, lhs, rhs *[]ast.ASTHandle, changed *bool) (bool, error) {
	ls, err := r.manager.SequenceFlattenConcat(l)
	if err != nil {
		return false, err
	}
	rs, err := r.manager.SequenceFlattenConcat(rr)
	if err != nil {
		return false, err
	}
	change := false
	ok, err := r.ReduceEqVec(&ls, &rs, lhs, rhs, &change)
	if err != nil || !ok {
		return false, err
	}
	if !change {
		*lhs = append(*lhs, l)
		*rhs = append(*rhs, rr)
	} else if err = r.AddSeqs(ls, rs, lhs, rhs); err != nil {
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
		*lhs = append(*lhs, emp)
		*rhs = append(*rhs, e)
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
		b, ok, err := r.manager.SequenceUnitByteElementValue(e)
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
					*lhs = append(*lhs, l)
					*rhs = append(*rhs, rr)
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
					*lhs = append(*lhs, l)
					*rhs = append(*rhs, n)
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
		*lhs = append(*lhs, a)
		b, err := r.concatNonEmptyN(rr)
		if err != nil {
			return false, err
		}
		*rhs = append(*rhs, b)
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
		*lhs = append(*lhs, a)
		b, err := r.mkConcatN(rs)
		if err != nil {
			return false, err
		}
		*rhs = append(*rhs, b)
	}
	return true, nil
}
