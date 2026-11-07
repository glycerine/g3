package smtcore

// Source: src/smt/theory_arith_core.h:959-1205 (bound axioms),
// 1209-1273 (internalize_atom, engine half), 1310-1318 (assign_eh),
// 1498-1555 (can_propagate, propagate_core, failed).

import "sort"

// addAtom registers the atom  v <kind> k  for bool var bv: the engine half
// of internalize_atom. k is already rounded for integer variables.
func (e *smtArithEngine) addAtom(bv SMTBoolVar, v SMTTheoryVar, k *smtArithmeticValue, kind smtArithmeticAtomKind) (*smtArithmeticAtom, error) {
	a, err := newSMTArithmeticAtom(bv, v, k, kind)
	if err != nil {
		return nil, err
	}
	if err = e.mkBoundAxioms(a); err != nil {
		return nil, err
	}
	e.unassignedAtoms[v]++
	e.varOccs[v] = append(e.varOccs[v], a)
	e.atoms = append(e.atoms, a)
	e.boolVarToAtom[bv] = a
	return a, nil
}

// assignEh records that bool var bv was assigned; atoms are asserted later,
// from propagate.
func (e *smtArithEngine) assignEh(bv SMTBoolVar, isTrue bool) error {
	a := e.boolVarToAtom[bv]
	if a == nil {
		return nil
	}
	if err := a.assign(isTrue, e.epsilon(a.variable)); err != nil {
		return err
	}
	e.assertedBounds = append(e.assertedBounds, a)
	return nil
}

// processAtoms is the adaptive switch (arith.adaptive).
func (e *smtArithEngine) processAtoms(totalConflicts uint32) bool {
	if !e.params.Adaptive {
		return true
	}
	if totalConflicts < 10 {
		return true
	}
	f := float64(e.numConflicts) / float64(totalConflicts)
	return f >= e.params.AdaptiveAssertionThreshold
}

func (e *smtArithEngine) canPropagate(totalConflicts uint32) bool {
	return e.processAtoms(totalConflicts) && int(e.assertedQhead) < len(e.assertedBounds)
}

// failed restores the last feasible assignment after a conflict.
func (e *smtArithEngine) failed() error {
	e.restoreAssignment()
	if err := e.toPatch.Reset(); err != nil {
		return err
	}
	e.toCheck = e.toCheck[:0]
	e.inToCheck.Reset()
	return nil
}

// propagateCore asserts the pending bounds, makes the tableau feasible and
// propagates bounds. It reports false when a conflict was set.
func (e *smtArithEngine) propagateCore() (bool, error) {
	if err := e.flushBoundAxioms(); err != nil {
		return false, err
	}
	if ok, err := e.propagateLinearMonomials(); err != nil || !ok {
		return false, err
	}
	for int(e.assertedQhead) < len(e.assertedBounds) {
		b := e.assertedBounds[e.assertedQhead]
		e.assertedQhead++
		ok, err := e.assertBound(b)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, e.failed()
		}
	}
	ok, err := e.makeFeasible()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, e.failed()
	}
	if e.host.cancelled() {
		return true, nil
	}
	e.discardUpdateTrail()
	return true, e.propagateBounds()
}

// ---- bound axioms ----

func (e *smtArithEngine) mkBoundAxioms(a1 *smtArithmeticAtom) error {
	if !e.host.isSearching() {
		// The user is assumed to propagate before pushing internal scopes,
		// which flushes newly asserted atoms into the right context.
		e.newAtoms = append(e.newAtoms, a1)
		return nil
	}
	occs := e.varOccs[a1.variable]
	k1, kind1 := &a1.threshold, a1.atomKind
	loInf, loSup, hiInf, hiSup := -1, -1, -1, -1
	for i, a2 := range occs {
		k2, kind2 := &a2.threshold, a2.atomKind
		if k1.cmp(k2) == 0 && kind1 == kind2 {
			continue
		}
		if kind2 == smtArithmeticLowerAtom {
			if k2.cmp(k1) < 0 {
				if loInf == -1 || k2.cmp(&occs[loInf].threshold) > 0 {
					loInf = i
				}
			} else if loSup == -1 || k2.cmp(&occs[loSup].threshold) < 0 {
				loSup = i
			}
		} else if k2.cmp(k1) < 0 {
			if hiInf == -1 || k2.cmp(&occs[hiInf].threshold) > 0 {
				hiInf = i
			}
		} else if hiSup == -1 || k2.cmp(&occs[hiSup].threshold) < 0 {
			hiSup = i
		}
	}
	for _, i := range []int{loInf, loSup, hiInf, hiSup} {
		if i != -1 {
			if err := e.mkBoundAxiom(a1, occs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *smtArithEngine) mkBoundAxiom(a1, a2 *smtArithmeticAtom) error {
	l1, l2 := NewSMTLiteral(a1.boolean, false), NewSMTLiteral(a2.boolean, false)
	k1, k2 := &a1.threshold, &a2.threshold
	kind1, kind2 := a1.atomKind, a2.atomKind
	vIsInt := e.isInt(a1.variable)
	if k1.cmp(k2) == 0 && kind1 == kind2 {
		return nil
	}
	clause := func(a, b SMTLiteral) error { return e.host.mkTheoryAxiom([]SMTLiteral{a, b}) }
	var one smtArithmeticValue
	one.rational.SetInt64(1)
	plusOne := func(x *smtArithmeticValue) *smtArithmeticValue {
		var r smtArithmeticValue
		r.set(x)
		r.add(&one)
		return &r
	}
	minusOne := func(x *smtArithmeticValue) *smtArithmeticValue {
		var r smtArithmeticValue
		r.set(x)
		r.sub(&one)
		return &r
	}
	switch {
	case kind1 == smtArithmeticLowerAtom:
		if kind2 == smtArithmeticLowerAtom {
			if k2.cmp(k1) <= 0 {
				return clause(l1.Negated(), l2)
			}
			return clause(l1, l2.Negated())
		}
		if k1.cmp(k2) <= 0 {
			// k1 <= k2, k1 <= x or x <= k2
			return clause(l1, l2)
		}
		// k1 > hi_inf, k1 <= x => ~(x <= hi_inf)
		if err := clause(l1.Negated(), l2.Negated()); err != nil {
			return err
		}
		if vIsInt && k1.cmp(plusOne(k2)) == 0 {
			// k1 <= x or x <= k1-1
			return clause(l1, l2)
		}
	case kind2 == smtArithmeticLowerAtom:
		if k1.cmp(k2) >= 0 {
			// k1 >= lo_inf, k1 >= x or lo_inf <= x
			return clause(l1, l2)
		}
		// k1 < k2, k2 <= x => ~(x <= k1)
		if err := clause(l1.Negated(), l2.Negated()); err != nil {
			return err
		}
		if vIsInt && k1.cmp(minusOne(k2)) == 0 {
			// x <= k1 or k1+1 <= x
			return clause(l1, l2)
		}
	default:
		// both upper
		if k1.cmp(k2) >= 0 {
			// k1 >= k2, x <= k2 => x <= k1
			return clause(l1, l2.Negated())
		}
		// k1 <= hi_sup, x <= k1 => x <= hi_sup
		return clause(l1.Negated(), l2)
	}
	return nil
}

func smtArithFirstAtom(kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, from int) int {
	for i := from; i < len(occs); i++ {
		if occs[i].atomKind == kind {
			return i
		}
	}
	return len(occs)
}

func smtArithNextInf(a1 *smtArithmeticAtom, kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, from int) (int, bool) {
	result, found := len(occs), false
	for i := from; i < len(occs); i++ {
		a2 := occs[i]
		if a1 == a2 || a2.atomKind != kind {
			continue
		}
		found = true
		if a2.threshold.cmp(&a1.threshold) <= 0 {
			result = i
		} else {
			break
		}
	}
	return result, found
}

func smtArithNextSup(a1 *smtArithmeticAtom, kind smtArithmeticAtomKind, occs []*smtArithmeticAtom, from int) (int, bool) {
	found := false
	for i := from; i < len(occs); i++ {
		a2 := occs[i]
		if a1 == a2 || a2.atomKind != kind {
			continue
		}
		found = true
		if a1.threshold.cmp(&a2.threshold) < 0 {
			return i, true
		}
	}
	return len(occs), found
}

// flushBoundAxioms creates the bound axioms of atoms registered before the
// search started, per variable, against the sorted atoms of the variable.
// std::sort is unstable in the source; atoms with equal thresholds keep
// their creation order here.
func (e *smtArithEngine) flushBoundAxioms() error {
	for len(e.newAtoms) > 0 {
		last := len(e.newAtoms) - 1
		atoms := []*smtArithmeticAtom{e.newAtoms[last]}
		e.newAtoms = e.newAtoms[:last]
		v := atoms[0].variable
		for i := 0; i < len(e.newAtoms); i++ {
			if e.newAtoms[i].variable == v {
				atoms = append(atoms, e.newAtoms[i])
				e.newAtoms[i] = e.newAtoms[len(e.newAtoms)-1]
				e.newAtoms = e.newAtoms[:len(e.newAtoms)-1]
				i--
			}
		}
		occs := append([]*smtArithmeticAtom(nil), e.varOccs[v]...)
		byK := func(s []*smtArithmeticAtom) func(i, j int) bool {
			return func(i, j int) bool { return s[i].threshold.cmp(&s[j].threshold) < 0 }
		}
		sort.SliceStable(atoms, byK(atoms))
		sort.SliceStable(occs, byK(occs))
		end := len(occs)
		begin1 := smtArithFirstAtom(smtArithmeticLowerAtom, occs, 0)
		begin2 := smtArithFirstAtom(smtArithmeticUpperAtom, occs, 0)
		loInf, loSup, hiInf, hiSup := begin1, begin1, begin2, begin2
		visited := make(map[*smtArithmeticAtom]bool)
		for _, a1 := range atoms {
			loInf1, floInf := smtArithNextInf(a1, smtArithmeticLowerAtom, occs, loInf)
			hiInf1, fhiInf := smtArithNextInf(a1, smtArithmeticUpperAtom, occs, hiInf)
			loSup1, floSup := smtArithNextSup(a1, smtArithmeticLowerAtom, occs, loSup)
			hiSup1, fhiSup := smtArithNextSup(a1, smtArithmeticUpperAtom, occs, hiSup)
			if loInf1 != end {
				loInf = loInf1
			}
			if loSup1 != end {
				loSup = loSup1
			}
			if hiInf1 != end {
				hiInf = hiInf1
			}
			if hiSup1 != end {
				hiSup = hiSup1
			}
			if !floInf {
				loInf = end
			}
			if !fhiInf {
				hiInf = end
			}
			if !floSup {
				loSup = end
			}
			if !fhiSup {
				hiSup = end
			}
			visited[a1] = true
			for _, c := range []struct{ one, cur int }{{loInf1, loInf}, {loSup1, loSup}, {hiInf1, hiInf}, {hiSup1, hiSup}} {
				if c.one != end && c.cur != end && !visited[occs[c.cur]] {
					if err := e.mkBoundAxiom(a1, occs[c.cur]); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// propagateLinearMonomials belongs to the nonlinear phase (theory_arith_nl.h).
func (e *smtArithEngine) propagateLinearMonomials() (bool, error) { return true, nil }
