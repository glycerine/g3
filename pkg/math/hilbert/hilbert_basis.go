package hilbert

// Source: src/math/hilbert/hilbert_basis.cpp/.h. The retained saturation uses
// the source saturate_orig work-list algorithm; Go slices replace heap_trie and
// C++ heap indexing without changing resolution or subsumption predicates.

import (
	"errors"
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

var (
	ErrDimension = errors.New("math/hilbert: inconsistent inequality dimension")
	ErrOverflow  = errors.New("math/hilbert: checked int64 overflow")
)

type inequality struct {
	values []int64
	equal  bool
}

type vector struct {
	values  []int64
	weight  int64
	weights []int64
}

type Basis struct {
	// q is the rational manager of the rational_vector arguments and results.
	q                  *numeral.RationalManager
	inequalities       []inequality
	integers           map[uint32]bool
	basis              []*vector
	current            int
	checkpoint         func() bool
	useSupport         bool
	orderedSupport     bool
	orderedSubsumption bool
	stats              basisStatistics
}

func New(q *numeral.RationalManager, checkpoint func() bool) *Basis {
	return &Basis{q: q, integers: make(map[uint32]bool), checkpoint: checkpoint, useSupport: true, orderedSupport: true, orderedSubsumption: true}
}

func (b *Basis) Reset() {
	b.inequalities = nil
	b.basis = nil
	b.current = 0
	b.integers = make(map[uint32]bool)
}

// toNumeral is hilbert_basis::to_numeral: a rational that is not an
// int64 throws checked_int64's overflow_exception.
func (b *Basis) toNumeral(value *numeral.Rational) (int64, error) {
	ok, err := b.q.IsInt64(value)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrOverflow
	}
	return b.q.Int64(value)
}

// toRational is hilbert_basis::to_rational: rational(i, rational::i64()).
func (b *Basis) toRational(value int64, out *numeral.Rational) error {
	return b.q.SetInt64(out, value)
}

func checkedNeg(value int64) (int64, error) {
	if value == math.MinInt64 {
		return 0, ErrOverflow
	}
	return -value, nil
}

func checkedAdd(left, right int64) (int64, error) {
	result := left + right
	if (left > 0 && right > 0 && result < 0) || (left < 0 && right < 0 && result >= 0) {
		return 0, ErrOverflow
	}
	return result, nil
}

func checkedMul(left, right int64) (int64, error) {
	if left == 0 || right == 0 {
		return 0, nil
	}
	if (left == -1 && right == math.MinInt64) || (right == -1 && left == math.MinInt64) {
		return 0, ErrOverflow
	}
	result := left * right
	if result/right != left {
		return 0, ErrOverflow
	}
	return result, nil
}

// addRow is add_ge/add_eq(rational_vector const& v, rational const& b):
// w = [to_numeral(-b), to_numeral(v[0]), ...]; with negate it is add_le,
// which negates v and b first. A nil bound is rational(0).
//
// DELIBERATE DIVERGENCE: the source negates the rationals before
// to_numeral, so a coordinate of exactly 2^63 becomes INT64_MIN and is
// accepted; saturation then needs on the order of 2^63 resolution steps.
// g3 converts first and applies checked_int64 negation, so such a row
// reports the checked_int64 overflow at once (as g3 always has).
func (b *Basis) addRow(values []numeral.Rational, bound *numeral.Rational, equal, negate bool) (err error) {
	if len(b.inequalities) != 0 && len(values)+1 != len(b.inequalities[len(b.inequalities)-1].values) {
		return ErrDimension
	}
	row := make([]int64, len(values)+1)
	if bound != nil {
		if row[0], err = b.toNumeral(bound); err != nil {
			return err
		}
		if !negate {
			if row[0], err = checkedNeg(row[0]); err != nil {
				return err
			}
		}
	}
	for i := range values {
		if row[i+1], err = b.toNumeral(&values[i]); err == nil && negate {
			row[i+1], err = checkedNeg(row[i+1])
		}
		if err != nil {
			return err
		}
	}
	b.inequalities = append(b.inequalities, inequality{values: row, equal: equal})
	return nil
}

func (b *Basis) AddGE(values []numeral.Rational, bound *numeral.Rational) error {
	return b.addRow(values, bound, false, false)
}

// AddLE is add_le: add_ge of the negated row and bound.
func (b *Basis) AddLE(values []numeral.Rational, bound *numeral.Rational) error {
	return b.addRow(values, bound, false, true)
}

func (b *Basis) AddEQ(values []numeral.Rational, bound *numeral.Rational) error {
	return b.addRow(values, bound, true, false)
}

func (b *Basis) SetIsInt(index uint32)   { b.integers[index+1] = true }
func (b *Basis) IsInt(index uint32) bool { return b.integers[index+1] }

func (b *Basis) proceed() bool { return b.checkpoint == nil || b.checkpoint() }

func (b *Basis) initBasis() {
	b.basis = nil
	if len(b.inequalities) == 0 {
		return
	}
	count := len(b.inequalities[0].values)
	for i := 0; i < count; i++ {
		values := make([]int64, count)
		values[i] = 1
		b.basis = append(b.basis, &vector{values: values})
	}
	indices := make([]int, 0, len(b.integers))
	for index := range b.integers {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	for _, index := range indices {
		if index >= 0 && index < count {
			values := make([]int64, count)
			values[index] = -1
			b.basis = append(b.basis, &vector{values: values})
		}
	}
}

func weight(value *vector, row []int64) (int64, error) {
	var result int64
	for i := range row {
		product, err := checkedMul(value.values[i], row[i])
		if err != nil {
			return 0, err
		}
		result, err = checkedAdd(result, product)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

func (b *Basis) selectInequality() error {
	best := b.current
	nonzero := countNonzero(b.inequalities[best].values)
	product, err := b.inequalityProduct(b.inequalities[best].values)
	if err != nil {
		return err
	}
	for i := best + 1; product != 0 && i < len(b.inequalities); i++ {
		candidateNonzero := countNonzero(b.inequalities[i].values)
		candidateProduct, productErr := b.inequalityProduct(b.inequalities[i].values)
		if productErr != nil {
			return productErr
		}
		if candidateProduct == 0 || candidateNonzero < nonzero || candidateNonzero == nonzero && candidateProduct < product {
			best, nonzero, product = i, candidateNonzero, candidateProduct
		}
	}
	if best != b.current {
		b.inequalities[best], b.inequalities[b.current] = b.inequalities[b.current], b.inequalities[best]
	}
	return nil
}

func countNonzero(values []int64) int {
	result := 0
	for _, value := range values {
		if value != 0 {
			result++
		}
	}
	return result
}

func (b *Basis) inequalityProduct(row []int64) (uint64, error) {
	var positive, negative uint64
	for _, value := range b.basis {
		w, err := weight(value, row)
		if err != nil {
			return 0, err
		}
		if w > 0 {
			positive++
		} else if w < 0 {
			negative++
		}
	}
	if positive != 0 && negative > math.MaxUint64/positive {
		return math.MaxUint64, nil
	}
	return positive * negative, nil
}

func abs64(value int64) (uint64, error) {
	if value == math.MinInt64 {
		return 0, ErrOverflow
	}
	if value < 0 {
		value = -value
	}
	return uint64(value), nil
}

func vectorMagnitude(value *vector) (uint64, error) {
	var result uint64
	for _, coordinate := range value.values {
		magnitude, err := abs64(coordinate)
		if err != nil || math.MaxUint64-result < magnitude {
			if err == nil {
				err = ErrOverflow
			}
			return 0, err
		}
		result += magnitude
	}
	return result, nil
}

func sign(value int64) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func (b *Basis) canResolve(left, right *vector) bool {
	if sign(left.weight) == sign(right.weight) {
		return false
	}
	if left.values[0] == 1 && right.values[0] == 1 {
		return false
	}
	for index := range b.integers {
		if sign(left.values[index]) != 0 && sign(left.values[index]) == -sign(right.values[index]) {
			return false
		}
	}
	return true
}

func resolve(left, right *vector) (*vector, error) {
	result := &vector{values: make([]int64, len(left.values)), weights: make([]int64, len(left.weights))}
	var err error
	for i := range result.values {
		result.values[i], err = checkedAdd(left.values[i], right.values[i])
		if err != nil {
			return nil, err
		}
	}
	result.weight, err = checkedAdd(left.weight, right.weight)
	if err != nil {
		return nil, err
	}
	for i := range result.weights {
		result.weights[i], err = checkedAdd(left.weights[i], right.weights[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (b *Basis) resolve(left, right *vector) (*vector, error) {
	b.stats.numResolves++
	return resolve(left, right)
}

func absGE(left, right int64) bool {
	if right < 0 {
		return left <= right
	}
	return left >= right
}

func (b *Basis) subsumes(candidate, existing *vector) bool {
	if candidate == existing || candidate.weight < existing.weight || existing.weight < 0 && candidate.weight != existing.weight {
		return false
	}
	for i := range candidate.values {
		if !absGE(candidate.values[i], existing.values[i]) {
			return false
		}
	}
	for i := range candidate.weights {
		if candidate.weights[i] < existing.weights[i] {
			return false
		}
	}
	return true
}

func (b *Basis) isSubsumed(candidate *vector, accepted []*vector) bool {
	for _, existing := range accepted {
		if b.subsumes(candidate, existing) {
			return true
		}
	}
	return false
}

func (b *Basis) saturateOne(row []int64, equal bool) (lbool.LBool, error) {
	active := make([]*vector, 0)
	passive := make([]*vector, 0)
	zero := make([]*vector, 0)
	accepted := make([]*vector, 0)
	support := make(map[*vector]bool)
	addGoal := func(value *vector) bool {
		if b.isSubsumed(value, accepted) {
			return false
		}
		accepted = append(accepted, value)
		if value.weight == 0 {
			zero = append(zero, value)
		} else {
			passive = append(passive, value)
		}
		return true
	}
	for _, value := range b.basis {
		var err error
		value.weight, err = weight(value, row)
		if err != nil {
			return lbool.Undef, err
		}
		value.weights = make([]int64, b.current)
		for i := 0; i < b.current; i++ {
			value.weights[i], err = weight(value, b.inequalities[i].values)
			if err != nil {
				return lbool.Undef, err
			}
		}
		addGoal(value)
		if b.useSupport {
			support[value] = true
		}
	}
	for len(passive) != 0 {
		if !b.proceed() {
			return lbool.Undef, nil
		}
		best := 0
		bestMagnitude, err := vectorMagnitude(passive[0])
		if err != nil {
			return lbool.Undef, err
		}
		for i := 1; i < len(passive); i++ {
			magnitude, magnitudeErr := vectorMagnitude(passive[i])
			if magnitudeErr != nil {
				return lbool.Undef, magnitudeErr
			}
			if magnitude < bestMagnitude {
				best, bestMagnitude = i, magnitude
			}
		}
		candidate := passive[best]
		passive[best] = passive[len(passive)-1]
		passive = passive[:len(passive)-1]
		others := acceptedWithout(accepted, candidate)
		if b.isSubsumed(candidate, others) {
			// The source recycles the candidate here, which also removes it
			// from the subsumption index. Keep the slice index equivalent.
			accepted = others
			continue
		}
		for _, prior := range active {
			if (!b.useSupport || support[prior]) && b.canResolve(candidate, prior) {
				resolved, resolveErr := b.resolve(candidate, prior)
				if resolveErr != nil {
					return lbool.Undef, resolveErr
				}
				addGoal(resolved)
			}
		}
		active = append(active, candidate)
	}
	b.basis = append([]*vector(nil), zero...)
	if !equal {
		for _, value := range active {
			if value.weight > 0 {
				b.basis = append(b.basis, value)
			}
		}
	}
	if len(b.basis) == 0 {
		return lbool.False, nil
	}
	return lbool.True, nil
}

func acceptedWithout(values []*vector, excluded *vector) []*vector {
	result := make([]*vector, 0, len(values)-1)
	for _, value := range values {
		if value != excluded {
			result = append(result, value)
		}
	}
	return result
}

func (b *Basis) Saturate() (lbool.LBool, error) {
	b.initBasis()
	b.current = 0
	// while (checkpoint() && m_current_ineq < m_ineqs.size()): the
	// checkpoint (m_limit.inc()) runs before the bound test, so the loop's
	// last test spends one more step.
	for b.proceed() && b.current < len(b.inequalities) {
		if err := b.selectInequality(); err != nil {
			return lbool.Undef, err
		}
		result, err := b.saturateOrdered(b.inequalities[b.current].values, b.inequalities[b.current].equal)
		b.stats.numSaturations++
		if err != nil || result != lbool.True {
			return result, err
		}
		b.current++
	}
	if !b.proceed() {
		return lbool.Undef, nil
	}
	return lbool.True, nil
}

func (b *Basis) BasisSize() uint32 { return uint32(len(b.basis)) }

// BasisSolution is get_basis_solution: the caller owns (and deletes) the
// returned rationals, which belong to the basis's manager.
func (b *Basis) BasisSolution(index uint32) ([]numeral.Rational, bool, error) {
	if uint64(index) >= uint64(len(b.basis)) {
		return nil, false, ErrDimension
	}
	value := b.basis[index]
	result := make([]numeral.Rational, len(value.values)-1)
	for i := range result {
		result[i] = numeral.NewRational()
	}
	for i := 1; i < len(value.values); i++ {
		if err := b.toRational(value.values[i], &result[i-1]); err != nil {
			return result, false, err
		}
	}
	return result, value.values[0] != 0, nil
}

// Inequality is get_ge: b = to_rational(-m_ineqs[i][0]), where the
// checked negation of INT64_MIN overflows. The caller owns the results.
func (b *Basis) Inequality(index uint32) (values []numeral.Rational, bound numeral.Rational, equal bool, err error) {
	bound = numeral.NewRational()
	if uint64(index) >= uint64(len(b.inequalities)) {
		return nil, bound, false, ErrDimension
	}
	row := b.inequalities[index]
	values = make([]numeral.Rational, len(row.values)-1)
	for i := range values {
		values[i] = numeral.NewRational()
	}
	for i := 1; i < len(row.values); i++ {
		if err = b.toRational(row.values[i], &values[i-1]); err != nil {
			return values, bound, false, err
		}
	}
	constant, err := checkedNeg(row.values[0])
	if err == nil {
		err = b.toRational(constant, &bound)
	}
	return values, bound, row.equal, err
}
