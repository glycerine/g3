package order

// A definition forall x. p(x) = body(x), with the variables in declaration
// order. This makes two definitions comparable binder by binder.
type predicateDef struct {
	assertion int32
	binder    expression
	head      expression
	body      int32
}

func (f *frameMatcher) definedPredicate(premises []int32, symbol int32) (predicateDef, bool) {
	t := f.t
	for _, id := range premises {
		if !f.visit() {
			break
		}
		q := t.terms[id]
		if q.op != opForall {
			continue
		}
		eq := t.terms[q.value]
		if eq.op != opEqual || eq.count != 2 {
			continue
		}
		head := t.terms[t.children[eq.start]]
		if head.op != opFunction || head.value != symbol || head.domain != boolean || head.count != q.count {
			continue
		}
		valid := true
		for i, arg := range t.arguments(&head) {
			v := t.terms[arg]
			if v.op != opVariable || v.value != int32(q.start)+int32(i) {
				valid = false
				break
			}
		}
		if valid {
			return predicateDef{id, q, head, t.children[eq.start+1]}, true
		}
	}
	return predicateDef{}, false
}

type binderPair struct{ left, right, count int32 }

// Compare two formulas under the pointwise implication leaf.premise =>
// leaf.goal. direction is +1 for left => right, -1 for right => left, and
// zero where the two subterms must be identical modulo bound-variable names.
func (f *frameMatcher) impliesUnderInclusion(left, right int32, direction int, leaf functionPair, bound []binderPair) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	x, y := t.terms[left], t.terms[right]
	if x.op != y.op || x.count != y.count || t.root(x.domain) != t.root(y.domain) {
		return false
	}
	switch x.op {
	case opVariable:
		for _, p := range bound {
			i := x.value - p.left
			if 0 <= i && i < p.count {
				return y.value == p.right+i
			}
		}
		return false
	case opForall, opExists:
		return f.underBinders(x, y, x.value, y.value, direction, leaf, bound)
	case opFunction:
		if x.value != y.value {
			a, b := x.value, y.value
			if direction < 0 {
				a, b = b, a
			}
			if direction == 0 || a != leaf.premise || b != leaf.goal {
				return false
			}
		}
	default:
		if x.value != y.value {
			return false
		}
	}
	for i, arg := range t.arguments(&x) {
		d := 0
		switch x.op {
		case opAnd, opOr:
			d = direction
		case opNot:
			d = -direction
		case opImplies:
			d = direction
			if i == 0 {
				d = -d
			}
		}
		if !f.impliesUnderInclusion(arg, t.children[y.start+uint32(i)], d, leaf, bound) {
			return false
		}
	}
	return true
}

// Equal-length binders pair their variables by position, including nested
// binders whose declaration IDs differ between the two definitions.
func (f *frameMatcher) underBinders(x, y expression, left, right int32, direction int, leaf functionPair, bound []binderPair) bool {
	t := f.t
	for i := uint32(0); i < x.count; i++ {
		if !f.visit() || t.root(t.declarations[x.start+i].domain) != t.root(t.declarations[y.start+i].domain) {
			return false
		}
	}
	pair := binderPair{int32(x.start), int32(y.start), int32(x.count)}
	return f.impliesUnderInclusion(left, right, direction, leaf, append(bound, pair))
}

// Suppose one required implication is P(a) => Q(a), and the other follows
// from forall x. P(x) => Q(x) by monotonicity of two quantified definitions.
// The private solver can establish both by refuting the negated universal.
// Only the independent private UNSAT result authorizes specialization.
func (f *frameMatcher) liftCase(premises, facts []int32) ([]int32, int32, bool) {
	if len(facts) != 2 {
		return nil, 0, false
	}
	t := f.t
	for i, fact := range facts {
		direct := t.arguments(&t.terms[fact])
		leaf := functionPair{t.terms[direct[0]].value, t.terms[direct[1]].value}
		wrapped := t.arguments(&t.terms[facts[1-i]])
		x, ok := f.definedPredicate(premises, t.terms[wrapped[0]].value)
		if !ok {
			continue
		}
		y, ok := f.definedPredicate(premises, t.terms[wrapped[1]].value)
		nested := t.terms[x.body].op
		if !ok || x.binder.count != y.binder.count || (nested != opExists && nested != opForall) {
			continue
		}
		if !f.underBinders(x.binder, y.binder, x.body, y.body, 1, leaf, nil) {
			continue
		}
		target, ok := f.definedPredicate(premises, leaf.goal)
		if !ok {
			continue
		}
		base := target.head
		base.value = leaf.premise
		q := target.binder
		n := int32(len(t.terms))
		t.terms = append(t.terms, base, target.head)
		// Both applications reuse the formal arguments of the leaf definition.
		q.value = f.booleanTerm(opImplies, n, n+1)
		n = int32(len(t.terms))
		t.terms = append(t.terms, q)
		neg := f.booleanTerm(opNot, n)
		kept := make([]int32, 0, len(premises)-2)
		for _, id := range premises {
			if id != x.assertion && id != y.assertion {
				kept = append(kept, id)
			}
		}
		return kept, neg, true
	}
	return nil, 0, false
}
