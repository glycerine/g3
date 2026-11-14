package smtcore

// Source: src/smt/theory_arith_int.h, ported in source order.

import (
	"errors"
	"math/big"

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
			if val.cmp(&l.base().value) != 0 && val.cmp(&u.base().value) != 0 {
				if err := t.setValue(v, &l.base().value); err != nil {
					return err
				}
			}
		case l != nil:
			if val.cmp(&l.base().value) != 0 {
				if err := t.setValue(v, &l.base().value); err != nil {
					return err
				}
			}
		case u != nil:
			if val.cmp(&u.base().value) != 0 {
				if err := t.setValue(v, &u.base().value); err != nil {
					return err
				}
			}
		default:
			if t.isInt(v) && !val.isInt() {
				var newVal smtArithmeticValue
				newVal.setRational(smtInfFloor(val))
				if err := t.setValue(v, &newVal); err != nil {
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
	var rng, newRange big.Rat
	smallRangeThreshold := big.NewRat(1024, 1)
	n := uint32(0)
	for i := range t.rows {
		v := t.rows[i].base
		if v == SMTNullTheoryVar || !t.isBase(v) || !t.isInt(v) || t.getValue(v).isInt() || !t.isBounded(v) {
			continue
		}
		newRange.Sub(&t.upperBound(v).rational, &t.lowerBound(v).rational)
		if newRange.Cmp(smallRangeThreshold) > 0 {
			continue
		}
		if result == SMTNullTheoryVar {
			result = v
			rng.Set(&newRange)
			n = 1
			continue
		}
		if newRange.Cmp(&rng) < 0 {
			n = 1
			result = v
			rng.Set(&newRange)
			continue
		}
		if newRange.Cmp(&rng) == 0 {
			n++
			if t.random.Next()%n == 0 {
				result = v
				rng.Set(&newRange)
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
	k := smtInfCeil(t.getValue(v))
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	e := t.var2expr(v)
	isIntE, err := t.isIntSort(e)
	if err != nil {
		return err
	}
	kn, err := t.mkNumeral(&x, k, isIntE)
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
	var rows [][]*big.Rat
	maxRow := 1 // all rows contain a constant in the first position.
	var2index := make(map[SMTTheoryVar]int)
	index2var := make(map[int]SMTTheoryVar)
	one := smtArithmeticValue{}
	one.rational.SetInt64(1)
	for ri := range t.rows {
		it := &t.rows[ri]
		b := it.base
		if b == SMTNullTheoryVar {
			continue
		}
		isTight := false
		constCoeff := new(big.Rat)
		l, u := t.lower(b), t.upper(b)
		var tmp smtArithmeticValue
		tmp.set(t.getValue(b))
		tmp.sub(&one)
		if l != nil && tmp.cmp(&l.base().value) < 0 {
			isTight = true
			constCoeff.Set(&l.base().value.rational)
		} else {
			tmp.set(t.getValue(b))
			tmp.add(&one)
			if u != nil && tmp.cmp(&u.base().value) > 0 {
				isTight = true
				constCoeff.Set(&u.base().value.rational)
			}
		}
		if !isTight {
			continue
		}
		row := make([]*big.Rat, maxRow)
		for i := range row {
			row[i] = new(big.Rat)
		}
		denom := big.NewInt(1)
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
				constCoeff.Add(constCoeff, new(big.Rat).Mul(coeff, &t.lower(x).base().value.rational))
				continue
			}
			if !t.isInt(x) {
				isTight = false
				continue
			}
			if index, found := var2index[x]; found {
				row[index] = new(big.Rat).Set(coeff)
			} else {
				row = append(row, new(big.Rat).Set(coeff))
				var2index[x] = maxRow
				index2var[maxRow] = x
				maxRow++
			}
			denom = smtRatLCM(coeff.Denom(), denom)
		}
		if !isTight {
			continue
		}
		row[0].Set(constCoeff)
		denom = smtRatLCM(constCoeff.Denom(), denom)
		if denom.Cmp(big.NewInt(1)) != 0 {
			d := new(big.Rat).SetInt(denom)
			for i := 0; i < maxRow && i < len(row); i++ {
				row[i].Mul(row[i], d)
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
			rows[i] = append(rows[i], new(big.Rat))
		}
	}
	intRows := make([][]*big.Int, len(rows))
	for i, row := range rows {
		intRows[i] = make([]*big.Int, len(row))
		for j, q := range row {
			if !q.IsInt() {
				return false, ast.ErrASTDomain
			}
			intRows[i][j] = new(big.Int).Set(q.Num())
		}
	}
	solver := smtArithmeticEquationSolver{}
	feasible, unsatRow, err := solver.SolveEquations(intRows)
	if err != nil || feasible {
		// The equalities were integer feasible.
		return false, err
	}
	var pol []smtArithLinearMonomial
	for i := 1; i < len(unsatRow); i++ {
		if unsatRow[i].Sign() == 0 {
			continue
		}
		v, found := index2var[i]
		if !found {
			return false, ast.ErrASTDomain // UNREACHABLE
		}
		var mono smtArithLinearMonomial
		mono.coeff.SetInt(unsatRow[i])
		mono.v = v
		pol = append(pol, mono)
	}
	if len(pol) == 0 {
		return false, nil // the witness is trivial
	}
	m := t.state.manager
	x := smtArithExprs{m: m}
	defer func() { err = errors.Join(err, x.release()) }()
	k0 := new(big.Rat).SetInt(unsatRow[0])
	k1 := new(big.Rat).Neg(k0)
	k1.Add(k1, big.NewRat(1, 1))
	p1, err := t.mkPolynomialGe(&x, pol, k1)
	if err != nil {
		return false, err
	}
	for i := range pol {
		pol[i].coeff.Neg(&pol[i].coeff)
	}
	k2 := new(big.Rat).Add(k0, big.NewRat(1, 1))
	p2, err := t.mkPolynomialGe(&x, pol, k2)
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
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || it.variable == b || !t.isFree(it.variable) {
			continue
		}
		v := it.variable
		zero, err := t.mkNumeral(&x, new(big.Rat), t.isInt(v))
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
func (t *SMTTheoryArith) mkPolynomialGe(x *smtArithExprs, args []smtArithLinearMonomial, k *big.Rat) (_ ast.ASTHandle, resultErr error) {
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
	numeral := func(r *big.Rat, isInt bool) (ast.ASTHandle, error) {
		if isInt {
			return m.ArithmeticIntegerNumeralText(r.RatString())
		}
		return m.ArithmeticRealNumeralText(r.RatString())
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
		if smtRatIsOne(&args[i].coeff) {
			if err = push(e); err != nil {
				return ast.ASTHandle{}, err
			}
			continue
		}
		c, err := numeral(&args[i].coeff, isIntE)
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
	kn, err := numeral(k, allInt)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := mk(smtArithOpGE, pol, kn)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.rewrite(x, result)
}

// fractionalPart is Ext::fractional_part: n - floor(n).
func smtArithFractionalPart(n *big.Rat) *big.Rat {
	f := new(big.Rat).SetInt(smtRatFloor(n))
	return f.Sub(n, f)
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
	// gomory will be   pol >= k
	k := big.NewRat(1, 1)
	var pol []smtArithLinearMonomial
	if !t.value[xi].isRational() {
		return false, ast.ErrASTDomain
	}
	f0 := smtArithFractionalPart(&t.value[xi].rational)
	oneMinusF0 := new(big.Rat).Sub(big.NewRat(1, 1), f0)
	lcmDen := big.NewInt(1)
	numInts := 0
	coeffs := t.coeffsEnabled()
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || it.variable == xi {
			continue
		}
		xj := it.variable
		aij := new(big.Rat).Neg(&it.coefficient) // the format of "Integrating Simplex with DPLL(T)"
		newAij := new(big.Rat)
		if t.isReal(xj) {
			if t.atLower(xj) {
				if aij.Sign() > 0 {
					newAij.Quo(aij, oneMinusF0)
				} else {
					newAij.Quo(aij, f0)
					newAij.Neg(newAij)
				}
				k.Add(k, new(big.Rat).Mul(newAij, &t.lowerBound(xj).rational))
				if err = t.lower(xj).pushJustification(ante, newAij, coeffs); err != nil {
					return false, err
				}
			} else {
				if aij.Sign() > 0 {
					newAij.Quo(aij, f0)
					newAij.Neg(newAij) // the upper terms are inverted.
				} else {
					newAij.Quo(aij, oneMinusF0)
				}
				k.Add(k, new(big.Rat).Mul(newAij, &t.upperBound(xj).rational))
				if err = t.upper(xj).pushJustification(ante, newAij, coeffs); err != nil {
					return false, err
				}
			}
			var mono smtArithLinearMonomial
			mono.coeff.Set(newAij)
			mono.v = xj
			pol = append(pol, mono)
			continue
		}
		numInts++
		fj := smtArithFractionalPart(aij)
		if fj.Sign() == 0 {
			continue
		}
		if t.atLower(xj) {
			if fj.Cmp(oneMinusF0) <= 0 {
				newAij.Quo(fj, oneMinusF0)
			} else {
				newAij.Sub(big.NewRat(1, 1), fj)
				newAij.Quo(newAij, f0)
			}
			k.Add(k, new(big.Rat).Mul(newAij, &t.lowerBound(xj).rational))
			if err = t.lower(xj).pushJustification(ante, newAij, coeffs); err != nil {
				return false, err
			}
		} else {
			if fj.Cmp(f0) <= 0 {
				newAij.Quo(fj, f0)
			} else {
				newAij.Sub(big.NewRat(1, 1), fj)
				newAij.Quo(newAij, oneMinusF0)
			}
			newAij.Neg(newAij) // the upper terms are inverted
			k.Add(k, new(big.Rat).Mul(newAij, &t.upperBound(xj).rational))
			if err = t.upper(xj).pushJustification(ante, newAij, coeffs); err != nil {
				return false, err
			}
		}
		var mono smtArithLinearMonomial
		mono.coeff.Set(newAij)
		mono.v = xj
		pol = append(pol, mono)
		lcmDen = smtRatLCM(lcmDen, newAij.Denom())
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
		k.Quo(k, &pol[0].coeff)
		isLower := pol[0].coeff.Sign() > 0
		if t.isInt(v) && !k.IsInt() {
			f := new(big.Rat).SetInt(smtRatFloor(k))
			if isLower {
				f.Add(f, big.NewRat(1, 1)) // ceil of a non-integer
			}
			k = f
		}
		kn, err := t.mkNumeral(&x, k, t.isInt(v))
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
			lcmDen = smtRatLCM(lcmDen, k.Denom())
			if lcmDen.Cmp(big.NewInt(1)) != 0 {
				// normalize coefficients of integer parameters to be integers.
				d := new(big.Rat).SetInt(lcmDen)
				for i := range pol {
					pol[i].coeff.Mul(&pol[i].coeff, d)
				}
				k.Mul(k, d)
			}
		}
		if bound, err = t.mkPolynomialGe(&x, pol, k); err != nil {
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
	// The numerals are theory scratch: extGcdTest reads them but nothing
	// keeps them.
	lcmDen := &t.gcdLcmDen
	lcmDen.SetInt64(1)
	r.denominatorLCMInto(lcmDen.Num(), &t.scratchInt)
	consts := t.gcdConsts.SetInt64(0)
	gcds := t.gcdGcds.SetInt64(0)
	leastCoeff := t.gcdLeastCoeff.SetInt64(0)
	aux := &t.gcdAux
	leastCoeffIsBounded := false
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if t.isFixed(v) {
			// get_value(v) may not satisfy v's bounds here; use the bound.
			smtRatMul(aux, lcmDen, &it.coefficient)
			smtRatAdd(consts, consts, smtRatMul(aux, aux, &t.lowerBound(v).rational))
		} else if t.isReal(v) {
			return true, nil
		} else if gcds.Sign() == 0 {
			gcds.Abs(smtRatMul(aux, lcmDen, &it.coefficient))
			leastCoeff.Set(gcds)
			leastCoeffIsBounded = t.isBounded(v)
		} else {
			aux.Abs(smtRatMul(aux, lcmDen, &it.coefficient))
			// gcds and aux are integers: gcd in place on gcds' numerator.
			gcds.Num().GCD(nil, nil, gcds.Num(), aux.Num())
			if aux.Cmp(leastCoeff) < 0 {
				leastCoeff.Set(aux)
				leastCoeffIsBounded = t.isBounded(v)
			} else if leastCoeffIsBounded && aux.Cmp(leastCoeff) == 0 {
				leastCoeffIsBounded = t.isBounded(v)
			}
		}
	}
	if gcds.Sign() == 0 {
		// All variables are fixed, and fixed integer variables have integer
		// values, so the row is satisfied.
		return true, nil
	}
	if !smtRatQuo(aux, consts, gcds).IsInt() {
		ante := t.acquireAntecedents()
		defer t.releaseAntecedentsScope()
		if err := t.collectFixedVarJustifications(r, ante); err != nil {
			return false, err
		}
		return false, t.gcdConflict(ante)
	}
	if smtRatIsOne(leastCoeff) && !leastCoeffIsBounded {
		return true, nil
	}
	if leastCoeffIsBounded {
		return t.extGcdTest(r, leastCoeff, lcmDen, consts)
	}
	return true, nil
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

func (t *SMTTheoryArith) extGcdTest(r *smtArithmeticRow, leastCoeff, lcmDen, consts *big.Rat) (bool, error) {
	gcds := new(big.Rat)
	l := new(big.Rat).Set(consts)
	u := new(big.Rat).Set(consts)
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	coeffs := t.coeffsEnabled()
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || t.isFixed(it.variable) {
			continue
		}
		v := it.variable
		ncoeff := new(big.Rat).Mul(lcmDen, &it.coefficient)
		absNcoeff := new(big.Rat).Abs(ncoeff)
		if absNcoeff.Cmp(leastCoeff) == 0 {
			if ncoeff.Sign() > 0 {
				l.Add(l, new(big.Rat).Mul(ncoeff, &t.lowerBound(v).rational))
				u.Add(u, new(big.Rat).Mul(ncoeff, &t.upperBound(v).rational))
			} else {
				l.Add(l, new(big.Rat).Mul(ncoeff, &t.upperBound(v).rational))
				u.Add(u, new(big.Rat).Mul(ncoeff, &t.lowerBound(v).rational))
			}
			if err := t.lower(v).pushJustification(ante, &it.coefficient, coeffs); err != nil {
				return false, err
			}
			if err := t.upper(v).pushJustification(ante, &it.coefficient, coeffs); err != nil {
				return false, err
			}
		} else if gcds.Sign() == 0 {
			gcds.Set(absNcoeff)
		} else {
			gcds.SetInt(new(big.Int).GCD(nil, nil, gcds.Num(), absNcoeff.Num()))
		}
	}
	if gcds.Sign() == 0 {
		return true, nil
	}
	lq := new(big.Rat).Quo(l, gcds)
	uq := new(big.Rat).Quo(u, gcds)
	l1 := new(big.Rat).SetInt(smtRatFloor(lq))
	if !lq.IsInt() {
		l1.Add(l1, big.NewRat(1, 1))
	}
	u1 := new(big.Rat).SetInt(smtRatFloor(uq))
	if u1.Cmp(l1) < 0 {
		if err := t.collectFixedVarJustifications(r, ante); err != nil {
			return false, err
		}
		return false, t.gcdConflict(ante)
	}
	return true, nil
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
func (t *SMTTheoryArith) patchIntInfeasibleVars() error {
	num := SMTTheoryVar(t.getNumVars())
	var infL, infU bool
	var l, u smtArithmeticValue
	var m big.Rat
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isNonBase(v) {
			continue
		}
		t.getFreedomInterval(v, &infL, &l, &infU, &u, &m)
		if smtRatIsOne(&m) && t.getValue(v).isInt() {
			continue
		}
		// check whether the value of v is already a multiple of m.
		if smtRatQuo(&t.scratchRat, &t.getValue(v).rational, &m).IsInt() {
			continue
		}
		if !infL {
			l.setRational(smtInfCeil(&l))
		}
		if !infU {
			u.setRational(smtInfFloor(&u))
		}
		if !smtRatIsOne(&m) {
			if !infL {
				var q smtArithmeticValue
				q.set(&l)
				q.quoRat(&m)
				c := smtInfCeil(&q)
				l.setRational(c.Mul(c, &m))
			}
			if !infU {
				var q smtArithmeticValue
				q.set(&u)
				q.quoRat(&m)
				f := smtInfFloor(&q)
				u.setRational(f.Mul(f, &m))
			}
		}
		if !infL && !infU && l.cmp(&u) > 0 {
			continue // cannot patch
		}
		var err error
		switch {
		case !infL:
			err = t.setValue(v, &l)
		case !infU:
			err = t.setValue(v, &u)
		default:
			var zero smtArithmeticValue
			err = t.setValue(v, &zero)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// fixNonBaseVars forces all non-base integer variables to integer values.
func (t *SMTTheoryArith) fixNonBaseVars() error {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isNonBase(v) || !t.isInt(v) || t.getValue(v).isInt() {
			continue
		}
		var newVal smtArithmeticValue
		newVal.setRational(smtInfFloor(t.getValue(v)))
		if err := t.setValue(v, &newVal); err != nil {
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

// getMonomial splits mon = a * pp.
func (b *smtArithEuclideanSolverBridge) getMonomial(mon ast.ASTHandle) (*big.Rat, ast.ASTHandle) {
	t := b.t
	if t.isOp(mon, smtArithOpMul) {
		if args, err := t.args(mon); err == nil && len(args) == 2 {
			if a, ok := t.isNumeral(args[0]); ok {
				return a, args[1]
			}
		}
	}
	return big.NewRat(1, 1), mon
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

func (b *smtArithEuclideanSolverBridge) setInt(out *numeral.Int, value *big.Int) error {
	return b.integers.SetString(out, value.String())
}

func (b *smtArithEuclideanSolverBridge) pushA(value *big.Int) (result error) {
	tmp := numeral.NewInt(0)
	if err := b.setInt(&tmp, value); err != nil {
		return errors.Join(err, b.integers.Del(&tmp))
	}
	return errors.Join(b.as.Push(&tmp), b.integers.Del(&tmp))
}

func (b *smtArithEuclideanSolverBridge) bigInt(value *numeral.Int) (*big.Int, error) {
	text, err := b.integers.String(value)
	if err != nil {
		return nil, err
	}
	r, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return r, nil
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
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isFixed(v) || !t.isInt(v) {
			continue // only fixed integer variables
		}
		n := t.var2expr(v)
		if _, ok := t.isNumeral(n); ok {
			continue // skip stupid equality c - c = 0
		}
		num := new(big.Rat).Neg(&t.getValue(v).rational)
		c := numeral.NewInt(0)
		if err := b.setInt(&c, num.Num()); err != nil {
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
			aVal, pp := b.getMonomial(arg)
			if !aVal.IsInt() {
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
			if err = b.pushA(aVal.Num()); err != nil {
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

func (b *smtArithEuclideanSolverBridge) mkBound(v SMTTheoryVar, k *big.Rat, lower bool, oldBound smtArithBound, js []uint32) error {
	t := b.t
	kind := smtArithmeticUpperBound
	if lower {
		kind = smtArithmeticLowerBound
	}
	var val smtArithmeticValue
	val.setRational(k)
	newBound := &smtArithmeticDerivedBound{}
	newBound.variable, newBound.kind = v, kind
	newBound.value.set(&val)
	clear(t.tmpLitSet)
	clear(t.tmpEqSet)
	zero := new(big.Rat) // refine for proof gen
	if oldBound != nil {
		if err := t.accumulateJustification(oldBound, newBound, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	for _, j := range js {
		fixedV := b.j2v[j]
		if err := t.accumulateJustification(t.lower(fixedV), newBound, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
		if err := t.accumulateJustification(t.upper(fixedV), newBound, zero, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	t.boundsToDelete = append(t.boundsToDelete, newBound)
	t.assertedBounds = append(t.assertedBounds, newBound)
	return nil
}

func (b *smtArithEuclideanSolverBridge) tightBoundsFor(v SMTTheoryVar) (propagated bool, result error) {
	t := b.t
	n := t.var2expr(v)
	g := new(big.Int) // gcd of the coefficients of the variables that are not in the solver
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
		aVal, pp := b.getMonomial(arg)
		if !aVal.IsInt() {
			return false, nil
		}
		x := b.getVar(pp)
		if x == smtArithUintMax {
			absA := new(big.Int).Abs(aVal.Num())
			if initG {
				g.GCD(nil, nil, g, absA)
			} else {
				g.Set(absA)
			}
			initG = true
			if g.Cmp(big.NewInt(1)) == 0 {
				return false, nil // gcd of the coeffs is one.
			}
		} else {
			if err = b.pushA(aVal.Num()); err != nil {
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
	aBig, err := b.bigInt(&a)
	if err != nil {
		return false, err
	}
	if initG {
		if aBig.Sign() != 0 {
			g.GCD(nil, nil, g, new(big.Int).Abs(aBig))
		}
	} else {
		g.Set(aBig)
	}
	if g.Cmp(big.NewInt(1)) == 0 {
		return false, nil
	}
	cBig, err := b.bigInt(&c)
	if err != nil {
		return false, err
	}
	c2 := new(big.Rat).SetInt(cBig)
	if g.Sign() == 0 {
		// The definition of v is 0 + c: v is fixed at c, justified by m_js
		// alone.
		if err = b.mkBound(v, c2, true, nil, b.js); err != nil {
			return false, err
		}
		return true, b.mkBound(v, c2, false, nil, b.js)
	}
	// l' := g*ceil((l - c)/g) + c and u' := g*floor((u - c)/g) + c
	gr := new(big.Rat).SetInt(g)
	if l := t.lower(v); l != nil {
		lOld := &l.base().value.rational
		q := new(big.Rat).Sub(lOld, c2)
		q.Quo(q, gr)
		ceil := new(big.Rat).SetInt(smtRatFloor(q))
		if !q.IsInt() {
			ceil.Add(ceil, big.NewRat(1, 1))
		}
		lNew := ceil.Mul(ceil, gr)
		lNew.Add(lNew, c2)
		if lNew.Cmp(lOld) > 0 {
			propagated = true
			if err = b.mkBound(v, lNew, true, l, b.js); err != nil {
				return false, err
			}
		}
	}
	if u := t.upper(v); u != nil {
		uOld := &u.base().value.rational
		q := new(big.Rat).Sub(uOld, c2)
		q.Quo(q, gr)
		uNew := new(big.Rat).SetInt(smtRatFloor(q))
		uNew.Mul(uNew, gr)
		uNew.Add(uNew, c2)
		if uNew.Cmp(uOld) < 0 {
			propagated = true
			if err = b.mkBound(v, uNew, false, u, b.js); err != nil {
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
