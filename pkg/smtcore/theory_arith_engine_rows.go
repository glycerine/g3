package smtcore

// Source: src/smt/theory_arith_core.h:792-945 (mk_row, init_row, quasi-base
// rows), src/smt/theory_arith_aux.h:482-510 (get_a_base_row_that_contains)
// and core.h:3315-3520 (scopes, restore_bounds, del_vars, del_row).

import "math/big"

// mkRow creates a new empty row and returns its id.
func (e *smtArithEngine) mkRow() int {
	var r int
	if n := len(e.deadRows); n == 0 {
		r = len(e.rows)
		e.rows = append(e.rows, newSMTArithmeticRow())
	} else {
		r = e.deadRows[n-1]
		e.deadRows = e.deadRows[:n-1]
	}
	e.inToCheck.AssureDomain(uint32(r))
	return r
}

// initRow initializes a new row; its last monomial, which must have
// coefficient 1, owns the row.
func (e *smtArithEngine) initRow(rowID int) error {
	r := &e.rows[rowID]
	if r.size == 0 || r.size != len(r.entries) || r.firstFree != -1 {
		return errSMTArithEngineInvariant
	}
	last := &r.entries[r.size-1]
	if last.coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		return errSMTArithEngineInvariant
	}
	s := last.variable
	r.base = s
	e.data[s].rowID = rowID
	if e.params.LazyPivotingLevel > 2 {
		e.data[s].kind = smtArithmeticQuasiBase
		if err := e.normalizeQuasiBaseRow(rowID); err != nil {
			return err
		}
	} else if err := e.normalizeBaseRow(rowID); err != nil {
		return err
	}
	if e.propagationMode() != smtArithBoundPropNone {
		e.markRowForBoundProp(rowID)
	}
	return nil
}

// collectVars returns the variables of the row with kind k other than the
// row owner, paired with the negated coefficient.
func (e *smtArithEngine) collectVars(rowID int, k smtArithmeticVariableKind) []smtArithLinearMonomial {
	r := &e.rows[rowID]
	base := r.base
	var result []smtArithLinearMonomial
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() && e.data[it.variable].kind == k && it.variable != base {
			var m smtArithLinearMonomial
			m.coefficient.Neg(&it.coefficient)
			m.variable = it.variable
			result = append(result, m)
		}
	}
	return result
}

// normalizeQuasiBaseRow: afterwards the row contains no quasi-base variable
// other than its owner.
func (e *smtArithEngine) normalizeQuasiBaseRow(rowID int) error {
	return e.addRows(rowID, e.collectVars(rowID, smtArithmeticQuasiBase))
}

// quasiBaseRow2BaseRow converts a quasi-base row into a base row.
func (e *smtArithEngine) quasiBaseRow2BaseRow(rowID int) error {
	if err := e.addRows(rowID, e.collectVars(rowID, smtArithmeticBase)); err != nil {
		return err
	}
	s := e.rows[rowID].base
	e.data[s].kind = smtArithmeticBase
	var tmp smtArithmeticValue
	if e.impliedOldValue(s, &tmp) {
		// restore_assignment will restore the old value of the variables the
		// row depends on; s must stay compatible with them (see the source
		// comment at core.h:900-927).
		e.value[s].set(&tmp)
		if e.inUpdateTrailStack.Contains(uint32(s)) {
			return errSMTArithEngineInvariant
		}
		e.saveValue(s)
	}
	e.value[s].set(e.impliedValue(s))
	return nil
}

// normalizeBaseRow: a base row has no quasi-base or base variable other
// than its owner.
func (e *smtArithEngine) normalizeBaseRow(rowID int) error {
	if e.params.LazyPivotingLevel > 0 {
		if err := e.normalizeQuasiBaseRow(rowID); err != nil {
			return err
		}
	}
	return e.quasiBaseRow2BaseRow(rowID)
}

// aBaseRowThatContains is get_a_base_row_that_contains: a column entry of a
// base row containing v, converting quasi-base rows on demand; nil if none.
func (e *smtArithEngine) aBaseRowThatContains(v SMTTheoryVar) (*smtArithmeticColumnEntry, error) {
	for {
		column := &e.columns[v]
		if column.size == 0 {
			return nil, nil
		}
		quasiBaseRow := -1
		for i := range column.entries {
			it := &column.entries[i]
			if it.dead() {
				continue
			}
			if e.isBase(e.rows[it.rowID].base) {
				return it, nil
			} else if quasiBaseRow == -1 {
				quasiBaseRow = it.rowID
			}
		}
		if quasiBaseRow == -1 {
			return nil, errSMTArithEngineInvariant
		}
		if err := e.quasiBaseRow2BaseRow(quasiBaseRow); err != nil {
			return nil, err
		}
	}
}

// ---- scopes (core.h:3315-3372) ----

func (e *smtArithEngine) pushScope() {
	e.scopes = append(e.scopes, smtArithEngineScope{
		numVars:                 uint32(len(e.columns)),
		atomsLim:                uint32(len(e.atoms)),
		boundTrailLim:           uint32(len(e.boundTrail)),
		unassignedAtomsTrailLim: uint32(len(e.unassignedTrail)),
		assertedBoundsLim:       uint32(len(e.assertedBounds)),
		assertedQhead:           e.assertedQhead,
		boundsToDeleteLim:       uint32(len(e.boundsToDelete)),
		nlMonomialsLim:          uint32(len(e.nlMonomials)),
		nlPropagatedLim:         uint32(len(e.nlPropagated)),
	})
}

func (e *smtArithEngine) popScopes(numScopes int) error {
	e.restoreAssignment()
	if err := e.toPatch.Reset(); err != nil {
		return err
	}
	newLevel := len(e.scopes) - numScopes
	if newLevel < 0 {
		return errSMTArithEngineInvariant
	}
	s := e.scopes[newLevel]
	if err := e.restoreBounds(int(s.boundTrailLim)); err != nil {
		return err
	}
	e.restoreUnassignedAtoms(int(s.unassignedAtomsTrailLim))
	e.assertedBounds = e.assertedBounds[:s.assertedBoundsLim]
	e.assertedQhead = s.assertedQhead
	e.restoreNLPropagatedFlag(int(s.nlPropagatedLim))
	e.nlMonomials = e.nlMonomials[:s.nlMonomialsLim]
	e.delAtoms(int(s.atomsLim))
	e.delBounds(int(s.boundsToDeleteLim))
	if err := e.delVars(int(s.numVars)); err != nil {
		return err
	}
	e.scopes = e.scopes[:newLevel]
	ok, err := e.makeFeasible()
	if err != nil {
		return err
	}
	if !ok || !e.toPatch.Empty() {
		return errSMTArithEngineInvariant // VERIFY(make_feasible()) in the source
	}
	e.toCheck = e.toCheck[:0]
	e.inToCheck.Reset()
	e.newAtoms = e.newAtoms[:0]
	return nil
}

func (e *smtArithEngine) restoreNLPropagatedFlag(oldTrailSize int) {
	for i := len(e.nlPropagated) - 1; i >= oldTrailSize; i-- {
		e.data[e.nlPropagated[i]].nonlinearPropagated = false
	}
	e.nlPropagated = e.nlPropagated[:oldTrailSize]
}

func (e *smtArithEngine) restoreBounds(oldTrailSize int) error {
	for i := len(e.boundTrail) - 1; i >= oldTrailSize; i-- {
		t := e.boundTrail[i]
		v := t.variable
		if t.upper {
			e.bounds[1][v] = t.old
		} else {
			e.bounds[0][v] = t.old
		}
		if e.params.LazyPivotingLevel > 2 && t.old == nil && e.isBase(v) && e.isFree(v) {
			if err := e.eliminate(false, v, false); err != nil {
				return err
			}
			if e.columns[v].size != 1 {
				return errSMTArithEngineInvariant
			}
			e.data[v].kind = smtArithmeticQuasiBase
		}
		e.boundTrail[i] = smtArithEngineBoundTrail{}
	}
	e.boundTrail = e.boundTrail[:oldTrailSize]
	return nil
}

func (e *smtArithEngine) restoreUnassignedAtoms(oldTrailSize int) {
	for i := len(e.unassignedTrail) - 1; i >= oldTrailSize; i-- {
		e.unassignedAtoms[e.unassignedTrail[i]]++
	}
	e.unassignedTrail = e.unassignedTrail[:oldTrailSize]
}

func (e *smtArithEngine) delAtoms(oldSize int) {
	for i := len(e.atoms) - 1; i >= oldSize; i-- {
		a := e.atoms[i]
		delete(e.boolVarToAtom, a.boolean)
		occs := e.varOccs[a.variable]
		e.varOccs[a.variable] = occs[:len(occs)-1]
		e.atoms[i] = nil
	}
	e.atoms = e.atoms[:oldSize]
}

func (e *smtArithEngine) delBounds(oldSize int) {
	clear(e.boundsToDelete[oldSize:])
	e.boundsToDelete = e.boundsToDelete[:oldSize]
}

// delVars removes the variables created since the scope with oldNumVars
// variables, deleting or pivoting their rows.
func (e *smtArithEngine) delVars(oldNumVars int) error {
	numVars := len(e.columns)
	if numVars == oldNumVars {
		return nil
	}
	for v := SMTTheoryVar(numVars) - 1; int(v) >= oldNumVars; v-- {
		switch e.varKind(v) {
		case smtArithmeticQuasiBase:
			e.delRow(e.varRow(v))
		case smtArithmeticBase:
			if e.params.LazyPivotingLevel > 0 {
				if err := e.eliminate(false, v, false); err != nil {
					return err
				}
			}
			e.delRow(e.varRow(v))
		case smtArithmeticNonBase:
			entry, err := e.aBaseRowThatContains(v)
			if err != nil {
				return err
			}
			if entry != nil {
				r := &e.rows[entry.rowID]
				coefficient := new(big.Rat).Set(&r.entries[entry.rowIndex].coefficient)
				if err := e.pivot(false, r.base, v, coefficient, false); err != nil {
					return err
				}
				e.delRow(e.varRow(v))
			}
		}
		e.inUpdateTrailStack.Remove(uint32(v))
		e.leftBasis.Remove(uint32(v))
		// The source removes a variable id from the row set m_in_to_check.
		if uint32(v) < e.inToCheck.Domain() {
			e.inToCheck.Remove(uint32(v))
		}
	}
	e.columns = e.columns[:oldNumVars]
	e.data = e.data[:oldNumVars]
	e.value = e.value[:oldNumVars]
	e.oldValue = e.oldValue[:oldNumVars]
	e.varOccs = e.varOccs[:oldNumVars]
	e.unassignedAtoms = e.unassignedAtoms[:oldNumVars]
	e.varPos = e.varPos[:oldNumVars]
	e.bounds[0] = e.bounds[0][:oldNumVars]
	e.bounds[1] = e.bounds[1][:oldNumVars]
	return nil
}

func (e *smtArithEngine) delRow(rowID int) {
	r := &e.rows[rowID]
	for i := range r.entries {
		it := &r.entries[i]
		if !it.dead() {
			_ = e.columns[it.variable].deleteEntry(it.columnIndex)
		}
	}
	r.base = SMTNullTheoryVar
	r.reset()
	e.deadRows = append(e.deadRows, rowID)
}

// addVariable is the engine half of mk_var: it extends every per-variable
// array and returns the new theory variable. pureMonomial records the
// variable as a nonlinear monomial.
func (e *smtArithEngine) addVariable(isInt, pureMonomial bool) (SMTTheoryVar, error) {
	r := SMTTheoryVar(len(e.columns))
	e.columns = append(e.columns, newSMTArithmeticColumn())
	e.data = append(e.data, smtArithmeticVariableData{integer: isInt})
	var value smtArithmeticValue
	if e.params.RandomInitialValue {
		// unsigned arithmetic, as in the source
		span := uint32(e.params.RandomUpper - e.params.RandomLower)
		val := e.random.Next()%span + uint32(e.params.RandomLower)
		value.rational.SetInt64(int64(val))
	}
	e.value = append(e.value, value)
	e.oldValue = append(e.oldValue, smtArithmeticValue{})
	e.varOccs = append(e.varOccs, nil)
	e.unassignedAtoms = append(e.unassignedAtoms, 0)
	e.varPos = append(e.varPos, -1)
	e.bounds[0] = append(e.bounds[0], nil)
	e.bounds[1] = append(e.bounds[1], nil)
	if int32(r) >= e.toPatch.Bounds() {
		if err := e.toPatch.SetBounds(int32(r) + 1); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	e.inUpdateTrailStack.AssureDomain(uint32(r))
	e.leftBasis.AssureDomain(uint32(r))
	e.inToCheck.AssureDomain(uint32(r))
	if pureMonomial {
		e.nlMonomials = append(e.nlMonomials, r)
	}
	return r, nil
}
