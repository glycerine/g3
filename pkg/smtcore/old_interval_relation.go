package smtcore

// Source: src/muz/rel/dl_interval_relation.cpp:125-190.
// The relation plugin's old_interval values without bound dependencies.
// They stay math/big values until the interval relation's rationals get
// the relation plugin's deallocation lifecycle.

import "math/big"

// relationEndpoint is an ext_numeral endpoint; nil is an infinity.
type relationEndpoint struct {
	value *big.Rat
	plus  bool // the infinity's sign
}

func (n relationEndpoint) cmp(other relationEndpoint) int {
	switch {
	case n.value == nil && other.value == nil:
		if n.plus == other.plus {
			return 0
		}
		if n.plus {
			return 1
		}
		return -1
	case n.value == nil:
		if n.plus {
			return 1
		}
		return -1
	case other.value == nil:
		if other.plus {
			return -1
		}
		return 1
	}
	return n.value.Cmp(other.value)
}

func (n relationEndpoint) clone() relationEndpoint {
	if n.value != nil {
		n.value = new(big.Rat).Set(n.value)
	}
	return n
}

// RelationInterval is an exact rational interval. A nil lower or upper bound
// denotes negative or positive infinity, respectively.
type RelationInterval struct{ value *relationInterval }

type relationInterval struct {
	lower, upper         relationEndpoint
	lowerOpen, upperOpen bool
}

func (i *relationInterval) clone() *relationInterval {
	return &relationInterval{lower: i.lower.clone(), upper: i.upper.clone(), lowerOpen: i.lowerOpen, upperOpen: i.upperOpen}
}

func NewRelationInterval(lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool) RelationInterval {
	i := &relationInterval{upper: relationEndpoint{plus: true}, lowerOpen: true, upperOpen: true}
	if lower != nil {
		i.lower, i.lowerOpen = relationEndpoint{value: new(big.Rat).Set(lower)}, lowerOpen
	}
	if upper != nil {
		i.upper, i.upperOpen = relationEndpoint{value: new(big.Rat).Set(upper)}, upperOpen
	}
	return RelationInterval{i}
}

// Bounds returns independent copies of the finite endpoints.
func (i RelationInterval) Bounds() (lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool) {
	return i.value.lower.clone().value, i.value.lowerOpen, i.value.upper.clone().value, i.value.upperOpen
}

func (i RelationInterval) Contains(value *big.Rat) bool {
	if value == nil {
		return false
	}
	if l := i.value.lower.value; l != nil {
		c := value.Cmp(l)
		if c < 0 || c == 0 && i.value.lowerOpen {
			return false
		}
	}
	if u := i.value.upper.value; u != nil {
		c := value.Cmp(u)
		if c > 0 || c == 0 && i.value.upperOpen {
			return false
		}
	}
	return true
}

// Equal compares endpoints and their openness.
func (i RelationInterval) Equal(other RelationInterval) bool {
	a, b := i.value, other.value
	return a.lower.cmp(b.lower) == 0 && a.upper.cmp(b.upper) == 0 && a.lowerOpen == b.lowerOpen && a.upperOpen == b.upperOpen
}

// EmptyBounds follows the relation plugin's check for reversed endpoints.
func (i RelationInterval) EmptyBounds() bool { return i.value.upper.cmp(i.value.lower) < 0 }

// Hull returns the least interval produced by the source union operation.
func (i RelationInterval) Hull(other RelationInterval) RelationInterval {
	a, b := i.value, other.value
	r := a.clone()
	if c := b.lower.cmp(r.lower); c < 0 || c == 0 && r.lowerOpen {
		r.lower, r.lowerOpen = b.lower.clone(), b.lowerOpen
	}
	if c := b.upper.cmp(r.upper); c > 0 || c == 0 && r.upperOpen {
		r.upper, r.upperOpen = b.upper.clone(), b.upperOpen
	}
	return RelationInterval{r}
}

// Widen follows the source's endpoint and openness rules.
func (i RelationInterval) Widen(other RelationInterval) RelationInterval {
	a, b := i.value, other.value
	r := a.clone()
	if c := b.lower.cmp(r.lower); c < 0 || c == 0 && r.lowerOpen && !b.lowerOpen {
		r.lower, r.lowerOpen = relationEndpoint{}, true
	}
	if c := b.upper.cmp(r.upper); c > 0 || c == 0 && !r.upperOpen && b.upperOpen {
		r.upper, r.upperOpen = relationEndpoint{plus: true}, true
	}
	return RelationInterval{r}
}

// Meet returns the intersection and reports a disjoint intersection.
func (i RelationInterval) Meet(other RelationInterval) (RelationInterval, bool) {
	a, b := i.value, other.value
	if a.upper.cmp(a.lower) < 0 || b.lower.value == nil && b.upper.value == nil {
		return RelationInterval{a.clone()}, false
	}
	if b.upper.cmp(b.lower) < 0 || a.lower.value == nil && a.upper.value == nil {
		return RelationInterval{b.clone()}, false
	}
	r := a.clone()
	if c := b.lower.cmp(r.lower); c > 0 || c == 0 && !r.lowerOpen {
		r.lower, r.lowerOpen = b.lower.clone(), b.lowerOpen
	}
	if c := b.upper.cmp(r.upper); c < 0 || c == 0 && !r.upperOpen {
		r.upper, r.upperOpen = b.upper.clone(), b.upperOpen
	}
	cmp := r.lower.cmp(r.upper)
	if cmp > 0 || cmp == 0 && (r.lowerOpen || r.upperOpen) {
		return NewRelationInterval(nil, true, nil, true), true
	}
	return RelationInterval{r}, false
}
