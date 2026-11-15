package lp

// Source: src/util/lp/lar_solution_signature.h;
// lar_core_solver.h:356-405,653-662. Actual pair and double consumers.

// LARSolutionSignature is lar_solution_signature,
// std::unordered_map<unsigned, non_basic_column_value_position>.
//
// ORDER MATTERS: prepare_solver_x_with_signature walks the map in libstdc++
// node order and draws a random bit for every boxed not_at_bound column, so
// the keys keep the libstdc++ unordered_map order (the user-approved
// StdUnorderedRationalMap emulation, AUDIT.md util_lp_03-F1). Clear keeps
// the bucket array, as std::unordered_map::clear does. Do not iterate the
// positions map directly and do not sort the keys.
type LARSolutionSignature struct {
	order     *StdUnorderedRationalMap // node order only; values unused (nil)
	positions map[uint32]NonBasicColumnValuePosition
}

// NewLARSolutionSignature is a default-constructed signature (one bucket).
func NewLARSolutionSignature() *LARSolutionSignature {
	return &LARSolutionSignature{order: NewStdUnorderedRationalMap(), positions: map[uint32]NonBasicColumnValuePosition{}}
}

// Clear is clear(): the bucket array and rehash state stay.
func (s *LARSolutionSignature) Clear() {
	if s.order == nil {
		s.order, s.positions = NewStdUnorderedRationalMap(), map[uint32]NonBasicColumnValuePosition{}
		return
	}
	s.order.Clear()
	clear(s.positions)
}

// Set is signature[j] = position.
func (s *LARSolutionSignature) Set(j uint32, position NonBasicColumnValuePosition) {
	if s.order == nil {
		s.order, s.positions = NewStdUnorderedRationalMap(), map[uint32]NonBasicColumnValuePosition{}
	}
	if _, ok := s.order.Get2(j); !ok {
		s.order.Put(j, nil)
	}
	s.positions[j] = position
}

// Get2 is find(j).
func (s *LARSolutionSignature) Get2(j uint32) (NonBasicColumnValuePosition, bool) {
	if s == nil {
		return 0, false
	}
	position, ok := s.positions[j]
	return position, ok
}

// Len is size().
func (s *LARSolutionSignature) Len() int {
	if s == nil {
		return 0
	}
	return len(s.positions)
}

// Keys lists the keys in libstdc++ iteration order (begin() to end()).
func (s *LARSolutionSignature) Keys() []uint32 {
	if s == nil || s.order == nil {
		return nil
	}
	return s.order.Keys()
}

// Positions returns a copy of the key/position pairs, for comparisons.
func (s *LARSolutionSignature) Positions() map[uint32]NonBasicColumnValuePosition {
	out := make(map[uint32]NonBasicColumnValuePosition, s.Len())
	if s != nil {
		for j, p := range s.positions {
			out[j] = p
		}
	}
	return out
}

func (c *LARCoreSolver) ExtractSignatureFromPairSolver(signature *LARSolutionSignature, s *PairPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if signature == nil || s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	signature.Clear()
	for j, h := range *s.Heading {
		if h < 0 {
			position, e := s.GetNonBasicColumnValuePosition(uint32(j))
			if e != nil {
				return e
			}
			signature.Set(uint32(j), position)
		}
	}
	return nil
}

func (c *LARCoreSolver) ExtractSignatureFromFloatSolver(signature *LARSolutionSignature, s *FloatPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if signature == nil || s == nil {
		return ErrBounds
	}
	signature.Clear()
	for j, h := range *s.Heading {
		if h < 0 {
			position, e := s.GetNonBasicColumnValuePosition(uint32(j))
			if e != nil {
				return e
			}
			signature.Set(uint32(j), position)
		}
	}
	return nil
}

// signatureBoundSide is only the common control flow of the two retained
// concrete preparations: 0 leaves X unchanged, 1/2 copy lower/upper, 3 sets zero.
func (c *LARCoreSolver) signatureBoundSide(j uint32, position NonBasicColumnValuePosition) (uint8, error) {
	if sourceDebugEnabled {
		if uint64(j) >= uint64(len(c.RHeading)) {
			return 0, ErrBounds
		}
		if c.RHeading[j] >= 0 {
			return 0, ErrAssertion
		}
	}
	switch position {
	case PositionAtLowBound:
		return 1, nil
	case PositionAtFixed, PositionAtUpperBound:
		return 2, nil
	case PositionFreeOfBounds:
		return 3, nil
	case PositionNotAtBound:
		if uint64(j) >= uint64(len(c.Bounds.Types.values)) {
			return 0, ErrBounds
		}
		switch c.Bounds.Types.values[j] {
		case ColumnFree:
			if sourceDebugEnabled {
				return 0, ErrAssertion
			}
			// Source intentionally falls through to the upper-bound case.
			return 2, nil
		case ColumnUpperBound:
			return 2, nil
		case ColumnLowBound, ColumnFixed:
			return 1, nil
		case ColumnBoxed:
			if c.settings.RandomNext()%2 != 0 {
				return 1, nil
			}
			return 2, nil
		default:
			return 0, sourceAssertionFailure()
		}
	default:
		return 0, sourceAssertionFailure()
	}
}

func (c *LARCoreSolver) PreparePairSolverXWithSignature(signature *LARSolutionSignature, s *PairPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	// ORDER MATTERS: for (auto &t : signature) walks libstdc++ node order.
	for _, j := range signature.Keys() {
		position, _ := signature.Get2(j)
		side, e := c.signatureBoundSide(j, position)
		if e != nil {
			return e
		}
		if side == 0 {
			continue
		}
		if uint64(j) >= uint64(len(*s.X)) {
			return ErrBounds
		}
		zero := NewRationalPair()
		value := &zero
		if side != 3 {
			bounds := s.LowBounds
			if side == 2 {
				bounds = s.UpperBounds
			}
			if uint64(j) >= uint64(len(*bounds)) {
				return ErrBounds
			}
			value = &(*bounds)[j]
		}
		before, e := s.calls.begin(s.manager)
		if e != nil {
			return e
		}
		e = (*s.X)[j].Copy(s.manager, value)
		if err = s.calls.after(s.manager, before, e); err != nil {
			return err
		}
	}
	if sourceDebugEnabled {
		for i := range *s.B {
			zero, e := (*s.B)[i].IsZero(s.manager)
			if e != nil {
				return e
			}
			if !zero {
				return ErrAssertion
			}
		}
	}
	return s.SolveAxEqualsB()
}

func (c *LARCoreSolver) PrepareFloatSolverXWithSignature(signature *LARSolutionSignature, s *FloatPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if s == nil {
		return ErrBounds
	}
	// ORDER MATTERS: for (auto &t : signature) walks libstdc++ node order.
	for _, j := range signature.Keys() {
		position, _ := signature.Get2(j)
		side, e := c.signatureBoundSide(j, position)
		if e != nil {
			return e
		}
		if side == 0 {
			continue
		}
		if uint64(j) >= uint64(len(*s.X)) {
			return ErrBounds
		}
		if side == 3 {
			(*s.X)[j] = 0
			continue
		}
		bounds := s.LowBounds
		if side == 2 {
			bounds = s.UpperBounds
		}
		if uint64(j) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		(*s.X)[j] = (*bounds)[j]
	}
	if sourceDebugEnabled {
		for _, b := range *s.B {
			if b != 0 {
				return ErrAssertion
			}
		}
	}
	return s.SolveAxEqualsB()
}
