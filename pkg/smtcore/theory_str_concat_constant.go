package smtcore

// Source: src/smt/theory_str.cpp solve_concat_eq_str and
// generate_mutual_exclusion.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTStringTheory) solveConcatConstant(concat, constant ast.ASTHandle) (result error) {
	m := t.state.manager
	first, second, matched, err := m.SequenceConcatParts(concat)
	if err != nil || !matched {
		return err
	}
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	// ctx.mk_eq_atom(concat, str) is built where each case first needs it.
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	if text == "" {
		equality, err := pin(t.state.MkEqAtom(concat, constant))
		if err != nil {
			return err
		}
		left, err := pin(t.state.MkEqAtom(first, constant))
		if err != nil {
			return err
		}
		right, err := pin(t.state.MkEqAtom(second, constant))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAnd(left, right))
		if err != nil {
			return err
		}
		if err := guards.add(equality); err != nil {
			return err
		}
		return guards.imply(conclusion)
	}
	a, ak, err := t.eqcStringValue(first)
	if err != nil {
		return err
	}
	b, bk, err := t.eqcStringValue(second)
	if err != nil {
		return err
	}
	newConcat := concat
	if a != first || b != second {
		if err := guards.equate(first, a); err != nil {
			return err
		}
		if err := guards.equate(second, b); err != nil {
			return err
		}
		newConcat, err = pin(t.concatTerm(a, b))
		if err != nil {
			return err
		}
		if newConcat != constant {
			conclusion, err := pin(t.state.MkEqAtom(concat, newConcat))
			if err != nil {
				return err
			}
			if err := guards.imply(conclusion); err != nil {
				return err
			}
		}
	}
	if newConcat == constant {
		return nil
	}
	stillConcat, err := m.IsSequenceConcat(newConcat)
	if err != nil || !stillConcat {
		return err
	}
	if ak && bk {
		// Active mk_concat folds two class constants, so this source case
		// cannot survive the non-concat early return above.
		return nil
	}
	if ak || bk {
		known, variable := a, b
		if bk {
			known, variable = b, a
		}
		piece, _, err := m.SequenceStringValue(known)
		if err != nil {
			return err
		}
		valid := strings.HasPrefix(text, piece)
		if bk {
			valid = strings.HasSuffix(text, piece)
		}
		newEquality, err := pin(t.state.MkEqAtom(newConcat, constant))
		if err != nil {
			return err
		}
		if !valid {
			negative, err := pin(m.MkNot(newEquality))
			if err != nil {
				return err
			}
			return t.assertAxiom(negative)
		}
		remainder := text[len(piece):]
		if bk {
			remainder = text[:len(text)-len(piece)]
		}
		value, err := pin(t.stringLiteral(remainder))
		if err != nil {
			return err
		}
		conclusion, err := pin(t.state.MkEqAtom(variable, value))
		if err != nil {
			return err
		}
		premises := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, premises.close()) }()
		if err := premises.add(newEquality); err != nil {
			return err
		}
		return premises.imply(conclusion)
	}
	// eval_concat cannot succeed when neither operand has a class value.
	// rational arg1Len, arg2Len
	al, bl := smtQZero(), smtQZero()
	defer t.qdel(&bl, &result)
	defer t.qdel(&al, &result)
	alk, err := t.lengthValue(a, &al)
	if err != nil {
		return err
	}
	blk, err := t.lengthValue(b, &bl)
	if err != nil {
		return err
	}
	if alk || blk {
		// expr_ref ax_l1(ctx.mk_eq_atom(concat, str)) opens Case 4's
		// known-length branch.
		equality, err := pin(t.state.MkEqAtom(concat, constant))
		if err != nil {
			return err
		}
		return t.solveConcatKnownLength(equality, a, b, text, &al, alk, &bl, blk)
	}
	return t.solveConcatArrangements(concat, constant, a, b, text)
}

func (t *SMTStringTheory) solveConcatKnownLength(equality, a, b ast.ASTHandle, text string, al *numeral.Rational, alk bool, bl *numeral.Rational, blk bool) (result error) {
	m := t.state.manager
	variable, length := a, al
	if !alk {
		variable, length = b, bl
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	// The length conflicts build m_autil.mk_ge/mk_le(mk_strlen(arg),
	// mk_int(bound)) only in their own branches; the release build (GCC)
	// evaluates the call arguments right to left, so mk_int comes first.
	bounded := func(bound string) (term, numeral ast.ASTHandle, err error) {
		if numeral, err = pin(m.ArithmeticIntegerNumeralText(bound)); err != nil {
			return
		}
		term, err = pin(m.MkSequenceLength(variable))
		return
	}
	q := t.q()
	if length.IsNeg() {
		term, numeral, err := bounded("0")
		if err != nil {
			return err
		}
		axiom, err := pin(m.MkArithmeticGE(term, numeral))
		if err != nil {
			return err
		}
		return t.assertAxiom(axiom)
	}
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	if err := guards.add(equality); err != nil {
		return err
	}
	// rational concatStrLen((unsigned)const_str.length())
	textLength := smtQZero()
	defer t.qdel(&textLength, &result)
	if err := q.SetUint64(&textLength, uint64(len(text))); err != nil {
		return err
	}
	if c, err := q.Compare(length, &textLength); err != nil {
		return err
	} else if c > 0 {
		term, numeral, err := bounded(strconv.Itoa(len(text)))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkArithmeticLE(term, numeral))
		if err != nil {
			return err
		}
		return guards.imply(conclusion)
	}
	splitLength, err := q.Uint64(length)
	if err != nil {
		return err
	}
	split := int(splitLength)
	if !alk {
		split = len(text) - split
	}
	for i, expression := range []ast.ASTHandle{a, b} {
		piece := text[:split]
		if i == 1 {
			piece = text[split:]
		}
		concat, err := m.IsSequenceConcat(expression)
		if err != nil {
			return err
		}
		if concat {
			possible, err := t.canConcatEqualString(expression, piece)
			if err != nil {
				return err
			}
			if !possible {
				premise := smtStringLengthPremises{theory: t}
				defer func() { result = errors.Join(result, premise.close()) }()
				if err := premise.length(variable, length); err != nil {
					return err
				}
				negative, err := pin(m.MkNot(premise.values[0]))
				if err != nil {
					return err
				}
				return guards.imply(negative)
			}
		}
	}
	if err := guards.length(variable, length); err != nil {
		return err
	}
	conclusions := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusions.close()) }()
	for i, expression := range []ast.ASTHandle{a, b} {
		piece := text[:split]
		if i == 1 {
			piece = text[split:]
		}
		value, err := pin(t.stringLiteral(piece))
		if err != nil {
			return err
		}
		eq, err := pin(t.state.MkEqAtom(expression, value))
		if err != nil {
			return err
		}
		if err := conclusions.add(eq); err != nil {
			return err
		}
	}
	for i, expression := range []ast.ASTHandle{a, b} {
		piece, known := text[:split], alk
		if i == 1 {
			piece, known = text[split:], blk
		}
		if !known {
			if err := t.withStringRational(int64(len(piece)), func(r *numeral.Rational) error { return conclusions.length(expression, r) }); err != nil {
				return err
			}
		}
	}
	// expr_ref lhs(m.mk_and(ax_l1, ax_l2)) is built before
	// expr_ref rhs(mk_and(r_items)); then assert_implication(lhs, rhs).
	premise, err := pin(m.MkAndArgs(guards.values))
	if err != nil {
		return err
	}
	conclusion, err := pin(m.MkAndArray(conclusions.values))
	if err != nil {
		return err
	}
	return t.assertImplication(premise, conclusion)
}

func (t *SMTStringTheory) solveConcatArrangements(concat, constant, a, b ast.ASTHandle, text string) (result error) {
	m := t.state.manager
	key, reverse := [2]ast.ASTHandle{a, b}, [2]ast.ASTHandle{b, a}
	if _, first := t.concatBreakFlags[key]; !first {
		if _, second := t.concatBreakFlags[reverse]; !second {
			flag, err := t.freshIntegerVariable("$$_xor")
			if err != nil {
				return err
			}
			t.concatBreakFlags[key] = flag
		}
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	var arrangements []ast.ASTHandle
	for split := 0; split <= len(text); split++ {
		possible := true
		for i, expression := range []ast.ASTHandle{a, b} {
			piece := text[:split]
			if i == 1 {
				piece = text[split:]
			}
			concat, err := m.IsSequenceConcat(expression)
			if err != nil {
				return err
			}
			if concat {
				valid, err := t.canConcatEqualString(expression, piece)
				if err != nil {
					return err
				}
				if !valid {
					possible = false
					break
				}
			}
		}
		if !possible {
			continue
		}
		prefix, err := pin(t.stringLiteral(text[:split]))
		if err != nil {
			return err
		}
		left, err := pin(t.state.MkEqAtom(a, prefix))
		if err != nil {
			return err
		}
		suffix, err := pin(t.stringLiteral(text[split:]))
		if err != nil {
			return err
		}
		right, err := pin(t.state.MkEqAtom(b, suffix))
		if err != nil {
			return err
		}
		arrangement, err := pin(m.MkAnd(left, right))
		if err != nil {
			return err
		}
		arrangements = append(arrangements, arrangement)
	}
	// expr_ref implyL(ctx.mk_eq_atom(concat, str)) follows the options.
	equality, err := pin(t.state.MkEqAtom(concat, constant))
	if err != nil {
		return err
	}
	if len(arrangements) == 0 {
		negative, err := pin(m.MkNot(equality))
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	disjunction, err := pin(m.MkOrArray(arrangements))
	if err != nil {
		return err
	}
	strong := true
	if t.parameterOwner != nil {
		parameters, err := t.stringParameterState()
		if err != nil {
			return err
		}
		strong = parameters.StrongArrangements
	}
	if strong {
		axiom, err := pin(t.state.MkEqAtom(equality, disjunction))
		if err != nil {
			return err
		}
		if err := t.assertAxiom(axiom); err != nil {
			return err
		}
	} else {
		guard := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guard.close()) }()
		if err := guard.add(equality); err != nil {
			return err
		}
		if err := guard.imply(disjunction); err != nil {
			return err
		}
	}
	literals := make([]SMTLiteral, len(arrangements))
	for i, expression := range arrangements {
		literal, err := t.state.Literal(expression)
		if err != nil {
			return err
		}
		literals[i] = literal
	}
	return t.state.MakeTheoryCaseSplit(literals)
}
