package smtcore

// Source: src/smt/theory_arith_eq.h, in source order.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTTheoryArith) valueSortPairOf(value *numeral.Rational, isInt bool) smtArithValueSortPair {
	return smtArithValueSortPair{value: t.qkey(value), isInt: isInt}
}

// lowerRational returns the rational part of the lower bound of v.
func (t *SMTTheoryArith) lowerRational(v SMTTheoryVar) *numeral.Rational {
	return &t.lower(v).base().value.rational
}

// fixedVarEh is invoked when a variable that was not fixed becomes fixed.
func (t *SMTTheoryArith) fixedVarEh(v SMTTheoryVar) error {
	if !t.propagateEqs() {
		return nil
	}
	// SASSERT(lower_bound(v).is_rational()) is debug only: the release
	// build keys the table on lower_bound(v).get_rational(), dropping an
	// epsilon part (AUDIT.md FUZZ_A-F7).
	if !t.isFixed(v) {
		return ast.ErrASTDomain
	}
	val := t.lowerRational(v)
	key := t.valueSortPairOf(val, t.isIntSrc(v))
	v2, found := t.fixedVarTable[key]
	if !found {
		t.fixedVarTable[key] = v
		return nil
	}
	if int(v2) < t.getNumVars() && t.isFixed(v2) && t.rcmp(t.lowerRational(v2), val) == 0 {
		// Only equate variables of the same sort. The table is not restored
		// on backtracking, so it may hold stale pairs: the checks above
		// confirm v2 really has this value.
		if !t.isEqual(v, v2) && t.isIntSrc(v) == t.isIntSrc(v2) {
			ante := t.acquireAntecedents()
			defer t.releaseAntecedentsScope()
			zero := smtQZero()
			// v <= k <= v2 => v <= v2 and v >= k >= v2 => v >= v2
			for _, b := range []smtArithBound{t.lower(v), t.upper(v2), t.lower(v2), t.upper(v)} {
				if err := b.pushJustification(t.q, ante, &zero, t.proofsEnabled()); err != nil {
					return err
				}
			}
			t.stats.fixedEqs++
			return t.propagateEqToCore(v, v2, ante)
		}
		return t.takeQErr(nil)
	}
	// the original fixed variable v2 was deleted or its bounds were removed
	// during backtracking.
	t.fixedVarTable[key] = v
	return nil
}

// isOffsetRow reports whether r can be written x = y + M, with x and y non
// fixed and M a linear polynomial over fixed variables evaluating to k.
// It returns x, y and k. The base variable is x if it is not fixed,
// otherwise x is the smallest variable.
func (t *SMTTheoryArith) isOffsetRow(r *smtArithmeticRow, k *numeral.Rational) (x, y SMTTheoryVar, ok bool, err error) {
	// Quick check without big numbers: more than 2 unbounded variables.
	bad := 0
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		if t.lower(it.variable) != nil && t.upper(it.variable) != nil {
			continue
		}
		bad++
		if bad > 2 {
			return SMTNullTheoryVar, SMTNullTheoryVar, false, nil
		}
	}
	x, y = SMTNullTheoryVar, SMTNullTheoryVar
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if t.isFixed(v) {
			continue
		}
		if it.coefficient.IsOne() && x == SMTNullTheoryVar {
			x = v
			continue
		}
		if it.coefficient.IsMinusOne() && y == SMTNullTheoryVar {
			y = v
			continue
		}
		return SMTNullTheoryVar, SMTNullTheoryVar, false, nil
	}
	if x == SMTNullTheoryVar && y == SMTNullTheoryVar {
		return x, y, false, nil
	}
	q := t.q
	if err := q.Reset(k); err != nil {
		return x, y, false, err
	}
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if v == x || v == y {
			continue
		}
		if err := q.SubMul(k, &it.coefficient, t.lowerRational(v), k); err != nil {
			return x, y, false, err
		}
	}
	if y == SMTNullTheoryVar {
		return x, y, true, nil
	}
	if x == SMTNullTheoryVar {
		x, y = y, x
		return x, y, true, q.Neg(k)
	}
	if r.base != x && x > y {
		x, y = y, x
		return x, y, true, q.Neg(k)
	}
	return x, y, true, nil
}

// propagateCheapEq propagates x_i = x_j when x_i = y + k and x_j = y + k,
// detected with the map (y, k) -> row id.
func (t *SMTTheoryArith) propagateCheapEq(rid int) error {
	if !t.propagateEqs() {
		return nil
	}
	r := &t.rows[rid]
	q := t.q
	k, k2 := smtQZero(), smtQZero()
	defer smtQDel(q, &k)
	defer smtQDel(q, &k2)
	x, y, ok, err := t.isOffsetRow(r, &k)
	if err != nil || !ok {
		return err
	}
	if y == SMTNullTheoryVar {
		// x is an implied fixed variable at k.
		key := t.valueSortPairOf(&k, t.isIntSrc(x))
		if x2, found := t.fixedVarTable[key]; found &&
			int(x2) < t.getNumVars() &&
			t.isFixed(x2) &&
			t.rcmp(t.lowerRational(x2), &k) == 0 &&
			t.isIntSrc(x) == t.isIntSrc(x2) &&
			!t.isEqual(x, x2) {
			ante := t.acquireAntecedents()
			err := t.collectFixedVarJustifications(r, ante)
			if err == nil {
				zero := smtQZero()
				err = t.lower(x2).pushJustification(t.q, ante, &zero, t.proofsEnabled())
				if err == nil {
					err = t.upper(x2).pushJustification(t.q, ante, &zero, t.proofsEnabled())
				}
			}
			if err == nil {
				t.stats.fixedEqs++
				err = t.propagateEqToCore(x, x2, ante)
			}
			t.releaseAntecedentsScope()
			if err != nil {
				return err
			}
			r = &t.rows[rid]
		}
	}
	if k.IsZero() && y != SMTNullTheoryVar && !t.isEqual(x, y) && t.isIntSrc(x) == t.isIntSrc(y) {
		// found equality x = y
		ante := t.acquireAntecedents()
		err := t.collectFixedVarJustifications(r, ante)
		if err == nil {
			t.stats.offsetEqs++
			err = t.propagateEqToCore(x, y, ante)
		}
		t.releaseAntecedentsScope()
		if err != nil {
			return err
		}
		r = &t.rows[rid]
	}
	key := smtArithVarOffset{v: y, offset: t.qkey(&k)}
	if rowID, found := t.varOffset2RowID[key]; found {
		r2 := &t.rows[rowID]
		if r.base == r2.base {
			return nil // it is the same row
		}
		if r2.base != SMTNullTheoryVar {
			x2, y2, isOffset, err := t.isOffsetRow(r2, &k2)
			if err != nil {
				return err
			}
			if isOffset {
				newEq := false
				if y == y2 && t.rcmp(&k, &k2) == 0 {
					newEq = true
				} else if y2 != SMTNullTheoryVar {
					x2, y2 = y2, x2
					if err := q.Neg(&k2); err != nil {
						return err
					}
					if y == y2 && t.rcmp(&k, &k2) == 0 {
						newEq = true
					}
				}
				if newEq {
					if !t.isEqual(x, x2) && t.isIntSrc(x) == t.isIntSrc(x2) {
						ante := t.acquireAntecedents()
						err := t.collectFixedVarJustifications(r, ante)
						if err == nil {
							err = t.collectFixedVarJustifications(r2, ante)
						}
						if err == nil {
							t.stats.offsetEqs++
							err = t.propagateEqToCore(x, x2, ante)
						}
						t.releaseAntecedentsScope()
						if err != nil {
							return err
						}
					}
					return nil
				}
			}
		}
		// the original row was deleted or is not an offset row any more
		delete(t.varOffset2RowID, key)
	}
	t.varOffset2RowID[key] = rid
	return t.takeQErr(nil)
}

// propagateEqToCore assigns x = y, justified by the antecedents.
func (t *SMTTheoryArith) propagateEqToCore(x, y SMTTheoryVar, antecedents *smtArithmeticAntecedents) (result error) {
	// Ignore equality if variables are already known to be equal.
	if t.isEqual(x, y) {
		return nil
	}
	m := t.state.manager
	// It doesn't make sense to propagate an equality of variables of
	// different sort.
	sx, err := m.GetSort(t.var2expr(x))
	if err != nil {
		return err
	}
	sy, err := m.GetSort(t.var2expr(y))
	if err != nil {
		return err
	}
	if sx != sy {
		return nil
	}
	nx, err := t.Enode(x)
	if err != nil {
		return err
	}
	ny, err := t.Enode(y)
	if err != nil {
		return err
	}
	params, err := antecedents.parameters(m, "eq-propagate")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, params)) }()
	js, err := NewSMTExtTheoryEqualityPropagationJustification(m, t.FamilyID(), antecedents.literals, antecedents.equalities, nx, ny, params)
	if err != nil {
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, js)
	return t.state.assignEquality(nx, ny, NewSMTEqualityJustification(js))
}
