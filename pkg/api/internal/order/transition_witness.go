package order

// The reader emits each child before its parent and reads the assertions in
// order. The prefix through the preceding root therefore covers every
// occurrence in the premises, including occurrences under binders.
func (f *frameMatcher) private(a, b int32) bool {
	t := f.t
	end := t.assertions[len(t.assertions)-2]
	for _, n := range t.terms[:end+1] {
		if !f.visit() || n.op == opFunction && (n.value == a || n.value == b) {
			return false
		}
	}
	return true
}

func (f *frameMatcher) flatten(id int32, op operator, out *[]int32) bool {
	if !f.visit() {
		return false
	}
	n := f.t.terms[id]
	if n.op != op {
		if len(*out) >= 32 {
			return false
		}
		*out = append(*out, id)
		return true
	}
	for _, x := range f.t.arguments(&n) {
		if !f.flatten(x, op, out) {
			return false
		}
	}
	return true
}

// Compare the left term after exchanging two constants with the right term.
// Equality is commutative. Conjunction and disjunction also associate.
func (f *frameMatcher) swapped(left, right, a, b int32) bool {
	if !f.visit() {
		return false
	}
	t := f.t
	x, y := t.terms[left], t.terms[right]
	if x.op != y.op || t.root(x.domain) != t.root(y.domain) || x.op == opForall || x.op == opExists {
		return false
	}
	if x.op == opFunction {
		switch x.value {
		case a:
			x.value = b
		case b:
			x.value = a
		}
	}
	if x.value != y.value {
		return false
	}
	if x.op == opAnd || x.op == opOr {
		var xs, ys []int32
		if !f.flatten(left, x.op, &xs) || !f.flatten(right, x.op, &ys) || len(xs) != len(ys) {
			return false
		}
		for _, i := range xs {
			found := false
			for k, j := range ys {
				if f.swapped(i, j, a, b) {
					ys[k] = ys[len(ys)-1]
					ys = ys[:len(ys)-1]
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	if x.count != y.count {
		return false
	}
	xs, ys := t.arguments(&x), t.arguments(&y)
	if x.op == opEqual && x.count == 2 && f.swapped(xs[0], ys[1], a, b) && f.swapped(xs[1], ys[0], a, b) {
		return true
	}
	for i, id := range xs {
		if !f.swapped(id, ys[i], a, b) {
			return false
		}
	}
	return true
}

// Copy only paths to a replaced constant. The lemma can share ground leaves
// with the original premises, which must retain their original symbols.
func (f *frameMatcher) replace(id, from, pivot int32) (int32, bool) {
	if !f.visit() {
		return id, false
	}
	t := f.t
	n := t.terms[id]
	if n.op == opFunction && n.count == 0 && n.value == from {
		return pivot, true
	}
	args := t.arguments(&n)
	var changed bool
	for i, arg := range args {
		newArg, ok := f.replace(arg, from, pivot)
		if !ok {
			return id, false
		}
		if newArg != arg {
			if !changed {
				changed = true
				n.start = uint32(len(t.children))
				t.children = append(t.children, args...)
			}
			t.children[n.start+uint32(i)] = newArg
		}
	}
	if changed {
		id = int32(len(t.terms))
		t.terms = append(t.terms, n)
	}
	return id, true
}
