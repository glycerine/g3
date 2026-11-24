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
func (q *GroundInstanceQuery) Reduce(limit *control.Reslimit, split bool) (conflict, complete bool) {
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
	conflict, complete = u.propagate(q.Clauses, mark, limit)
	if !split || conflict || !complete {
		return conflict, complete
	}
	return q.splitGoal(mark, 4, limit)
}

// splitGoal branches on a residual binary clause exposed by the last assignments.
// Only a conflict on both signs proves a goal. Neither branch may leave units
// on the shared trail, even if the private budget expires.
func (q *GroundInstanceQuery) splitGoal(mark, depth int, limit *control.Reslimit) (conflict, complete bool) {
	if depth == 0 {
		return false, true
	}
	candidate, visited := q.splitCandidate(mark)
	u := q.units
	if !limit.IncOffset(uint32(visited + 1)) {
		return false, false
	}
	if candidate == 0 {
		return false, true
	}
	start := len(u.trail)
	for _, lit := range [2]int32{candidate, -candidate} {
		u.assign(lit)
		conflict, complete = u.propagate(q.Clauses, start, limit)
		if complete && !conflict {
			conflict, complete = q.splitGoal(start, depth-1, limit)
		}
		u.undo(start)
		if !complete || !conflict {
			return false, complete
		}
	}
	return true, true
}

// splitCandidate looks at a bounded number of trail positions and clause
// occurrences. Prefer variables that occur with both signs in the premises.
func (q *GroundInstanceQuery) splitCandidate(mark int) (int32, int) {
	var candidate int32
	best, visited := 0, 0
	u := q.units
	const maxVisits = 128
	for i := len(u.trail) - 1; i >= mark && visited < maxVisits; i-- {
		visited++
		affected := u.occurrences[literalSlot(-u.trail[i])]
		for _, id := range affected[:min(len(affected), maxVisits-visited)] {
			visited++
			// This clause contains a false literal and has at most three operands.
			// It has a residual pair exactly when its other two literals are free.
			var pair [2]int32
			count := 0
			for _, lit := range q.Clauses[id].literals() {
				if q.Value(lit) == 0 {
					pair[count] = lit
					count++
				}
			}
			if count != 2 {
				continue
			}
			for _, lit := range pair {
				score := 1 + min(len(u.occurrences[literalSlot(lit)]), len(u.occurrences[literalSlot(-lit)]))
				if score > best {
					best, candidate = score, lit
				}
			}
		}
	}
	return candidate, visited
}
