package order

// A negated property after a transition can have the same structure as a
// universal premise, with some functions replaced by their updated versions.
// Matching those versions suggests ground values for the premise's binders.
// The lemma uses the original premise and substitutes only its variables.
type inductionMatcher struct {
	instanceMatcher
	functions []functionPair
	changed   int
}

type functionPair struct{ premise, goal int32 }

func (t *inference) addInductionInstance(steps uint32) uint32 {
	last := len(t.assertions) - 1
	if last < 1 {
		return 0
	}
	goal := t.assertions[last]
	n := t.terms[goal]
	if n.op != opNot {
		return 0
	}
	target := t.children[n.start]
	if t.terms[target].op != opImplies {
		return 0
	}
	m := inductionMatcher{instanceMatcher: instanceMatcher{t: t, limit: min(steps, 4096)}}
	if !m.ground(target) {
		return m.work
	}
	for _, id := range t.assertions[:last] {
		if !m.visit() {
			break
		}
		q := t.terms[id]
		if q.op != opForall || q.count < 4 || t.terms[q.value].op != opImplies {
			continue
		}
		if m.bindings == nil {
			m.bindings = make([]int32, len(t.declarations))
		}
		if m.similar(q.value, target) && len(m.touched) == int(q.count) &&
			m.changed > 0 && m.changed < len(m.touched) {
			lemma := m.instantiate(q.value)
			var spent uint32
			lemma, goal, spent = m.specialize(lemma, goal, steps-m.work)
			m.work += spent
			start := uint32(len(t.children))
			t.children = append(t.children, goal, lemma)
			t.assertions[last] = int32(len(t.terms))
			t.terms = append(t.terms, expression{op: opAnd, domain: boolean, start: start, count: 2})
			break
		}
		for _, v := range m.touched {
			m.bindings[v] = 0
		}
		m.touched = m.touched[:0]
		m.functions = m.functions[:0]
		m.changed = 0
	}
	return m.work
}

func (m *inductionMatcher) similar(pattern, actual int32) bool {
	if !m.visit() {
		return false
	}
	t := m.t
	x, y := t.terms[pattern], t.terms[actual]
	if t.root(x.domain) != t.root(y.domain) {
		return false
	}
	if x.op == opVariable {
		return m.match(pattern, actual)
	}
	if x.op != y.op || x.count != y.count || x.op == opForall || x.op == opExists {
		return false
	}
	if x.op == opFunction {
		if !m.correspond(x.value, y.value) {
			return false
		}
	} else if x.value != y.value {
		return false
	}
	for i, child := range t.arguments(&x) {
		if !m.similar(child, t.children[y.start+uint32(i)]) {
			return false
		}
	}
	return true
}

func (m *inductionMatcher) correspond(premise, goal int32) bool {
	for _, p := range m.functions {
		if p.premise == premise || p.goal == goal {
			return p.premise == premise && p.goal == goal
		}
	}
	m.functions = append(m.functions, functionPair{premise, goal})
	if premise != goal {
		m.changed++
	}
	return true
}

func (m *inductionMatcher) instantiate(id int32) int32 {
	t := m.t
	n := t.terms[id]
	if n.op == opVariable {
		return m.bindings[n.value] - 1
	}
	if n.count == 0 {
		return id
	}
	args := t.arguments(&n)
	n.start = uint32(len(t.children))
	t.children = append(t.children, make([]int32, n.count)...)
	for i, arg := range args {
		child := m.instantiate(arg)
		t.children[n.start+uint32(i)] = child
	}
	id = int32(len(t.terms))
	t.terms = append(t.terms, n)
	return id
}
