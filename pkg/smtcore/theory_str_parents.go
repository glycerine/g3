package smtcore

// Source: src/smt/theory_str.cpp eval_concat, simplify_parent and
// the initial concat shortcuts/parent dispatch of handle_equality.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) evalStringConcat(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	l, lk, err := t.eqcStringValue(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, rk, err := t.eqcStringValue(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if lk && rk {
		first, _, err := m.SequenceStringValue(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		second, _, err := m.SequenceStringValue(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return t.stringLiteral(first + second)
	}
	if lk {
		text, _, err := m.SequenceStringValue(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return right, nil
		}
	} else if rk {
		text, _, err := m.SequenceStringValue(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return left, nil
		}
	}
	return ast.ASTHandle{}, nil
}

func (t *SMTStringTheory) simplifyEqualityParents(left, right ast.ASTHandle) error {
	l, lk, err := t.eqcStringValue(left)
	if err != nil {
		return err
	}
	r, rk, err := t.eqcStringValue(right)
	if err != nil {
		return err
	}
	if lk && !rk {
		return t.simplifyStringParents(right, l)
	}
	if !lk && rk {
		return t.simplifyStringParents(left, r)
	}
	return nil
}

func (t *SMTStringTheory) stringConcatEqualityShortcut(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	l0, l1, lc, err := m.SequenceConcatParts(left)
	if err != nil {
		return false, err
	}
	r0, r1, rc, err := m.SequenceConcatParts(right)
	if err != nil || !lc || !rc {
		return false, err
	}
	if err := t.simplifyEqualityParents(left, right); err != nil {
		return false, err
	}
	if l0 == r0 {
		same, err := t.sameStringClass(l1, r1)
		if err != nil || same {
			return same, err
		}
	}
	if l1 == r1 {
		return t.sameStringClass(l0, r0)
	}
	return false, nil
}

func (t *SMTStringTheory) simplifyStringParents(expression, constant ast.ASTHandle) error {
	if err := t.state.Internalize(expression, false); err != nil {
		return err
	}
	for _, member := range t.stringClass(expression) {
		node := t.state.context.enodes.get(member)
		if node == nil {
			return ast.ErrASTDomain
		}
		// Source snapshots the current member's parent list before adding
		// axioms. Boolean parents introduced by publication are not revisited.
		for _, parent := range node.Parents() {
			if err := t.simplifyStringParent(member, constant, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) simplifyStringParent(member, constant ast.ASTHandle, parent *SMTEnode) (result error) {
	m := t.state.manager
	first, second, concat, err := m.SequenceConcatParts(parent.Owner())
	if err != nil || !concat {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(expression ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil || expression.IsNull() {
			return expression, err
		}
		return expression, locals.add(expression)
	}
	// bool parentLen_exists = get_len_value(a_parent, parentLen)
	parentLength, parentKnown, err := t.lengthValue(parent.Owner())
	if err != nil {
		return err
	}
	// Both direct argument positions are independent, including x ++ x.
	for i, argument := range []ast.ASTHandle{first, second} {
		if argument != member {
			continue
		}
		other := second
		left, right := constant, second
		if i == 1 {
			other, left, right = first, first, constant
		}
		if err := t.makeUpParentArgumentLength(parent.Owner(), member, constant, other, i == 0, parentLength, parentKnown); err != nil {
			return err
		}
		evaluated, err := pin(t.evalStringConcat(left, right))
		if err != nil {
			return err
		}
		if !evaluated.IsNull() {
			otherValue, _, err := t.eqcStringValue(other)
			if err != nil {
				return err
			}
			// simplify_parent #1/#2 build implyL before in_same_eqc.
			if err := t.publishParentSimplificationAfterPremises(member, constant, other, otherValue, parent.Owner(), evaluated); err != nil {
				return err
			}
		} else {
			memberConcat, err := m.IsSequenceConcat(member)
			if err != nil {
				return err
			}
			if memberConcat {
				simplified, err := pin(t.concatTerm(left, right))
				if err != nil {
					return err
				}
				if err := t.publishParentSimplification(member, constant, member, member, parent.Owner(), simplified); err != nil {
					return err
				}
			}
		}
	}
	// Cases 3/4: combine a new class constant with an adjacent literal in
	// the other operand's concat, preserving its remaining operand.
	for i, argument := range []ast.ASTHandle{first, second} {
		if argument != member {
			continue
		}
		other := second
		if i == 1 {
			other = first
		}
		left, right, nested, err := m.SequenceConcatParts(other)
		if err != nil {
			return err
		}
		if !nested {
			continue
		}
		literal, rest := left, right
		if i == 1 {
			literal, rest = right, left
		}
		if _, isLiteral, err := m.SequenceStringValue(literal); err != nil {
			return err
		} else if !isLiteral {
			continue
		}
		if err := t.foldParentBoundary(member, constant, literal, rest, parent.Owner(), i == 0, i == 0); err != nil {
			return err
		}
	}
	// Cases 5/6 look one parent level further. Newly introduced equality
	// parents are not concats, so they add no new folding candidates.
	for _, trailingLiteral := range []bool{true, false} {
		for _, grandparent := range parent.Parents() {
			left, right, outerConcat, err := m.SequenceConcatParts(grandparent.Owner())
			if err != nil {
				return err
			}
			if !outerConcat {
				continue
			}
			if trailingLiteral && second == member && left == parent.Owner() {
				if _, literal, err := m.SequenceStringValue(right); err != nil {
					return err
				} else if literal {
					if err := t.foldParentBoundary(member, constant, right, first, grandparent.Owner(), true, false); err != nil {
						return err
					}
				}
			}
			if !trailingLiteral && first == member && right == parent.Owner() {
				if _, literal, err := m.SequenceStringValue(left); err != nil {
					return err
				} else if literal {
					if err := t.foldParentBoundary(member, constant, left, second, grandparent.Owner(), false, true); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// Constant byte order and rebuilt operand order are separate in the
// nested-operand and grandparent cases.
func (t *SMTStringTheory) foldParentBoundary(member, constant, literal, rest, parent ast.ASTHandle, constantFirst, combinedFirst bool) (result error) {
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	left, right := constant, literal
	if !constantFirst {
		left, right = literal, constant
	}
	combined, err := t.evalStringConcat(left, right)
	if err != nil {
		return err
	}
	if combined.IsNull() {
		return ast.ErrASTDomain
	}
	if err := locals.add(combined); err != nil {
		return err
	}
	// simplify_parent #3-#6 build implyL before mk_concat and in_same_eqc.
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := t.parentSimplificationPremises(&premises, member, constant, member, member); err != nil {
		return err
	}
	left, right = combined, rest
	if !combinedFirst {
		left, right = rest, combined
	}
	simplified, err := t.concatTerm(left, right)
	if err != nil {
		return err
	}
	if err := locals.add(simplified); err != nil {
		return err
	}
	same, err := t.sameStringClass(parent, simplified)
	if err != nil || same {
		return err
	}
	return t.parentSimplificationConclusion(&premises, parent, simplified)
}

func (t *SMTStringTheory) publishParentSimplification(member, constant, other, otherValue, parent, simplified ast.ASTHandle) (result error) {
	same, err := t.sameStringClass(parent, simplified)
	if err != nil || same {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := t.parentSimplificationPremises(&premises, member, constant, other, otherValue); err != nil {
		return err
	}
	return t.parentSimplificationConclusion(&premises, parent, simplified)
}

func (t *SMTStringTheory) publishParentSimplificationAfterPremises(member, constant, other, otherValue, parent, simplified ast.ASTHandle) (result error) {
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := t.parentSimplificationPremises(&premises, member, constant, other, otherValue); err != nil {
		return err
	}
	same, err := t.sameStringClass(parent, simplified)
	if err != nil || same {
		return err
	}
	return t.parentSimplificationConclusion(&premises, parent, simplified)
}

// parentSimplificationPremises builds simplify_parent's implyL.
func (t *SMTStringTheory) parentSimplificationPremises(premises *smtStringLengthPremises, member, constant, other, otherValue ast.ASTHandle) error {
	// Native guards the pending member equality even if it is reflexive.
	// simplify_parent builds every equality with ctx.mk_eq_atom.
	equality, err := t.state.MkEqAtom(member, constant)
	if err != nil {
		return err
	}
	if err := premises.add(equality); err != nil {
		return err
	}
	if err := premises.equate(other, otherValue); err != nil {
		return err
	}
	// implyL = m.mk_and(eq_ast1, eq_ast2) is built before implyR.
	if len(premises.values) > 1 {
		guard, err := t.state.manager.MkAndArray(premises.values)
		if err != nil {
			return err
		}
		if err := premises.hold(guard); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTStringTheory) parentSimplificationConclusion(premises *smtStringLengthPremises, parent, simplified ast.ASTHandle) error {
	conclusion, err := t.state.MkEqAtom(parent, simplified)
	if err != nil {
		return err
	}
	return premises.imply(conclusion)
}

// makeUpParentArgumentLength is simplify_parent's "make up len for arg1"
// (#1) and "make up len for arg0" (#2) step. Each reads the two argument
// lengths (the class constant's for member) and, when the parent length is
// known but the other argument's is not, asserts
// |parent| = p && |member| = c  ->  |other| = p - c   (or its premise's
// negation when p - c < 0). m.mk_and(eq1, eq2) evaluates eq2 first and each
// ctx.mk_eq_atom(mk_strlen(e), mk_int(v)) builds the numeral first.
func (t *SMTStringTheory) makeUpParentArgumentLength(parent, member, constant, other ast.ASTHandle, memberFirst bool, parentLength *big.Int, parentKnown bool) (result error) {
	var memberLength, otherLength *big.Int
	var otherKnown bool
	var err error
	if memberFirst {
		if memberLength, _, err = t.lengthValue(constant); err != nil {
			return err
		}
		if otherLength, otherKnown, err = t.lengthValue(other); err != nil {
			return err
		}
	} else {
		if otherLength, otherKnown, err = t.lengthValue(other); err != nil {
			return err
		}
		if memberLength, _, err = t.lengthValue(constant); err != nil {
			return err
		}
	}
	_ = otherLength
	if !parentKnown || otherKnown {
		return nil
	}
	if memberLength == nil {
		memberLength = new(big.Int)
	}
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	lengthEquality := func(expression ast.ASTHandle, value *big.Int) (ast.ASTHandle, error) {
		numeral, err := m.ArithmeticIntegerNumeralText(value.String())
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err := locals.hold(numeral); err != nil {
			return ast.ASTHandle{}, err
		}
		length, err := m.MkSequenceLength(expression)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err := locals.hold(length); err != nil {
			return ast.ASTHandle{}, err
		}
		equality, err := t.state.MkEqAtom(length, numeral)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return equality, locals.hold(equality)
	}
	memberEquality, err := lengthEquality(member, memberLength)
	if err != nil {
		return err
	}
	parentEquality, err := lengthEquality(parent, parentLength)
	if err != nil {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.add(parentEquality); err != nil {
		return err
	}
	if err := premises.add(memberEquality); err != nil {
		return err
	}
	guard, err := m.MkAndArray(premises.values)
	if err != nil {
		return err
	}
	if err := locals.hold(guard); err != nil {
		return err
	}
	difference := new(big.Int).Sub(parentLength, memberLength)
	if difference.Sign() < 0 {
		return premises.assertNegation()
	}
	conclusion, err := lengthEquality(other, difference)
	if err != nil {
		return err
	}
	return premises.imply(conclusion)
}
