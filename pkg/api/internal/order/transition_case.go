package order

// A changed predicate can account for a counterexample in one direction.
// For a positive occurrence the old atom was true and the new atom is false.
// Negation and the antecedent of an implication reverse that direction.
type changedAtom struct {
	before, after int32
	positive      bool
}

type transitionCase struct {
	premises        []int32
	primaryPremises []int32
	negatedFacts    int32
	specialized     int32
}

// In a single model, suppose the ground old property holds, its new version
// fails, and all directional changes except one have been ruled out. Then
// the remaining atom must differ. If it is a point update, each argument
// outside which the definition agrees must equal its pivot. Only the checked
// implications can authorize substituting these pivots into the goal.
func (m *inductionMatcher) proposeCase(lemma, goal int32, steps uint32) uint32 {
	if m.changed < 2 {
		return 0
	}
	t := m.t
	f := frameMatcher{instanceMatcher: instanceMatcher{t: t, limit: min(steps, 8192)}}
	var atoms []changedAtom
	if !f.directionalChanges(lemma, t.children[t.terms[goal].start], true, &atoms) || len(atoms) < 2 {
		return f.work
	}
	for i, a := range atoms {
		before, after := t.terms[a.before], t.terms[a.after]
		if after.count < 2 {
			continue
		}
		f.updates = f.updates[:0]
		if !f.findUpdates(functionPair{before.value, after.value}) || len(f.updates) < 2 {
			continue
		}
		premises, nested := f.casePremises(atoms, i)
		if !nested {
			continue
		}
		var facts []int32
		for j, other := range atoms {
			if j == i {
				continue
			}
			x, y := other.before, other.after
			if !other.positive {
				x, y = y, x
			}
			facts = append(facts, f.booleanTerm(opImplies, x, y))
		}
		negated := f.booleanTerm(opNot, f.booleanTerm(opAnd, facts...))
		args := append([]int32{goal, lemma}, facts...)
		replaced := 0
		for _, update := range f.updates {
			w := t.terms[t.children[after.start+uint32(update.at)]]
			if w.op != opFunction || w.count != 0 || w.value == t.terms[update.pivot].value {
				continue
			}
			for j, arg := range args {
				next, ok := f.replace(arg, w.value, update.pivot)
				if !ok {
					return f.work
				}
				args[j] = next
			}
			replaced++
		}
		if replaced >= 2 {
			c := &transitionCase{premises: premises, negatedFacts: negated, specialized: f.booleanTerm(opAnd, args...)}
			f.liftCase(c, facts, args[0])
			t.transition = c
			break
		}
	}
	return f.work
}

func (f *frameMatcher) directionalChanges(left, right int32, positive bool, out *[]changedAtom) bool {
	if !f.visit() {
		return false
	}
	if left == right || f.match(left, right) {
		return true
	}
	t := f.t
	x, y := t.terms[left], t.terms[right]
	if x.op != y.op || x.count != y.count || t.root(x.domain) != t.root(y.domain) {
		return false
	}
	switch x.op {
	case opFunction:
		if x.value == y.value || x.domain != boolean || len(*out) >= 8 {
			return false
		}
		for i, arg := range t.arguments(&x) {
			if !f.match(arg, t.children[y.start+uint32(i)]) {
				return false
			}
		}
		*out = append(*out, changedAtom{left, right, positive})
	case opNot, opImplies, opAnd, opOr:
		for i, arg := range t.arguments(&x) {
			sign := positive
			if x.op == opNot || x.op == opImplies && i == 0 {
				sign = !sign
			}
			if !f.directionalChanges(arg, t.children[y.start+uint32(i)], sign, out) {
				return false
			}
		}
	default:
		return false
	}
	return true
}

func containsFunction(functions []int32, id int32) bool {
	for _, f := range functions {
		if f == id {
			return true
		}
	}
	return false
}

// A defined predicate containing a quantifier makes direct comparison of
// the two states expensive. Try a separate proof using ground guards, the
// definitions in the proposed implications, and short related axioms. A
// missing premise can only make that proof inconclusive.
func (f *frameMatcher) casePremises(atoms []changedAtom, point int) ([]int32, bool) {
	t := f.t
	var before, after []int32
	for i, a := range atoms {
		if i != point {
			before = append(before, t.terms[a.before].value)
			after = append(after, t.terms[a.after].value)
		}
	}
	var out []int32
	hasQuantifiedDefinition := false
	for _, id := range t.assertions[:len(t.assertions)-1] {
		if !f.visit() {
			return nil, false
		}
		q := t.terms[id]
		if q.op != opForall && q.op != opExists {
			out = append(out, id)
			continue
		}
		body := t.terms[q.value]
		if q.op == opForall && body.op == opEqual && body.count == 2 {
			lhs, rhs := t.terms[t.children[body.start]], t.terms[t.children[body.start+1]]
			if lhs.op == opFunction && (containsFunction(before, lhs.value) || containsFunction(after, lhs.value)) {
				if containsFunction(after, lhs.value) && (rhs.op == opExists || rhs.op == opForall) {
					hasQuantifiedDefinition = true
				}
				out = append(out, id)
				continue
			}
		}
		var storage [2]int32
		names := storage[:0]
		var related bool
		if f.caseSymbols(q.value, before, after, &names, &related) && (related || len(names) == 0) {
			out = append(out, id)
		}
	}
	return out, hasQuantifiedDefinition
}

func (f *frameMatcher) caseSymbols(id int32, before, after []int32, names *[]int32, related *bool) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	v := t.terms[id]
	if v.op == opExists || v.op == opForall {
		return f.caseSymbols(v.value, before, after, names, related)
	}
	if v.op == opFunction {
		*related = *related || containsFunction(before, v.value)
		if !containsFunction(after, v.value) && !containsFunction(*names, v.value) {
			if len(*names) == 2 {
				return false
			}
			*names = append(*names, v.value)
		}
	}
	for _, arg := range t.arguments(&v) {
		if !f.caseSymbols(arg, before, after, names, related) {
			return false
		}
	}
	return true
}
