package smtcore

// Source: src/smt/theory_str.cpp finalcheck_str2int/finalcheck_int2str.
// These are the retained conversion-agreement stage; the enclosing final-check
// pipeline must establish its source prerequisites before dispatching them.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// stringDecimalValue is finalcheck_str2int/int2str's decimal reading:
// convertedRepresentation = (ten * convertedRepresentation) + rational(val)
// over rationals; value is the caller's rational, valid reports a nonempty
// all-digit text.
func stringDecimalValue(q *numeral.RationalManager, text string, value *numeral.Rational) (bool, error) {
	if err := q.Reset(value); err != nil {
		return false, err
	}
	if len(text) == 0 {
		return false, nil
	}
	ten := smtQInt(10)
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false, nil
		}
		digit := smtQInt(int32(text[i] - '0'))
		if err := q.Mul(&ten, value, value); err != nil {
			return false, err
		}
		if err := q.Add(value, &digit, value); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (t *SMTStringTheory) assertConversionAgreement(axiom ast.ASTHandle) (bool, error) {
	if _, found := t.conversionAxioms[axiom]; found {
		return false, nil
	}
	// Native insert_obj_trail removes only the set membership at scope pop;
	// assert_axiom's lifetime trail continues to own the expression.
	t.conversionAxioms[axiom] = t.level
	return true, t.assertAxiom(axiom)
}

func (t *SMTStringTheory) finalCheckStringToInt(expression ast.ASTHandle) (added bool, result error) {
	m := t.state.manager
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	publish := func(premise, conclusion ast.ASTHandle) error {
		negative, err := pin(m.MkNotSimplified(premise))
		if err != nil {
			return err
		}
		axiom, err := pin(m.MkOr(negative, conclusion))
		if err != nil {
			return err
		}
		changed, err := t.assertConversionAgreement(axiom)
		added = added || changed
		return err
	}
	// rational Ival
	q := t.q()
	integer := smtQZero()
	defer t.qdel(&integer, &result)
	known, err := t.arithmeticValue(expression, &integer)
	if err != nil {
		return added, err
	}
	if known {
		if !integer.IsMinusOne() {
			value, err := pin(m.ArithmeticNumeralFromRational(&integer, true))
			if err != nil {
				return added, err
			}
			premise, err := pin(t.state.MkEqAtom(expression, value))
			if err != nil {
				return added, err
			}
			// zstring Ival_str(Ival.to_string().c_str())
			ivalText, err := q.String(&integer)
			if err != nil {
				return added, err
			}
			text, err := pin(t.stringLiteral(ivalText))
			if err != nil {
				return added, err
			}
			conclusion, err := pin(t.state.MkEqAtom(word, text))
			if err != nil {
				return added, err
			}
			if err := publish(premise, conclusion); err != nil {
				return added, err
			}
		}
	} else {
		zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return added, err
		}
		isZero, err := pin(t.state.MkEqAtom(expression, zero))
		if err != nil {
			return added, err
		}
		if _, err := t.ensureEnode(isZero); err != nil {
			return added, err
		}
		// Source mk_literal creates the zero equality without asserting it,
		// then requests continuation even if that equality already existed.
		if _, err := t.state.Literal(isZero); err != nil {
			return added, err
		}
		added = true
	}
	constant, known, err := t.eqcStringValue(word)
	if err != nil || !known {
		return added, err
	}
	text, _, err := m.SequenceStringValue(constant)
	if err != nil {
		return added, err
	}
	// rational convertedRepresentation(0); rational::minus_one() when invalid.
	converted := smtQZero()
	defer t.qdel(&converted, &result)
	valid, err := stringDecimalValue(q, text, &converted)
	if err != nil {
		return added, err
	}
	if !valid {
		if err = q.SetInt64(&converted, -1); err != nil {
			return added, err
		}
	}
	premise, err := pin(t.state.MkEqAtom(word, constant))
	if err != nil {
		return added, err
	}
	value, err := pin(m.ArithmeticNumeralFromRational(&converted, true))
	if err != nil {
		return added, err
	}
	conclusion, err := pin(t.state.MkEqAtom(expression, value))
	if err != nil {
		return added, err
	}
	err = publish(premise, conclusion)
	return added, err
}

func (t *SMTStringTheory) finalCheckIntToString(expression ast.ASTHandle) (added bool, result error) {
	m := t.state.manager
	integer, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	constant, known, err := t.eqcStringValue(expression)
	if err != nil {
		return false, err
	}
	if !known {
		// Source NOT_IMPLEMENTED_YET: the enclosing stage expects an assigned
		// String class and must not accept an unassigned conversion as done.
		return false, fmt.Errorf("string conversion agreement requires a concrete int-to-string class value")
	}
	text, _, err := m.SequenceStringValue(constant)
	if err != nil || len(text) == 0 {
		return false, err // Empty result is covered by the initial axiom.
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	premise, err := pin(t.state.MkEqAtom(expression, constant))
	if err != nil {
		return false, err
	}
	converted := smtQZero()
	defer t.qdel(&converted, &result)
	valid, err := stringDecimalValue(t.q(), text, &converted)
	if err != nil {
		return false, err
	}
	if !valid {
		// Native deliberately does not memoize this conflict clause.
		axiom, err := pin(m.MkNot(premise))
		if err != nil {
			return false, err
		}
		return true, t.assertAxiom(axiom)
	}
	value, err := pin(m.ArithmeticNumeralFromRational(&converted, true))
	if err != nil {
		return false, err
	}
	conclusion, err := pin(t.state.MkEqAtom(integer, value))
	if err != nil {
		return false, err
	}
	negative, err := pin(m.MkNotSimplified(premise))
	if err != nil {
		return false, err
	}
	axiom, err := pin(m.MkOr(negative, conclusion))
	if err != nil {
		return false, err
	}
	return t.assertConversionAgreement(axiom)
}
