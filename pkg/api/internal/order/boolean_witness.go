package order

import "github.com/glycerine/g3/pkg/util/control"

// These bounds keep the two propagation passes on the caller's stack.
const (
	witnessMaxVars    = 256
	witnessMaxClauses = 512
)

type witnessOccurrences struct {
	first [2*witnessMaxVars + 1]int16
	next  [3*witnessMaxClauses + 1]int16
}

// HasSatisfyingAssignment tries positive decisions in forward and reverse
// variable order on a reduced projection. A conflict abandons that order.
// Only a complete assignment satisfying every clause can bypass SAT search.
func (k *BooleanProblem) HasSatisfyingAssignment(limit *control.Reslimit) bool {
	if k.Vars <= 0 || k.Vars > witnessMaxVars || len(k.Clauses) == 0 || len(k.Clauses) > witnessMaxClauses {
		return false
	}
	var occurrences witnessOccurrences
	for i := range k.Clauses {
		if i%64 == 0 && !limit.IncOffset(uint32(min(64, len(k.Clauses)-i))) {
			return false
		}
		row := k.Clauses[i].literals()
		if len(row) < 2 {
			return false
		}
		for j, lit := range row {
			if lit == 0 || lit < -int32(k.Vars) || lit > int32(k.Vars) {
				return false
			}
			// The occurrence encodes the clause index and fits in int16.
			id := int16(1 + 3*i + j)
			slot := witnessMaxVars + lit
			occurrences.next[id] = occurrences.first[slot]
			occurrences.first[slot] = id
		}
	}
	return occurrences.try(k, limit, false) || limit.IncOffset(0) && occurrences.try(k, limit, true)
}

func (o *witnessOccurrences) try(k *BooleanProblem, limit *control.Reslimit, reverse bool) bool {
	var values [witnessMaxVars + 1]int8
	var queue [witnessMaxVars]int32
	read, end := 0, 0
	for decision := 1; decision <= k.Vars; decision++ {
		v := decision
		if reverse {
			v = k.Vars + 1 - decision
		}
		if values[v] != 0 {
			continue
		}
		values[v] = 1
		queue[end] = int32(v)
		end++
		for read < end {
			lit := queue[read]
			read++
			for id := o.first[witnessMaxVars-lit]; id != 0; id = o.next[id] {
				if !limit.Inc() {
					return false
				}
				row := &k.Clauses[(id-1)/3]
				forced, active := unitClause(row.literals(), values[:])
				if !active {
					continue
				}
				if forced == 0 {
					return false
				}
				if forced < 0 {
					values[-forced] = -1
				} else {
					values[forced] = 1
				}
				queue[end] = forced
				end++
			}
		}
	}
	for i := range k.Clauses {
		if i%64 == 0 && !limit.IncOffset(uint32(min(64, len(k.Clauses)-i))) {
			return false
		}
		satisfied := false
		for _, lit := range k.Clauses[i].literals() {
			if lit > 0 && values[lit] > 0 || lit < 0 && values[-lit] < 0 {
				satisfied = true
				break
			}
		}
		if !satisfied {
			return false
		}
	}
	return true
}
