package order

// A total point update with one equation for all of its arguments.
type updateDefinition struct {
	index int
	old   int32
}

// In a branch where every function retains its old interpretation, the
// ground induction instance contradicts the negated updated goal. When all
// definitions have the same condition for a possible change, assert that
// condition and keep only the active arms of those definitions.
func (f *frameMatcher) requireChangedUpdates(facts []int32) []int32 {
	t := f.t
	terms, children := len(t.terms), len(t.children)
	on, off := f.booleanTerm(opTrue), f.booleanTerm(opFalse)
	guard := int32(-1)
	rewritten := make([]int32, 0, len(f.definitions))
	for _, def := range f.definitions {
		q := t.terms[t.assertions[def.index]]
		eq := t.terms[q.value]
		args := t.arguments(&eq)
		head := t.terms[args[0]]
		g, body := f.activeUpdate(args[1], def.old, t.arguments(&head), on, off)
		if g == on || g == off || (guard >= 0 && !f.sameTerm(guard, g)) {
			break
		}
		guard = g
		q.value = f.booleanTerm(opEqual, args[0], body)
		rewritten = append(rewritten, int32(len(t.terms)))
		t.terms = append(t.terms, q)
	}
	if guard < 0 || len(rewritten) != len(f.definitions) || f.work == f.limit {
		t.terms, t.children = t.terms[:terms], t.children[:children]
		return facts
	}
	for i, def := range f.definitions {
		t.assertions[def.index] = rewritten[i]
	}
	return append(facts, guard)
}

// The guard is false only when the RHS is exactly old(formals). A
// nonconstant or argument-dependent condition remains in the active body.
func (f *frameMatcher) activeUpdate(id, old int32, params []int32, on, off int32) (int32, int32) {
	if !f.visit() {
		return on, id
	}
	t := f.t
	n := t.terms[id]
	if f.oldApplication(n, params, old) {
		return off, id
	}
	if n.op != opIte {
		return on, id
	}
	args := t.arguments(&n)
	test := t.terms[args[0]]
	if test.op != opFunction || test.count != 0 || test.domain != boolean {
		return on, id
	}
	left, yes := f.activeUpdate(args[1], old, params, on, off)
	right, no := f.activeUpdate(args[2], old, params, on, off)
	guard := f.updateChoice(args[0], left, right)
	if left == off {
		return guard, no
	}
	if right == off {
		return guard, yes
	}
	return guard, f.updateChoice(args[0], yes, no)
}

func (f *frameMatcher) updateChoice(test, yes, no int32) int32 {
	if f.sameTerm(yes, no) {
		return yes
	}
	t := f.t
	y, n := t.terms[yes], t.terms[no]
	if y.op == opTrue && n.op == opFalse {
		return test
	}
	if y.op == opFalse && n.op == opTrue {
		return f.booleanTerm(opNot, test)
	}
	node := expression{op: opIte, domain: y.domain, start: uint32(len(t.children)), count: 3}
	t.children = append(t.children, test, yes, no)
	id := int32(len(t.terms))
	t.terms = append(t.terms, node)
	return id
}
