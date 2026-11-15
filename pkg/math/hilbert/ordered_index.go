package hilbert

// orderedIndex is the concrete semantic portion of the source
// hilbert_basis::index over value_index2. As in the source, positive and
// zero weights have one shared index each and every negative weight has its
// own index, so a candidate is only compared against entries of its own
// partition (src/math/hilbert/hilbert_basis.cpp:243-325). Native slices
// replace heap_trie storage; the dominance relation is unchanged.
type orderedIndex struct {
	owner *Basis
	pos   []*vector
	zero  []*vector
	neg   map[int64][]*vector
}

func (i *orderedIndex) insert(value *vector) {
	i.owner.stats.indexInsert++
	switch {
	case value.weight > 0:
		i.pos = append(i.pos, value)
	case value.weight == 0:
		i.zero = append(i.zero, value)
	default:
		if i.neg == nil {
			i.neg = make(map[int64][]*vector)
		}
		i.neg[value.weight] = append(i.neg[value.weight], value)
	}
}

func (i *orderedIndex) subsumes(candidate *vector) bool {
	i.owner.stats.indexFind++
	var values []*vector
	switch {
	case candidate.weight > 0:
		values = i.pos
	case candidate.weight == 0:
		values = i.zero
	default:
		values = i.neg[candidate.weight]
	}
	for _, existing := range values {
		i.owner.stats.indexComparisons++
		if i.owner.subsumes(candidate, existing) {
			i.owner.stats.indexHit++
			i.owner.stats.numSubsumptions++
			return true
		}
	}
	i.owner.stats.indexMiss++
	return false
}
