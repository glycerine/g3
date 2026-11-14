package smtcore

// Source: src/smt/theory_str.cpp infer_len_concat_arg,
// infer_len_concat_equality and the active simplify_concat_equality dispatch.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) inferStringConcatArgument(concat ast.ASTHandle, total *big.Int) (result error) {
	if total.Sign() < 0 {
		return nil
	}
	m := t.state.manager
	first, second, isConcat, err := m.SequenceConcatParts(concat)
	if err != nil || !isConcat {
		return err
	}
	left, lk, err := t.lengthValue(first)
	if err != nil {
		return err
	}
	right, rk, err := t.lengthValue(second)
	if err != nil || lk == rk {
		return err
	}
	missing, known, value := first, second, right
	if lk {
		missing, known, value = second, first, left
	}
	difference := new(big.Int).Sub(total, value)
	if difference.Sign() < 0 {
		return nil
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.length(concat, total); err != nil {
		return err
	}
	if err := premises.length(known, value); err != nil {
		return err
	}
	conclusion := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusion.close()) }()
	if err := conclusion.length(missing, difference); err != nil {
		return err
	}
	return premises.imply(conclusion.values[0])
}

func (t *SMTStringTheory) inferStringConcatEquality(left, right ast.ASTHandle) error {
	total, known, err := t.lengthValue(left)
	if err != nil {
		return err
	}
	if !known {
		total, known, err = t.lengthValue(right)
		if err != nil {
			return err
		}
	}
	// Native infer_len_concat's assertion branch is unreachable with the
	// recursive get_len_value: known children imply a known concat sum.
	if !known {
		return nil
	}
	if err := t.inferStringConcatArgument(left, total); err != nil {
		return err
	}
	return t.inferStringConcatArgument(right, total)
}

// Native shared-concat groups skip step 1 entirely. Otherwise select the
// first nonoverlapping pair, falling back to the first pair if all overlap.
func (t *SMTStringTheory) simplifyStringConcatGroups(left, right smtStringClassGroups) error {
	for _, l := range left.concats {
		for _, r := range right.concats {
			if l == r {
				return nil
			}
		}
	}
	if len(left.concats) == 0 || len(right.concats) == 0 {
		return nil
	}
	for _, l := range left.concats {
		for _, r := range right.concats {
			overlap, err := t.stringConcatOverlap(l, r)
			if err != nil {
				return err
			}
			if !overlap {
				return t.simplifyStringConcatEquality(l, r)
			}
		}
	}
	return t.simplifyStringConcatEquality(left.concats[0], right.concats[0])
}

func (t *SMTStringTheory) simplifyStringConcatEquality(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(left)
	if err != nil {
		return err
	}
	a, b, rc, err := m.SequenceConcatParts(right)
	if err != nil || !lc || !rc {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	// Snapshot all four lengths before inference, as in the source.
	lengths := [4]*big.Int{}
	known := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		lengths[i], known[i], err = t.lengthValue(expression)
		if err != nil {
			return err
		}
	}
	if err := t.inferStringConcatEquality(left, right); err != nil {
		return err
	}
	// The C++ creates ctx.mk_eq_atom(nn1, nn2) only inside the branch that
	// uses it, and GCC evaluates call arguments right to left. Branch-local
	// expr_refs are released in reverse declaration order.
	scope := smtStringScope{theory: t}
	defer func() { result = errors.Join(result, scope.close()) }()
	temporaries := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, temporaries.close()) }()
	// ctx.mk_eq_atom(mk_strlen(first), mk_strlen(second)).
	lengthEquality := func(first, second ast.ASTHandle) (ast.ASTHandle, error) {
		r, err := m.MkSequenceLength(second)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		l, err := m.MkSequenceLength(first)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return t.state.MkEqAtom(l, r)
	}
	// a1_arg0 == a2_arg0 (or a1_arg1 == a2_arg1): premise(eq(nn1, nn2)),
	// eq1(eq(first, second)), eq2(eq(len first, len second)),
	// conclusion(m.mk_and(eq1, eq2)).
	cancel := func(first, second ast.ASTHandle) error {
		premise, err := scope.own(t.state.MkEqAtom(left, right))
		if err != nil {
			return err
		}
		eq1, err := scope.own(t.state.MkEqAtom(first, second))
		if err != nil {
			return err
		}
		eq2, err := scope.own(lengthEquality(first, second))
		if err != nil {
			return err
		}
		conclusion, err := scope.own(m.MkAnd(eq1, eq2))
		if err != nil {
			return err
		}
		return t.assertImplication(premise, conclusion)
	}
	if x == a {
		same, err := t.sameStringClass(y, b)
		if err != nil || same {
			return err
		}
		return cancel(y, b)
	}
	if y == b {
		same, err := t.sameStringClass(x, a)
		if err != nil || same {
			return err
		}
		return cancel(x, a)
	}
	// Quick path: premise(m.mk_and(eq(nn1, nn2), eq(s0, s1))) and
	// conclusion(m.mk_and(eq(o0, o1), eq(len o0, len o1))), each built
	// right to left.
	quick := func(s0, s1, o0, o1 ast.ASTHandle) error {
		sameEquality, err := t.state.MkEqAtom(s0, s1)
		if err != nil {
			return err
		}
		parent, err := t.state.MkEqAtom(left, right)
		if err != nil {
			return err
		}
		premise, err := scope.own(m.MkAnd(parent, sameEquality))
		if err != nil {
			return err
		}
		otherLength, err := lengthEquality(o0, o1)
		if err != nil {
			return err
		}
		otherEquality, err := t.state.MkEqAtom(o0, o1)
		if err != nil {
			return err
		}
		conclusion, err := scope.own(m.MkAnd(otherEquality, otherLength))
		if err != nil {
			return err
		}
		return t.assertImplication(premise, conclusion)
	}
	sameFirst, err := t.sameStringClass(x, a)
	if err != nil {
		return err
	}
	sameSecond, err := t.sameStringClass(y, b)
	if err != nil {
		return err
	}
	if sameFirst {
		if sameSecond {
			return nil
		}
		return quick(x, a, y, b)
	}
	if sameSecond {
		return quick(y, b, x, a)
	}
	// Quick paths 2-1 and 2-2 (opt_NoQuickReturn_IntegerTheory is false).
	for i, pair := range [][2]ast.ASTHandle{{x, a}, {y, b}} {
		if !known[i] || !known[i+2] || lengths[i].Cmp(lengths[i+2]) != 0 {
			continue
		}
		same, err := t.sameStringClass(pair[0], pair[1])
		if err != nil {
			return err
		}
		if same {
			continue
		}
		left1, err := scope.own(t.state.MkEqAtom(left, right))
		if err != nil {
			return err
		}
		left2, err := scope.own(lengthEquality(pair[0], pair[1]))
		if err != nil {
			return err
		}
		right1, err := scope.own(t.state.MkEqAtom(x, a))
		if err != nil {
			return err
		}
		right2, err := scope.own(t.state.MkEqAtom(y, b))
		if err != nil {
			return err
		}
		premise, err := scope.own(m.MkAnd(left1, left2))
		if err != nil {
			return err
		}
		conclusion, err := scope.own(m.MkAnd(right1, right2))
		if err != nil {
			return err
		}
		return t.assertImplication(premise, conclusion)
	}
	newLeft, err := t.simplifyStringConcat(left)
	if err != nil {
		return err
	}
	if err := temporaries.add(newLeft); err != nil {
		return err
	}
	newRight, err := t.simplifyStringConcat(right)
	if err != nil || newLeft == newRight {
		return err
	}
	if err := temporaries.add(newRight); err != nil {
		return err
	}
	possible, err := t.canTwoNodesEqual(newLeft, newRight)
	if err != nil {
		return err
	}
	if !possible {
		equality, err := t.state.MkEqAtom(newLeft, newRight)
		if err != nil {
			return err
		}
		if err := temporaries.add(equality); err != nil {
			return err
		}
		negative, err := m.MkNot(equality)
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	x, y, lc, err = m.SequenceConcatParts(newLeft)
	if err != nil {
		return err
	}
	a, b, rc, err = m.SequenceConcatParts(newRight)
	if err != nil {
		return err
	}
	if !lc || !rc {
		if _, literal, err := m.SequenceStringValue(newLeft); err != nil {
			return err
		} else if literal && rc {
			return t.simplifyStringParents(newRight, newLeft)
		}
		if _, literal, err := m.SequenceStringValue(newRight); err != nil {
			return err
		} else if literal && lc {
			return t.simplifyStringParents(newLeft, newRight)
		}
		return nil
	}
	same, err := t.sameStringClass(newLeft, newRight)
	if err != nil {
		return err
	}
	if !same && (left != newLeft || right != newRight) {
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		for _, pair := range [][2]ast.ASTHandle{{left, newLeft}, {right, newRight}, {left, right}} {
			if err := guards.equate(pair[0], pair[1]); err != nil {
				return err
			}
		}
		equality, err := t.state.MkEqAtom(newLeft, newRight)
		if err != nil {
			return err
		}
		if err := guards.imply(equality); err != nil {
			return err
		}
	}
	allVariables := true
	literals := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		if err := t.initializeStringCut(expression); err != nil {
			return err
		}
		_, literal, err := m.SequenceStringValue(expression)
		if err != nil {
			return err
		}
		allVariables = allVariables && !literal
		literals[i] = literal
	}
	if allVariables {
		return t.arrangeVariableConcats(newLeft, newRight)
	}
	if literals == [4]bool{false, false, false, true} || literals == [4]bool{false, true, false, false} {
		return t.arrangeLiteralSuffixConcats(newLeft, newRight)
	}
	if literals == [4]bool{true, false, false, false} || literals == [4]bool{false, false, true, false} {
		return t.arrangeLiteralPrefixConcats(newLeft, newRight)
	}
	if literals == [4]bool{true, false, true, false} {
		return t.arrangeMatchingLiteralBoundaries(newLeft, newRight, false)
	}
	if literals == [4]bool{false, true, false, true} {
		return t.arrangeMatchingLiteralBoundaries(newLeft, newRight, true)
	}
	if literals == [4]bool{true, false, false, true} || literals == [4]bool{false, true, true, false} {
		return t.arrangeOpposingLiteralBoundaries(newLeft, newRight)
	}
	return nil
}

// checkConcatLenInEqc is check_concat_len_in_eqc: infer_len_concat on each
// concat of the class, from concat around get_eqc_next. Its result is unused
// by new_eq_check, its only active caller.
func (t *SMTStringTheory) checkConcatLenInEqc(concat ast.ASTHandle) error {
	m := t.state.manager
	for _, member := range t.stringClass(concat) {
		_, _, isConcat, err := m.SequenceConcatParts(member)
		if err != nil {
			return err
		}
		if isConcat {
			if err = t.inferLenConcat(member); err != nil {
				return err
			}
		}
	}
	return nil
}

// inferLenConcat is infer_len_concat: when both argument lengths are known
// and the concat's is not, assert len(a0) = l0 /\ len(a1) = l1 => len(n) = l0+l1.
func (t *SMTStringTheory) inferLenConcat(concat ast.ASTHandle) (result error) {
	m := t.state.manager
	first, second, _, err := m.SequenceConcatParts(concat)
	if err != nil {
		return err
	}
	firstLength, firstKnown, err := t.lengthValue(first)
	if err != nil {
		return err
	}
	secondLength, secondKnown, err := t.lengthValue(second)
	if err != nil {
		return err
	}
	_, concatKnown, err := t.lengthValue(concat)
	if err != nil || !firstKnown || !secondKnown || concatKnown {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.length(first, firstLength); err != nil {
		return err
	}
	if err := premises.length(second, secondLength); err != nil {
		return err
	}
	conclusion := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusion.close()) }()
	if err := conclusion.length(concat, new(big.Int).Add(firstLength, secondLength)); err != nil {
		return err
	}
	return premises.imply(conclusion.values[0])
}
