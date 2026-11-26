package sat

import (
	"math/bits"
	"sort"
)

// The solver reuses this sort.Interface for clauses that need a comparison sort.
type satLiteralOrder struct{ values []Literal }

func (a *satLiteralOrder) Len() int           { return len(a.values) }
func (a *satLiteralOrder) Less(i, j int) bool { return a.values[i] < a.values[j] }
func (a *satLiteralOrder) Swap(i, j int)      { a.values[i], a.values[j] = a.values[j], a.values[i] }

func (a *satLiteralOrder) sort(values []Literal) {
	// Short clauses use direct comparisons on the literal words.
	if len(values) < 16 {
		for i := 1; i < len(values); i++ {
			lit := values[i]
			j := i
			for j > 0 && values[j-1] > lit {
				values[j] = values[j-1]
				j--
			}
			values[j] = lit
		}
		return
	}
	i := 1
	for i < len(values) && values[i-1] <= values[i] {
		i++
	}
	if i == len(values) {
		return
	}
	if i == 1 && values[len(values)-1] <= values[1] {
		for i < len(values) && values[i-1] >= values[i] {
			i++
		}
		if i == len(values) {
			for l, r := 0, len(values)-1; l < r; l, r = l+1, r-1 {
				values[l], values[r] = values[r], values[l]
			}
			return
		}
	}
	if sortDenseLiterals(values) {
		return
	}
	a.values = values
	sort.Sort(a)
	a.values = nil
}

// When all literal words are distinct and fit in one aligned 4096-word range,
// enumerating set bits yields the same unsigned order as Less. Leave the input
// untouched on a miss so the comparison sort retains every occurrence.
func sortDenseLiterals(values []Literal) bool {
	base := uint32(values[0]) &^ 4095
	var words [64]uint64
	first, last := uint32(len(words)-1), uint32(0)
	for _, lit := range values {
		offset := uint32(lit) - base
		if offset >= 4096 {
			return false
		}
		index := offset / 64
		mask := uint64(1) << (offset % 64)
		if words[index]&mask != 0 {
			return false
		}
		words[index] |= mask
		first, last = min(first, index), max(last, index)
	}
	write := 0
	for index := first; index <= last; index++ {
		for word := words[index]; word != 0; word &= word - 1 {
			values[write] = Literal(base + 64*index + uint32(bits.TrailingZeros64(word)))
			write++
		}
	}
	return true
}
