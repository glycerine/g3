package smtcore

// Source: src/smt/theory_str.cpp get_arith_value, get_len_value and the
// check_length_consistency family. Only actual core numeral roots are read.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// q is the AST manager's rational manager (rational's manager): theory_str's
// lengths and bounds are rationals of it.
func (t *SMTStringTheory) q() *numeral.RationalManager {
	return t.state.manager.NumeralService().Manager
}

// qdel frees a rational local of q, keeping the first failure in *err.
func (t *SMTStringTheory) qdel(r *numeral.Rational, err *error) {
	if e := smtQDel(t.q(), r); e != nil && *err == nil {
		*err = e
	}
}

// arithmeticValue is get_arith_value(e, val): val is the caller's rational
// (is_numeral writes it even when it is not an integer).
func (t *SMTStringTheory) arithmeticValue(expression ast.ASTHandle, value *numeral.Rational) (bool, error) {
	node := t.state.context.enodes.get(expression)
	if node == nil {
		return false, nil
	}
	_, matched, err := t.state.manager.ArithmeticNumeralRational(node.Root().Owner(), value)
	if err != nil || !matched {
		return false, err
	}
	return value.IsInt(), nil
}

// lengthValue is get_len_value(e, val): val is reset, then accumulates the
// string literals' lengths and the arithmetic values of the other leaves'
// lengths (through the local rational val1).
func (t *SMTStringTheory) lengthValue(expression ast.ASTHandle, value *numeral.Rational) (known bool, result error) {
	m := t.state.manager
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	switch arithmetic := t.state.Theory(family).(type) {
	case *SMTTheoryArith: // theory_mi_arith only
		if arithmetic.integerExt {
			return false, nil
		}
	default:
		return false, nil
	}
	q := t.q()
	val1 := smtQZero()
	defer t.qdel(&val1, &result)
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = smtErrJoin(result, m.DelRef(&ref)) }()
	if err = q.Reset(value); err != nil {
		return false, err
	}
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		left, right, concat, err := m.SequenceConcatParts(current)
		if err != nil {
			return false, err
		}
		if concat {
			// Source get_len_value visits the right child first.
			work = append(work, left, right)
			continue
		}
		text, literal, err := m.SequenceStringValue(current)
		if err != nil {
			return false, err
		}
		if literal {
			// val += rational(sl)
			sl := smtQZero()
			if err = q.SetUint64(&sl, uint64(len(text))); err == nil {
				err = q.Add(value, &sl, value)
			}
			t.qdel(&sl, &err)
			if err != nil {
				return false, err
			}
			continue
		}
		length, err := m.MkSequenceLength(current)
		if err != nil {
			return false, err
		}
		if err = m.SetRef(ref, length); err != nil {
			return false, err
		}
		exists, err := t.arithmeticValue(length, &val1)
		if err != nil || !exists {
			return false, err
		}
		if err = q.Add(value, &val1, value); err != nil {
			return false, err
		}
	}
	return true, nil
}

type smtStringLengthPremises struct {
	theory *SMTStringTheory
	values []ast.ASTHandle
	refs   []ast.ASTRef
}

func (p *smtStringLengthPremises) close() (result error) {
	// Source premise collections are expr_ref_vectors (forward release).
	for i := range p.refs {
		result = errors.Join(result, p.theory.state.manager.DelRef(&p.refs[i]))
	}
	return result
}

func (p *smtStringLengthPremises) add(expression ast.ASTHandle) error {
	ref, err := p.theory.state.manager.NewRef(expression)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	p.values = append(p.values, expression)
	return nil
}

// hold keeps expression alive with the premises without making it one.
func (p *smtStringLengthPremises) hold(expression ast.ASTHandle) error {
	ref, err := p.theory.state.manager.NewRef(expression)
	if err != nil {
		return err
	}
	p.refs = append(p.refs, ref)
	return nil
}

func (p *smtStringLengthPremises) length(expression ast.ASTHandle, value *numeral.Rational) error {
	m := p.theory.state.manager
	// ctx.mk_eq_atom(mk_strlen(e), mk_int(v)): the release build (GCC)
	// evaluates the call arguments right to left, so mk_int comes first.
	numeral, err := m.ArithmeticNumeralFromRational(value, true)
	if err != nil {
		return err
	}
	length, err := m.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	equality, err := p.theory.state.MkEqAtom(length, numeral)
	if err != nil {
		return err
	}
	return p.add(equality)
}

// lengthLenFirst is the statement-by-statement form
// expr_ref l(mk_strlen(e)); expr_ref v(mk_int(value)); ctx.mk_eq_atom(l, v),
// which builds the length before the numeral.
func (p *smtStringLengthPremises) lengthLenFirst(expression ast.ASTHandle, value *numeral.Rational) error {
	m := p.theory.state.manager
	length, err := m.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	numeral, err := m.ArithmeticNumeralFromRational(value, true)
	if err != nil {
		return err
	}
	equality, err := p.theory.state.MkEqAtom(length, numeral)
	if err != nil {
		return err
	}
	return p.add(equality)
}

func (p *smtStringLengthPremises) refuse(left, right ast.ASTHandle) error {
	equality, err := p.theory.state.MkEqAtom(left, right)
	if err != nil {
		return err
	}
	if err = p.add(equality); err != nil {
		return err
	}
	return p.assertNegation()
}

// assertNegation is assert_axiom(mgr.mk_not(mk_and(items))).
func (p *smtStringLengthPremises) assertNegation() error {
	m := p.theory.state.manager
	conjunction, err := m.MkAndArray(p.values)
	if err != nil {
		return err
	}
	negative, err := m.MkNot(conjunction)
	if err != nil {
		return err
	}
	return p.theory.assertAxiom(negative)
}

func (t *SMTStringTheory) checkLengthConstant(expression, constant ast.ASTHandle) (consistent bool, result error) {
	m := t.state.manager
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	q := t.q()
	// rational const_str_len, the sum, and each leaf's length.
	target, sum, value := smtQZero(), smtQZero(), smtQZero()
	defer t.qdel(&value, &result)
	defer t.qdel(&sum, &result)
	defer t.qdel(&target, &result)
	if err = q.SetUint64(&target, uint64(len(text))); err != nil {
		return false, err
	}
	concat, err := m.IsSequenceConcat(expression)
	if err != nil {
		return false, err
	}
	if !concat {
		known, err := t.lengthValue(expression, &value)
		if err != nil {
			return false, err
		}
		if c, err := q.Compare(&value, &target); err != nil {
			return false, err
		} else if known && c != 0 {
			return false, t.assertStringLengthEqualityOrder(expression, constant, true)
		}
		return t.checkLengthConstantTail(expression, constant)
	}
	leaves, err := t.concatLeaves(expression)
	if err != nil {
		return false, err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	for _, leaf := range leaves {
		known, err := t.lengthValue(leaf, &value)
		if err != nil {
			return false, err
		}
		if !known {
			continue
		}
		if _, literal, err := m.SequenceStringValue(leaf); err != nil {
			return false, err
		} else if !literal {
			if err = premises.length(leaf, &value); err != nil {
				return false, err
			}
		}
		if err = q.Add(&sum, &value, &sum); err != nil {
			return false, err
		}
		if c, err := q.Compare(&sum, &target); err != nil {
			return false, err
		} else if c > 0 {
			return false, premises.refuse(expression, constant)
		}
	}
	return t.checkLengthConstantTail(expression, constant)
}

// checkLengthConstantTail is check_length_const_string's final step for both
// branches: a second get_len_value(n1), and n1 = c => len(n1) = len(c) when
// the length is still unknown.
func (t *SMTStringTheory) checkLengthConstantTail(expression, constant ast.ASTHandle) (_ bool, err error) {
	value := smtQZero()
	defer t.qdel(&value, &err)
	known, err := t.lengthValue(expression, &value)
	if err != nil {
		return false, err
	}
	if !known {
		return true, t.assertStringLengthEqualityOrder(expression, constant, true)
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthConcats(left, right ast.ASTHandle) (consistent bool, result error) {
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	q := t.q()
	sums := [2]numeral.Rational{smtQZero(), smtQZero()}
	value := smtQZero()
	defer t.qdel(&value, &result)
	defer t.qdel(&sums[1], &result)
	defer t.qdel(&sums[0], &result)
	fixed := [2]bool{true, true}
	for side, expression := range []ast.ASTHandle{left, right} {
		leaves, err := t.concatLeaves(expression)
		if err != nil {
			return false, err
		}
		for _, leaf := range leaves {
			known, err := t.lengthValue(leaf, &value)
			if err != nil {
				return false, err
			}
			if !known {
				fixed[side] = false
				continue
			}
			if err = q.Add(&sums[side], &value, &sums[side]); err != nil {
				return false, err
			}
			if _, literal, err := t.state.manager.SequenceStringValue(leaf); err != nil {
				return false, err
			} else if !literal {
				if err = premises.length(leaf, &value); err != nil {
					return false, err
				}
			}
		}
	}
	// items.push_back(ctx.mk_eq_atom(n1, n2)) precedes the conflict test.
	equality, err := t.state.MkEqAtom(left, right)
	if err != nil {
		return false, err
	}
	if err = premises.add(equality); err != nil {
		return false, err
	}
	comparison, err := q.Compare(&sums[0], &sums[1])
	if err != nil {
		return false, err
	}
	conflict := fixed[0] && fixed[1] && comparison != 0 || !fixed[0] && fixed[1] && comparison > 0 || fixed[0] && !fixed[1] && comparison < 0
	if conflict {
		return false, premises.assertNegation()
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthConcatVariable(concat, variable ast.ASTHandle) (consistent bool, result error) {
	q := t.q()
	target, sum, value := smtQZero(), smtQZero(), smtQZero()
	defer t.qdel(&value, &result)
	defer t.qdel(&sum, &result)
	defer t.qdel(&target, &result)
	known, err := t.lengthValue(variable, &target)
	if err != nil || !known {
		return true, err
	}
	leaves, err := t.concatLeaves(concat)
	if err != nil {
		return false, err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	for _, leaf := range leaves {
		known, err := t.lengthValue(leaf, &value)
		if err != nil {
			return false, err
		}
		if !known {
			continue
		}
		if _, literal, err := t.state.manager.SequenceStringValue(leaf); err != nil {
			return false, err
		} else if !literal && value.Sign() != 0 {
			if err = premises.length(leaf, &value); err != nil {
				return false, err
			}
		}
		if err = q.Add(&sum, &value, &sum); err != nil {
			return false, err
		}
		if c, err := q.Compare(&sum, &target); err != nil {
			return false, err
		} else if c > 0 {
			if err = premises.length(variable, &target); err != nil {
				return false, err
			}
			return false, premises.refuse(concat, variable)
		}
	}
	return true, nil
}

func (t *SMTStringTheory) checkLengthVariables(left, right ast.ASTHandle) (consistent bool, result error) {
	l, r := smtQZero(), smtQZero()
	defer t.qdel(&r, &result)
	defer t.qdel(&l, &result)
	lk, err := t.lengthValue(left, &l)
	if err != nil {
		return false, err
	}
	rk, err := t.lengthValue(right, &r)
	if err != nil {
		return false, err
	}
	if !lk || !rk {
		return true, nil
	}
	if eq, err := t.q().Equal(&l, &r); err != nil || eq {
		return err == nil, err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err = premises.length(left, &l); err != nil {
		return false, err
	}
	if err = premises.length(right, &r); err != nil {
		return false, err
	}
	return false, premises.refuse(left, right)
}

func (t *SMTStringTheory) checkLengthConsistency(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	_, ll, err := m.SequenceStringValue(left)
	if err != nil {
		return false, err
	}
	_, rl, err := m.SequenceStringValue(right)
	if err != nil {
		return false, err
	}
	if ll && rl {
		return true, nil
	}
	if ll {
		return t.checkLengthConstant(right, left)
	}
	if rl {
		return t.checkLengthConstant(left, right)
	}
	lc, err := m.IsSequenceConcat(left)
	if err != nil {
		return false, err
	}
	rc, err := m.IsSequenceConcat(right)
	if err != nil {
		return false, err
	}
	if lc && rc {
		return t.checkLengthConcats(left, right)
	}
	if lc {
		return t.checkLengthConcatVariable(left, right)
	}
	if rc {
		return t.checkLengthConcatVariable(right, left)
	}
	return t.checkLengthVariables(left, right)
}
