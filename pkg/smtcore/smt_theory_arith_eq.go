package smtcore

// Source: src/smt/theory_arith_eq.h, in source order.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func smtArithValueSortPairOf(value *big.Rat, isInt bool) smtArithValueSortPair {
	return smtArithValueSortPair{value: value.RatString(), isInt: isInt}
}

// lowerRational returns the rational part of the lower bound of v.
func (t *SMTTheoryArith) lowerRational(v SMTTheoryVar) *big.Rat {
	return &t.lower(v).base().value.rational
}

// fixedVarEh is invoked when a variable that was not fixed becomes fixed.
func (t *SMTTheoryArith) fixedVarEh(v SMTTheoryVar) error {
	if !t.propagateEqs() {
		return nil
	}
	if !t.isFixed(v) || !t.lower(v).base().value.isRational() {
		return ast.ErrASTDomain
	}
	val := t.lowerRational(v)
	key := smtArithValueSortPairOf(val, t.isIntSrc(v))
	v2, found := t.fixedVarTable[key]
	if !found {
		t.fixedVarTable[key] = v
		return nil
	}
	if int(v2) < t.getNumVars() && t.isFixed(v2) && t.lowerRational(v2).Cmp(val) == 0 {
		// Only equate variables of the same sort. The table is not restored
		// on backtracking, so it may hold stale pairs: the checks above
		// confirm v2 really has this value.
		if !t.isEqual(v, v2) && t.isIntSrc(v) == t.isIntSrc(v2) {
			ante := t.acquireAntecedents()
			defer t.releaseAntecedentsScope()
			zero := new(big.Rat)
			// v <= k <= v2 => v <= v2 and v >= k >= v2 => v >= v2
			for _, b := range []smtArithBound{t.lower(v), t.upper(v2), t.lower(v2), t.upper(v)} {
				if err := b.pushJustification(ante, zero, t.proofsEnabled()); err != nil {
					return err
				}
			}
			t.stats.fixedEqs++
			return t.propagateEqToCore(v, v2, ante)
		}
		return nil
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
func (t *SMTTheoryArith) isOffsetRow(r *smtArithmeticRow) (x, y SMTTheoryVar, k big.Rat, ok bool) {
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
			return SMTNullTheoryVar, SMTNullTheoryVar, k, false
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
		if smtRatIsOne(&it.coefficient) && x == SMTNullTheoryVar {
			x = v
			continue
		}
		if smtRatIsMinusOne(&it.coefficient) && y == SMTNullTheoryVar {
			y = v
			continue
		}
		return SMTNullTheoryVar, SMTNullTheoryVar, k, false
	}
	if x == SMTNullTheoryVar && y == SMTNullTheoryVar {
		return x, y, k, false
	}
	var term big.Rat
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		v := it.variable
		if v == x || v == y {
			continue
		}
		k.Sub(&k, term.Mul(&it.coefficient, t.lowerRational(v)))
	}
	if y == SMTNullTheoryVar {
		return x, y, k, true
	}
	if x == SMTNullTheoryVar {
		x, y = y, x
		k.Neg(&k)
		return x, y, k, true
	}
	if r.base != x && x > y {
		x, y = y, x
		k.Neg(&k)
	}
	return x, y, k, true
}

// propagateCheapEq propagates x_i = x_j when x_i = y + k and x_j = y + k,
// detected with the map (y, k) -> row id.
func (t *SMTTheoryArith) propagateCheapEq(rid int) error {
	if !t.propagateEqs() {
		return nil
	}
	r := &t.rows[rid]
	x, y, k, ok := t.isOffsetRow(r)
	if !ok {
		return nil
	}
	if y == SMTNullTheoryVar {
		// x is an implied fixed variable at k.
		key := smtArithValueSortPairOf(&k, t.isIntSrc(x))
		if x2, found := t.fixedVarTable[key]; found &&
			int(x2) < t.getNumVars() &&
			t.isFixed(x2) &&
			t.lowerRational(x2).Cmp(&k) == 0 &&
			t.isIntSrc(x) == t.isIntSrc(x2) &&
			!t.isEqual(x, x2) {
			ante := t.acquireAntecedents()
			err := t.collectFixedVarJustifications(r, ante)
			if err == nil {
				zero := new(big.Rat)
				err = t.lower(x2).pushJustification(ante, zero, t.proofsEnabled())
				if err == nil {
					err = t.upper(x2).pushJustification(ante, zero, t.proofsEnabled())
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
	if k.Sign() == 0 && y != SMTNullTheoryVar && !t.isEqual(x, y) && t.isIntSrc(x) == t.isIntSrc(y) {
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
	key := smtArithVarOffset{v: y, offset: k.RatString()}
	if rowID, found := t.varOffset2RowID[key]; found {
		r2 := &t.rows[rowID]
		if r.base == r2.base {
			return nil // it is the same row
		}
		if r2.base != SMTNullTheoryVar {
			if x2, y2, k2, isOffset := t.isOffsetRow(r2); isOffset {
				newEq := false
				if y == y2 && k.Cmp(&k2) == 0 {
					newEq = true
				} else if y2 != SMTNullTheoryVar {
					x2, y2 = y2, x2
					k2.Neg(&k2)
					if y == y2 && k.Cmp(&k2) == 0 {
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
	return nil
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
