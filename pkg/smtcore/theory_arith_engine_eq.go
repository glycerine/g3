package smtcore

// Source: src/smt/theory_arith_eq.h (fixed_var_eh, is_offset_row,
// propagate_cheap_eq) and core.h:3102-3116 (collect_fixed_var_justifications).

import "math/big"

// smtArithFixedKey is value_sort_pair: a fixed value and whether the
// variable is an integer in the source. The table is not restored on
// backtracking, as in the source, so entries are validated before use.
type smtArithFixedKey struct {
	value string
	isInt bool
}

// smtArithOffsetKey is var_offset: (y, k) for rows of the form x = y + k.
type smtArithOffsetKey struct {
	variable SMTTheoryVar
	offset   string
}

func smtArithFixedKeyOf(value *big.Rat, isInt bool) smtArithFixedKey {
	return smtArithFixedKey{value: value.RatString(), isInt: isInt}
}

func (e *smtArithEngine) propagateEqs() bool {
	return e.params.PropagateEqs && e.numConflicts < e.params.PropagationThreshold
}

// lowerRational returns the rational part of the lower bound of v.
func (e *smtArithEngine) lowerRational(v SMTTheoryVar) *big.Rat {
	return &e.lower(v).core().value.rational
}

// fixedVarEh is invoked when a variable that was not fixed becomes fixed.
func (e *smtArithEngine) fixedVarEh(v SMTTheoryVar) error {
	if !e.propagateEqs() {
		return nil
	}
	if !e.isFixed(v) || !e.lower(v).core().value.isRational() {
		return errSMTArithEngineInvariant
	}
	val := e.lowerRational(v)
	key := smtArithFixedKeyOf(val, e.host.isIntSource(v))
	v2, found := e.fixedVarTable[key]
	if !found {
		e.fixedVarTable[key] = v
		return nil
	}
	if int(v2) < e.numVars() && e.isFixed(v2) && e.lowerRational(v2).Cmp(val) == 0 {
		// Only equate variables of the same sort. The table is not restored
		// on backtracking, so it may hold stale pairs: the checks above
		// confirm v2 really has this value.
		if !e.host.isEqual(v, v2) && e.host.isIntSource(v) == e.host.isIntSource(v2) {
			ante := e.newAntecedents()
			defer e.releaseAntecedents()
			zero := new(big.Rat)
			// v <= k <= v2 => v <= v2 and v >= k >= v2 => v >= v2
			for _, b := range []smtArithEngineBound{e.lower(v), e.upper(v2), e.lower(v2), e.upper(v)} {
				if err := b.pushJustification(ante, zero, e.proofs); err != nil {
					return err
				}
			}
			e.stats.fixedEqs++
			return e.host.propagateEqToCore(v, v2, ante)
		}
		return nil
	}
	// the original fixed variable v2 was deleted or its bounds were removed
	// during backtracking.
	e.fixedVarTable[key] = v
	return nil
}

// isOffsetRow reports whether r can be written x = y + M, with x and y non
// fixed and M a linear polynomial over fixed variables evaluating to k.
// It returns x, y and k. The base variable is x if it is not fixed,
// otherwise x is the smallest variable.
func (e *smtArithEngine) isOffsetRow(r *smtArithmeticRow) (x, y SMTTheoryVar, k big.Rat, ok bool) {
	// Quick check without big numbers: more than 2 unbounded variables.
	bad := 0
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		if e.lower(it.variable) != nil && e.upper(it.variable) != nil {
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
		if e.isFixed(v) {
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
		k.Sub(&k, term.Mul(&it.coefficient, e.lowerRational(v)))
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

func smtRatIsOne(q *big.Rat) bool { return q.IsInt() && q.Num().IsInt64() && q.Num().Int64() == 1 }
func smtRatIsMinusOne(q *big.Rat) bool {
	return q.IsInt() && q.Num().IsInt64() && q.Num().Int64() == -1
}

// collectFixedVarJustifications pushes the bound justifications of the fixed
// variables in r.
func (e *smtArithEngine) collectFixedVarJustifications(r *smtArithmeticRow, ante *smtArithmeticAntecedents) error {
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() || !e.isFixed(it.variable) {
			continue
		}
		if err := e.lower(it.variable).pushJustification(ante, &it.coefficient, e.coeffsEnabled()); err != nil {
			return err
		}
		if err := e.upper(it.variable).pushJustification(ante, &it.coefficient, e.coeffsEnabled()); err != nil {
			return err
		}
	}
	return nil
}

// propagateCheapEq propagates x_i = x_j when x_i = y + k and x_j = y + k,
// detected with the map (y, k) -> row id.
func (e *smtArithEngine) propagateCheapEq(rid int) error {
	if !e.propagateEqs() {
		return nil
	}
	r := &e.rows[rid]
	x, y, k, ok := e.isOffsetRow(r)
	if !ok {
		return nil
	}
	if y == SMTNullTheoryVar {
		// x is an implied fixed variable at k.
		key := smtArithFixedKeyOf(&k, e.host.isIntSource(x))
		if x2, found := e.fixedVarTable[key]; found &&
			int(x2) < e.numVars() &&
			e.isFixed(x2) &&
			e.lowerRational(x2).Cmp(&k) == 0 &&
			e.host.isIntSource(x) == e.host.isIntSource(x2) &&
			!e.host.isEqual(x, x2) {
			ante := e.newAntecedents()
			err := e.collectFixedVarJustifications(r, ante)
			if err == nil {
				zero := new(big.Rat)
				err = e.lower(x2).pushJustification(ante, zero, e.proofs)
				if err == nil {
					err = e.upper(x2).pushJustification(ante, zero, e.proofs)
				}
			}
			if err == nil {
				e.stats.fixedEqs++
				err = e.host.propagateEqToCore(x, x2, ante)
			}
			e.releaseAntecedents()
			if err != nil {
				return err
			}
			r = &e.rows[rid]
		}
	}
	if k.Sign() == 0 && y != SMTNullTheoryVar && !e.host.isEqual(x, y) && e.host.isIntSource(x) == e.host.isIntSource(y) {
		// found equality x = y
		ante := e.newAntecedents()
		err := e.collectFixedVarJustifications(r, ante)
		if err == nil {
			e.stats.offsetEqs++
			err = e.host.propagateEqToCore(x, y, ante)
		}
		e.releaseAntecedents()
		if err != nil {
			return err
		}
		r = &e.rows[rid]
	}
	key := smtArithOffsetKey{variable: y, offset: k.RatString()}
	if rowID, found := e.varOffsetToRow[key]; found {
		r2 := &e.rows[rowID]
		if r.base == r2.base {
			return nil // it is the same row
		}
		if r2.base != SMTNullTheoryVar {
			if x2, y2, k2, isOffset := e.isOffsetRow(r2); isOffset {
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
					if !e.host.isEqual(x, x2) && e.host.isIntSource(x) == e.host.isIntSource(x2) {
						ante := e.newAntecedents()
						err := e.collectFixedVarJustifications(r, ante)
						if err == nil {
							err = e.collectFixedVarJustifications(r2, ante)
						}
						if err == nil {
							e.stats.offsetEqs++
							err = e.host.propagateEqToCore(x, x2, ante)
						}
						e.releaseAntecedents()
						if err != nil {
							return err
						}
					}
					return nil
				}
			}
		}
		// the original row was deleted or is not an offset row any more
		delete(e.varOffsetToRow, key)
	}
	e.varOffsetToRow[key] = rid
	return nil
}
