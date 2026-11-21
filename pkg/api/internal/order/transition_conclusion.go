package order

// After the side proof establishes the ground implications between two
// quantified predicates, their definitions can be omitted from a weaker
// primary query. Supply two instances of a universal premise whose conclusion
// equates applications of the predicate required false by the goal. Each
// instance fixes the arguments on one side of the equality.
func (f *frameMatcher) conclusionPremises(goal, first, second int32) []int32 {
	t := f.t
	neg := t.terms[goal]
	if neg.op != opNot {
		return nil
	}
	body := t.terms[t.children[neg.start]]
	if body.op != opImplies {
		return nil
	}
	target := t.children[body.start+1]
	if t.terms[target].op != opFunction {
		return nil
	}
	premises := t.assertions[:len(t.assertions)-1]
	for _, id := range premises {
		if !f.visit() {
			break
		}
		q := t.terms[id]
		if q.op != opForall || q.count < 2 {
			continue
		}
		rule := t.terms[q.value]
		if rule.op != opImplies {
			continue
		}
		a, b, ok := f.conclusionInstances(q, t.children[rule.start+1], target)
		if !ok {
			continue
		}
		projected := make([]int32, 0, len(premises))
		for _, p := range premises {
			if p != first && p != second {
				projected = append(projected, p)
			}
		}
		return append(projected, a, b)
	}
	return nil
}

func (f *frameMatcher) conclusionInstances(q expression, id, target int32) (int32, int32, bool) {
	if !f.visit() {
		return 0, 0, false
	}
	t := f.t
	n := t.terms[id]
	if n.op == opAnd {
		for _, child := range t.arguments(&n) {
			if a, b, ok := f.conclusionInstances(q, child, target); ok {
				return a, b, true
			}
		}
	}
	if n.op != opEqual || n.count != 2 {
		return 0, 0, false
	}
	x, y := t.children[n.start], t.children[n.start+1]
	symbol := t.terms[target].value
	for _, side := range [2]int32{x, y} {
		app := t.terms[side]
		if app.op != opFunction || app.value != symbol {
			return 0, 0, false
		}
	}
	if f.sameTerm(x, y) {
		return 0, 0, false
	}
	a, ok := f.fixArguments(q, x, target)
	if !ok {
		return 0, 0, false
	}
	b, ok := f.fixArguments(q, y, target)
	return a, b, ok
}

// Substituting ground terms under the original quantifier preserves its
// remaining variables. The SMT simplifier discards the unused binders.
func (f *frameMatcher) fixArguments(q expression, pattern, target int32) (int32, bool) {
	t := f.t
	if f.bindings == nil {
		f.bindings = make([]int32, len(t.declarations))
	}
	defer func() {
		for _, v := range f.touched {
			f.bindings[v] = 0
		}
		f.touched = f.touched[:0]
	}()
	if !f.match(pattern, target) || len(f.touched) == 0 || len(f.touched) >= int(q.count) {
		return 0, false
	}
	id, ok := f.substitute(q.value, 0, 0, f.bindings)
	if !ok {
		return 0, false
	}
	q.value = id
	id = int32(len(t.terms))
	t.terms = append(t.terms, q)
	return id, true
}
