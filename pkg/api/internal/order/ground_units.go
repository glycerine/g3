package order

import "github.com/glycerine/g3/pkg/ast"

// Each signed literal has a contiguous list of non-unit clauses that contain
// it. Assigning its negation can turn any of those clauses into a unit.
type groundUnits struct {
	clauses              []BooleanClause
	offsets, occurrences []int32
	unitAssignments
}

type unitAssignments struct {
	values []int8
	trail  []int32
}

func literalSlot(lit int32) int {
	if lit < 0 {
		return int(-2*lit + 1)
	}
	return int(2 * lit)
}
func (u *unitAssignments) assign(lit int32) bool {
	v := lit
	sign := int8(1)
	if v < 0 {
		v = -v
		sign = -1
	}
	if u.values[v] != 0 {
		return u.values[v] == sign
	}
	u.values[v] = sign
	u.trail = append(u.trail, lit)
	return true
}
func (u *unitAssignments) undo(mark int) {
	for _, lit := range u.trail[mark:] {
		if lit < 0 {
			lit = -lit
		}
		u.values[lit] = 0
	}
	u.trail = u.trail[:mark]
}

func (u *groundUnits) prepare(m *ast.ASTManager, vars int, budget *int) (conflict, complete bool) {
	// Two clause passes and one offset pass build the index.
	cost := 2*len(u.clauses) + 2*(vars+1)
	if cost > *budget {
		return false, false
	}
	*budget -= cost
	u.values = make([]int8, vars+1)
	offsets := make([]int32, 2*len(u.values)+1)
	for i := range u.clauses {
		if i%256 == 0 && m.ResourceLimit().CancelFlag() {
			return false, false
		}
		row := u.clauses[i].literals()
		switch len(row) {
		case 0:
			return true, true
		case 1:
			if !u.assign(row[0]) {
				return true, true
			}
		default:
			for _, x := range row {
				offsets[literalSlot(x)]++
			}
		}
	}
	for i := 1; i < len(offsets); i++ {
		offsets[i] += offsets[i-1]
	}
	u.occurrences = make([]int32, offsets[len(offsets)-1])
	// Fill backwards from the end of each range. The resulting starts are
	// the offsets used by propagation, so a separate cursor array is unnecessary.
	for i := len(u.clauses) - 1; i >= 0; i-- {
		if i%256 == 0 && m.ResourceLimit().CancelFlag() {
			return false, false
		}
		row := u.clauses[i].literals()
		if len(row) < 2 {
			continue
		}
		for _, x := range row {
			idx := literalSlot(x)
			offsets[idx]--
			u.occurrences[offsets[idx]] = int32(i)
		}
	}
	u.offsets = offsets
	return u.propagate(m, 0, budget)
}
func (u *groundUnits) propagate(m *ast.ASTManager, head int, budget *int) (conflict, complete bool) {
	for head < len(u.trail) {
		if m.ResourceLimit().CancelFlag() {
			return false, false
		}
		slot := literalSlot(-u.trail[head])
		head++
		rows := u.occurrences[u.offsets[slot]:u.offsets[slot+1]]
		for i, idx := range rows {
			if i%256 == 0 && m.ResourceLimit().CancelFlag() {
				return false, false
			}
			if *budget <= 0 {
				return false, false
			}
			*budget--
			unit, active := unitClause(u.clauses[idx].literals(), u.values)
			if active && (unit == 0 || !u.assign(unit)) {
				return true, true
			}
		}
	}
	return false, true
}
