package order

// An assertion and the negation of one of its universal instances cannot
// both hold. Look for required ground subterms of the last assertion, then
// try the opposite requirements from the premises. Each attempt has its own
// bindings. An unsuccessful search has no effect on the auxiliary solver.
type instanceMatcher struct {
	t        *inference
	limit    uint32
	work     uint32
	targets  map[instanceHead][]int32
	bindings []int32 // ground term ID + 1, or zero for an unbound variable
	touched  []int32
	refuted  bool
}

type instanceHead struct {
	op       operator
	positive bool
	arity    uint32
	symbol   int32
}

func (t *inference) matchFinalAssertion(steps uint32) (refuted bool, work uint32) {
	last := len(t.assertions) - 1
	if last < 1 {
		return false, 0
	}
	m := instanceMatcher{
		t: t, limit: min(steps, 32768),
		targets: make(map[instanceHead][]int32),
	}
	m.requirements(t.assertions[last], true, false)
	if len(m.targets) > 0 {
		m.bindings = make([]int32, len(t.declarations))
		for _, id := range t.assertions[:last] {
			m.requirements(id, true, true)
			if m.refuted || m.work == m.limit {
				break
			}
		}
	}
	return m.refuted, m.work
}

func (m *instanceMatcher) visit() bool {
	if m.work == m.limit || m.refuted {
		return false
	}
	if m.work%256 == 0 && m.t.m.ResourceLimit().CancelFlag() {
		m.limit = m.work
		return false
	}
	m.work++
	return true
}

// Descend only through connectives that require a child's signed value.
// In premises a positive forall or negative exists requires its signed body
// for every assignment. Targets must be ground and independent of binders.
func (m *instanceMatcher) requirements(id int32, positive, premise bool) {
	if !m.visit() {
		return
	}
	t := m.t
	n := &t.terms[id]
	switch n.op {
	case opForall, opExists:
		if premise && positive == (n.op == opForall) {
			m.requirements(n.value, positive, premise)
		}
		return
	case opNot:
		m.requirements(t.children[n.start], !positive, premise)
		return
	case opVariable:
		return
	}
	head := instanceHead{op: n.op, arity: n.count, positive: positive}
	switch n.op {
	case opNumeral, opOrder, opFunction:
		head.symbol = n.value
	}
	if premise {
		head.positive = !positive
		for _, target := range m.targets[head] {
			if m.match(id, target) {
				m.refuted = true
				return
			}
			for _, v := range m.touched {
				m.bindings[v] = 0
			}
			m.touched = m.touched[:0]
			if m.work == m.limit {
				return
			}
		}
	} else if m.ground(id) {
		m.targets[head] = append(m.targets[head], id)
	}
	if positive && n.op == opAnd || !positive && n.op == opOr {
		for _, child := range t.arguments(n) {
			m.requirements(child, positive, premise)
			if m.refuted || m.work == m.limit {
				return
			}
		}
	} else if !positive && n.op == opImplies {
		m.requirements(t.children[n.start], true, premise)
		m.requirements(t.children[n.start+1], false, premise)
	}
}

func (m *instanceMatcher) ground(id int32) bool {
	if !m.visit() {
		return false
	}
	n := &m.t.terms[id]
	switch n.op {
	case opVariable, opForall, opExists:
		return false
	}
	for _, child := range m.t.arguments(n) {
		if !m.ground(child) {
			return false
		}
	}
	return true
}

func (m *instanceMatcher) match(pattern, target int32) bool {
	if !m.visit() {
		return false
	}
	t := m.t
	x, y := &t.terms[pattern], &t.terms[target]
	if t.root(x.domain) != t.root(y.domain) {
		return false
	}
	if x.op == opVariable {
		if old := m.bindings[x.value]; old != 0 {
			// The earlier binding is ground, so this comparison cannot bind.
			return m.match(old-1, target)
		}
		m.bindings[x.value] = target + 1
		m.touched = append(m.touched, x.value)
		return true
	}
	if pattern == target {
		return true
	}
	if x.op != y.op || x.count != y.count || x.value != y.value {
		return false
	}
	for i, child := range t.arguments(x) {
		if !m.match(child, t.children[y.start+uint32(i)]) {
			return false
		}
	}
	return true
}

func (m *instanceMatcher) sameTerm(left, right int32) bool {
	if left == right {
		return true
	}
	if !m.visit() {
		return false
	}
	t := m.t
	x, y := t.terms[left], t.terms[right]
	if x.op != y.op || x.value != y.value || x.count != y.count || t.root(x.domain) != t.root(y.domain) ||
		x.op == opForall || x.op == opExists {
		return false
	}
	for i, child := range t.arguments(&x) {
		if !m.sameTerm(child, t.children[y.start+uint32(i)]) {
			return false
		}
	}
	return true
}
