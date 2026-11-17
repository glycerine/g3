package smtcore

// Source: src/smt/theory_arith_aux.h, ported in source order. row, column,
// antecedents_t, bound, atom, eq_bound, derived_bound and
// justified_derived_bound (aux.h:30-438, 730-805) are theory_arith_tableau.go
// and theory_arith_bounds.go.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ---- aux.h:440-723 (auxiliary methods) ----

func (t *SMTTheoryArith) atBound(v SMTTheoryVar) bool {
	if l := t.lower(v); l != nil && t.vcmp(t.getValue(v), &l.base().value) == 0 {
		return true
	}
	u := t.upper(v)
	return u != nil && t.vcmp(t.getValue(v), &u.base().value) == 0
}

func (t *SMTTheoryArith) isFixed(v SMTTheoryVar) bool {
	l := t.lower(v)
	if l == nil {
		return false
	}
	u := t.upper(v)
	if u == nil {
		return false
	}
	return t.vcmp(&l.base().value, &u.base().value) == 0
}

func (t *SMTTheoryArith) setBound(newBound smtArithBound, upper bool) error {
	if upper != (newBound.base().kind == smtArithmeticUpperBound) {
		return ast.ErrASTDomain
	}
	v := newBound.base().variable
	t.setBoundCore(v, newBound, upper)
	if (t.propagateEqs() || t.propagateDiseqs()) && t.isFixed(v) {
		return t.fixedVarEh(v)
	}
	return nil
}

// getABaseRowThatContains returns the column entry of a base row that
// contains v, or nil when no row contains v.
func (t *SMTTheoryArith) getABaseRowThatContains(v SMTTheoryVar) (*smtArithmeticColumnEntry, error) {
	for {
		c := &t.columns[v]
		if c.size == 0 {
			return nil, nil
		}
		quasiBaseRID := -1
		for i := range c.entries {
			it := &c.entries[i]
			if it.dead() {
				continue
			}
			rid := it.rowID
			if t.isBase(t.rows[rid].base) {
				return it, nil
			} else if quasiBaseRID == -1 {
				quasiBaseRID = rid
			}
		}
		if quasiBaseRID == -1 {
			return nil, ast.ErrASTDomain // since c.size() != 0
		}
		if err := t.quasiBaseRow2BaseRow(quasiBaseRID); err != nil {
			return nil, err
		}
		// This loop terminates: it creates a base row that contains v, or
		// decreases c.size().
	}
}

// allCoeffInt reports whether all coefficients of the row are integers.
func (t *SMTTheoryArith) allCoeffInt(r *smtArithmeticRow) bool {
	for i := range r.entries {
		if !r.entries[i].dead() && !r.entries[i].coefficient.IsInt() {
			return false
		}
	}
	return true
}

// getRowForEliminating returns the column entry of a row containing v that
// is not owned by an unconstrained quasi-base variable, or nil. Used by
// move_unconstrained_to_base.
func (t *SMTTheoryArith) getRowForEliminating(v SMTTheoryVar) *smtArithmeticColumnEntry {
	c := &t.columns[v]
	if c.size == 0 {
		return nil
	}
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() {
			continue
		}
		r := &t.rows[it.rowID]
		s := r.base
		if t.isQuasiBase(s) && len(t.varOccs[s]) == 0 {
			continue
		}
		if t.isInt(v) {
			coeff := &r.entries[it.rowIndex].coefficient
			// With c == 1 or c == -1 and all other coefficients integer,
			// pivoting v with the base var guarantees an integer v when the
			// non-base variables are integer.
			if !coeff.IsOne() && !coeff.IsMinusOne() {
				continue
			}
			if !t.allCoeffInt(r) {
				continue
			}
		}
		return it
	}
	return nil
}

func (t *SMTTheoryArith) moveUnconstrainedToBase() error {
	if t.lazyPivotingLvl() == 0 {
		return nil
	}
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if len(t.varOccs[v]) != 0 || !t.isFree(v) {
			continue
		}
		switch t.getVarKind(v) {
		case smtArithmeticQuasiBase:
		case smtArithmeticBase:
			if t.isInt(v) && !t.allCoeffInt(&t.rows[t.getVarRow(v)]) {
				// v may receive a non-integer value even if all non-base
				// variables are integer, so it is not eliminated.
				break
			}
			if err := t.eliminate(false, v, t.eagerGCD); err != nil {
				return err
			}
		case smtArithmeticNonBase:
			if entry := t.getRowForEliminating(v); entry != nil {
				r := &t.rows[entry.rowID]
				if err := t.pivotCopy(false, r.base, v, &r.entries[entry.rowIndex].coefficient, t.eagerGCD); err != nil {
					return err
				}
				t.setVarKind(v, smtArithmeticQuasiBase)
			}
		}
	}
	return nil
}

// elimQuasiBaseRows forces all quasi-base rows to become base rows.
func (t *SMTTheoryArith) elimQuasiBaseRows() error {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if t.isQuasiBase(v) {
			if err := t.quasiBaseRow2BaseRow(t.getVarRow(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeFixedVarsFromBase pivots fixed base variables out of the base.
func (t *SMTTheoryArith) removeFixedVarsFromBase() error {
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		if !t.isBase(v) || !t.isFixed(v) {
			continue
		}
		r := &t.rows[t.getVarRow(v)]
		for i := range r.entries {
			it := &r.entries[i]
			if !it.dead() && it.variable != v && !t.isFixed(it.variable) {
				if err := t.pivotCopy(true, v, it.variable, &it.coefficient, false); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// tryToMinimizeRationalCoeffs pivots x_i and x_j for rows
// x_i + 1/n * x_j + ... = 0 where x_j is a non-fixed non-base variable and
// the denominators of the other non-fixed coefficients divide n, preferring
// free variables, then those with the widest bounds.
func (t *SMTTheoryArith) tryToMinimizeRationalCoeffs() (err error) {
	q := t.q
	maxDen, num, den, aij := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	a, b := newSMTArithmeticValue(), newSMTArithmeticValue()
	defer func() {
		for _, r := range []*numeral.Rational{&maxDen, &num, &den, &aij} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
		for _, v := range []*smtArithmeticValue{&a, &b} {
			if e := v.del(q); e != nil && err == nil {
				err = e
			}
		}
	}()
	// numUnit reports numerator(c) is 1 or -1, leaving denominator(c) in den.
	numUnit := func(c *numeral.Rational) (bool, error) {
		if err := q.NumeratorRational(c, &num); err != nil {
			return false, err
		}
		if !num.IsOne() && !num.IsMinusOne() {
			return false, nil
		}
		return true, q.DenominatorRational(c, &den)
	}
	nv := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < nv; v++ {
		if !t.isBase(v) || !t.isInt(v) {
			continue
		}
		if err := q.Reset(&maxDen); err != nil {
			return err
		}
		r := &t.rows[t.getVarRow(v)]
		for i := range r.entries {
			it := &r.entries[i]
			if it.dead() || it.variable == v || t.isFixed(it.variable) {
				continue
			}
			if ok, err := numUnit(&it.coefficient); err != nil {
				return err
			} else if !ok {
				continue
			}
			if t.rcmp(&den, &maxDen) > 0 {
				if err := q.Copy(&maxDen, &den); err != nil {
					return err
				}
			}
		}
		one := smtQInt(1)
		if t.rcmp(&maxDen, &one) <= 0 {
			continue
		}
		// check whether all a_ik denominators divide max_den
		divides := true
		for i := range r.entries {
			it := &r.entries[i]
			if it.dead() || t.isFixed(it.variable) {
				continue
			}
			if err := q.DenominatorRational(&it.coefficient, &den); err != nil {
				return err
			}
			ok, err := q.Divides(&den, &maxDen)
			if err != nil {
				return err
			}
			if !ok {
				divides = false
				break
			}
		}
		if !divides {
			continue
		}
		// pick best candidate
		xj := SMTNullTheoryVar
		for i := range r.entries {
			it := &r.entries[i]
			if it.dead() || it.variable == v || t.isFixed(it.variable) {
				continue
			}
			if ok, err := numUnit(&it.coefficient); err != nil {
				return err
			} else if !ok {
				continue
			}
			if t.rcmp(&den, &maxDen) != 0 {
				continue
			}
			wider := func() bool {
				if !t.isBounded(xj) || !t.isBounded(it.variable) {
					return false
				}
				err := t.vsub(&a, t.upperBound(xj), t.lowerBound(xj))
				if err == nil {
					err = t.vsub(&b, t.upperBound(it.variable), t.lowerBound(it.variable))
				}
				if err != nil && t.qErr == nil {
					t.qErr = err
				}
				return t.vcmp(&a, &b) > 0
			}
			if xj == SMTNullTheoryVar || t.isFree(it.variable) ||
				(t.isBounded(xj) && !t.isBounded(it.variable)) || wider() {
				xj = it.variable
				if err := q.Copy(&aij, &it.coefficient); err != nil {
					return err
				}
				if t.isFree(xj) {
					break
				}
			}
		}
		if xj != SMTNullTheoryVar {
			if err := t.pivot(true, v, xj, &aij, false); err != nil {
				return err
			}
		}
	}
	return t.takeQErr(nil)
}

// ---- aux.h:800-905 (derived bounds) ----

// accumulateJustification copies the justification of b to newBound. Only
// literals and equalities not in lits and eqs are copied; the
// justification of b is also added to lits and eqs.
func (t *SMTTheoryArith) accumulateJustification(b smtArithBound, newBound smtArithDerivedBound, coeff *numeral.Rational, lits map[int32]bool, eqs map[SMTEnodePair]bool) (err error) {
	ante := t.acquireAntecedents()
	defer t.releaseAntecedentsScope()
	proofs := t.proofsEnabled()
	if err = b.pushJustification(t.q, ante, coeff, proofs); err != nil {
		return err
	}
	zero := smtQZero()
	for i, l := range ante.literals {
		if lits[l.Index()] {
			continue
		}
		if proofs {
			err = newBound.pushLiteral(t.q, l, &ante.literalCoefficients[i])
		} else {
			err = newBound.pushLiteral(t.q, l, &zero)
			lits[l.Index()] = true
		}
		if err != nil {
			return err
		}
	}
	for i, p := range ante.equalities {
		if eqs[p] {
			continue
		}
		if proofs {
			err = newBound.pushEquality(t.q, p, &ante.equalityCoefficients[i])
		} else {
			err = newBound.pushEquality(t.q, p, &zero)
			eqs[p] = true
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// normalizeBound sets out to normalize_bound(v, k, kind).
func (t *SMTTheoryArith) normalizeBound(v SMTTheoryVar, k *smtArithmeticValue, kind smtArithmeticBoundKind, out *smtArithmeticValue) error {
	if t.isReal(v) {
		return out.set(t.q, k)
	}
	// inf_numeral(ceil(k)) for a lower bound, inf_numeral(floor(k)) for an upper.
	return smtNormalizeArithmeticIntegerBound(t.q, k, kind, out)
}

// newDerivedBound allocates justified_derived_bound when proofs are
// enabled, otherwise derived_bound.
func (t *SMTTheoryArith) newDerivedBound(v SMTTheoryVar, value *smtArithmeticValue, kind smtArithmeticBoundKind) (smtArithDerivedBound, error) {
	if t.proofsEnabled() {
		b := &smtArithmeticJustifiedDerivedBound{}
		b.variable, b.kind = v, kind
		b.value.init()
		return b, b.value.set(t.q, value)
	}
	b := &smtArithmeticDerivedBound{}
	b.variable, b.kind = v, kind
	b.value.init()
	return b, b.value.set(t.q, value)
}

// mkBoundFromRow creates a derived bound for v using the row as explanation.
func (t *SMTTheoryArith) mkBoundFromRow(v SMTTheoryVar, k *smtArithmeticValue, kind smtArithmeticBoundKind, r *smtArithmeticRow) error {
	kNorm := newSMTArithmeticValue()
	defer kNorm.del(t.q)
	if err := t.normalizeBound(v, k, kind, &kNorm); err != nil {
		return err
	}
	newBound, err := t.newDerivedBound(v, &kNorm, kind)
	if err != nil {
		return err
	}
	t.boundsToDelete = append(t.boundsToDelete, newBound)
	t.assertedBounds = append(t.assertedBounds, newBound)
	clear(t.tmpLitSet)
	clear(t.tmpEqSet)
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		useUpper := kind == smtArithmeticUpperBound
		if it.coefficient.Sign() <= 0 {
			useUpper = !useUpper
		}
		b := t.getBound(it.variable, useUpper)
		if b == nil {
			return ast.ErrASTDomain
		}
		if err = t.accumulateJustification(b, newBound, &it.coefficient, t.tmpLitSet, t.tmpEqSet); err != nil {
			return err
		}
	}
	return nil
}

// ---- aux.h:900-1300 (maximization/minimization support) ----

// addTmpRow sets r1 <- r1 + coeff * r2, where r1 is a temporary row.
// Columns are not updated for temporary rows.
func (t *SMTTheoryArith) addTmpRow(r1 *smtArithmeticRow, coeff *numeral.Rational, r2 *smtArithmeticRow) error {
	if err := r1.saveVariablePositions(t.varPos); err != nil {
		return err
	}
	q := t.q
	isOne, isMinusOne := coeff.IsOne(), coeff.IsMinusOne()
	tmp := smtQZero()
	defer smtQDel(q, &tmp)
	for i := range r2.entries {
		it := &r2.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		pos := t.varPos[v]
		if pos == -1 {
			// variable v is not in row1
			_, rEntry := r1.addEntry()
			rEntry.variable = v
			if err := q.Copy(&rEntry.coefficient, &it.coefficient); err != nil {
				return err
			}
			var err error
			switch {
			case isOne:
			case isMinusOne:
				err = q.Neg(&rEntry.coefficient)
			default:
				err = q.Mul(&rEntry.coefficient, coeff, &rEntry.coefficient)
			}
			if err != nil {
				return err
			}
		} else {
			// variable v is in row1
			rEntry := &r1.entries[pos]
			var err error
			switch {
			case isOne:
				err = q.Add(&rEntry.coefficient, &it.coefficient, &rEntry.coefficient)
			case isMinusOne:
				err = q.Sub(&rEntry.coefficient, &it.coefficient, &rEntry.coefficient)
			default:
				if err = q.Mul(&it.coefficient, coeff, &tmp); err == nil {
					err = q.Add(&rEntry.coefficient, &tmp, &rEntry.coefficient)
				}
			}
			if err != nil {
				return err
			}
			if rEntry.coefficient.IsZero() {
				if err := r1.deleteEntry(q, pos); err != nil {
					return err
				}
			}
			t.varPos[v] = -1
		}
	}
	return r1.resetVariablePositions(t.varPos)
}

func (t *SMTTheoryArith) isSafeToLeave(x SMTTheoryVar, inc bool, hasInt, shared *bool) (bool, error) {
	n, err := t.Enode(x)
	if err != nil {
		return false, err
	}
	s0, err := t.ctxIsShared(n)
	if err != nil {
		return false, err
	}
	*shared = *shared || s0
	c := &t.columns[x]
	*hasInt = false
	unbounded := (inc && t.upper(x) == nil) || (!inc && t.lower(x) == nil)
	wasUnsafe := false
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() {
			continue
		}
		r := &t.rows[it.rowID]
		s := r.base
		coeff := &r.entries[it.rowIndex].coefficient
		if s != SMTNullTheoryVar && t.isInt(s) {
			*hasInt = true
		}
		isUnsafe := s != SMTNullTheoryVar && t.isInt(s) && !coeff.IsInt()
		if s != SMTNullTheoryVar {
			ns, err := t.Enode(s)
			if err != nil {
				return false, err
			}
			ss, err := t.ctxIsShared(ns)
			if err != nil {
				return false, err
			}
			*shared = *shared || ss
		}
		wasUnsafe = wasUnsafe || isUnsafe
		incS := !inc
		if coeff.Sign() < 0 {
			incS = inc
		}
		unbounded = unbounded && t.getBound(s, incS) == nil
		if wasUnsafe && !unbounded {
			return false, nil
		}
	}
	return !wasUnsafe || unbounded, nil
}

func (t *SMTTheoryArith) getTheoryVars(n ast.ASTHandle, vars map[SMTTheoryVar]bool) (bool, error) {
	if _, ok := t.isNumeral(n); ok {
		return true, nil
	}
	op, own := t.arithOp(n)
	if own && op == smtArithOpAdd {
		args, err := t.args(n)
		if err != nil {
			return false, err
		}
		for _, arg := range args {
			if ok, err := t.getTheoryVars(arg, vars); err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	}
	if own && (op == smtArithOpToReal || op == smtArithOpToInt) {
		x, err := t.state.manager.GetArg(n, 0)
		if err != nil {
			return false, err
		}
		return t.getTheoryVars(x, vars)
	}
	if own && op == smtArithOpMul {
		args, err := t.args(n)
		if err != nil {
			return false, err
		}
		if len(args) == 2 {
			if _, ok := t.isNumeral(args[0]); ok {
				return t.getTheoryVars(args[1], vars)
			}
			if _, ok := t.isNumeral(args[1]); ok {
				return t.getTheoryVars(args[0], vars)
			}
		}
	}
	if !t.isApp(n) {
		return false, nil
	}
	if own {
		return false, nil
	}
	e := t.state.context.enodes.get(n)
	if e == nil {
		return false, ast.ErrASTDomain
	}
	if t.IsAttachedToVariable(e) {
		vars[e.TheoryVar(t.FamilyID())] = true
	}
	return true, nil
}

// conflictMinimize is conflict_minimize: the least upper bound recorded on
// the watched objective since enable_record_conflict.
func (t *SMTTheoryArith) conflictMinimize(out *smtArithmeticValue) error {
	return out.set(t.q, &t.upperBoundRecorded)
}

// enableRecordConflict watches the bound atom of an objective.
func (t *SMTTheoryArith) enableRecordConflict(boundExpr ast.ASTHandle) {
	t.params.BoundProp = smtArithBPNone // bound propagation rules are not (yet) handled
	if !boundExpr.IsNull() {
		bv, _ := t.state.BoolVar(boundExpr)
		t.boundWatch = bv
	} else {
		t.boundWatch = SMTNullBoolVar
	}
	if err := t.upperBoundRecorded.reset(t.q); err != nil && t.qErr == nil {
		t.qErr = err
	}
	if err := t.q.SetInt64(&t.upperBoundRecorded.infinity, -1); err != nil && t.qErr == nil { // -infinity
		t.qErr = err
	}
}

// recordConflict derives, from the Farkas combination of a conflict that
// involves the watched bound atom, an upper bound on its objective:
// val/q <= v.
// The farkas coefficients are read from bounds, the antecedents that the
// params of the source are built from (params[0] is the rule name).
func (t *SMTTheoryArith) recordConflict(lits []SMTLiteral, eqs []SMTEnodePair, bounds *smtArithmeticAntecedents) (result error) {
	if t.boundWatch == SMTNullBoolVar {
		return nil
	}
	idx := len(lits)
	for i, l := range lits {
		if l.Var() == t.boundWatch {
			idx = i
			break
		}
	}
	if idx == len(lits) {
		return nil
	}
	s := t.state
	m := s.manager
	if s.smtParams == nil || len(bounds.literalCoefficients) != len(lits) || len(bounds.equalityCoefficients) != len(eqs) {
		return ast.ErrASTDomain
	}
	farkas, err := NewSMTFarkasUtil(m, s.smtParams.Context, s.smtParams.Globals)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, farkas.Close()) }()
	x := smtArithExprs{m: m}
	defer func() { result = errors.Join(result, x.release()) }()
	var q *big.Rat
	for i, l := range lits {
		pa, err := smtQBig(t.q, &bounds.literalCoefficients[i])
		if err != nil {
			return err
		}
		pa.Abs(pa)
		if idx == i {
			q = pa
			continue
		}
		tmp, err := s.literalExpression(l)
		if err != nil {
			return err
		}
		if _, err = x.hold(tmp, nil); err != nil {
			return err
		}
		if err = farkas.Add(pa, tmp); err != nil {
			return err
		}
	}
	for i, p := range eqs {
		tmp, err := x.hold(m.MkEq(p.First.owner, p.Second.owner))
		if err != nil {
			return err
		}
		pa, err := smtQBig(t.q, &bounds.equalityCoefficients[i])
		if err != nil {
			return err
		}
		pa.Abs(pa)
		if err = farkas.Add(pa, tmp); err != nil {
			return err
		}
	}
	tmpRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	x.refs = append(x.refs, tmpRef)
	if err = farkas.Get(tmpRef); err != nil {
		return err
	}
	tmp, err := m.RefGet(tmpRef)
	if err != nil {
		return err
	}
	a := t.getBv2a(t.boundWatch)
	if a == nil {
		return ast.ErrASTDomain
	}
	var lhs, rhs ast.ASTHandle
	strict := false
	if t.isOp(tmp, smtArithOpLE) || t.isOp(tmp, smtArithOpGE) || t.isEqualityApp(tmp) {
		args, err := t.args(tmp)
		if err != nil {
			return err
		}
		lhs, rhs = args[0], args[1]
		if t.isOp(tmp, smtArithOpGE) {
			lhs, rhs = rhs, lhs // ge(y, x) is x <= y
		}
	} else if negated, e, err := m.IsNot(tmp); err == nil && negated && (t.isOp(e, smtArithOpLE) || t.isOp(e, smtArithOpGE)) {
		args, err := t.args(e)
		if err != nil {
			return err
		}
		// not (y <= x) or not (x >= y)
		lhs, rhs = args[1], args[0]
		if t.isOp(e, smtArithOpGE) {
			lhs, rhs = args[0], args[1]
		}
		strict = true
	} else {
		return ast.ErrASTDomain // UNREACHABLE
	}
	e := t.var2expr(a.variable)
	factor, err := farkas.NormalizeFactor()
	if err != nil {
		return err
	}
	q.Mul(q, factor)
	var vq ast.ASTHandle
	if smtRatIsOne(q) {
		vq = e
	} else {
		qn, err := t.mkNumeral(&x, q, q.IsInt())
		if err != nil {
			return err
		}
		if vq, err = t.mkApp(&x, smtArithOpMul, qn, e); err != nil {
			return err
		}
	}
	diff, err := t.mkApp(&x, smtArithOpSub, lhs, rhs)
	if err != nil {
		return err
	}
	if vq, err = t.mkApp(&x, smtArithOpAdd, diff, vq); err != nil {
		return err
	}
	if !smtRatIsOne(q) {
		qn, err := t.mkNumeral(&x, q, q.IsInt())
		if err != nil {
			return err
		}
		if vq, err = t.mkApp(&x, smtArithOpDiv, vq, qn); err != nil {
			return err
		}
	}
	rw, err := s.theoryRewriter.FreshDefault() // th_rewriter rw(m)
	if rw != nil {
		defer func() { result = errors.Join(result, rw.Close()) }()
	}
	if err != nil {
		return err
	}
	outRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	x.refs = append(x.refs, outRef)
	if err = rw.Rewrite(vq, outRef); err != nil {
		return err
	}
	out, err := m.RefGet(outRef)
	if err != nil {
		return err
	}
	qv := smtQZero()
	defer smtQDel(t.q, &qv)
	if ok, err := t.isNumeralQ(out, &qv); err != nil {
		return err
	} else if !ok {
		return ast.ErrASTDomain // VERIFY(m_util.is_numeral(tmp, q))
	}
	cand := newSMTArithmeticValue()
	defer cand.del(t.q)
	if err := cand.setRational(t.q, &qv); err != nil {
		return err
	}
	if t.vcmp(&t.upperBoundRecorded, &cand) < 0 {
		if err := t.upperBoundRecorded.set(t.q, &cand); err != nil {
			return err
		}
		if strict {
			if err := t.upperBoundRecorded.sub(t.q, t.getEpsilon(a.variable)); err != nil {
				return err
			}
		}
		// IF_VERBOSE(1, "new upper bound: " << m_upper_bound) (AUDIT.md
		// FUZZ-F5; g3trace builds only).
		if diagnostic.VerboseProgress {
			if err := progressVerbose(1, "new upper bound: "+smtInfEpsText(t.q, &t.upperBoundRecorded)+"\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTTheoryArith) isEqualityApp(e ast.ASTHandle) bool {
	ok, _, _, err := t.isEqApp(e)
	return err == nil && ok
}

// ---- aux.h:1300-1855 ----

// pickVarToLeave selects the tightest base variable x_i to pivot with x_j so
// that x_j moves in direction inc and the tableau stays feasible.
func (t *SMTTheoryArith) pickVarToLeave(xj SMTTheoryVar, inc bool, aij *numeral.Rational, minGain, maxGain *smtArithmeticValue, hasShared *bool) (bool, SMTTheoryVar, error) {
	xi := SMTNullTheoryVar
	t.initGains(xj, inc, minGain, maxGain)
	nj, err := t.Enode(xj)
	if err != nil {
		return false, xi, err
	}
	sj, err := t.ctxIsShared(nj)
	if err != nil {
		return false, xi, err
	}
	*hasShared = *hasShared || sj
	if t.isInt(xj) && !t.getValue(xj).isInt() {
		return false, xi, nil
	}
	c := &t.columns[xj]
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() {
			continue
		}
		r := &t.rows[it.rowID]
		s := r.base
		coeffIJ := &r.entries[it.rowIndex].coefficient
		if t.updateGains(inc, s, coeffIJ, minGain, maxGain) || (xi == SMTNullTheoryVar && !t.unboundedGain(maxGain)) {
			xi = s
			if err := t.q.Copy(aij, coeffIJ); err != nil {
				return false, xi, err
			}
		}
		ns, err := t.Enode(s)
		if err != nil {
			return false, xi, err
		}
		ss, err := t.ctxIsShared(ns)
		if err != nil {
			return false, xi, err
		}
		*hasShared = *hasShared || ss
	}
	return t.safeGain(minGain, maxGain), xi, t.takeQErr(nil)
}

func (t *SMTTheoryArith) unboundedGain(maxGain *smtArithmeticValue) bool { return maxGain.isMinusOne() }

// safeGain: the selected variable is unbounded (max_gain == -1), or
// min_gain <= max_gain.
func (t *SMTTheoryArith) safeGain(minGain, maxGain *smtArithmeticValue) bool {
	return t.unboundedGain(maxGain) || t.vcmp(minGain, maxGain) <= 0
}

// normalizeGain makes max_gain divisible by divisor:
// max_gain = floor(max_gain/divisor)*divisor. Failures are kept in qErr.
func (t *SMTTheoryArith) normalizeGain(divisor *numeral.Rational, maxGain *smtArithmeticValue) {
	if divisor.IsMinusOne() || maxGain.isMinusOne() {
		return
	}
	q := t.q
	v := newSMTArithmeticValue()
	f := smtQZero()
	err := v.set(q, maxGain)
	if err == nil {
		err = v.quoRat(q, divisor)
	}
	if err == nil {
		err = smtInfFloor(q, &v, &f)
	}
	if err == nil {
		err = q.Mul(&f, divisor, &f)
	}
	if err == nil {
		err = maxGain.setRational(q, &f)
	}
	t.keep(err)
	t.keep(v.del(q))
	t.keep(smtQDel(q, &f))
}

// initGains initializes the gains of x from its bounds: min_gain is -1 for
// rationals, max_gain is -1 when unbounded.
func (t *SMTTheoryArith) initGains(x SMTTheoryVar, inc bool, minGain, maxGain *smtArithmeticValue) {
	q := t.q
	t.keep(minGain.setInt(q, -1))
	t.keep(maxGain.setInt(q, -1))
	if inc && t.upper(x) != nil {
		t.keep(t.vsub(maxGain, t.upperBound(x), t.getValue(x)))
	} else if !inc && t.lower(x) != nil {
		t.keep(t.vsub(maxGain, t.getValue(x), t.lowerBound(x)))
	}
	if t.isInt(x) {
		t.keep(minGain.setInt(q, 1))
	}
}

func (t *SMTTheoryArith) updateGains(inc bool, xi SMTTheoryVar, aij *numeral.Rational, minGain, maxGain *smtArithmeticValue) bool {
	// x_i = row + a_ij*x_j
	if !t.safeGain(minGain, maxGain) {
		return false
	}
	q := t.q
	maxInc := newSMTArithmeticValue()
	denAij, r := smtQInt(1), smtQZero()
	defer func() {
		t.keep(maxInc.del(q))
		t.keep(smtQDel(q, &denAij))
		t.keep(smtQDel(q, &r))
	}()
	t.keep(maxInc.setInt(q, -1))
	decrementXi := (inc && aij.Sign() > 0) || (!inc && aij.Sign() < 0)
	if decrementXi && t.lower(xi) != nil {
		t.keep(t.vsub(&maxInc, t.getValue(xi), t.lowerBound(xi)))
		t.keep(maxInc.quoRat(q, aij))
		t.keep(maxInc.abs(q))
	} else if !decrementXi && t.upper(xi) != nil {
		t.keep(t.vsub(&maxInc, t.upperBound(xi), t.getValue(xi)))
		t.keep(maxInc.quoRat(q, aij))
		t.keep(maxInc.abs(q))
	}
	isTighter := false
	if t.isInt(xi) {
		t.keep(q.DenominatorRational(aij, &denAij))
	}
	if t.isInt(xi) && !denAij.IsOne() {
		if minGain.sign() < 0 {
			t.keep(minGain.setRational(q, &denAij))
		} else {
			t.keep(q.LCM(&minGain.rational, &denAij, &r))
			t.keep(minGain.setRational(q, &r))
		}
		t.normalizeGain(&minGain.rational, maxGain)
	}
	if t.isInt(xi) && !maxGain.isInt() {
		t.keep(smtInfFloor(q, maxGain, &r))
		t.keep(maxGain.setRational(q, &r))
		t.normalizeGain(&minGain.rational, maxGain)
	}
	if !maxInc.isMinusOne() {
		if t.isInt(xi) {
			t.keep(smtInfFloor(q, &maxInc, &r))
			t.keep(maxInc.setRational(q, &r))
			t.normalizeGain(&minGain.rational, &maxInc)
		}
		if t.unboundedGain(maxGain) {
			t.keep(maxGain.set(q, &maxInc))
			isTighter = true
		} else if t.vcmp(maxGain, &maxInc) > 0 {
			t.keep(maxGain.set(q, &maxInc))
			isTighter = true
		}
	}
	return isTighter
}

// hasInterfaceEquality reports whether a bound change of x affects an
// interface equality.
func (t *SMTTheoryArith) hasInterfaceEquality(x SMTTheoryVar) (bool, error) {
	num := SMTTheoryVar(t.getNumVars())
	nx, err := t.Enode(x)
	if err != nil {
		return false, err
	}
	r := nx.Root()
	for v := SMTTheoryVar(0); v < num; v++ {
		if v == x {
			continue
		}
		n, err := t.Enode(v)
		if err != nil {
			return false, err
		}
		shared, err := t.ctxIsShared(n)
		if err != nil {
			return false, err
		}
		if shared && n.Root() == r {
			return true, nil
		}
	}
	return false, nil
}

type smtArithMaxMin uint8

const (
	smtArithUnbounded smtArithMaxMin = iota
	smtArithAtBound
	smtArithOptimized
	smtArithBestEffort
)

// maxMinRow maximizes (minimizes) the temporary row r.
func (t *SMTTheoryArith) maxMinRow(r *smtArithmeticRow, max, maintainIntegrality bool, hasShared *bool) (smtArithMaxMin, error) {
	t.stats.maxMin++
	bestEfforts := uint32(0)
	inc := false
	q := t.q
	aij, currAij, coeff, currCoeff := smtQZero(), smtQZero(), smtQZero(), smtQZero()
	minGain, maxGain, currMinGain, currMaxGain := newSMTArithmeticValue(), newSMTArithmeticValue(), newSMTArithmeticValue(), newSMTArithmeticValue()
	defer func() {
		for _, r := range []*numeral.Rational{&aij, &currAij, &coeff, &currCoeff} {
			t.keep(smtQDel(q, r))
		}
		for _, v := range []*smtArithmeticValue{&minGain, &maxGain, &currMinGain, &currMaxGain} {
			t.keep(v.del(q))
		}
	}()
	result := smtArithOptimized
	*hasShared = false
	maxEfforts := 10 + t.state.random.Next()%20
	for bestEfforts < maxEfforts && !t.cancelFlag() {
		xj := SMTNullTheoryVar
		xi := SMTNullTheoryVar
		hasBound := false
		t.keep(maxGain.reset(t.q))
		t.keep(minGain.reset(t.q))
		for i := range r.entries {
			it := &r.entries[i]
			if it.dead() {
				continue
			}
			currXj := it.variable
			t.keep(t.q.Copy(&currCoeff, &it.coefficient))
			currInc := !max
			if currCoeff.Sign() > 0 {
				currInc = max
			}
			if (currInc && t.upper(currXj) != nil) || (!currInc && t.lower(currXj) != nil) {
				hasBound = true
			}
			if (currInc && t.atUpper(currXj)) || (!currInc && t.atLower(currXj)) {
				continue // variable cannot be used for max/min.
			}
			safeToLeave, currXi, err := t.pickVarToLeave(currXj, currInc, &currAij, &currMinGain, &currMaxGain, hasShared)
			if err != nil {
				return result, err
			}
			if !safeToLeave {
				hasBound = true
				bestEfforts++
			} else if currXi == SMTNullTheoryVar {
				// curr_x_j can move as much as we want.
				xi = SMTNullTheoryVar // unbounded
				xj = currXj
				inc = currInc
				t.keep(minGain.set(t.q, &currMinGain))
				t.keep(maxGain.set(t.q, &currMaxGain))
				break
			} else if t.vcmp(&currMaxGain, &maxGain) > 0 {
				xi, xj = currXi, currXj
				t.keep(t.q.Copy(&aij, &currAij))
				t.keep(t.q.Copy(&coeff, &currCoeff))
				t.keep(maxGain.set(t.q, &currMaxGain))
				t.keep(minGain.set(t.q, &currMinGain))
				inc = currInc
			} else if currMaxGain.isZero() && (xi == SMTNullTheoryVar || currXi < xi) {
				xi, xj = currXi, currXj
				t.keep(t.q.Copy(&aij, &currAij))
				t.keep(t.q.Copy(&coeff, &currCoeff))
				t.keep(maxGain.set(t.q, &currMaxGain))
				t.keep(minGain.set(t.q, &currMinGain))
				inc = currInc
			}
		}
		if !hasBound && xi == SMTNullTheoryVar && xj == SMTNullTheoryVar {
			*hasShared = false
			bestEfforts = 0
			result = smtArithUnbounded
			break
		}
		if xj == SMTNullTheoryVar {
			result = smtArithOptimized
			break
		}
		if minGain.sign() > 0 && !minGain.isOne() {
			bestEfforts++
		}
		if xi == SMTNullTheoryVar {
			// can increase/decrease x_j as much as we want.
			if inc && t.upper(xj) != nil {
				if maxGain.isZero() {
					return smtArithBestEffort, nil
				}
				if err := t.updateValue(xj, &maxGain); err != nil {
					return result, err
				}
				continue
			}
			if !inc && t.lower(xj) != nil {
				if maxGain.isZero() {
					return smtArithBestEffort, nil
				}
				t.keep(maxGain.neg(t.q))
				if err := t.updateValue(xj, &maxGain); err != nil {
					return result, err
				}
				continue
			}
			// As it stands this is a possibly unsound conclusion for shared
			// theories; the tradeoff is non-termination for unbounded
			// objectives in the presence of sharing.
			*hasShared = false
			bestEfforts = 0
			result = smtArithUnbounded
			break
		}
		if !t.isFixed(xj) && t.isBounded(xj) {
			width := newSMTArithmeticValue()
			t.keep(t.vsub(&width, t.upperBound(xj), t.lowerBound(xj)))
			c := t.vcmp(&width, &maxGain)
			t.keep(width.del(q))
			if c == 0 {
				// can increase/decrease x_j up to upper/lower bound.
				if !inc {
					t.keep(maxGain.neg(t.q))
				}
				if err := t.updateValue(xj, &maxGain); err != nil {
					return result, err
				}
				continue
			}
		}
		if err := t.pivot(true, xi, xj, &aij, false); err != nil {
			return result, err
		}
		incXi := aij.Sign() > 0
		if inc {
			incXi = aij.Sign() < 0
		}
		if _, err := t.moveToBound(xi, incXi, &bestEfforts, hasShared); err != nil {
			return result, err
		}
		r2 := &t.rows[t.getVarRow(xj)]
		t.keep(t.q.Neg(&coeff))
		if err := t.addTmpRow(r, &coeff, r2); err != nil {
			return result, err
		}
	}
	if bestEfforts > 0 || t.cancelFlag() {
		return smtArithBestEffort, t.takeQErr(nil)
	}
	return result, t.takeQErr(nil)
}

// moveToBound moves x_i maximally towards its bound as long as bounds of
// other variables are not violated. It reports false if an integer bound
// was truncated and no progress was made.
func (t *SMTTheoryArith) moveToBound(xi SMTTheoryVar, inc bool, bestEfforts *uint32, hasShared *bool) (bool, error) {
	minGain, maxGain := newSMTArithmeticValue(), newSMTArithmeticValue()
	defer func() {
		t.keep(minGain.del(t.q))
		t.keep(maxGain.del(t.q))
	}()
	if t.isInt(xi) && !t.getValue(xi).isInt() {
		*bestEfforts++
		return false, nil
	}
	t.initGains(xi, inc, &minGain, &maxGain)
	c := &t.columns[xi]
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() {
			continue
		}
		r := &t.rows[it.rowID]
		s := r.base
		t.updateGains(inc, s, &r.entries[it.rowIndex].coefficient, &minGain, &maxGain)
		ns, err := t.Enode(s)
		if err != nil {
			return false, err
		}
		ss, err := t.ctxIsShared(ns)
		if err != nil {
			return false, err
		}
		*hasShared = *hasShared || ss
	}
	result := false
	if t.safeGain(&minGain, &maxGain) {
		if !inc {
			t.keep(maxGain.neg(t.q))
		}
		if err := t.updateValue(xi, &maxGain); err != nil {
			return false, err
		}
		if minGain.sign() <= 0 || minGain.isOne() {
			*bestEfforts++
		}
		result = !maxGain.isZero()
	}
	if !result {
		*bestEfforts++
	}
	return result, t.takeQErr(nil)
}

// addTmpRowEntry adds coeff * v (negated when invert) to a temporary row.
func (t *SMTTheoryArith) addTmpRowEntry(invert bool, r *smtArithmeticRow, coeff *numeral.Rational, v SMTTheoryVar) error {
	_, rEntry := r.addEntry()
	rEntry.variable = v
	if err := t.q.Copy(&rEntry.coefficient, coeff); err != nil {
		return err
	}
	if invert {
		return t.q.Neg(&rEntry.coefficient)
	}
	return nil
}

// loadTmpRow is the row of v, as a temporary row: v itself when it is a
// non-base variable, otherwise the negated other monomials of its row.
func (t *SMTTheoryArith) loadTmpRow(v SMTTheoryVar) error {
	if err := t.tmpRow.reset(t.q); err != nil {
		return err
	}
	if t.isNonBase(v) {
		return t.addTmpRowEntry(false, &t.tmpRow, smtQIntPtr(1), v)
	}
	r := &t.rows[t.getVarRow(v)]
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() && it.variable != v {
			if err := t.addTmpRowEntry(true, &t.tmpRow, &it.coefficient, it.variable); err != nil {
				return err
			}
		}
	}
	return nil
}

// maxMin maximizes/minimizes v; its bounds are updated on success.
func (t *SMTTheoryArith) maxMin(v SMTTheoryVar, max, maintainIntegrality bool, hasShared *bool) (smtArithMaxMin, error) {
	if (max && t.atUpper(v)) || (!max && t.atLower(v)) {
		return smtArithAtBound, nil // nothing to be done...
	}
	if err := t.loadTmpRow(v); err != nil {
		return smtArithOptimized, err
	}
	r, err := t.maxMinRow(&t.tmpRow, max, maintainIntegrality, hasShared)
	if err != nil {
		return r, err
	}
	if r == smtArithOptimized {
		kind := smtArithmeticLowerBound
		if max {
			kind = smtArithmeticUpperBound
		}
		val := newSMTArithmeticValue()
		defer val.del(t.q)
		if err = val.set(t.q, t.getValue(v)); err != nil {
			return r, err
		}
		if err = t.mkBoundFromRow(v, &val, kind, &t.tmpRow); err != nil {
			return r, err
		}
	}
	return r, nil
}

// maxMinVars maximizes and minimizes the variables; false means an
// inconsistency was detected.
func (t *SMTTheoryArith) maxMinVars(vars []SMTTheoryVar) (bool, error) {
	succ := false
	hasShared := false
	for _, v := range vars {
		r, err := t.maxMin(v, true, false, &hasShared)
		if err != nil {
			return false, err
		}
		if r == smtArithOptimized && !hasShared {
			succ = true
		}
		if r, err = t.maxMin(v, false, false, &hasShared); err != nil {
			return false, err
		}
		if r == smtArithOptimized && !hasShared {
			succ = true
		}
	}
	if succ {
		// process new bounds
		return t.propagateCore()
	}
	return true, nil
}

// ---- aux.h:1857-1940 (freedom intervals) ----

// getFreedomInterval computes the interval [l, u] in which the non-base x_j
// can move, and in m the lcm of the denominators of the a_ij of integer
// rows (see the model-based theory combination paper). It reports false
// when x_j is a base variable. Manager failures are kept in qErr.
func (t *SMTTheoryArith) getFreedomInterval(xj SMTTheoryVar, infL *bool, l *smtArithmeticValue, infU *bool, u *smtArithmeticValue, m *numeral.Rational) bool {
	if t.isBase(xj) {
		return false
	}
	q := t.q
	// get_value(xj) of a quasi-base xj lives in the shared t.tmp, so it is
	// copied into scratch. The row bases x_i below are skipped when
	// quasi-base, so get_value(x_i) is m_value[x_i], read in place: nothing
	// here changes m_value.
	xjVal := &t.scratchXjVal
	t.keep(xjVal.set(q, t.getValue(xj)))
	*infL, *infU = true, true
	t.keep(l.reset(q))
	t.keep(u.reset(q))
	t.keep(q.SetInt64(m, 1))
	isFixed := func() bool { return !*infL && !*infU && t.vcmp(l, u) == 0 }
	setLower := func(val *smtArithmeticValue) bool {
		if *infL || t.vcmp(val, l) > 0 {
			t.keep(l.set(q, val))
			*infL = false
		}
		return isFixed()
	}
	setUpper := func(val *smtArithmeticValue) bool {
		if *infU || t.vcmp(val, u) < 0 {
			t.keep(u.set(q, val))
			*infU = false
		}
		return isFixed()
	}
	if t.lower(xj) != nil && setLower(t.lowerBound(xj)) {
		return true
	}
	if t.upper(xj) != nil && setUpper(t.upperBound(xj)) {
		return true
	}
	den := smtQZero()
	defer func() { t.keep(smtQDel(q, &den)) }()
	c := &t.columns[xj]
	for i := range c.entries {
		it := &c.entries[i]
		if it.dead() {
			continue
		}
		r := &t.rows[it.rowID]
		xi := r.base
		if xi == SMTNullTheoryVar || t.isQuasiBase(xi) {
			continue
		}
		aij := &r.entries[it.rowIndex].coefficient
		xiVal := &t.value[xi]
		if t.isInt(xi) && t.isInt(xj) && !aij.IsInt() {
			// m = lcm(m, denominator(a_ij))
			t.keep(q.DenominatorRational(aij, &den))
			t.keep(q.LCM(m, &den, m))
		}
		xiLower, xiUpper := t.lower(xi), t.upper(xi)
		// bound is a scratch value; setLower/setUpper copy it into l/u.
		bound := func(b smtArithBound) *smtArithmeticValue {
			v := &t.scratchBound
			t.keep(t.vsub(v, xiVal, &b.base().value))
			t.keep(v.quoRat(q, aij))
			t.keep(v.add(q, xjVal))
			return v
		}
		if aij.Sign() < 0 {
			if xiLower != nil && setLower(bound(xiLower)) {
				return true
			}
			if xiUpper != nil && setUpper(bound(xiUpper)) {
				return true
			}
		} else {
			if xiUpper != nil && setLower(bound(xiUpper)) {
				return true
			}
			if xiLower != nil && setUpper(bound(xiLower)) {
				return true
			}
		}
	}
	return true
}

// ---- aux.h:1942-2031 (implied eqs) ----

// tryToImplyEq builds the row v1 - v2. The check that follows it in the
// source is disabled (#if 0), so it never reports an equality.
func (t *SMTTheoryArith) tryToImplyEq(v1, v2 SMTTheoryVar) (bool, error) {
	if t.isQuasiBase(v1) || t.isQuasiBase(v2) {
		return false, nil
	}
	if err := t.loadTmpRow(v1); err != nil {
		return false, err
	}
	if err := t.tmpRow.saveVariablePositions(t.varPos); err != nil {
		return false, err
	}
	addEntry := func(coeff *numeral.Rational, v SMTTheoryVar) error {
		pos := t.varPos[v]
		if pos == -1 {
			return t.addTmpRowEntry(false, &t.tmpRow, coeff, v)
		}
		rEntry := &t.tmpRow.entries[pos]
		if err := t.q.Add(&rEntry.coefficient, coeff, &rEntry.coefficient); err != nil {
			return err
		}
		if rEntry.coefficient.IsZero() {
			if err := t.tmpRow.deleteEntry(t.q, pos); err != nil {
				return err
			}
		}
		t.varPos[v] = -1
		return nil
	}
	if t.isNonBase(v2) {
		if err := addEntry(smtQIntPtr(-1), v2); err != nil {
			return false, err
		}
	} else {
		neg := smtQZero()
		defer smtQDel(t.q, &neg)
		r := &t.rows[t.getVarRow(v2)]
		for i := range r.entries {
			it := &r.entries[i]
			if !it.dead() && it.variable != v2 {
				if err := t.q.Copy(&neg, &it.coefficient); err != nil {
					return false, err
				}
				if err := t.q.Neg(&neg); err != nil {
					return false, err
				}
				if err := addEntry(&neg, it.variable); err != nil {
					return false, err
				}
			}
		}
	}
	if err := t.tmpRow.resetVariablePositions(t.varPos); err != nil {
		return false, err
	}
	return false, nil
}

// ---- aux.h:2033-2249 (assume eqs) ----

const smtArithRange = 10000 // RANGE

// randomUpdate performs a random update of v within its freedom interval and
// reports whether v changed.
func (t *SMTTheoryArith) randomUpdate(v SMTTheoryVar) (_ bool, err error) {
	if t.isFixed(v) || !t.isNonBase(v) {
		return false, nil
	}
	q := t.q
	var infL, infU bool
	l, u, newVal, tmp := newSMTArithmeticValue(), newSMTArithmeticValue(), newSMTArithmeticValue(), newSMTArithmeticValue()
	m, c, delta := smtQZero(), smtQZero(), smtQZero()
	defer func() {
		for _, x := range []*smtArithmeticValue{&l, &u, &newVal, &tmp} {
			if e := x.del(q); e != nil && err == nil {
				err = e
			}
		}
		for _, x := range []*numeral.Rational{&m, &c, &delta} {
			if e := smtQDel(q, x); e != nil && err == nil {
				err = e
			}
		}
	}()
	t.getFreedomInterval(v, &infL, &l, &infU, &u, &m)
	if err := t.takeQErr(nil); err != nil {
		return false, err
	}
	// rnd sets delta = m_random() % (n + 1).
	rnd := func(n uint32) error { return q.SetInt64(&delta, int64(t.random.Next()%(n+1))) }
	if infL && infU {
		if err := rnd(smtArithRange); err != nil {
			return false, err
		}
		if err := newVal.setRational(q, &delta); err != nil {
			return false, err
		}
		return true, t.setValue(v, &newVal)
	}
	// roundToM sets x = m*ceil(x/m) (or floor) after x = ceil(x) (floor).
	roundToM := func(x *smtArithmeticValue, up bool) error {
		round := smtInfFloor
		if up {
			round = smtInfCeil
		}
		if err := round(q, x, &c); err != nil {
			return err
		}
		if err := x.setRational(q, &c); err != nil {
			return err
		}
		if m.IsOne() {
			return nil
		}
		if err := tmp.set(q, x); err != nil {
			return err
		}
		if err := tmp.quoRat(q, &m); err != nil {
			return err
		}
		if err := round(q, &tmp, &c); err != nil {
			return err
		}
		if err := q.Mul(&m, &c, &c); err != nil {
			return err
		}
		return x.setRational(q, &c)
	}
	if t.isInt(v) {
		if !infL {
			if err := roundToM(&l, true); err != nil {
				return false, err
			}
		}
		if !infU {
			if err := roundToM(&u, false); err != nil {
				return false, err
			}
		}
	}
	if !infL && !infU && t.vcmp(&l, &u) >= 0 {
		return false, t.takeQErr(nil)
	}
	// offset sets newVal = base + sign*m*delta.
	offset := func(base *smtArithmeticValue, n uint32, subtract bool) error {
		if err := rnd(n); err != nil {
			return err
		}
		if err := newVal.set(q, base); err != nil {
			return err
		}
		if err := q.Mul(&m, &delta, &c); err != nil {
			return err
		}
		if subtract {
			return q.Sub(&newVal.rational, &c, &newVal.rational)
		}
		return q.Add(&newVal.rational, &c, &newVal.rational)
	}
	if infU {
		if err := offset(&l, smtArithRange, false); err != nil {
			return false, err
		}
		return true, t.setValue(v, &newVal)
	}
	if infL {
		if err := offset(&u, smtArithRange, true); err != nil {
			return false, err
		}
		return true, t.setValue(v, &newVal)
	}
	if !t.isInt(v) {
		if err := rnd(smtArithRange); err != nil {
			return false, err
		}
		rangeQ := smtQInt(smtArithRange)
		if err := t.vsub(&tmp, &u, &l); err != nil {
			return false, err
		}
		if err := tmp.mulRat(q, &delta); err != nil {
			return false, err
		}
		if err := tmp.quoRat(q, &rangeQ); err != nil {
			return false, err
		}
		if err := newVal.set(q, &l); err != nil {
			return false, err
		}
		if err := newVal.add(q, &tmp); err != nil {
			return false, err
		}
		return true, t.setValue(v, &newVal)
	}
	rng := uint32(smtArithRange)
	if err := q.Sub(&u.rational, &l.rational, &c); err != nil {
		return false, err
	}
	if err := q.Div(&c, &m, &c); err != nil {
		return false, err
	}
	rangeQ := smtQInt(smtArithRange)
	if t.rcmp(&c, &rangeQ) < 0 {
		// u - l and m are integers here, so r is one below RANGE.
		n, err := q.Int64(&c)
		if err != nil {
			return false, err
		}
		rng = uint32(n)
	}
	if err := offset(&l, rng, false); err != nil {
		return false, err
	}
	return true, t.setValue(v, &newVal)
}

// isRelevantAndShared is theory::is_relevant_and_shared.
func (t *SMTTheoryArith) isRelevantAndShared(n *SMTEnode) (bool, error) {
	if !t.state.isRelevant(n.owner) {
		return false, nil
	}
	return t.ctxIsShared(n)
}

// varValueKey is the key of m_var_value_table: value and is_int_src.
func (t *SMTTheoryArith) varValueKey(v SMTTheoryVar) smtArithVarValueKey {
	val := t.getValue(v)
	return smtArithVarValueKey{
		rational:      t.qkey(&val.rational),
		infinitesimal: t.qkey(&val.infinitesimal),
		isInt:         t.isIntSrc(v),
	}
}

type smtArithVarValueKey struct {
	rational, infinitesimal smtQMapKey
	isInt                   bool
}

// insertIfNotThere is m_var_value_table.insert_if_not_there(v).
func (t *SMTTheoryArith) insertIfNotThere(v SMTTheoryVar) SMTTheoryVar {
	key := t.varValueKey(v)
	if other, found := t.varValueTable[key]; found {
		return other
	}
	t.varValueTable[key] = v
	return v
}

func (t *SMTTheoryArith) mutateAssignment() error {
	if err := t.removeFixedVarsFromBase(); err != nil {
		return err
	}
	numVars := SMTTheoryVar(t.getNumVars())
	clear(t.varValueTable)
	tmpVarSet := make(map[SMTTheoryVar]bool)
	var candidates []SMTTheoryVar
	for v := SMTTheoryVar(0); v < numVars; v++ {
		n1, err := t.Enode(v)
		if err != nil {
			return err
		}
		ok, err := t.isRelevantAndShared(n1)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		other := t.insertIfNotThere(v)
		if other == v {
			continue // first node with the given value...
		}
		n2, err := t.Enode(other)
		if err != nil {
			return err
		}
		if n1.Root() == n2.Root() {
			continue
		}
		if !t.isFixed(v) {
			candidates = append(candidates, v)
		} else if !t.isFixed(other) && !tmpVarSet[other] {
			tmpVarSet[other] = true
			candidates = append(candidates, other)
		}
	}
	for _, v := range candidates {
		if t.isBase(v) {
			r := &t.rows[t.getVarRow(v)]
			for i := range r.entries {
				it := &r.entries[i]
				if it.dead() || it.variable == v || t.isFixed(it.variable) {
					continue
				}
				updated, err := t.randomUpdate(it.variable)
				if err != nil {
					return err
				}
				if updated {
					break
				}
			}
		} else if _, err := t.randomUpdate(v); err != nil {
			return err
		}
	}
	return nil
}

// isShared: the arithmetic theory has underspecified operators such as
// division by 0, so a child of one is a shared variable.
func (t *SMTTheoryArith) isShared(v SMTTheoryVar) (bool, error) {
	if !t.foundUnderspecifiedOp {
		return false, nil
	}
	n, err := t.Enode(v)
	if err != nil {
		return false, err
	}
	for _, parent := range n.Root().sourceParents {
		if op, own := t.arithOp(parent.owner); own {
			switch op {
			case smtArithOpDiv, smtArithOpIDiv, smtArithOpRem, smtArithOpMod:
				return true, nil
			}
		}
	}
	return false, nil
}

func (t *SMTTheoryArith) assumeEqsCore() (bool, error) {
	// See the comment on m_liberal_final_check.
	if t.liberalFinalCheck {
		if err := t.mutateAssignment(); err != nil {
			return false, err
		}
	}
	clear(t.varValueTable)
	num := SMTTheoryVar(t.getNumVars())
	for v := SMTTheoryVar(0); v < num; v++ {
		n, err := t.Enode(v)
		if err != nil {
			return false, err
		}
		ok, err := t.isRelevantAndShared(n)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		other := t.insertIfNotThere(v)
		if other == v {
			continue
		}
		n2, err := t.Enode(other)
		if err != nil {
			return false, err
		}
		if n.Root() == n2.Root() {
			continue
		}
		t.assumeEqCandidates = append(t.assumeEqCandidates, [2]SMTTheoryVar{other, v})
	}
	// The candidates are trimmed on backtracking by the scope record
	// (restore_size_trail).
	return t.delayedAssumeEqs()
}

func (t *SMTTheoryArith) delayedAssumeEqs() (bool, error) {
	if t.assumeEqHead == uint32(len(t.assumeEqCandidates)) {
		return false, nil
	}
	// m_assume_eq_head is restored by the scope record (value_trail).
	for t.assumeEqHead < uint32(len(t.assumeEqCandidates)) {
		p := t.assumeEqCandidates[t.assumeEqHead]
		v1, v2 := p[0], p[1]
		t.assumeEqHead++
		n1, err := t.Enode(v1)
		if err != nil {
			return false, err
		}
		n2, err := t.Enode(v2)
		if err != nil {
			return false, err
		}
		if t.vcmp(t.getValue(v1), t.getValue(v2)) == 0 && n1.Root() != n2.Root() {
			progress, err := t.state.assumeEquality(n1, n2)
			if err != nil || progress {
				return progress, err
			}
		}
	}
	return false, nil
}
