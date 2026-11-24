package order

import "github.com/glycerine/g3/pkg/util/control"

// The trail between queries contains only assignments implied by permanent
// clauses. Each literal's occurrence list grows as new instances arrive.
type instanceUnits struct {
	unitAssignments
	occurrences [][]int32
	indexed     int
	conflict    bool
}

func (q *GroundInstanceQuery) PermanentUnits() []int32 { return q.units.trail }

// Value returns the sign of the permanent assignment to a literal, or zero
// when that literal is still free.
func (q *GroundInstanceQuery) Value(lit int32) int8 {
	if lit < 0 {
		return -q.units.values[-lit]
	}
	return q.units.values[lit]
}

func (u *instanceUnits) propagate(rows []BooleanClause, head int, limit *control.Reslimit) (conflict, complete bool) {
	for ; head < len(u.trail); head++ {
		affected := u.occurrences[literalSlot(-u.trail[head])]
		for i, id := range affected {
			if i%256 == 0 && !limit.IncOffset(uint32(min(256, len(affected)-i))) {
				return false, false
			}
			unit, active := unitClause(rows[id].literals(), u.values)
			if active && (unit == 0 || !u.assign(unit)) {
				return true, true
			}
		}
	}
	return false, true
}

// Reduce first propagates newly installed permanent clauses and then tries
// the temporary goal. On interruption the caller discards any SAT state
// that was synchronized against the previous permanent trail.
func (q *GroundInstanceQuery) Reduce(limit *control.Reslimit) (conflict, complete bool) {
	u := q.units
	if u.conflict {
		return true, true
	}
	if n := q.Variables + 1 - len(u.values); n > 0 {
		u.values = append(u.values, make([]int8, n)...)
		u.occurrences = append(u.occurrences, make([][]int32, 2*n)...)
	}
	head := len(u.trail)
	for i := u.indexed; i < len(q.Clauses); i++ {
		if (i-u.indexed)%256 == 0 && !limit.IncOffset(uint32(min(256, len(q.Clauses)-i))) {
			*u = instanceUnits{}
			return false, false
		}
		row := q.Clauses[i].literals()
		if len(row) > 1 {
			for _, lit := range row {
				slot := literalSlot(lit)
				u.occurrences[slot] = append(u.occurrences[slot], int32(i))
			}
		}
		unit, active := unitClause(row, u.values)
		if active && (unit == 0 || !u.assign(unit)) {
			u.conflict = true
			return true, true
		}
	}
	u.indexed = len(q.Clauses)
	conflict, complete = u.propagate(q.Clauses, head, limit)
	if !complete {
		*u = instanceUnits{}
		return false, false
	}
	if conflict {
		u.conflict = true
		return true, true
	}
	mark := len(u.trail)
	if !u.assign(q.Goal) {
		return true, true
	}
	defer u.undo(mark)
	return u.propagate(q.Clauses, mark, limit)
}
