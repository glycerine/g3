package order

import "github.com/glycerine/g3/pkg/util/control"

// Reduce propagates Boolean units in place and renumbers the remaining
// variables by their first occurrence. complete is false on cancellation or
// exhaustion, when callers must discard the partially reduced input.
func (k *BooleanProblem) Reduce(rounds int, limit *control.Reslimit) (conflict, complete bool) {
	size := k.Vars + 1
	v := make([]int8, size)
	set := func(x int32) bool {
		sign := int8(1)
		if x < 0 {
			x = -x
			sign = -1
		}
		if v[x] != 0 && v[x] != sign {
			return false
		}
		v[x] = sign
		return true
	}
	value := func(x int32) int8 {
		if x < 0 {
			return -v[-x]
		}
		return v[x]
	}
	for ci := range k.Clauses {
		if ci%256 == 0 && !limit.IncOffset(uint32(min(256, len(k.Clauses)-ci))) {
			return false, false
		}
		cl := k.Clauses[ci].literals()
		if len(cl) == 1 && !set(cl[0]) {
			return true, true
		}
	}
	for rnd := 0; rnd < rounds; rnd++ {
		progress := false
		// Gates are emitted after their operands. Propagate asserted roots
		// toward the leaves first, then newly fixed operands toward the roots.
		backward := rnd%2 == 0
		for ci := range k.Clauses {
			if ci%256 == 0 && !limit.IncOffset(uint32(min(256, len(k.Clauses)-ci))) {
				return false, false
			}
			if backward {
				ci = len(k.Clauses) - 1 - ci
			}
			cl := k.Clauses[ci].literals()
			var unit int32
			skip := false
			for _, a := range cl {
				val := value(a)
				if val == 1 {
					skip = true
					break
				}
				if val == 0 {
					if unit != 0 {
						skip = true
						break
					}
					unit = a
				}
			}
			if skip {
				continue
			}
			if unit == 0 {
				return true, true
			}
			if !set(unit) {
				return true, true
			}
			progress = true
		}
		if !progress {
			break
		}
	}
	seen := make([]int32, size)
	next := int32(0)
	write := 0
	for ci := range k.Clauses {
		if ci%256 == 0 && !limit.IncOffset(uint32(min(256, len(k.Clauses)-ci))) {
			return false, false
		}
		cl := k.Clauses[ci].literals()
		satisfied := false
		for _, a := range cl {
			if value(a) > 0 {
				satisfied = true
				break
			}
		}
		if satisfied {
			continue
		}
		newcl := cl[:0]
		for _, a := range cl {
			if value(a) != 0 {
				continue
			}
			x := a
			if x < 0 {
				x = -x
			}
			if seen[x] == 0 {
				next++
				seen[x] = next
			}
			a2 := seen[x]
			if a < 0 {
				a2 = -a2
			}
			newcl = append(newcl, a2)
		}
		if len(newcl) == 0 {
			return true, true
		}
		var compact BooleanClause
		copy(compact[:], newcl)
		k.Clauses[write] = compact
		write++
	}
	k.Clauses = k.Clauses[:write]
	k.Vars = int(next)
	return false, true
}
