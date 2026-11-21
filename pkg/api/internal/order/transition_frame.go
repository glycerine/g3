package order

// A point update agrees with the original function whenever argument "at"
// differs from pivot. Remember the two declarations and the constant term
// witnessing this frame condition.
type pointUpdate struct {
	before, after int32
	at            int
	pivot         int32
}

type frameMatcher struct {
	instanceMatcher
	updates []pointUpdate
}

// Specialize a ground counterexample to an argument changed by the transition.
// If every changed application is outside its update, the old property and
// new property coincide, contradicting the ground lemma. A counterexample
// therefore puts at least one of the collected witnesses at the pivot.
func (m *inductionMatcher) specialize(lemma, goal int32, steps uint32) (int32, int32, uint32) {
	f := frameMatcher{instanceMatcher: instanceMatcher{t: m.t, limit: min(steps, 16384)}}
	for _, pair := range m.functions {
		if pair.premise != pair.goal && !f.findUpdates(pair) {
			return lemma, goal, f.work
		}
	}
	target := f.t.children[f.t.terms[goal].start]
	for _, update := range f.updates {
		if update.before != f.updates[0].before || !f.visit() {
			break
		}
		var witnesses []int32
		if !f.collect(lemma, target, update.pivot, &witnesses) || len(witnesses) == 0 {
			continue
		}
		from := witnesses[0]
		if len(witnesses) == 2 {
			other := witnesses[1]
			if !f.private(from, other) || !f.swapped(lemma, lemma, from, other) ||
				!f.swapped(target, target, from, other) {
				continue
			}
			// Either witness may be chosen: exchanging their interpretations
			// preserves the premises and both ground formulas.
			from = max(from, other)
		}
		// For a single witness the equality follows directly. Substitution
		// in the last assertion preserves a model even for a public symbol.
		oldLemma, ok := f.replace(lemma, from, update.pivot)
		if ok {
			if newGoal, ok := f.replace(goal, from, update.pivot); ok {
				return oldLemma, newGoal, f.work
			}
		}
		break
	}
	return lemma, goal, f.work
}

func (f *frameMatcher) findUpdates(pair functionPair) bool {
	t := f.t
	for _, id := range t.assertions[:len(t.assertions)-1] {
		if !f.visit() {
			return false
		}
		q := t.terms[id]
		if q.op != opForall {
			continue
		}
		body := t.terms[q.value]
		if body.op != opEqual || body.count != 2 {
			continue
		}
		lhs, rhs := t.terms[t.children[body.start]], t.children[body.start+1]
		if lhs.op != opFunction || lhs.value != pair.goal || lhs.count != q.count ||
			t.terms[rhs].op != opIte {
			continue
		}
		// Require a universally defined function of distinct variables in
		// declaration order. This also excludes a restricted or aliased LHS.
		args := t.arguments(&lhs)
		valid := true
		for i, id := range args {
			p := t.terms[id]
			if p.op != opVariable || p.value != int32(q.start)+int32(i) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		start := len(f.updates)
		f.guards(rhs, pair, args, start)
		i := start
		for _, candidate := range f.updates[start:] {
			if f.outside(rhs, args, candidate) {
				f.updates[i] = candidate
				i++
			}
		}
		f.updates = f.updates[:i]
		if i > start {
			return true
		}
	}
	return false
}

// Collect candidates only from conditions of the update's ITE tree. A
// candidate is used only after outside proves the complete definition.
func (f *frameMatcher) guards(id int32, p functionPair, args []int32, start int) {
	if !f.visit() {
		return
	}
	t := f.t
	n := t.terms[id]
	switch n.op {
	case opIte, opAnd:
		for _, child := range t.arguments(&n) {
			f.guards(child, p, args, start)
		}
	case opEqual:
		cs := t.arguments(&n)
		if len(cs) != 2 {
			return
		}
		for j, v := range cs {
			param, pivotID := t.terms[v], cs[1-j]
			pivot := t.terms[pivotID]
			if param.op != opVariable || pivot.op != opFunction || pivot.count != 0 || pivot.domain == boolean {
				continue
			}
			at := int(param.value - t.terms[args[0]].value)
			if at < 0 || at >= len(args) {
				continue
			}
			for _, u := range f.updates[start:] {
				if u.at == at && t.terms[u.pivot].value == pivot.value {
					return
				}
			}
			if len(f.updates)-start < 8 {
				f.updates = append(f.updates, pointUpdate{p.premise, p.goal, at, pivotID})
			}
		}
	}
}

func (f *frameMatcher) guardFalse(id, param, pivot int32) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	n := t.terms[id]
	switch n.op {
	case opAnd:
		for _, c := range t.arguments(&n) {
			if f.guardFalse(c, param, pivot) {
				return true
			}
		}
	case opEqual:
		if n.count == 2 {
			a, b := t.terms[t.children[n.start]], t.terms[t.children[n.start+1]]
			if b.op == opVariable {
				a, b = b, a
			}
			return a.op == opVariable && a.value == param && b.op == opFunction && b.count == 0 && b.value == pivot
		}
	}
	return false
}

func (f *frameMatcher) outside(id int32, params []int32, u pointUpdate) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	n := t.terms[id]
	if n.op == opIte {
		args := t.arguments(&n)
		// The else arm must agree even when the guard is undecided.
		return f.outside(args[2], params, u) && (f.guardFalse(args[0], t.terms[params[u.at]].value, t.terms[u.pivot].value) ||
			f.outside(args[1], params, u))
	}
	if n.op != opFunction || n.value != u.before || int(n.count) != len(params) {
		return false
	}
	for i, child := range t.arguments(&n) {
		v := t.terms[child]
		if v.op != opVariable || v.value != t.terms[params[i]].value {
			return false
		}
	}
	return true
}

func (f *frameMatcher) collect(old, next, pivot int32, witnesses *[]int32) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	x, y := t.terms[old], t.terms[next]
	if x.op != y.op || x.count != y.count || x.op == opForall || x.op == opExists {
		return false
	}
	if x.value != y.value {
		if x.op != opFunction {
			return false
		}
		found := false
		for _, u := range f.updates {
			if u.before != x.value || u.after != y.value || t.terms[u.pivot].value != t.terms[pivot].value {
				continue
			}
			w := t.terms[t.children[y.start+uint32(u.at)]]
			if w.op != opFunction || w.count != 0 || w.value == t.terms[pivot].value || t.root(w.domain) != t.root(t.terms[pivot].domain) {
				continue
			}
			if len(*witnesses) == 0 || (*witnesses)[0] != w.value && (len(*witnesses) < 2 || (*witnesses)[1] != w.value) {
				if len(*witnesses) == 2 {
					return false
				}
				*witnesses = append(*witnesses, w.value)
			}
			found = true
			break
		}
		if !found {
			return false
		}
	}
	for i, a := range t.arguments(&x) {
		if !f.collect(a, t.children[y.start+uint32(i)], pivot, witnesses) {
			return false
		}
	}
	return true
}
