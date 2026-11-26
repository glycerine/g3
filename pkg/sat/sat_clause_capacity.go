package sat

import (
	"math/bits"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const (
	capacityMaxVars     = 4096
	capacityMaxMembers  = 256
	capacityWords       = capacityMaxMembers / 64
	capacityMaxRows     = 32
	capacityWatchBudget = 8192
)

// ProbeClauseCapacity looks for a set of disjoint clauses whose selected
// literals can each be put in a group with capacity one. Its proof needs only
// the selected clauses and the original binaries excluding pairs of members.
// It is a read-only probe for the one-shot DIMACS frontend. A false result
// leaves the verdict to the ordinary solver.
func (s *Solver) ProbeClauseCapacity() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if s.propagation.inconsistent || s.Variables.scopeLevel != 0 || s.propagation.extensionPresent ||
		s.conflict.conflicts != 0 || len(s.Lifecycle.Original) < 3 || len(s.Lifecycle.Original) > 512 ||
		len(s.Variables.watches.lists) > 2*capacityMaxVars || s.Limit.Closed() ||
		s.Limit.Limit != 0 || s.Limit.CancelFlag() {
		return false, nil
	}
	cfg, err := s.config.State()
	if err != nil {
		return false, err
	}
	if cfg.DimacsDisplay || cfg.MaxConflicts != ^uint32(0) || cfg.NumParallel > 1 || cfg.MaxMemory != uint64(^uint(0)) {
		return false, nil
	}
	verbose, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || verbose {
		return false, err
	}
	return s.probeSmallClauseCapacity()
}

func (s *Solver) probeSmallClauseCapacity() (bool, error) {
	var vars [capacityMaxVars]int16 // 1 + index of the member with that variable
	var members [capacityMaxMembers]Literal
	var offsets [capacityMaxRows + 1]int
	size, rows := 0, 0
	for _, ref := range s.Lifecycle.Original {
		view, err := s.Clauses.view(ref)
		if err != nil {
			return false, err
		}
		arity := int(view.size())
		if arity < 4 || arity > 32 || size+arity > len(members) || view.flag(satClauseFlagRemoved) {
			continue
		}
		start := size
		for i := 0; i < arity; i++ {
			lit := view.literal(uint32(i))
			v := lit.variable()
			if v >= capacityMaxVars || vars[v] != 0 {
				break
			}
			members[size] = lit
			vars[v] = int16(size + 1)
			size++
		}
		if size-start != arity {
			for _, lit := range members[start:size] {
				vars[lit.variable()] = 0
			}
			size = start
			continue
		}
		rows++
		offsets[rows] = size
		if rows == capacityMaxRows {
			break
		}
	}
	if rows < 3 {
		return false, nil
	}

	// If the selected literals are a and b, their mutual exclusion is the
	// binary (not a or not b), stored on the watch list for a.
	var links [capacityMaxMembers][capacityWords]uint64
	found := false
	watches := s.Variables.watches
	remaining := capacityWatchBudget
	for i, lit := range members[:size] {
		if s.Limit.CancelFlag() {
			return false, nil
		}
		list, err := watches.Get(lit.index())
		if err != nil {
			return false, err
		}
		ref, err := list.ref()
		if err != nil {
			return false, err
		}
		iter, err := watches.store.Iter(ref)
		if err != nil {
			return false, err
		}
		for {
			w, more, err := iter.Next()
			if err != nil {
				return false, err
			}
			if !more {
				break
			}
			if remaining == 0 {
				return false, nil
			}
			remaining--
			if !w.Binary() {
				continue
			}
			learned, err := w.Learned()
			if err != nil {
				return false, err
			}
			if learned {
				continue
			}
			other, err := w.Literal()
			if err != nil {
				return false, err
			}
			b := other.Negated()
			if b.variable() >= capacityMaxVars {
				continue
			}
			j := int(vars[b.variable()]) - 1
			if j <= i || members[j] != b {
				continue
			}
			links[i][j/64] |= 1 << (j % 64)
			links[j][i/64] |= 1 << (i % 64)
			found = true
		}
	}
	return found && clauseCapacityDeficient(links[:size], offsets[:rows+1]) && !s.Limit.CancelFlag(), nil
}

func clauseCapacityDeficient(links [][capacityWords]uint64, offsets []int) bool {
	var group [capacityMaxMembers]uint16
	var pending [capacityMaxMembers]int
	groupCount := 0
	for i := range links {
		if group[i] != 0 {
			continue
		}
		groupCount++
		group[i] = uint16(groupCount)
		pending[0] = i
		next := 1
		for cursor := 0; cursor < next; cursor++ {
			for block, mask := range links[pending[cursor]] {
				for mask != 0 {
					j := 64*block + bits.TrailingZeros64(mask)
					mask &= mask - 1
					if group[j] == 0 {
						group[j] = uint16(groupCount)
						pending[next] = j
						next++
					}
				}
			}
		}

		// An incomplete component is not an at-most-one constraint.
		// Treat each literal as independent unless every pair is excluded.
		for _, member := range pending[:next] {
			degree := 0
			for _, word := range links[member] {
				degree += bits.OnesCount64(word)
			}
			if degree != next-1 {
				for _, member := range pending[1:next] {
					groupCount++
					group[member] = uint16(groupCount)
				}
				break
			}
		}
	}
	if groupCount < len(offsets)-1 {
		return true
	}

	// Every satisfied row needs its own group. An augmenting-path search
	// also detects a subset of rows with too few groups, even if the total
	// group count is large enough.
	var match, seen [capacityMaxMembers + 1]int
	for row := 1; row < len(offsets); row++ {
		if !clauseCapacityAugment(row, row, offsets, &group, &match, &seen) {
			return true
		}
	}
	return false
}

func clauseCapacityAugment(row, pass int, offsets []int, group *[capacityMaxMembers]uint16, match, seen *[capacityMaxMembers + 1]int) bool {
	for _, g := range group[offsets[row-1]:offsets[row]] {
		if seen[g] == pass {
			continue
		}
		seen[g] = pass
		if match[g] == 0 || clauseCapacityAugment(match[g], pass, offsets, group, match, seen) {
			match[g] = row
			return true
		}
	}
	return false
}
