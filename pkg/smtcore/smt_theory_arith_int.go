package smtcore

// Source: src/smt/theory_arith_int.h, ported in source order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/euclid"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// moveNonBaseVarsToBounds moves non-base variables to one of their bounds;
// an unbounded integer variable with a non-integer value is rounded down.
func (t *SMTTheoryArith) moveNonBaseVarsToBounds() error {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isNonBase(v) {
			continue
		}
		l, u := t.lower(v), t.upper(v)
		val := t.getValue(v)
		switch {
		case l != nil && u != nil:
			if t.vcmp(val, &l.base().value) != 0 && t.vcmp(val, &u.base().value) != 0 {
				if err := t.setValue(v, &l.base().value); err != nil {
					return err
				}
			}
		case l != nil:
			if t.vcmp(val, &l.base().value) != 0 {
				if err := t.setValue(v, &l.base().value); err != nil {
					return err
				}
			}
		case u != nil:
			if t.vcmp(val, &u.base().value) != 0 {
				if err := t.setValue(v, &u.base().value); err != nil {
					return err
				}
			}
		default:
			if t.isInt(v) && !val.isInt() {
				if err := t.setValueFloor(v, val); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// hasInfeasibleIntVar reports whether an integer variable has a non-integer
// value.
func (t *SMTTheoryArith) hasInfeasibleIntVar() bool {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if t.isInt(v) && !t.getValue(v).isInt() {
			return true
		}
	}
	return false
}

// findBoundedInfeasibleIntBaseVar finds a bounded integer base variable with
// a non-integer value, preferring the tightest bounds.
func (t *SMTTheoryArith) findBoundedInfeasibleIntBaseVar() SMTTheoryVar {
	result := SMTNullTheoryVar
	q := t.q
	rng, newRange := smtQZero(), smtQZero()
	defer func() {
		t.keep(smtQDel(q, &rng))
		t.keep(smtQDel(q, &newRange))
	}()
	smallRangeThreshold := smtQInt(1024)
	n := uint32(0)
	for i := range t.rows {
		v := t.rows[i].base
		if v == SMTNullTheoryVar || !t.isBase(v) || !t.isInt(v) || t.getValue(v).isInt() || !t.isBounded(v) {
			continue
		}
		t.keep(q.Sub(&t.upperBound(v).rational, &t.lowerBound(v).rational, &newRange))
		if t.rcmp(&newRange, &smallRangeThreshold) > 0 {
			continue
		}
		if result == SMTNullTheoryVar {
			result = v
			t.keep(q.Copy(&rng, &newRange))
			n = 1
			continue
		}
		if t.rcmp(&newRange, &rng) < 0 {
			n = 1
			result = v
			t.keep(q.Copy(&rng, &newRange))
			continue
		}
		if t.rcmp(&newRange, &rng) == 0 {
			n++
			if t.random.Next()%n == 0 {
				result = v
				t.keep(q.Copy(&rng, &newRange))
				continue
			}
		}
	}
	return result
}

// findInfeasibleIntBaseVar prefers bounded variables, then picks a random
// infeasible base variable, then a quasi-base one.
func (t *SMTTheoryArith) findInfeasibleIntBaseVar() (SMTTheoryVar, error) {
	if v := t.findBoundedInfeasibleIntBaseVar(); v != SMTNullTheoryVar {
		return v, nil
	}
	n := uint32(0)
	r := SMTNullTheoryVar
	selectVar := func(v SMTTheoryVar) {
		if r == SMTNullTheoryVar {
			n = 1
			r = v
		} else {
			n++
			if t.random.Next()%n == 0 {
				r = v
			}
		}
	}
	for i := range t.rows {
		v := t.rows[i].base
		if v != SMTNullTheoryVar && t.isBase(v) && t.isInt(v) && !t.getValue(v).isInt() {
			selectVar(v)
		}
	}
	if r == SMTNullTheoryVar {
		for i := range t.rows {
			v := t.rows[i].base
			if v != SMTNullTheoryVar && t.isQuasiBase(v) && t.isInt(v) && !t.getValue(v).isInt() {
				if err := t.quasiBaseRow2BaseRow(t.getVarRow(v)); err != nil {
					return SMTNullTheoryVar, err
				}
				selectVar(v)
			}
		}
	}
	return r, nil
}

// branchInfeasibleIntVar creates the branch-and-bound case split v >= ceil.
func (t *SMTTheoryArith) branchInfeasibleIntVar(v SMTTheoryVar) (err error) {
	t.stats.branches++
	k := smtQZero()
	defer smtQDel(t.q, &k)
	if err := smtInfCeil(t.q, t.getValue(v), &k); err != nil {
		return err
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	e := t.var2expr(v)
	isIntE, err := t.isIntSort(e)
	if err != nil {
		return err
	}
	kn, err := t.mkNumeralQ(&x, &k, isIntE)
	if err != nil {
		return err
	}
	bound, err := t.mkApp(&x, smtArithOpGE, e, kn)
	if err != nil {
		return err
	}
	if err = t.state.Internalize(bound, true); err != nil {
		return err
	}
	return t.state.markRelevant(bound)
}

// branchInfeasibleIntEquality creates a "cut from proof" lemma from the rows
// whose base variable is tight.
func (t *SMTTheoryArith) branchInfeasibleIntEquality() (result bool, err error) {
	var rows [][]numeral.Rational
	maxRow := 1 // all rows contain a constant in the first position.
	var2index := make(map[SMTTheoryVar]int)
	index2var := make(map[int]SMTTheoryVar)
	q := t.q
	one := newSMTArithmeticValue()
	one.rational = smtQInt(1)
	tmp := newSMTArithmeticValue()
	defer tmp.del(q)
	constCoeff, denom, d, prod := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	var unsatRow []numeral.Rational
	defer func() {
		for _, r := range []*numeral.Rational{&constCoeff, &denom, &d, &prod} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
		for i := range rows {
			if _, e := smtQDelAll(q, rows[i]); e != nil && err == nil {
				err = e
			}
		}
		if _, e := smtQDelAll(q, unsatRow); e != nil && err == nil {
			err = e
		}
	}()
	for ri := range t.rows {
		it := &t.rows[ri]
		b := it.base
		if b == SMTNullTheoryVar {
			continue
		}
		isTight := false
		t.keep(q.SetInt64(&constCoeff, 0))
		l, u := t.lower(b), t.upper(b)
		t.keep(t.vsub(&tmp, t.getValue(b), &one))
		if l != nil && t.vcmp(&tmp, &l.base().value) < 0 {
			isTight = true
			t.keep(q.Copy(&constCoeff, &l.base().value.rational))
		} else {
			t.keep(tmp.set(q, t.getValue(b)))
			t.keep(tmp.add(q, &one))
			if u != nil && t.vcmp(&tmp, &u.base().value) > 0 {
				isTight = true
				t.keep(q.Copy(&constCoeff, &u.base().value.rational))
			}
		}
		if !isTight {
			continue
		}
		row := make([]numeral.Rational, maxRow)
		for i := range row {
			row[i] = smtQZero()
		}
		t.keep(q.SetInt64(&denom, 1))
		for ei := range it.entries {
			if !isTight {
				break
			}
			e := &it.entries[ei]
			if e.dead() {
				continue
			}
			x := e.variable
			if x == b {
				continue
			}
			coeff := &e.coefficient
			if t.isFixed(x) {
				t.keep(q.Mul(coeff, &t.lower(x).base().value.rational, &prod))
				t.keep(q.Add(&constCoeff, &prod, &constCoeff))
				continue
			}
			if !t.isInt(x) {
				isTight = false
				continue
			}
			if index, found := var2index[x]; found {
				t.keep(q.Copy(&row[index], coeff))
			} else {
				c, e := smtQNew(q, coeff)
				t.keep(e)
				row = append(row, c)
				var2index[x] = maxRow
				index2var[maxRow] = x
				maxRow++
			}
			t.keep(q.DenominatorRational(coeff, &d))
			t.keep(q.LCM(&d, &denom, &denom))
		}
		if !isTight {
			_, e := smtQDelAll(q, row)
			t.keep(e)
			continue
		}
		t.keep(q.Copy(&row[0], &constCoeff))
		t.keep(q.DenominatorRational(&constCoeff, &d))
		t.keep(q.LCM(&d, &denom, &denom))
		if one, e := q.IsOne(&denom); e != nil || !one {
			t.keep(e)
			for i := 0; i < maxRow && i < len(row); i++ {
				t.keep(q.Mul(&row[i], &denom, &row[i]))
			}
		}
		rows = append(rows, row)
	}
	// Align the sizes of rows; the sizes are monotonically increasing.
	for i := range rows {
		if len(rows[i]) == maxRow {
			break
		}
		for len(rows[i]) < maxRow {
			rows[i] = append(rows[i], smtQZero())
		}
	}
	if err := t.takeQErr(nil); err != nil {
		return false, err
	}
	solver := smtArithmeticEquationSolver{q: q}
	feasible, unsatRow, err := solver.SolveEquations(rows)
	if err != nil || feasible {
		// The equalities were integer feasible.
		return false, err
	}
	var pol []smtArithLinearMonomial
	defer func() {
		for i := range pol {
			if e := smtQDel(q, &pol[i].coeff); e != nil && err == nil {
				err = e
			}
		}
	}()
	for i := 1; i < len(unsatRow); i++ {
		zero, err := q.IsZero(&unsatRow[i])
		if err != nil {
			return false, err
		}
		if zero {
			continue
		}
		v, found := index2var[i]
		if !found {
			return false, ast.ErrASTDomain // UNREACHABLE
		}
		c, err := smtQNew(q, &unsatRow[i])
		if err != nil {
			return false, err
		}
		pol = append(pol, smtArithLinearMonomial{coeff: c, v: v})
	}
	if len(pol) == 0 {
		return false, nil // the witness is trivial
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	k0, k1, k2 := smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&k0, &k1, &k2} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	if err := q.Copy(&k0, &unsatRow[0]); err != nil {
		return false, err
	}
	if err := q.Copy(&k1, &k0); err != nil {
		return false, err
	}
	if err := q.Neg(&k1); err != nil {
		return false, err
	}
	if err := q.Inc(&k1); err != nil {
		return false, err
	}
	p1, err := t.mkPolynomialGe(&x, pol, &k1)
	if err != nil {
		return false, err
	}
	for i := range pol {
		if err := q.Neg(&pol[i].coeff); err != nil {
			return false, err
		}
	}
	if err := q.Copy(&k2, &k0); err != nil {
		return false, err
	}
	if err := q.Inc(&k2); err != nil {
		return false, err
	}
	p2, err := t.mkPolynomialGe(&x, pol, &k2)
	if err != nil {
		return false, err
	}
	if err = t.state.Internalize(p1, false); err != nil {
		return false, err
	}
	if err = t.state.Internalize(p2, false); err != nil {
		return false, err
	}
	l1, err := t.state.Literal(p1)
	if err != nil {
		return false, err
	}
	l2, err := t.state.Literal(p2)
	if err != nil {
		return false, err
	}
	if err = t.state.markRelevant(p1); err != nil {
		return false, err
	}
	if err = t.state.markRelevant(p2); err != nil {
		return false, err
	}
	return true, t.state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{l1, l2})
}

// constrainFreeVars creates bounds v >= 0 for free non-base variables of the
// row, enabling Gomory cuts. It reports whether a variable was constrained.
func (t *SMTTheoryArith) constrainFreeVars(r *smtArithmeticRow) (result bool, err error) {
	b := r.base
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	// The C++ walks [begin_entries(), end_entries()) with both iterators
	// fixed before the loop, but ctx.internalize(bound) can reach add_row,
	// whose compress_if_needed shrinks r's entries in place; the C++ then
	// reads destroyed slots past the new end (undefined behavior). Re-read
	// the current length on every step so a compressed row ends the walk.
	for i := 0; i < len(r.entries); i++ {
		it := &r.entries[i]
		if it.dead() || it.variable == b || !t.isFree(it.variable) {
			continue
		}
		v := it.variable
		zero, err := t.mkNumeralInt(&x, 0, t.isInt(v))
		if err != nil {
			return false, err
		}
		bound, err := t.mkApp(&x, smtArithOpGE, t.var2expr(v), zero)
		if err != nil {
			return false, err
		}
		if err = t.state.Internalize(bound, true); err != nil {
			return false, err
		}
		if err = t.state.markRelevant(bound); err != nil {
			return false, err
		}
		result = true
	}
	return result, nil
}

// isGomoryCutTarget: all non-base variables are at a bound and have
// rational values.
func (t *SMTTheoryArith) isGomoryCutTarget(r *smtArithmeticRow) bool {
	b := r.base
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() && it.variable != b && (!t.atBound(it.variable) || !t.getValue(it.variable).isRational()) {
			return false
		}
	}
	return true
}

// mkPolynomialGe builds sum(args) >= k, simplified by the context rewriter.
// _args (an expr_ref_vector, released forward) and pol are locals; result is
// the caller's expr_ref, rewritten in place.
func (t *SMTTheoryArith) mkPolynomialGe(x *smtArithExprs, args []smtArithLinearMonomial, k *numeral.Rational) (_ ast.ASTHandle, resultErr error) {
	m := t.state.manager
	allInt := true
	for i := 0; i < len(args) && allInt; i++ {
		allInt = t.isInt(args[i].v)
	}
	argRefs := make([]ast.ASTRef, 0, len(args))
	var polRef ast.ASTRef
	defer func() {
		if polRef != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&polRef))
		}
		for i := range argRefs {
			resultErr = errors.Join(resultErr, m.DelRef(&argRefs[i]))
		}
	}()
	mk := func(op int32, a ...ast.ASTHandle) (ast.ASTHandle, error) {
		return m.MkFamilyApp(t.FamilyID(), op, nil, a)
	}
	mkNum := func(q *numeral.Rational, isInt bool) (ast.ASTHandle, error) {
		return m.ArithmeticNumeralFromRational(q, isInt)
	}
	terms := make([]ast.ASTHandle, 0, len(args))
	push := func(e ast.ASTHandle) error {
		r, err := m.NewRef(e)
		if err != nil {
			return err
		}
		argRefs = append(argRefs, r)
		terms = append(terms, e)
		return nil
	}
	for i := range args {
		e := t.var2expr(args[i].v)
		isIntE, err := t.isIntSort(e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isIntE && !allInt {
			if e, err = mk(smtArithOpToReal, e); err != nil {
				return ast.ASTHandle{}, err
			}
			isIntE = false
		}
		if args[i].coeff.IsOne() {
			if err = push(e); err != nil {
				return ast.ASTHandle{}, err
			}
			continue
		}
		c, err := mkNum(&args[i].coeff, isIntE)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := mk(smtArithOpMul, c, e)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = push(term); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	var pol ast.ASTHandle
	var err error
	if len(terms) == 1 {
		pol = terms[0] // m_util.mk_add with one argument returns it
	} else if pol, err = mk(smtArithOpAdd, terms...); err != nil {
		return ast.ASTHandle{}, err
	}
	if polRef, err = m.NewRef(pol); err != nil {
		return ast.ASTHandle{}, err
	}
	kn, err := mkNum(k, allInt)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := mk(smtArithOpGE, pol, kn)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.rewrite(x, result)
}

// smtArithFractionalPart is Ext::fractional_part: out = n - floor(n).
func smtArithFractionalPart(q *numeral.RationalManager, n, out *numeral.Rational) error {
	f := smtQZero()
	defer smtQDel(q, &f)
	if err := q.Floor(n, &f); err != nil {
		return err
	}
	return q.Sub(n, &f, out)
}

// mkGomoryCut creates a Gomory cut for the row.
func (t *SMTTheoryArith) mkGomoryCut(r *smtArithmeticRow) (result bool, err error) {
	xi := r.base
	constrained, err := t.constrainFreeVars(r)
	if err != nil {
		return false, err
	}
	if constrained || !t.isGomoryCutTarget(r) {
		return false, nil
	}
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	t.stats.gomoryCuts++
	q := t.q
	// gomory will be   pol >= k
	k, f0, oneMinusF0, lcmDen := smtQInt(1), smtQZero(), smtQZero(), smtQInt(1)
	aij, newAij, fj, den := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	one := smtQInt(1)
	var pol []smtArithLinearMonomial
	defer func() {
		for _, r := range []*numeral.Rational{&k, &f0, &oneMinusF0, &lcmDen, &aij, &newAij, &fj, &den} {
			t.keep(smtQDel(q, r))
		}
		for i := range pol {
			t.keep(smtQDel(q, &pol[i].coeff))
		}
		err = t.takeQErr(err)
	}()
	if !t.value[xi].isRational() {
		return false, ast.ErrASTDomain
	}
	t.keep(smtArithFractionalPart(q, &t.value[xi].rational, &f0))
	t.keep(q.Sub(&one, &f0, &oneMinusF0))
	numInts := 0
	coeffs := t.coeffsEnabled()
	push := func(xj SMTTheoryVar) {
		pol = append(pol, smtArithLinearMonomial{coeff: smtQZero(), v: xj})
		t.keep(q.Copy(&pol[len(pol)-1].coeff, &newAij))
	}
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || it.variable == xi {
			continue
		}
		xj := it.variable
		t.keep(q.Copy(&aij, &it.coefficient))
		t.keep(q.Neg(&aij)) // the format of "Integrating Simplex with DPLL(T)"
		if t.isReal(xj) {
			if t.atLower(xj) {
				if aij.Sign() > 0 {
					t.keep(q.Div(&aij, &oneMinusF0, &newAij))
				} else {
					t.keep(q.Div(&aij, &f0, &newAij))
					t.keep(q.Neg(&newAij))
				}
				t.keep(q.AddMul(&k, &newAij, &t.lowerBound(xj).rational, &k))
				if err = t.lower(xj).pushJustification(q, ante, &newAij, coeffs); err != nil {
					return false, err
				}
			} else {
				if aij.Sign() > 0 {
					t.keep(q.Div(&aij, &f0, &newAij))
					t.keep(q.Neg(&newAij)) // the upper terms are inverted.
				} else {
					t.keep(q.Div(&aij, &oneMinusF0, &newAij))
				}
				t.keep(q.AddMul(&k, &newAij, &t.upperBound(xj).rational, &k))
				if err = t.upper(xj).pushJustification(q, ante, &newAij, coeffs); err != nil {
					return false, err
				}
			}
			push(xj)
			continue
		}
		numInts++
		t.keep(smtArithFractionalPart(q, &aij, &fj))
		if fj.IsZero() {
			continue
		}
		if t.atLower(xj) {
			if t.rcmp(&fj, &oneMinusF0) <= 0 {
				t.keep(q.Div(&fj, &oneMinusF0, &newAij))
			} else {
				t.keep(q.Sub(&one, &fj, &newAij))
				t.keep(q.Div(&newAij, &f0, &newAij))
			}
			t.keep(q.AddMul(&k, &newAij, &t.lowerBound(xj).rational, &k))
			if err = t.lower(xj).pushJustification(q, ante, &newAij, coeffs); err != nil {
				return false, err
			}
		} else {
			if t.rcmp(&fj, &f0) <= 0 {
				t.keep(q.Div(&fj, &f0, &newAij))
			} else {
				t.keep(q.Sub(&one, &fj, &newAij))
				t.keep(q.Div(&newAij, &oneMinusF0, &newAij))
			}
			t.keep(q.Neg(&newAij)) // the upper terms are inverted
			t.keep(q.AddMul(&k, &newAij, &t.upperBound(xj).rational, &k))
			if err = t.upper(xj).pushJustification(q, ante, &newAij, coeffs); err != nil {
				return false, err
			}
		}
		push(xj)
		t.keep(q.DenominatorRational(&newAij, &den))
		t.keep(q.LCM(&lcmDen, &den, &lcmDen))
	}
	if err := t.takeQErr(nil); err != nil {
		return false, err
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	var bound ast.ASTHandle
	switch {
	case len(pol) == 0:
		// conflict 0 >= k where k is positive
		return true, t.setConflict(ante, "gomory-cut")
	case len(pol) == 1:
		v := pol[0].v
		if err := q.Div(&k, &pol[0].coeff, &k); err != nil {
			return false, err
		}
		isLower := pol[0].coeff.Sign() > 0
		if t.isInt(v) && !k.IsInt() {
			if isLower {
				err = q.Ceil(&k, &k)
			} else {
				err = q.Floor(&k, &k)
			}
			if err != nil {
				return false, err
			}
		}
		kn, err := t.mkNumeralQ(&x, &k, t.isInt(v))
		if err != nil {
			return false, err
		}
		op := smtArithOpLE
		if isLower {
			op = smtArithOpGE
		}
		if bound, err = t.mkApp(&x, op, t.var2expr(v), kn); err != nil {
			return false, err
		}
	default:
		if numInts > 0 {
			if err := q.DenominatorRational(&k, &den); err != nil {
				return false, err
			}
			if err := q.LCM(&lcmDen, &den, &lcmDen); err != nil {
				return false, err
			}
			if !lcmDen.IsOne() {
				// normalize coefficients of integer parameters to be integers.
				for i := range pol {
					if err := q.Mul(&pol[i].coeff, &lcmDen, &pol[i].coeff); err != nil {
						return false, err
					}
				}
				if err := q.Mul(&k, &lcmDen, &k); err != nil {
					return false, err
				}
			}
		}
		if bound, err = t.mkPolynomialGe(&x, pol, &k); err != nil {
			return false, err
		}
	}
	if err = t.state.Internalize(bound, true); err != nil {
		return false, err
	}
	l, err := t.state.Literal(bound)
	if err != nil {
		return false, err
	}
	if err = t.markLiteralAsRelevant(l); err != nil {
		return false, err
	}
	if err = t.dumpLemmas(l, ante.literals, ante.equalities); err != nil {
		return false, err
	}
	params, err := ante.parameters(m, "gomory-cut")
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, smtArithmeticReleaseParameters(m, params)) }()
	owner, err := NewSMTExtTheoryPropagationJustification(m, t.FamilyID(), ante.literals, ante.equalities, l, params)
	if err != nil {
		return false, err
	}
	// gomory_cut_justification: the assignment must be propagated back to arith.
	owner.returnToOwner = true
	justification, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return false, errors.Join(err, owner.Release(m))
	}
	if err = t.state.AssignLiteral(l, justification); err != nil {
		return false, errors.Join(err, owner.Release(m))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	return true, nil
}

// gcdTest reports false when the row fails the GCD test, i.e. a conflict
// was set. With bounded least-coefficient variables, ext_gcd_test runs.
func (t *SMTTheoryArith) gcdTest(rID int) (bool, error) {
	r := &t.rows[rID]
	if !t.params.GCDTest {
		return true, nil
	}
	t.stats.gcdTests++
	q := t.q
	lcmDen, consts, gcds, leastCoeff, aux := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&lcmDen, &consts, &gcds, &leastCoeff, &aux} {
			t.keep(smtQDel(q, r))
		}
	}()
	if err := r.denominatorLCM(q, &lcmDen); err != nil {
		return false, err
	}
	leastCoeffIsBounded := false
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if t.isFixed(v) {
			// get_value(v) may not satisfy v's bounds here; use the bound.
			if err := q.Mul(&lcmDen, &it.coefficient, &aux); err != nil {
				return false, err
			}
			if err := q.AddMul(&consts, &aux, &t.lowerBound(v).rational, &consts); err != nil {
				return false, err
			}
		} else if t.isReal(v) {
			return true, nil
		} else if gcds.IsZero() {
			if err := q.Mul(&lcmDen, &it.coefficient, &gcds); err != nil {
				return false, err
			}
			if err := q.Abs(&gcds); err != nil {
				return false, err
			}
			if err := q.Copy(&leastCoeff, &gcds); err != nil {
				return false, err
			}
			leastCoeffIsBounded = t.isBounded(v)
		} else {
			if err := q.Mul(&lcmDen, &it.coefficient, &aux); err != nil {
				return false, err
			}
			if err := q.Abs(&aux); err != nil {
				return false, err
			}
			if err := q.GCD(&gcds, &aux, &gcds); err != nil {
				return false, err
			}
			if c := t.rcmp(&aux, &leastCoeff); c < 0 {
				if err := q.Copy(&leastCoeff, &aux); err != nil {
					return false, err
				}
				leastCoeffIsBounded = t.isBounded(v)
			} else if leastCoeffIsBounded && c == 0 {
				leastCoeffIsBounded = t.isBounded(v)
			}
		}
	}
	if gcds.IsZero() {
		// All variables are fixed, and fixed integer variables have integer
		// values, so the row is satisfied.
		return true, t.takeQErr(nil)
	}
	if err := q.Div(&consts, &gcds, &aux); err != nil {
		return false, err
	}
	if !aux.IsInt() {
		ante := t.acquireAntecedents()
		defer t.releaseAntecedentsScope()
		if err := t.collectFixedVarJustifications(r, ante); err != nil {
			return false, err
		}
		return false, t.gcdConflict(ante)
	}
	if leastCoeff.IsOne() && !leastCoeffIsBounded {
		return true, t.takeQErr(nil)
	}
	if leastCoeffIsBounded {
		return t.extGcdTest(r, &leastCoeff, &lcmDen, &consts)
	}
	return true, t.takeQErr(nil)
}

// gcdConflict is ctx.set_conflict(ext_theory_conflict_justification(...,
// "gcd-test")); it does not go through theory_arith::set_conflict.
func (t *SMTTheoryArith) gcdConflict(ante *smtArithmeticAntecedents) (result error) {
	m := t.state.manager
	params, err := ante.parameters(m, "gcd-test")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, params)) }()
	if len(ante.equalities) == 0 {
		return t.state.SetTheoryConflict(t.FamilyID(), ante.literals, params...)
	}
	if t.state.inconsistent {
		return nil
	}
	owner, err := NewSMTExtTheoryConflictJustification(m, t.FamilyID(), ante.literals, ante.equalities, params)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(m))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	t.state.inconsistent = true
	t.state.conflict = wrapped
	t.state.conflictLiteral = SMTNullLiteral
	return nil
}

func (t *SMTTheoryArith) extGcdTest(r *smtArithmeticRow, leastCoeff, lcmDen, consts *numeral.Rational) (_ bool, err error) {
	q := t.q
	gcds, l, u, ncoeff, absNcoeff, l1, u1 := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&gcds, &l, &u, &ncoeff, &absNcoeff, &l1, &u1} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	if err := q.Copy(&l, consts); err != nil {
		return false, err
	}
	if err := q.Copy(&u, consts); err != nil {
		return false, err
	}
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	coeffs := t.coeffsEnabled()
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || t.isFixed(it.variable) {
			continue
		}
		v := it.variable
		if err := q.Mul(lcmDen, &it.coefficient, &ncoeff); err != nil {
			return false, err
		}
		if err := q.Copy(&absNcoeff, &ncoeff); err != nil {
			return false, err
		}
		if err := q.Abs(&absNcoeff); err != nil {
			return false, err
		}
		if t.rcmp(&absNcoeff, leastCoeff) == 0 {
			lo, hi := &t.lowerBound(v).rational, &t.upperBound(v).rational
			if ncoeff.Sign() <= 0 {
				lo, hi = hi, lo
			}
			if err := q.AddMul(&l, &ncoeff, lo, &l); err != nil {
				return false, err
			}
			if err := q.AddMul(&u, &ncoeff, hi, &u); err != nil {
				return false, err
			}
			if err := t.lower(v).pushJustification(q, ante, &it.coefficient, coeffs); err != nil {
				return false, err
			}
			if err := t.upper(v).pushJustification(q, ante, &it.coefficient, coeffs); err != nil {
				return false, err
			}
		} else if gcds.IsZero() {
			if err := q.Copy(&gcds, &absNcoeff); err != nil {
				return false, err
			}
		} else if err := q.GCD(&gcds, &absNcoeff, &gcds); err != nil {
			return false, err
		}
	}
	if gcds.IsZero() {
		return true, t.takeQErr(nil)
	}
	// l1 = ceil(l/gcds), u1 = floor(u/gcds)
	if err := q.Div(&l, &gcds, &l1); err != nil {
		return false, err
	}
	if err := q.Ceil(&l1, &l1); err != nil {
		return false, err
	}
	if err := q.Div(&u, &gcds, &u1); err != nil {
		return false, err
	}
	if err := q.Floor(&u1, &u1); err != nil {
		return false, err
	}
	if t.rcmp(&u1, &l1) < 0 {
		if err := t.collectFixedVarJustifications(r, ante); err != nil {
			return false, err
		}
		return false, t.gcdConflict(ante)
	}
	return true, t.takeQErr(nil)
}

// gcdTestAll reports whether all rows pass the GCD test.
func (t *SMTTheoryArith) gcdTestAll() (bool, error) {
	if !t.params.GCDTest || t.eagerGCD {
		return true, nil
	}
	for i := range t.rows {
		v := t.rows[i].base
		if v == SMTNullTheoryVar || !t.isInt(v) || t.getValue(v).isInt() {
			continue
		}
		ok, err := t.gcdTest(i)
		if err != nil {
			return false, err
		}
		if !ok {
			if t.params.AdaptiveGCD {
				t.eagerGCD = true
			}
			return false, nil
		}
	}
	return true, nil
}

// maxMinInfeasibleIntVars tries to create bounds for unbounded infeasible
// integer variables. As in the source, it returns false when max_min over a
// batch reports consistency.
func (t *SMTTheoryArith) maxMinInfeasibleIntVars() (bool, error) {
	alreadyProcessed := make(map[SMTTheoryVar]bool)
	for {
		var vars []SMTTheoryVar
		for i := range t.rows {
			v := t.rows[i].base
			if v != SMTNullTheoryVar && t.isInt(v) && !t.getValue(v).isInt() && !t.isBounded(v) && !alreadyProcessed[v] {
				vars = append(vars, v)
				alreadyProcessed[v] = true
			}
		}
		if len(vars) == 0 {
			return true, nil
		}
		ok, err := t.maxMinVars(vars)
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}
}

// patchIntInfeasibleVars patches integer-infeasible non-base variables
// using their freedom intervals.
func (t *SMTTheoryArith) patchIntInfeasibleVars() (err error) {
	q := t.q
	num := SMTTheoryVar(t.getNumVars())
	var infL, infU bool
	l, u, tmp := newSMTArithmeticValue(), newSMTArithmeticValue(), newSMTArithmeticValue()
	m, c := smtQZero(), smtQZero()
	defer func() {
		for _, x := range []*smtArithmeticValue{&l, &u, &tmp} {
			if e := x.del(q); e != nil && err == nil {
				err = e
			}
		}
		for _, x := range []*numeral.Rational{&m, &c} {
			if e := smtQDel(q, x); e != nil && err == nil {
				err = e
			}
		}
	}()
	// round sets x = round(x), then x = m*round(x/m) when m is not one.
	round := func(x *smtArithmeticValue, up bool) error {
		r := smtInfFloor
		if up {
			r = smtInfCeil
		}
		if err := r(q, x, &c); err != nil {
			return err
		}
		return x.setRational(q, &c)
	}
	roundM := func(x *smtArithmeticValue, up bool) error {
		r := smtInfFloor
		if up {
			r = smtInfCeil
		}
		if err := tmp.set(q, x); err != nil {
			return err
		}
		if err := tmp.quoRat(q, &m); err != nil {
			return err
		}
		if err := r(q, &tmp, &c); err != nil {
			return err
		}
		if err := q.Mul(&c, &m, &c); err != nil {
			return err
		}
		return x.setRational(q, &c)
	}
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isNonBase(v) {
			continue
		}
		t.getFreedomInterval(v, &infL, &l, &infU, &u, &m)
		if err := t.takeQErr(nil); err != nil {
			return err
		}
		if m.IsOne() && t.getValue(v).isInt() {
			continue
		}
		// check whether the value of v is already a multiple of m.
		if err := q.Div(&t.getValue(v).rational, &m, &c); err != nil {
			return err
		}
		if c.IsInt() {
			continue
		}
		if !infL {
			if err := round(&l, true); err != nil {
				return err
			}
		}
		if !infU {
			if err := round(&u, false); err != nil {
				return err
			}
		}
		if !m.IsOne() {
			if !infL {
				if err := roundM(&l, true); err != nil {
					return err
				}
			}
			if !infU {
				if err := roundM(&u, false); err != nil {
					return err
				}
			}
		}
		if !infL && !infU && t.vcmp(&l, &u) > 0 {
			continue // cannot patch
		}
		var err error
		switch {
		case !infL:
			err = t.setValue(v, &l)
		case !infU:
			err = t.setValue(v, &u)
		default:
			zero := newSMTArithmeticValue()
			err = t.setValue(v, &zero)
		}
		if err != nil {
			return err
		}
	}
	return t.takeQErr(nil)
}

// fixNonBaseVars forces all non-base integer variables to integer values.
func (t *SMTTheoryArith) fixNonBaseVars() error {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isNonBase(v) || !t.isInt(v) || t.getValue(v).isInt() {
			continue
		}
		if err := t.setValueFloor(v, t.getValue(v)); err != nil {
			return err
		}
	}
	ok, err := t.makeFeasible()
	if err != nil {
		return err
	}
	if !ok {
		return t.failed()
	}
	return nil
}

// checkIntFeasibility returns done when the assignment is integer feasible;
// otherwise it applies the GCD test, branch and bound and Gomory cuts.
func (t *SMTTheoryArith) checkIntFeasibility() (SMTFinalCheckStatus, error) {
	if !t.hasInfeasibleIntVar() {
		return SMTFinalCheckDone, nil
	}
	if t.params.IgnoreInt {
		return SMTFinalCheckGiveUp, nil
	}
	ok, err := t.gcdTestAll()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if !ok {
		return SMTFinalCheckContinue, nil
	}
	if t.params.EuclideanSolver {
		if _, err = t.applyEuclideanSolver(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
	}
	if t.state.inconsistent {
		return SMTFinalCheckContinue, nil
	}
	if err = t.removeFixedVarsFromBase(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err = t.patchIntInfeasibleVars(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err = t.fixNonBaseVars(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.state.inconsistent {
		return SMTFinalCheckContinue, nil
	}
	done := func() SMTFinalCheckStatus {
		if t.liberalFinalCheck || !t.changedAssignment {
			return SMTFinalCheckDone
		}
		return SMTFinalCheckContinue
	}
	intVar, err := t.findInfeasibleIntBaseVar()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if intVar == SMTNullTheoryVar {
		return done(), nil
	}
	t.branchCutCounter++
	if t.branchCutCounter%t.params.BranchCutRatio == 0 {
		if err = t.moveNonBaseVarsToBounds(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		ok, err := t.makeFeasible()
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if !ok {
			return SMTFinalCheckContinue, t.failed()
		}
		intVar, err := t.findInfeasibleIntBaseVar()
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if intVar != SMTNullTheoryVar {
			// a failure to cut is silent
			if _, err = t.mkGomoryCut(&t.rows[t.getVarRow(intVar)]); err != nil {
				return SMTFinalCheckGiveUp, err
			}
			return SMTFinalCheckContinue, nil
		}
	} else {
		if t.params.IntEqBranching {
			branched, err := t.branchInfeasibleIntEquality()
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if branched {
				return SMTFinalCheckContinue, nil
			}
		}
		intVar, err := t.findInfeasibleIntBaseVar()
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if intVar != SMTNullTheoryVar {
			// apply branching
			return SMTFinalCheckContinue, t.branchInfeasibleIntVar(intVar)
		}
	}
	return done(), nil
}

// ---- int.h:932-1259 (euclidean_solver_bridge, apply_euclidean_solver) ----

const smtArithUintMax = ^uint32(0)

type smtArithEuclideanSolverBridge struct {
	t        *SMTTheoryArith
	solver   *euclid.Solver
	integers *numeral.IntegerManager
	tv2v     []uint32       // theory var to euclidean solver var
	j2v      []SMTTheoryVar // justification to theory var
	as       *numeral.ScopedIntVector
	xs       []uint32
	js       []uint32
}

// esNumManager is m_es_num_manager, the numeral manager of the euclidean
// solver, owned by the theory.
func (t *SMTTheoryArith) esNumManager() (*numeral.RationalManager, error) {
	if t.esNumerals == nil {
		budget := arena.NewBudget(^uint64(0))
		m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
		if err != nil {
			return nil, err
		}
		t.esNumerals = m
	}
	return t.esNumerals, nil
}

func (b *smtArithEuclideanSolverBridge) close() error {
	return errors.Join(b.as.Close(), b.solver.Close())
}

func (b *smtArithEuclideanSolverBridge) mkVarFor(v SMTTheoryVar) (uint32, error) {
	for int(v) >= len(b.tv2v) {
		b.tv2v = append(b.tv2v, smtArithUintMax)
	}
	if b.tv2v[v] == smtArithUintMax {
		x, err := b.solver.MkVar()
		if err != nil {
			return 0, err
		}
		b.tv2v[v] = x
	}
	return b.tv2v[v], nil
}

// getMonomial splits mon = a * pp; a is a rational of t.q that the caller
// frees.
func (b *smtArithEuclideanSolverBridge) getMonomial(mon ast.ASTHandle, a *numeral.Rational) (ast.ASTHandle, error) {
	t := b.t
	if t.isOp(mon, smtArithOpMul) {
		if args, err := t.args(mon); err == nil && len(args) == 2 {
			ok, err := t.isNumeralQ(args[0], a)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if ok {
				return args[1], nil
			}
		}
	}
	return mon, t.q.SetInt64(a, 1)
}

// getTheoryVar is the theory var associated with the power product.
func (b *smtArithEuclideanSolverBridge) getTheoryVar(pp ast.ASTHandle) SMTTheoryVar {
	if e := b.t.state.context.enodes.get(pp); e != nil && b.t.IsAttachedToVariable(e) {
		return e.TheoryVar(b.t.FamilyID())
	}
	return SMTNullTheoryVar
}

func (b *smtArithEuclideanSolverBridge) mkVarExpr(pp ast.ASTHandle) (uint32, error) {
	v := b.getTheoryVar(pp)
	if v == SMTNullTheoryVar {
		return smtArithUintMax, nil
	}
	return b.mkVarFor(v)
}

func (b *smtArithEuclideanSolverBridge) getVar(pp ast.ASTHandle) uint32 {
	v := b.getTheoryVar(pp)
	if v == SMTNullTheoryVar || int(v) >= len(b.tv2v) {
		return smtArithUintMax
	}
	return b.tv2v[v]
}

// setNumerator is m.set(out, r.to_mpq().numerator()): out is an mpz of the
// euclidean solver's manager, r a rational of t.q.
func (b *smtArithEuclideanSolverBridge) setNumerator(out *numeral.Int, r *numeral.Rational) error {
	z := b.t.q.Integers()
	n := numeral.NewInt(0)
	err := b.t.q.Numerator(r, &n)
	if err == nil {
		err = b.integers.CopyFrom(out, z, &n)
	}
	return errors.Join(err, z.Del(&n))
}

// rationalOf is rational(a) for an mpz a of the euclidean solver's
// manager, into out, a rational of t.q.
func (b *smtArithEuclideanSolverBridge) rationalOf(a *numeral.Int, out *numeral.Rational) error {
	z := b.t.q.Integers()
	n := numeral.NewInt(0)
	err := z.CopyFrom(&n, b.integers, a)
	if err == nil {
		err = b.t.q.SetInteger(out, &n)
	}
	return errors.Join(err, z.Del(&n))
}

// pushA is m.set(a, a_val.to_mpq().numerator()); as.push_back(a).
func (b *smtArithEuclideanSolverBridge) pushA(value *numeral.Rational) (result error) {
	tmp := numeral.NewInt(0)
	if err := b.setNumerator(&tmp, value); err != nil {
		return errors.Join(err, b.integers.Del(&tmp))
	}
	return errors.Join(b.as.Push(&tmp), b.integers.Del(&tmp))
}

func (b *smtArithEuclideanSolverBridge) monomials(n ast.ASTHandle) ([]ast.ASTHandle, error) {
	if b.t.isOp(n, smtArithOpAdd) {
		return b.t.args(n)
	}
	return []ast.ASTHandle{n}, nil
}

// assertEqs traverses the definitions of fixed integer variables looking
// for equalities.
func (b *smtArithEuclideanSolverBridge) assertEqs() (result error) {
	t := b.t
	val, aVal := smtQZero(), smtQZero()
	defer func() { result = errors.Join(result, smtQDel(t.q, &val), smtQDel(t.q, &aVal)) }()
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isFixed(v) || !t.isInt(v) {
			continue // only fixed integer variables
		}
		n := t.var2expr(v)
		if ok, err := t.isNumeralQ(n, &aVal); err != nil || ok {
			if err != nil {
				return err
			}
			continue // skip stupid equality c - c = 0
		}
		if err := t.q.Copy(&val, &t.getValue(v).rational); err != nil {
			return err
		}
		if err := t.q.Neg(&val); err != nil {
			return err
		}
		c := numeral.NewInt(0)
		if err := b.setNumerator(&c, &val); err != nil {
			return errors.Join(err, b.integers.Del(&c))
		}
		j, err := b.solver.MkJustification()
		if err != nil {
			return errors.Join(err, b.integers.Del(&c))
		}
		for int(j) >= len(b.j2v) {
			b.j2v = append(b.j2v, SMTNullTheoryVar)
		}
		b.j2v[j] = v
		if err = b.as.Reset(); err != nil {
			return errors.Join(err, b.integers.Del(&c))
		}
		b.xs = b.xs[:0]
		failed := false
		args, err := b.monomials(n)
		if err != nil {
			return errors.Join(err, b.integers.Del(&c))
		}
		for _, arg := range args {
			pp, err := b.getMonomial(arg, &aVal)
			if err != nil {
				return errors.Join(err, b.integers.Del(&c))
			}
			isInt, err := t.q.IsInt(&aVal)
			if err != nil {
				return errors.Join(err, b.integers.Del(&c))
			}
			if !isInt {
				failed = true
				break
			}
			x, err := b.mkVarExpr(pp)
			if err != nil {
				return errors.Join(err, b.integers.Del(&c))
			}
			if x == smtArithUintMax {
				failed = true
				break
			}
			if err = b.pushA(&aVal); err != nil {
				return errors.Join(err, b.integers.Del(&c))
			}
			b.xs = append(b.xs, x)
		}
		if !failed {
			as, err := b.as.BorrowedValues()
			if err == nil {
				err = b.solver.AssertEq(as, b.xs, &c, j)
			}
			if err != nil {
				return errors.Join(err, b.integers.Del(&c))
			}
		}
		if err = b.integers.Del(&c); err != nil {
			return err
		}
	}
	return nil
}

func (b *smtArithEuclideanSolverBridge) mkBound(v SMTTheoryVar, k *numeral.Rational, lower bool, oldBound smtArithBound, js []uint32) error {
	t := b.t
	kind := smtArithmeticUpperBound
	if lower {
		kind = smtArithmeticLowerBound
	}
	newBound := &smtArithmeticDerivedBound{}
	newBound.variable, newBound.kind = v, kind
	newBound.value.init()
	if err := t.q.Copy(&newBound.value.rational, k); err != nil {
		return err
	}
	clear(t.tmpLitSet)
	clear(t.tmpEqSet)
	zero := smtQZero() // refine for proof gen
	if oldBound != nil {
		if err := t.accumulateJustification(oldBound, newBound, &zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	for _, j := range js {
		fixedV := b.j2v[j]
		if err := t.accumulateJustification(t.lower(fixedV), newBound, &zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
		if err := t.accumulateJustification(t.upper(fixedV), newBound, &zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	t.boundsToDelete = append(t.boundsToDelete, newBound)
	t.assertedBounds = append(t.assertedBounds, newBound)
	return nil
}

func (b *smtArithEuclideanSolverBridge) tightBoundsFor(v SMTTheoryVar) (propagated bool, result error) {
	t := b.t
	q := t.q
	n := t.var2expr(v)
	// g is the gcd of the coefficients of the variables that are not in the
	// solver.
	g, c2, aVal, bound, tmp := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&g, &c2, &aVal, &bound, &tmp} {
			result = errors.Join(result, smtQDel(q, r))
		}
	}()
	initG := false
	if err := b.as.Reset(); err != nil {
		return false, err
	}
	b.xs = b.xs[:0]
	args, err := b.monomials(n)
	if err != nil {
		return false, err
	}
	for _, arg := range args {
		pp, err := b.getMonomial(arg, &aVal)
		if err != nil {
			return false, err
		}
		isInt, err := q.IsInt(&aVal)
		if err != nil || !isInt {
			return false, err
		}
		x := b.getVar(pp)
		if x == smtArithUintMax {
			if err = q.Abs(&aVal); err != nil {
				return false, err
			}
			if initG {
				err = q.GCD(&g, &aVal, &g)
			} else {
				err = q.Copy(&g, &aVal)
			}
			if err != nil {
				return false, err
			}
			initG = true
			if one, err := q.IsOne(&g); err != nil || one {
				return false, err // gcd of the coeffs is one.
			}
		} else {
			if err = b.pushA(&aVal); err != nil {
				return false, err
			}
			b.xs = append(b.xs, x)
		}
	}
	b.js = b.js[:0]
	a, c := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { result = errors.Join(result, b.integers.Del(&a), b.integers.Del(&c)) }()
	as, err := b.as.BorrowedValues()
	if err != nil {
		return false, err
	}
	// m_solver.normalize(as, xs, c, a, c, m_js): c is both input (zero) and
	// output.
	cIn := numeral.NewInt(0)
	defer func() { result = errors.Join(result, b.integers.Del(&cIn)) }()
	if err = b.solver.Normalize(as, b.xs, &cIn, &a, &c, &b.js); err != nil {
		return false, err
	}
	if initG {
		zero, err := b.integers.IsZero(&a)
		if err != nil {
			return false, err
		}
		if !zero {
			if err = b.rationalOf(&a, &tmp); err != nil {
				return false, err
			}
			if err = q.GCD(&g, &tmp, &g); err != nil {
				return false, err
			}
		}
	} else if err = b.rationalOf(&a, &g); err != nil {
		return false, err
	}
	if one, err := q.IsOne(&g); err != nil || one {
		return false, err
	}
	if err = b.rationalOf(&c, &c2); err != nil {
		return false, err
	}
	if zero, err := q.IsZero(&g); err != nil {
		return false, err
	} else if zero {
		// The definition of v is 0 + c: v is fixed at c, justified by m_js
		// alone.
		if err = b.mkBound(v, &c2, true, nil, b.js); err != nil {
			return false, err
		}
		return true, b.mkBound(v, &c2, false, nil, b.js)
	}
	// l' := g*ceil((l - c)/g) + c and u' := g*floor((u - c)/g) + c
	newBound := func(old *numeral.Rational, ceil bool) error {
		if err := q.Sub(old, &c2, &bound); err != nil {
			return err
		}
		if err := q.Div(&bound, &g, &bound); err != nil {
			return err
		}
		var err error
		if ceil {
			err = q.Ceil(&bound, &bound)
		} else {
			err = q.Floor(&bound, &bound)
		}
		if err != nil {
			return err
		}
		if err = q.Mul(&g, &bound, &bound); err != nil {
			return err
		}
		return q.Add(&bound, &c2, &bound)
	}
	if l := t.lower(v); l != nil {
		lOld := &l.base().value.rational
		if err = newBound(lOld, true); err != nil {
			return false, err
		}
		cmp, err := q.Compare(&bound, lOld)
		if err != nil {
			return false, err
		}
		if cmp > 0 {
			propagated = true
			if err = b.mkBound(v, &bound, true, l, b.js); err != nil {
				return false, err
			}
		}
	}
	if u := t.upper(v); u != nil {
		uOld := &u.base().value.rational
		if err = newBound(uOld, false); err != nil {
			return false, err
		}
		cmp, err := q.Compare(&bound, uOld)
		if err != nil {
			return false, err
		}
		if cmp < 0 {
			propagated = true
			if err = b.mkBound(v, &bound, false, u, b.js); err != nil {
				return false, err
			}
		}
	}
	return propagated, nil
}

func (b *smtArithEuclideanSolverBridge) tightBounds() (bool, error) {
	t := b.t
	propagated := false
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if t.isFixed(v) || !t.isInt(v) || (t.lower(v) == nil && t.upper(v) == nil) {
			continue
		}
		p, err := b.tightBoundsFor(v)
		if err != nil {
			return false, err
		}
		if p {
			propagated = true
		}
		if t.state.inconsistent {
			break
		}
	}
	return propagated, nil
}

func (b *smtArithEuclideanSolverBridge) run() (bool, error) {
	if err := b.assertEqs(); err != nil {
		return false, err
	}
	if _, err := b.solver.Solve(); err != nil {
		return false, err
	}
	inconsistent, err := b.solver.Inconsistent()
	if err != nil || inconsistent {
		return false, err // TODO in the source: set conflict
	}
	return b.tightBounds()
}

func (t *SMTTheoryArith) applyEuclideanSolver() (result bool, err error) {
	m, err := t.esNumManager()
	if err != nil {
		return false, err
	}
	solver, err := euclid.NewSolver(m)
	if err != nil {
		return false, err
	}
	sm, err := solver.Manager()
	if err != nil {
		return false, errors.Join(err, solver.Close())
	}
	as, err := numeral.NewScopedIntVector(sm.Integers())
	if err != nil {
		return false, errors.Join(err, solver.Close())
	}
	esb := smtArithEuclideanSolverBridge{t: t, solver: solver, integers: sm.Integers(), as: as}
	defer func() { err = errors.Join(err, esb.close()) }()
	ok, err := esb.run()
	if err != nil || !ok {
		return false, err
	}
	_, err = t.propagateCore()
	return true, err
}
