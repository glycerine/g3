package interval

// Source: src/tactic/bv/bv_bounds_tactic.cpp:28-60 (uMaxInt and struct
// interval: uint64_t l, h with sz <= 64).

// ModularInterval is bv_bounds_tactic's cyclic unsigned bit-vector
// interval. Lower > Upper is the source wrapped representation. Like the
// source, it holds uint64_t bounds and widths of at most 64 bits.
type ModularInterval struct {
	lower, upper uint64
	size         uint32
	tight        bool
}

func NewModularInterval(lower, upper uint64, size uint32, tight bool) (*ModularInterval, error) {
	if size == 0 || size > 64 {
		return nil, ErrExtendedDomain
	}
	bound := modularBound(size)
	if lower > bound || upper > bound {
		return nil, ErrExtendedDomain
	}
	v := &ModularInterval{lower: lower, upper: upper, size: size, tight: tight}
	if !v.Invariant() {
		return nil, ErrExtendedDomain
	}
	return v, nil
}

// modularBound is uMaxInt(sz): ULLONG_MAX >> (64u - sz).
func modularBound(size uint32) uint64 {
	return ^uint64(0) >> (64 - size)
}
