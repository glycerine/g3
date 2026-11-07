package smtcore

// Source: src/smt/theory_arith_core.h:2545-3050 (bound propagation) and
// src/smt/theory_arith_aux.h:806-885 (derived bounds). dump_lemmas is a
// diagnostic that prints a lemma as an SMT problem (arith.dump_lemmas); it
// is not ported.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// smtRatIsBig is rational::is_big: the numerator or denominator does not
// fit the small (machine int) representation.
func smtRatIsBig(q *big.Rat) bool {
	small := func(x *big.Int) bool {
		return x.IsInt64() && x.Int64() >= math.MinInt32 && x.Int64() <= math.MaxInt32
	}
	return !small(q.Num()) || !small(q.Denom())
}

// markRowForBoundProp marks row r1 for bound propagation.
func (e *smtArithEngine) markRowForBoundProp(r1 int) {
	if !e.inToCheck.Contains(uint32(r1)) && e.rows[r1].base != SMTNullTheoryVar {
		e.inToCheck.Insert(uint32(r1))
		e.toCheck = append(e.toCheck, uint32(r1))
	}
}

// markRowsForBoundProp marks every row that contains v.
func (e *smtArithEngine) markRowsForBoundProp(v SMTTheoryVar) {
	column := &e.columns[v]
	for i := range column.entries {
		if !column.entries[i].dead() {
			e.markRowForBoundProp(column.entries[i].rowID)
		}
	}
}

// isRowUsefulForBoundProp computes lower_idx and upper_idx: >= 0 the single
// monomial for which the row can imply a bound, -1 every monomial, -2 none.
func (e *smtArithEngine) isRowUsefulForBoundProp(r *smtArithmeticRow) (lowerIdx, upperIdx int) {
	lowerIdx, upperIdx = -1, -1
	update := func(idx *int, i int) {
		if *idx == -1 {
			*idx = i
		} else {
			*idx = -2
		}
	}
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		if e.params.SkipRowsWithBigCoeffs && smtRatIsBig(&it.coefficient) {
			return -2, -2
		}
		isPos := it.coefficient.Sign() > 0
		if e.lower(it.variable) == nil {
			if isPos {
				update(&upperIdx, i)
			} else {
				update(&lowerIdx, i)
			}
		}
		if e.upper(it.variable) == nil {
			if isPos {
				update(&lowerIdx, i)
			} else {
				update(&upperIdx, i)
			}
		}
		if lowerIdx == -2 && upperIdx == -2 {
			return
		}
	}
	return
}

// useUpperBound reports which bound of a monomial with coefficient c is
// used when implying a lower (isLower) or upper bound.
func smtArithUseUpper(c *big.Rat, isLower bool) bool {
	if isLower {
		return c.Sign() > 0
	}
	return c.Sign() < 0
}

// implyBoundForMonomial implies a bound for the monomial at idx from the
// others, and from it a bound for the monomial variable.
func (e *smtArithEngine) implyBoundForMonomial(r *smtArithmeticRow, idx int, isLower bool) error {
	entry := &r.entries[idx]
	if e.unassignedAtoms[entry.variable] == 0 {
		return nil
	}
	var impliedK smtArithmeticValue
	for idx2 := range r.entries {
		it := &r.entries[idx2]
		if it.dead() || idx == idx2 {
			continue
		}
		b := e.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		if b == nil {
			return errSMTArithEngineInvariant
		}
		impliedK.subMul(&it.coefficient, &b.core().value)
	}
	impliedK.quoRat(&entry.coefficient)
	if (entry.coefficient.Sign() > 0) == isLower {
		// impliedK is a lower bound for entry.variable
		curr := e.lower(entry.variable)
		if curr == nil || impliedK.cmp(&curr.core().value) > 0 {
			return e.mkImpliedBound(r, idx, isLower, entry.variable, smtArithmeticLowerBound, &impliedK)
		}
		return nil
	}
	curr := e.upper(entry.variable)
	if curr == nil || impliedK.cmp(&curr.core().value) < 0 {
		return e.mkImpliedBound(r, idx, isLower, entry.variable, smtArithmeticUpperBound, &impliedK)
	}
	return nil
}

// implyBoundForAllMonomials implies a lower (upper) bound for every
// monomial in the row, using one pass to compute the common sum.
func (e *smtArithEngine) implyBoundForAllMonomials(r *smtArithmeticRow, isLower bool) error {
	var bb smtArithmeticValue
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() {
			b := e.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
			bb.subMul(&it.coefficient, &b.core().value)
		}
	}
	var impliedK smtArithmeticValue
	for idx := range r.entries {
		it := &r.entries[idx]
		if it.dead() || e.unassignedAtoms[it.variable] == 0 {
			continue
		}
		b := e.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		impliedK.set(&bb)
		impliedK.addMul(&it.coefficient, &b.core().value)
		impliedK.quoRat(&it.coefficient)
		if (it.coefficient.Sign() > 0) == isLower {
			curr := e.lower(it.variable)
			if curr == nil || impliedK.cmp(&curr.core().value) > 0 {
				if err := e.mkImpliedBound(r, idx, isLower, it.variable, smtArithmeticLowerBound, &impliedK); err != nil {
					return err
				}
			}
		} else {
			curr := e.upper(it.variable)
			if curr == nil || impliedK.cmp(&curr.core().value) < 0 {
				if err := e.mkImpliedBound(r, idx, isLower, it.variable, smtArithmeticUpperBound, &impliedK); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// explainBound creates the explanation for the bound of the monomial at idx.
// delta relaxes the explanation: the implied bound may be delta weaker.
// isLower says whether the other monomials imply a lower (upper) bound.
func (e *smtArithEngine) explainBound(r *smtArithmeticRow, idx int, isLower bool, delta *smtArithmeticValue, ante *smtArithmeticAntecedents) error {
	if delta.sign() < 0 {
		return errSMTArithEngineInvariant
	}
	if !e.relaxBounds() && (len(ante.literals) > 0 || len(ante.equalities) > 0) {
		return nil
	}
	proofs := e.coeffsEnabled()
	if e.relaxBounds() {
		// a delta change of the variable at idx is a |coeff|*delta change of
		// its monomial.
		coeff := new(big.Rat).Abs(&r.entries[idx].coefficient)
		delta.mulRat(coeff)
	}
	for idx2 := range r.entries {
		it := &r.entries[idx2]
		if it.dead() || idx == idx2 {
			continue
		}
		b := e.getBound(it.variable, smtArithUseUpper(&it.coefficient, isLower))
		if b == nil {
			return errSMTArithEngineInvariant
		}
		if !e.relaxBounds() || delta.isZero() {
			if err := b.pushJustification(ante, &it.coefficient, proofs); err != nil {
				return err
			}
			continue
		}
		coeff := new(big.Rat).Abs(&it.coefficient)
		invCoeff := new(big.Rat).Inv(coeff)
		bc := b.core()
		isBLower := bc.kind == smtArithmeticLowerBound
		var k1, limitK1, k2 smtArithmeticValue
		k1.set(&bc.value)
		limitK1.set(&k1)
		// if the max decrease (increase) of the current monomial is delta,
		// the maximal decrease (increase) of the variable is delta/|coeff|.
		if isBLower {
			limitK1.subMul(invCoeff, delta)
		} else {
			limitK1.addMul(invCoeff, delta)
		}
		k2.set(&k1)
		var newAtom *smtArithmeticAtom
		for _, a := range e.varOccs[it.variable] {
			if smtArithEngineBound(a) == b {
				continue
			}
			val := e.host.assignment(a.boolean)
			if val == lbool.Undef {
				continue
			}
			if err := a.assign(val == lbool.True, e.epsilon(a.variable)); err != nil {
				return err
			}
			if a.kind == bc.kind {
				if isBLower {
					if a.value.cmp(&limitK1) >= 0 && a.value.cmp(&k2) < 0 {
						k2.set(&a.value)
						newAtom = a
					}
				} else if a.value.cmp(&limitK1) <= 0 && a.value.cmp(&k2) > 0 {
					k2.set(&a.value)
					newAtom = a
				}
			}
		}
		if newAtom == nil {
			if err := b.pushJustification(ante, coeff, proofs); err != nil {
				return err
			}
			continue
		}
		var diff smtArithmeticValue
		if isBLower {
			diff.set(&k1)
			diff.sub(&k2)
		} else {
			diff.set(&k2)
			diff.sub(&k1)
		}
		delta.subMul(coeff, &diff)
		if err := newAtom.pushJustification(ante, coeff, proofs); err != nil {
			return err
		}
	}
	return nil
}

// mkImpliedBound assigns the atoms of v that the implied bound k decides.
func (e *smtArithEngine) mkImpliedBound(r *smtArithmeticRow, idx int, isLower bool, v SMTTheoryVar, kind smtArithmeticBoundKind, k *smtArithmeticValue) error {
	epsilon := e.epsilon(v)
	var delta smtArithmeticValue
	// assign may re-enter assignBoundLiteral only through the host, which
	// does not modify varOccs; index to tolerate growth anyway.
	for i := 0; i < len(e.varOccs[v]); i++ {
		a := e.varOccs[v][i]
		bv := a.boolean
		l := NewSMTLiteral(bv, false)
		if e.host.assignment(bv) != lbool.Undef {
			continue
		}
		k2 := &a.threshold
		delta.reset()
		if a.atomKind == smtArithmeticLowerAtom {
			// v >= k, k >= k2 |- v >= k2
			if kind == smtArithmeticLowerBound && k.cmp(k2) >= 0 {
				if e.relaxBounds() {
					delta.set(k)
					delta.sub(k2)
				}
				if err := e.assignBoundLiteral(l, r, idx, isLower, &delta); err != nil {
					return err
				}
			}
			// v <= k, k < k2 |- v < k2 |- not v >= k2
			if kind == smtArithmeticUpperBound && k.cmp(k2) < 0 {
				// k < k2 is not enough: with k = -1/5*eps and k2 = 0,
				// v <= -1/5*eps means not v >= 0, i.e. v <= -eps.
				delta.set(k2)
				delta.sub(k)
				delta.sub(epsilon)
				if delta.sign() >= 0 {
					if err := e.assignBoundLiteral(l.Negated(), r, idx, isLower, &delta); err != nil {
						return err
					}
				}
			}
		} else {
			// v >= k, k > k2 |- v > k2 |- not v <= k2
			if kind == smtArithmeticLowerBound && k.cmp(k2) > 0 {
				delta.set(k)
				delta.sub(k2)
				delta.sub(epsilon)
				if delta.sign() >= 0 {
					if err := e.assignBoundLiteral(l.Negated(), r, idx, isLower, &delta); err != nil {
						return err
					}
				}
			}
			// v <= k, k <= k2 |- v <= k2
			if kind == smtArithmeticUpperBound && k.cmp(k2) <= 0 {
				if e.relaxBounds() {
					delta.set(k2)
					delta.sub(k)
				}
				if err := e.assignBoundLiteral(l, r, idx, isLower, &delta); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// assignBoundLiteral assigns l, justified by the explanation of the bound
// at idx: as a small auxiliary lemma, or as a theory propagation.
func (e *smtArithEngine) assignBoundLiteral(l SMTLiteral, r *smtArithmeticRow, idx int, isLower bool, delta *smtArithmeticValue) error {
	e.stats.boundProps++
	ante := e.newAntecedents()
	defer e.releaseAntecedents()
	if err := e.explainBound(r, idx, isLower, delta, ante); err != nil {
		return err
	}
	if uint32(len(ante.literals)) < e.params.SmallLemmaSize && len(ante.equalities) == 0 {
		lits := make([]SMTLiteral, 0, 1+len(ante.literals))
		lits = append(lits, l)
		for _, a := range ante.literals {
			lits = append(lits, a.Negated())
		}
		return e.host.mkAuxLemma(lits, ante)
	}
	return e.host.assignBoundLiteral(l, ante)
}

// propagateBounds traverses the rows in to_check and derives improved bounds.
func (e *smtArithEngine) propagateBounds() error {
	for i := 0; i < len(e.toCheck); i++ {
		r := &e.rows[e.toCheck[i]]
		if r.base == SMTNullTheoryVar {
			continue
		}
		if uint32(r.size) < e.params.MaxLemmaSize { // ignore big rows
			lowerIdx, upperIdx := e.isRowUsefulForBoundProp(r)
			var err error
			if lowerIdx >= 0 {
				err = e.implyBoundForMonomial(r, lowerIdx, true)
			} else if lowerIdx == -1 {
				err = e.implyBoundForAllMonomials(r, true)
			}
			if err != nil {
				return err
			}
			if upperIdx >= 0 {
				err = e.implyBoundForMonomial(r, upperIdx, false)
			} else if upperIdx == -1 {
				err = e.implyBoundForAllMonomials(r, false)
			}
			if err != nil {
				return err
			}
			// cheap equality detection piggybacks on this loop
			if err = e.propagateCheapEq(int(e.toCheck[i])); err != nil {
				return err
			}
		}
	}
	e.toCheck = e.toCheck[:0]
	e.inToCheck.Reset()
	return nil
}

// ---- derived bounds (aux.h:806-885) ----

// normalizeBound rounds the bound of an integer variable inward.
func (e *smtArithEngine) normalizeBound(v SMTTheoryVar, k *smtArithmeticValue, kind smtArithmeticBoundKind) (smtArithmeticValue, error) {
	if e.isReal(v) {
		var r smtArithmeticValue
		r.set(k)
		return r, nil
	}
	return smtNormalizeArithmeticIntegerBound(k, kind)
}

// newDerivedBound allocates the derived (or justified derived) bound that
// rows and nonlinear reasoning install, and registers it for deletion on
// backtracking.
func (e *smtArithEngine) newDerivedBound(v SMTTheoryVar, value *smtArithmeticValue, kind smtArithmeticBoundKind) (smtArithEngineBound, smtArithmeticDerivedTarget) {
	if e.proofs {
		b := &smtArithmeticJustifiedDerivedBound{}
		b.variable, b.kind = v, kind
		b.value.set(value)
		e.boundsToDelete = append(e.boundsToDelete, b)
		e.assertedBounds = append(e.assertedBounds, b)
		return b, b
	}
	b := &smtArithmeticDerivedBound{}
	b.variable, b.kind = v, kind
	b.value.set(value)
	e.boundsToDelete = append(e.boundsToDelete, b)
	e.assertedBounds = append(e.assertedBounds, b)
	return b, b
}

// accumulateJustification copies the justification of b to target; only
// literals and equalities not already in the sets are copied.
func (e *smtArithEngine) accumulateJustification(b smtArithEngineBound, target smtArithmeticDerivedTarget, coeff *big.Rat) error {
	return smtAccumulateArithmeticJustification(b, target, coeff, e.proofs, e.tmpLitSet, e.tmpEqSet)
}

// mkBoundFromRow creates a derived bound for v, explained by the row.
func (e *smtArithEngine) mkBoundFromRow(v SMTTheoryVar, k *smtArithmeticValue, kind smtArithmeticBoundKind, r *smtArithmeticRow) error {
	kNorm, err := e.normalizeBound(v, k, kind)
	if err != nil {
		return err
	}
	_, target := e.newDerivedBound(v, &kNorm, kind)
	clear(e.tmpLitSet)
	clear(e.tmpEqSet)
	for i := range r.entries {
		it := &r.entries[i]
		if it.dead() {
			continue
		}
		useUpper := kind == smtArithmeticUpperBound
		if it.coefficient.Sign() <= 0 {
			useUpper = !useUpper
		}
		b := e.getBound(it.variable, useUpper)
		if b == nil {
			return errSMTArithEngineInvariant
		}
		if err := e.accumulateJustification(b, target, &it.coefficient); err != nil {
			return err
		}
	}
	return nil
}
