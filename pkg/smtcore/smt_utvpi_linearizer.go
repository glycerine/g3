package smtcore

// Source: src/smt/theory_utvpi.cpp utvpi_tester::linearize overloads.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtUTVPITerm is a (expr, rational) pair; its coefficient is owned by the
// list that holds it.
type smtUTVPITerm struct {
	expression  ast.ASTHandle
	coefficient numeral.Rational
}

// smtUTVPILinearizer is utvpi_tester: rationals on the AST manager's mpq
// manager q.
type smtUTVPILinearizer struct {
	manager *ast.ASTManager
	q       *numeral.RationalManager
	family  ast.FamilyID
	terms   []smtUTVPITerm // m_terms
	weight  numeral.Rational
	// coeffMap is m_coeff_map: an obj_map whose table order (kept across
	// resets) decides the order of the linearization's terms; its values
	// are coefficients[entry.Num-1].
	coeffMap     *ast.ObjMap
	coefficients []numeral.Rational
}

func newSMTUTVPILinearizer(m *ast.ASTManager) (*smtUTVPILinearizer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	numerals := m.NumeralService()
	if numerals == nil || numerals.Manager == nil {
		return nil, errors.New("smt: utvpi needs the AST manager's rational manager")
	}
	return &smtUTVPILinearizer{manager: m, q: numerals.Manager, family: family, weight: smtQZero(), coeffMap: ast.NewObjMap(m)}, nil
}

// numeral is a.is_numeral(expression, out).
func (l *smtUTVPILinearizer) numeral(expression ast.ASTHandle, out *numeral.Rational) (bool, error) {
	_, matched, err := l.manager.ArithmeticNumeralRational(expression, out)
	if err != nil || !matched {
		return false, err
	}
	return true, nil
}

// delTerms frees a term list's coefficients and empties it.
func (l *smtUTVPILinearizer) delTerms(terms []smtUTVPITerm) []smtUTVPITerm {
	for i := range terms {
		smtQDel(l.q, &terms[i].coefficient)
	}
	return terms[:0]
}

// A null right operand selects the source single-term overload. Successful
// output is left-right = sum(terms)+weight, with at most two unit coefficients.
func (l *smtUTVPILinearizer) linearize(left, right ast.ASTHandle) (result bool, err error) {
	q := l.q
	l.terms = l.delTerms(l.terms)
	if err = q.Reset(&l.weight); err != nil {
		return false, err
	}
	work := []smtUTVPITerm{{left, smtQInt(1)}}
	if !right.IsNull() {
		work = append(work, smtUTVPITerm{right, smtQInt(-1)})
	}
	n := smtQZero()
	defer func() {
		l.delTerms(work)
		if e := smtQDel(q, &n); err == nil {
			err = e
		}
	}()
	l.coeffMap.Reset()
	for i := range l.coefficients {
		smtQDel(q, &l.coefficients[i])
	}
	l.coefficients = l.coefficients[:0]
	// push appends (expression, coefficient) to the work list, taking a
	// copy of coefficient.
	push := func(expression ast.ASTHandle, coefficient *numeral.Rational) error {
		work = append(work, smtUTVPITerm{expression, smtQZero()})
		return q.Copy(&work[len(work)-1].coefficient, coefficient)
	}
	// pushMul appends (expression, a*b).
	pushMul := func(expression ast.ASTHandle, a, b *numeral.Rational) error {
		work = append(work, smtUTVPITerm{expression, smtQZero()})
		return q.Mul(a, b, &work[len(work)-1].coefficient)
	}
	for len(work) > 0 {
		i := len(work) - 1
		term := work[i]
		work = work[:i]
		ok, err := l.linearizeTerm(&term, &n, push, pushMul)
		if e := smtQDel(q, &term.coefficient); err == nil {
			err = e
		}
		if err != nil || !ok {
			return false, err
		}
	}
	for _, entry := range l.coeffMap.Entries() {
		coefficient := &l.coefficients[entry.Num-1]
		if coefficient.IsZero() {
			continue
		}
		l.terms = append(l.terms, smtUTVPITerm{entry.Key, smtQZero()})
		if err = q.Copy(&l.terms[len(l.terms)-1].coefficient, coefficient); err != nil {
			return false, err
		}
		if len(l.terms) > 2 {
			return false, nil
		}
		if !coefficient.IsOne() && !coefficient.IsMinusOne() {
			return false, nil
		}
	}
	return true, nil
}

// linearizeTerm processes one work item (e, mul); n is a scratch rational.
func (l *smtUTVPILinearizer) linearizeTerm(term *smtUTVPITerm, n *numeral.Rational, push func(ast.ASTHandle, *numeral.Rational) error, pushMul func(ast.ASTHandle, *numeral.Rational, *numeral.Rational) error) (bool, error) {
	q := l.q
	kind, err := l.manager.GetKind(term.expression)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := l.manager.GetFamilyID(term.expression)
	if err != nil {
		return false, err
	}
	count, err := l.manager.GetNumArgs(term.expression)
	if err != nil {
		return false, err
	}
	if family == l.family {
		operation, err := l.manager.GetDeclKind(term.expression)
		if err != nil {
			return false, err
		}
		switch operation {
		case smtArithOpAdd:
			for j := uint32(0); j < count; j++ {
				child, err := l.manager.GetArg(term.expression, j)
				if err != nil {
					return false, err
				}
				if err = push(child, &term.coefficient); err != nil {
					return false, err
				}
			}
			return true, nil
		case smtArithOpMul:
			if count == 2 {
				x, err := l.manager.GetArg(term.expression, 0)
				if err != nil {
					return false, err
				}
				y, err := l.manager.GetArg(term.expression, 1)
				if err != nil {
					return false, err
				}
				isNumeral, err := l.numeral(x, n)
				if err != nil {
					return false, err
				}
				if !isNumeral {
					x, y = y, x
					if isNumeral, err = l.numeral(x, n); err != nil {
						return false, err
					}
				}
				if isNumeral {
					return true, pushMul(y, &term.coefficient, n)
				}
			}
		case smtArithOpSub:
			if count == 2 {
				x, err := l.manager.GetArg(term.expression, 0)
				if err != nil {
					return false, err
				}
				y, err := l.manager.GetArg(term.expression, 1)
				if err != nil {
					return false, err
				}
				if err = push(x, &term.coefficient); err != nil {
					return false, err
				}
				if err = pushMul(y, &term.coefficient, smtQIntPtr(-1)); err != nil {
					return false, err
				}
				return true, nil
			}
		case smtArithOpUMinus, smtArithOpToReal:
			if count == 1 {
				child, err := l.manager.GetArg(term.expression, 0)
				if err != nil {
					return false, err
				}
				if operation == smtArithOpUMinus {
					return true, pushMul(child, &term.coefficient, smtQIntPtr(-1))
				}
				return true, push(child, &term.coefficient)
			}
		}
		isNumeral, err := l.numeral(term.expression, n)
		if err != nil {
			return false, err
		}
		if isNumeral {
			// m_weight += mul*n
			return true, q.AddMul(&l.weight, &term.coefficient, n, &l.weight)
		}
	}
	if family != ast.NullFamilyID || count != 0 {
		return false, nil
	}
	// m_coeff_map.insert_if_not_there2(e, 0)->m_value += mul
	entry, err := l.coeffMap.InsertIfNotThere(term.expression)
	if err != nil {
		return false, err
	}
	if entry.Num == 0 {
		l.coefficients = append(l.coefficients, smtQZero())
		entry.Num = uint64(len(l.coefficients))
	}
	c := &l.coefficients[entry.Num-1]
	return true, q.Add(c, &term.coefficient, c)
}
