package order

// After specializing one symmetric witness to the update point, the other
// witness must differ if identifying them makes the whole conclusion true.
// The point-update definitions then give equal pre- and post-state values
// for accesses at the other witness.
func (f *frameMatcher) offPivotEqualities(lemma, goal, other, pivot int32) []int32 {
	t := f.t
	target := t.terms[t.children[t.terms[goal].start]]
	old := t.terms[lemma]
	if target.op != opImplies || old.op != opImplies {
		return nil
	}
	left, right := t.children[old.start+1], t.children[target.start+1]
	p := t.terms[pivot].value
	if !f.reflexiveAt(right, other, p) {
		return nil
	}
	var facts []int32
	witness := int32(-1)
	f.collectEqualities(left, right, other, p, &facts, &witness)
	if witness >= 0 {
		eq := f.booleanTerm(opEqual, pivot, witness)
		facts = append(facts, f.booleanTerm(opNot, eq))
	}
	return facts
}

// Swapping the two constants is the identity in any model that equates them.
func (f *frameMatcher) reflexiveAt(id, a, b int32) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	n := t.terms[id]
	if n.op == opEqual && n.count == 2 {
		return f.swapped(t.children[n.start], t.children[n.start+1], a, b)
	}
	if n.op != opAnd {
		return false
	}
	for _, arg := range t.arguments(&n) {
		if !f.reflexiveAt(arg, a, b) {
			return false
		}
	}
	return true
}

func (f *frameMatcher) collectEqualities(old, updated, other, pivot int32, facts *[]int32, witness *int32) {
	if !f.visit() || len(*facts) >= 12 {
		return
	}
	t := f.t
	x, y := t.terms[old], t.terms[updated]
	if x.op != y.op || x.count != y.count {
		return
	}
	if x.op == opFunction && x.value != y.value {
		for _, u := range f.updates {
			if u.before != x.value || u.after != y.value || t.terms[u.pivot].value != pivot {
				continue
			}
			arg := t.children[y.start+uint32(u.at)]
			v := t.terms[arg]
			if v.op != opFunction || v.count != 0 || v.value != other {
				continue
			}
			for i, c := range t.arguments(&x) {
				if !f.match(c, t.children[y.start+uint32(i)]) {
					return
				}
			}
			for _, eq := range *facts {
				if f.match(updated, t.children[t.terms[eq].start]) {
					return
				}
			}
			*facts = append(*facts, f.booleanTerm(opEqual, updated, old))
			*witness = arg
			return
		}
	}
	for i, c := range t.arguments(&x) {
		f.collectEqualities(c, t.children[y.start+uint32(i)], other, pivot, facts, witness)
	}
}

func (f *frameMatcher) booleanTerm(op operator, args ...int32) int32 {
	t := f.t
	n := expression{op: op, domain: boolean, start: uint32(len(t.children)), count: uint32(len(args))}
	t.children = append(t.children, args...)
	id := int32(len(t.terms))
	t.terms = append(t.terms, n)
	return id
}
