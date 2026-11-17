package rel

// Source: src/muz/rel/dl_table.h/.cpp bitvector_table_plugin and
// bitvector_table. The table stores one bit per tuple of a small domain whose
// column sizes are powers of two; rows are visited in increasing bit offset,
// that is ordered by the last column, then the one before it, down to column
// 0. bitvector_table overrides only add/remove/contains and iteration, so
// every other operation is the relation_manager default table function,
// which the generic FiniteTable fallbacks implement.

import (
	"fmt"
	mathbits "math/bits"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"

	"github.com/glycerine/g3/pkg/util/bitset"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

const bitvectorTableSmallDomains = "bit-vector table is specialized to small domains that are powers of two"

// bitvectorColumnBits is the num_bits loop of bitvector_table_plugin::
// can_handle_signature and the bitvector_table constructor: for s = 2^k it
// yields k+1 (one more than log2 s), capped at 32.
func bitvectorColumnBits(s uint32) uint32 {
	numBits := uint32(1)
	bitPos := uint32(1)
	for numBits = 1; numBits < 32; numBits++ {
		if bitPos&s != 0 {
			break
		}
		bitPos <<= 1
	}
	return numBits
}

// isPowerOfTwo is util.h is_power_of_two.
func isPowerOfTwo(v uint32) bool { return v&(v-1) == 0 && v != 0 }

// BitvectorCanHandleSignature is bitvector_table_plugin::can_handle_signature.
func BitvectorCanHandleSignature(signature Signature, functional int) bool {
	if functional != 0 {
		return false
	}
	shift := uint32(0)
	for _, domain := range signature.domains {
		s := uint32(domain)
		if uint64(s) != domain || !isPowerOfTwo(s) {
			return false
		}
		shift += bitvectorColumnBits(s)
		if shift >= 32 {
			return false
		}
	}
	return true
}

// BitvectorTable is bitvector_table.
type BitvectorTable struct {
	signature Signature
	bv        *bitset.Vector
	numCols   int
	shift     []uint32
	mask      []uint32
	// count tracks the set bits so emptiness and Len need no scan; the
	// source answers empty() by iterating to the first set bit.
	count  int
	closed bool
	// sparse is the relation manager's sparse_table_plugin, which
	// get_appropriate_plugin falls back to for signatures the bitvector
	// plugin rejects (nil outside a manager).
	sparse *SparseTablePlugin
}

// newBitvectorTableIn is NewBitvectorTable in the manager of plugin.
func newBitvectorTableIn(plugin *SparseTablePlugin, signature Signature) (*BitvectorTable, error) {
	t, err := NewBitvectorTable(signature)
	if err != nil {
		return nil, err
	}
	t.sparse = plugin
	return t, nil
}

// NewBitvectorTable is bitvector_table_plugin::mk_empty and the
// bitvector_table constructor.
func NewBitvectorTable(signature Signature) (*BitvectorTable, error) {
	t := &BitvectorTable{signature: Signature{domains: signature.Domains()}, bv: bitset.New(0)}
	t.numCols = signature.Len()
	shift := uint32(0)
	for i := 0; i < t.numCols; i++ {
		domain := signature.domains[i]
		s := uint32(domain)
		if uint64(s) != domain || !isPowerOfTwo(s) {
			return nil, z3exception.NewDefaultException(bitvectorTableSmallDomains)
		}
		t.shift = append(t.shift, shift)
		t.mask = append(t.mask, s-1)
		shift += bitvectorColumnBits(s)
		if shift >= 32 {
			return nil, z3exception.NewDefaultException(bitvectorTableSmallDomains)
		}
	}
	// FORK FIX #56: reserve after the column loop so a nullary table holds
	// one bit.
	t.bv.Reserve(uint32(1)<<shift, false)
	return t, nil
}

func (t *BitvectorTable) open() error {
	if t == nil || t.closed || t.bv == nil {
		return ErrTable
	}
	return nil
}

func (t *BitvectorTable) Close() error {
	if t == nil || t.closed {
		return nil
	}
	t.closed = true
	t.bv = nil
	t.signature = Signature{}
	return nil
}

func (t *BitvectorTable) Signature() (Signature, error) {
	if err := t.open(); err != nil {
		return Signature{}, err
	}
	return Signature{domains: t.signature.Domains()}, nil
}

func (t *BitvectorTable) validateFact(fact Fact) error {
	if len(fact) != t.signature.Len() {
		return fmt.Errorf("%w: fact arity mismatch", ErrTable)
	}
	for i, value := range fact {
		if uint64(value) >= t.signature.domains[i] {
			return fmt.Errorf("%w: fact value outside column domain", ErrTable)
		}
	}
	return nil
}

// fact2offset is bitvector_table::fact2offset.
func (t *BitvectorTable) fact2offset(f Fact) uint32 {
	result := uint32(0)
	for i := 0; i < t.numCols; i++ {
		result += uint32(f[i]) << t.shift[i]
	}
	return result
}

// offset2fact is bitvector_table::offset2fact.
func (t *BitvectorTable) offset2fact(offset uint32) Fact {
	f := make(Fact, t.numCols)
	for i := 0; i < t.numCols; i++ {
		f[i] = Element(t.mask[i] & (offset >> t.shift[i]))
	}
	return f
}

// Add is bitvector_table::add_fact; it reports whether the bit was clear.
func (t *BitvectorTable) Add(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	offset := t.fact2offset(fact)
	if t.bv.Get(offset) {
		return false, nil
	}
	t.bv.Set(offset, true)
	t.count++
	return true, nil
}

// Remove is bitvector_table::remove_fact.
func (t *BitvectorTable) Remove(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	offset := t.fact2offset(fact)
	if !t.bv.Get(offset) {
		return false, nil
	}
	t.bv.Unset(offset)
	t.count--
	return true, nil
}

// RemoveFacts is table_base::remove_facts: remove_fact in input order.
func (t *BitvectorTable) RemoveFacts(facts []Fact) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, fact := range facts {
		if _, err := t.Remove(fact); err != nil {
			return err
		}
	}
	return nil
}

// Contains is bitvector_table::contains_fact.
func (t *BitvectorTable) Contains(fact Fact) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if err := t.validateFact(fact); err != nil {
		return false, err
	}
	return t.bv.Get(t.fact2offset(fact)), nil
}

func (t *BitvectorTable) Len() (int, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.count, nil
}

// SizeEstimateRows and SizeEstimateBytes are the table_base defaults
// (UINT_MAX); bitvector_table does not override them.
func (t *BitvectorTable) SizeEstimateRows() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return ^uint32(0), nil
}

func (t *BitvectorTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return ^uint32(0), nil
}

// forEachOffset is bv_iterator: increasing offsets whose bit is set.
func (t *BitvectorTable) forEachOffset(visit func(offset uint32)) {
	size := t.bv.Size()
	remaining := t.count
	for word := uint32(0); remaining > 0 && word < t.bv.NumWords(); word++ {
		bits := t.bv.Word(word)
		for bits != 0 && remaining > 0 {
			bit := uint32(mathbits.TrailingZeros32(bits))
			offset := word*32 + bit
			if offset >= size {
				return
			}
			visit(offset)
			remaining--
			bits &^= uint32(1) << bit
		}
	}
}

// Facts copies the rows in bv_iterator order.
func (t *BitvectorTable) Facts() ([]Fact, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	result := make([]Fact, 0, t.count)
	t.forEachOffset(func(offset uint32) { result = append(result, t.offset2fact(offset)) })
	return result, nil
}

// Clone is table_base::clone: mk_empty from the same plugin, then the rows.
func (t *BitvectorTable) Clone() (*BitvectorTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	result, err := newBitvectorTableIn(t.sparse, t.signature)
	if err != nil {
		return nil, err
	}
	result.bv.Assign(t.bv)
	result.count = t.count
	return result, nil
}

func (t *BitvectorTable) CloneFinite() (FiniteTable, error) {
	clone, err := t.Clone()
	if err != nil {
		return nil, err
	}
	return clone, nil
}

// Reset is table_base::reset: every row is removed.
func (t *BitvectorTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	t.bv.Fill0()
	t.count = 0
	return nil
}

// emptyLike is get_plugin().mk_empty(sig) for a signature the plugin handles
// (projections and renamings of a bitvector table always fit).
func (t *BitvectorTable) emptyLike(signature Signature) (*BitvectorTable, error) {
	return newBitvectorTableIn(t.sparse, signature)
}

// FilterEqual is the default table filter_equal: remove the rows whose
// column differs from value.
func (t *BitvectorTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if column < 0 || column >= t.signature.Len() {
		return fmt.Errorf("%w: filter column out of range", ErrTable)
	}
	facts, _ := t.Facts()
	for _, fact := range facts {
		if fact[column] != value {
			if _, err := t.Remove(fact); err != nil {
				return err
			}
		}
	}
	return nil
}

// FilterIdentical is the default identical-columns filter.
func (t *BitvectorTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, column := range columns {
		if column < 0 || column >= t.signature.Len() {
			return fmt.Errorf("%w: identical-filter column out of range", ErrTable)
		}
	}
	if len(columns) < 2 {
		return nil
	}
	facts, _ := t.Facts()
	for _, fact := range facts {
		value := fact[columns[0]]
		for _, column := range columns[1:] {
			if fact[column] != value {
				if _, err := t.Remove(fact); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// Project is default_table_project_fn (the result uses this plugin) for a
// nonempty result signature.
func (t *BitvectorTable) Project(removed []int) (*BitvectorTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	signature, err := projectedSignature(t.signature, removed)
	if err != nil {
		return nil, err
	}
	result, err := t.emptyLike(signature)
	if err != nil {
		return nil, err
	}
	remove := make([]bool, t.signature.Len())
	for _, column := range removed {
		remove[column] = true
	}
	facts, _ := t.Facts()
	for _, source := range facts {
		fact := make(Fact, 0, signature.Len())
		for i, value := range source {
			if !remove[i] {
				fact = append(fact, value)
			}
		}
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// SelectEqualAndProject is the default select_equal_and_project: filter a
// clone, then project the column out.
func (t *BitvectorTable) SelectEqualAndProject(value Element, column int) (*BitvectorTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= t.signature.Len() {
		return nil, fmt.Errorf("%w: selection column out of range", ErrTable)
	}
	signature, err := projectedSignature(t.signature, []int{column})
	if err != nil {
		return nil, err
	}
	result, err := t.emptyLike(signature)
	if err != nil {
		return nil, err
	}
	facts, _ := t.Facts()
	for _, source := range facts {
		if source[column] != value {
			continue
		}
		fact := make(Fact, 0, signature.Len())
		fact = append(fact, source[:column]...)
		fact = append(fact, source[column+1:]...)
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// RenameCycle is default_table_rename_fn: each listed column receives the
// next column's value and the last receives the first.
func (t *BitvectorTable) RenameCycle(cycle []int) (*BitvectorTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return t.Clone()
	}
	permutation := make([]int, t.signature.Len())
	for i := range permutation {
		permutation[i] = i
	}
	seen := make(map[int]bool)
	for _, column := range cycle {
		if column < 0 || column >= len(permutation) || seen[column] {
			return nil, fmt.Errorf("%w: invalid rename cycle", ErrTable)
		}
		seen[column] = true
	}
	for i := 1; i < len(cycle); i++ {
		permutation[cycle[i-1]] = cycle[i]
	}
	permutation[cycle[len(cycle)-1]] = cycle[0]
	domains := make([]uint64, len(permutation))
	for i, source := range permutation {
		domains[i] = t.signature.domains[source]
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	result, err := t.emptyLike(signature)
	if err != nil {
		return nil, err
	}
	facts, _ := t.Facts()
	for _, source := range facts {
		fact := make(Fact, len(permutation))
		for i, column := range permutation {
			fact[i] = source[column]
		}
		if _, err = result.Add(fact); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Complement is table_base::complement for zero- and one-column tables.
func (t *BitvectorTable) Complement() (*BitvectorTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if len(t.signature.domains) > 1 {
		return nil, fmt.Errorf("%w: complement requires compiler decomposition", ErrTable)
	}
	result, err := t.emptyLike(t.signature)
	if err != nil {
		return nil, err
	}
	if len(t.signature.domains) == 0 {
		if t.count == 0 {
			if _, err := result.Add(Fact{}); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	for value := uint64(0); value < t.signature.domains[0]; value++ {
		fact := Fact{Element(value)}
		if !t.bv.Get(t.fact2offset(fact)) {
			if _, err = result.Add(fact); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

var _ FiniteTable = (*BitvectorTable)(nil)

// filterBitvectorInterpreted is default_table_filter_interpreted_fn over a
// bitvector_table: rows whose ground condition rewrites to false are
// removed.
func filterBitvectorInterpreted(table *BitvectorTable, manager *RelationManager, rewriter *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if table == nil || table.open() != nil || manager == nil || manager.open() != nil || rewriter == nil || len(conditions) == 0 {
		return ErrTable
	}
	facts, err := table.Facts()
	if err != nil {
		return err
	}
	relation := &TableRelation{manager: manager}
	removed := make([]Fact, 0)
	for _, fact := range facts {
		accepted, evalErr := relation.acceptsInterpretedFact(rewriter, conditions, columns, sorts, fact, nil)
		if evalErr != nil {
			return evalErr
		}
		if !accepted {
			removed = append(removed, fact)
		}
	}
	return table.RemoveFacts(removed)
}
