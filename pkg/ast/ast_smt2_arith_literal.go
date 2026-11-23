package ast

// Source: src/ast/ast_smt2_pp.cpp:296-368.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type smt2AlgebraicText struct{ strings.Builder }

func (s *smt2AlgebraicText) AppendInt32(value int32) error {
	_, err := s.WriteString(strconv.FormatInt(int64(value), 10))
	return err
}
func (s *smt2AlgebraicText) AppendCString(value string) error {
	_, err := s.WriteString(value)
	return err
}
func (s *smt2AlgebraicText) AppendNumeralText(value string) error {
	_, err := s.WriteString(value)
	return err
}

// SMT2PPArithLiteral formats the concrete arithmetic numeral branches used by
// the SMT-LIB printer. matched is false for non-arithmetic constants.
func (m *ASTManager) SMT2PPArithLiteral(h ASTHandle, decimal bool, precision uint32) (format ASTHandle, matched bool, err error) {
	return m.smt2PPArithLiteral(h, decimal, precision)
}

func (m *ASTManager) smt2PPArithLiteral(h ASTHandle, decimal bool, precision uint32) (format ASTHandle, matched bool, result error) {
	// rational val; bool is_int;
	service := m.NumeralService()
	if service == nil {
		return ASTHandle{}, false, ErrASTHandle
	}
	q := service.Manager
	val := unumeral.NewRational()
	defer func() { result = errors.Join(result, q.Del(&val)) }()
	integer, matched, err := m.ArithmeticNumeralRational(h, &val)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if !matched {
		family, e := m.GetFamilyID(h)
		if e != nil {
			return ASTHandle{}, false, e
		}
		arithmetic, e := m.ArithmeticFamilyID()
		if e != nil || family != arithmetic {
			return ASTHandle{}, false, e
		}
		if !decimal && m.unboundedAlgebraicReads() {
			text, ok, err := m.exactAlgebraicSMT2Text(h, arithmetic)
			if err != nil || !ok {
				return ASTHandle{}, ok, err
			}
			format, err = m.FormatString(text)
			return format, true, err
		}
		var algebraic polynomial.Algebraic
		matched, e = m.ArithmeticAlgebraicNumeralValue(h, &algebraic)
		if e != nil || !matched {
			return ASTHandle{}, matched, e
		}
		manager, e := m.ArithmeticAlgebraicManager()
		if e != nil {
			return ASTHandle{}, true, e
		}
		defer func() { result = errors.Join(result, manager.Del(&algebraic)) }()
		var text smt2AlgebraicText
		negative := false
		if decimal {
			negative, e = manager.IsNeg(&algebraic)
			if e == nil && negative {
				e = manager.Neg(&algebraic)
			}
			if e == nil {
				e = manager.DisplayDecimal(&text, &algebraic, precision)
			}
		} else {
			e = manager.DisplayRootSMT2Into(&text, &algebraic)
		}
		if e != nil {
			return ASTHandle{}, true, e
		}
		format, e = m.FormatString(text.String())
		if e == nil && decimal && negative {
			format, e = m.FormatSeq1("-", []ASTHandle{format})
		}
		return format, true, e
	}
	negative := val.IsNeg()
	if negative {
		if err = q.Neg(&val); err != nil {
			return ASTHandle{}, true, err
		}
	}
	// mk_float: val.to_string() + ".0"
	float := func(r *unumeral.Rational) (ASTHandle, error) {
		text, e := q.String(r)
		if e != nil {
			return ASTHandle{}, e
		}
		return m.FormatString(text + ".0")
	}
	if integer {
		if !val.IsInt() {
			return ASTHandle{}, true, ErrASTDomain
		}
		var text string
		if text, err = q.String(&val); err == nil {
			format, err = m.FormatString(text)
		}
	} else if val.IsInt() {
		format, err = float(&val)
	} else if decimal {
		var text string
		text, err = q.DecimalString(&val, precision, false)
		if err == nil {
			format, err = m.FormatString(text)
		}
	} else {
		part := unumeral.NewRational()
		var numerator, denominator ASTHandle
		err = q.NumeratorRational(&val, &part)
		if err == nil {
			numerator, err = float(&part)
		}
		if err == nil {
			err = q.DenominatorRational(&val, &part)
		}
		if err == nil {
			denominator, err = float(&part)
		}
		err = errors.Join(err, q.Del(&part))
		if err == nil {
			format, err = m.FormatSeq1("/", []ASTHandle{numerator, denominator})
		}
	}
	if err != nil {
		return ASTHandle{}, true, err
	}
	if negative {
		format, err = m.FormatSeq1("-", []ASTHandle{format})
	}
	return format, true, err
}

// The exact printer reads the plugin's number in place. The general printer
// can remember its root index on that cell. A compact square root already
// identifies the polynomial and which of its two real roots was chosen.
func (m *ASTManager) exactAlgebraicSMT2Text(h ASTHandle, arithmetic FamilyID) (string, bool, error) {
	if err := m.lock(); err != nil {
		return "", false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return "", false, err
	}
	id, ok, err := m.arithmeticAlgebraicID(arithmetic, r)
	if err != nil || !ok {
		return "", false, err
	}
	w := m.arithmeticPlugin.algebraic
	if n := w.signedRadicand(id); n != 0 {
		index := "2"
		if n < 0 {
			n, index = -n, "1"
		}
		squared := "(^ x 2)"
		if denominator := w.squareDenominator(id); denominator != 1 {
			squared = "(* " + strconv.FormatInt(int64(denominator), 10) + " " + squared + ")"
		}
		return "(root-obj (+ " + squared + " (- " + strconv.FormatInt(int64(n), 10) + ")) " + index + ")", true, nil
	}
	value, err := w.value(id)
	if err != nil {
		return "", true, err
	}
	var text smt2AlgebraicText
	err = w.manager.DisplayRootSMT2Into(&text, value)
	return text.String(), true, err
}
