package order

// In a private refutation, two premises a = E and b = E let every occurrence
// of b use a. Every model of the original premises gives both Booleans the
// same value. Compute substitutions anew for each set of premises. The last
// assertion is the query and cannot supply a definition.
func (t *inference) renderProof(remaining *uint32) Problem {
	t.booleanAliases = nil
	if len(t.assertions) >= 3 {
		m := instanceMatcher{t: t, limit: min(*remaining/16, 2048)}
		m.matchBooleanDefinitions(t.assertions[:len(t.assertions)-1])
		*remaining -= m.work
	}
	return t.render()
}

func (m *instanceMatcher) matchBooleanDefinitions(premises []int32) {
	type definition struct{ symbol, body int32 }
	var seen [8]definition
	count := 0
	t := m.t
	for _, id := range premises {
		if !m.visit() {
			return
		}
		n := &t.terms[id]
		if n.op != opEqual || n.count != 2 {
			continue
		}
		args := t.arguments(n)
		left := t.terms[args[0]]
		if left.op != opFunction || left.domain != boolean || left.count != 0 {
			continue
		}
		matched := false
		for _, previous := range seen[:count] {
			if m.sameTerm(previous.body, args[1]) {
				if previous.symbol != left.value {
					if t.booleanAliases == nil {
						t.booleanAliases = make([]int32, len(t.signatures))
					}
					t.booleanAliases[left.value] = previous.symbol + 1
				}
				matched = true
				break
			}
		}
		if !matched && count < len(seen) {
			seen[count] = definition{left.value, args[1]}
			count++
		}
	}
}
