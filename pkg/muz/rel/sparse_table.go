package rel

// Source: src/muz/rel/dl_sparse_table.{h,cpp} sparse_table.

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

// SparseTable is sparse_table: rows of factSize bytes in the entry_storage
// m_data, in storage order. plugin is the sparse_table_plugin whose pool
// deallocate returns the table to (nil for a table made outside a relation
// manager); data is nil once the table is deallocated or released.
type SparseTable struct {
	plugin     *SparseTablePlugin
	signature  Signature
	layout     SparseLayout
	factSize   uint32
	data       *entryStorage
	keyIndexes *sparseKeyIndexMap
	functional int
	// deferredPool: made for a placeholder without the plugin's pool; the
	// source's mk_empty pops the pool when the relation is created
	// (adoptPooledTables).
	deferredPool bool
}

// newSparseTable is sparse_table(p, sig, init_capacity = 0).
func newSparseTable(plugin *SparseTablePlugin, signature Signature, functionalColumns int) (*SparseTable, error) {
	layout, e := NewSparseLayout(signature, functionalColumns)
	if e != nil {
		return nil, e
	}
	t := &SparseTable{plugin: plugin, signature: signature, layout: layout, functional: functionalColumns, keyIndexes: newSparseKeyIndexMap()}
	t.data = newEntryStorage(layout.entryBytes, layout.functionalBytes, 0)
	t.factSize = t.data.entrySize
	return t, nil
}

// NewSparseTable makes a sparse table outside any relation manager's
// sparse_table_plugin: its deallocation pools nothing.
func NewSparseTable(signature Signature, functionalColumns int) (*SparseTable, error) {
	return newSparseTable(nil, signature, functionalColumns)
}

func (t *SparseTable) open() error {
	if t == nil || t.data == nil {
		return ErrTable
	}
	return nil
}

// writeIntoReserve is sparse_table::write_into_reserve.
func (t *SparseTable) writeIntoReserve(f Fact) {
	t.data.ensureReserve()
	reserve := t.data.reservePtr()
	for i := range t.layout.columns {
		t.layout.set(reserve, i, f[i])
	}
}

func (t *SparseTable) getCell(ofs storeOffset, column int) Element {
	return t.layout.get(t.data.get(ofs), column)
}

func (t *SparseTable) setCell(ofs storeOffset, column int, val Element) {
	t.layout.set(t.data.get(ofs), column, val)
}

// firstFunctional is table_signature::first_functional.
func (t *SparseTable) firstFunctional() int { return t.signature.Len() - t.functional }

// Add is sparse_table::add_fact(const table_fact&): write_into_reserve and
// add_reserve_content. It reports whether the row was new.
func (t *SparseTable) Add(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	if e := t.layout.checkFact(t.signature, f); e != nil {
		return false, e
	}
	t.writeIntoReserve(f)
	return t.data.insertReserveContent(), nil
}

// Contains is sparse_table::contains_fact without its verbose_action. A
// functional row matches when its functional columns agree too (the
// source compares the cells from functional_columns() on, which covers
// them for the source's signatures, whose functional columns never
// outnumber the others).
func (t *SparseTable) Contains(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	if e := t.layout.checkFact(t.signature, f); e != nil {
		return false, e
	}
	t.writeIntoReserve(f)
	if t.functional == 0 {
		return t.data.reserveContentAlreadyPresent(), nil
	}
	ofs, found := t.data.findReserveContent()
	if !found {
		return false, nil
	}
	// The functional columns, from first_functional() on (fixed fork:
	// FORK FIX #116; 4.7.1 started at functional_columns()).
	for i := t.firstFunctional(); i < t.signature.Len(); i++ {
		if t.getCell(ofs, i) != f[i] {
			return false, nil
		}
	}
	return true, nil
}

// Remove is sparse_table::remove_fact without its verbose_action.
func (t *SparseTable) Remove(f Fact) (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	if e := t.layout.checkFact(t.signature, f); e != nil {
		return false, e
	}
	return t.removeFact(f), nil
}

// removeFact is sparse_table::remove_fact's body: the fact goes into the
// reserve to find its stored row.
func (t *SparseTable) removeFact(f Fact) bool {
	t.writeIntoReserve(f)
	if !t.data.removeReserveContent() {
		// the fact was not in the table
		return false
	}
	t.resetIndexes()
	return true
}

func (t *SparseTable) Len() (int, error) {
	if e := t.open(); e != nil {
		return 0, e
	}
	return t.data.entryCount(), nil
}

// Empty is sparse_table::empty.
func (t *SparseTable) Empty() (bool, error) {
	if e := t.open(); e != nil {
		return false, e
	}
	return t.data.entryCount() == 0, nil
}

// SizeEstimateRows is sparse_table::get_size_estimate_rows (row_count).
func (t *SparseTable) SizeEstimateRows() (uint32, error) {
	rows, err := t.Len()
	return uint32(rows), err
}

// SizeEstimateBytes is sparse_table::get_size_estimate_bytes: the storage's
// data and indexer capacities and 8 bytes per m_key_indexes slot.
func (t *SparseTable) SizeEstimateBytes() (uint32, error) {
	if err := t.open(); err != nil {
		return 0, err
	}
	return t.sizeEstimateBytes(), nil
}

func (t *SparseTable) sizeEstimateBytes() uint32 {
	sz := t.data.getSizeEstimateBytes()
	sz += uint32(t.keyIndexes.capacity) * 8
	return sz
}

func (t *SparseTable) KnowsExactSize() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	return true, nil
}

// Facts reads the rows in storage order (our_iterator_core).
func (t *SparseTable) Facts() ([]Fact, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	afterLast := t.data.afterLastOffset()
	r := make([]Fact, 0, t.data.entryCount())
	for ofs := storeOffset(0); ofs < afterLast; ofs += storeOffset(t.factSize) {
		r = append(r, t.layout.unpack(t.data.get(ofs)))
	}
	return r, nil
}

// Clone is sparse_table::clone, sparse_table_plugin::mk_clone.
func (t *SparseTable) Clone() (*SparseTable, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return t.plugin.mkClone(t)
}

// Reset is sparse_table::reset.
func (t *SparseTable) Reset() error {
	if e := t.open(); e != nil {
		return e
	}
	t.reset()
	return nil
}

func (t *SparseTable) reset() {
	t.resetIndexes()
	t.data.reset()
}

// swapStorage exchanges the storage of two tables of one signature.
func (t *SparseTable) swapStorage(o *SparseTable) {
	t.data, o.data = o.data, t.data
	t.keyIndexes, o.keyIndexes = o.keyIndexes, t.keyIndexes
}

// release drops the rows without sparse_table::deallocate: a table the
// source never deallocates (it leaks) prints no recycle line.
func (t *SparseTable) release() error {
	if t == nil {
		return ErrTable
	}
	t.data = nil
	return nil
}

// Close is sparse_table::deallocate, sparse_table_plugin::recycle.
func (t *SparseTable) Close() error {
	if t == nil {
		return ErrTable
	}
	if t.data == nil {
		return nil
	}
	return t.plugin.recycle(t)
}

// sparseKeyIndexMap is sparse_table::m_key_indexes, a map from key specs to
// key indexers (core_hashtable). The indexers keep no state g3 observes;
// the map's capacity enters get_size_estimate_bytes. Its entries are only
// removed by reset, so the capacity follows from the number of specs.
type sparseKeyIndexMap struct {
	specs    map[string]struct{}
	capacity int
}

func newSparseKeyIndexMap() *sparseKeyIndexMap {
	return &sparseKeyIndexMap{capacity: offsetHashInitialCapacity}
}

// insertIfNotThere is the key_index_map's insert_if_not_there2: the table
// expands before the lookup.
func (m *sparseKeyIndexMap) insertIfNotThere(spec []int) {
	if len(m.specs)<<2 > m.capacity*3 {
		m.capacity <<= 1
	}
	if m.specs == nil {
		m.specs = make(map[string]struct{})
	}
	m.specs[factKey(intsFact(spec))] = struct{}{}
}

// reset is core_hashtable::reset: a mostly free table above 16 slots halves.
func (m *sparseKeyIndexMap) reset() {
	if len(m.specs) == 0 {
		return
	}
	overhead := m.capacity - len(m.specs)
	if m.capacity > 16 && overhead<<2 > m.capacity*3 {
		m.capacity >>= 1
	}
	m.specs = nil
}

func intsFact(columns []int) Fact {
	f := make(Fact, len(columns))
	for i, c := range columns {
		f[i] = Element(c)
	}
	return f
}

// resetIndexes is sparse_table::reset_indexes.
func (t *SparseTable) resetIndexes() { t.keyIndexes.reset() }

// sparseKeyIndexer is the key_indexer get_key_indexer returns: whether it
// is a full_signature_key_indexer, whose lookups write the key into the
// table's reserve, and its m_key_fact.
type sparseKeyIndexer struct {
	table         *SparseTable
	columns       []int
	fullSignature bool
	keyFact       Fact
}

// getKeyIndexer is sparse_table::get_key_indexer with its verbose_action
// _va("get_key_indexer").
func (t *SparseTable) getKeyIndexer(columns []int) (*sparseKeyIndexer, error) {
	action, err := base.StartVerboseAction("get_key_indexer", base.VerboseActionDefaultLevel)
	if err != nil {
		return nil, err
	}
	t.keyIndexes.insertIfNotThere(columns)
	indexer := &sparseKeyIndexer{table: t, columns: append([]int(nil), columns...)}
	if sparseFullSignatureCanHandle(columns, t) {
		indexer.fullSignature = true
		indexer.keyFact = make(Fact, t.signature.Len())
	}
	return indexer, action.End()
}

// sparseFullSignatureCanHandle is full_signature_key_indexer::can_handle:
// the key is a permutation of the non-functional columns.
func sparseFullSignatureCanHandle(columns []int, t *SparseTable) bool {
	nonFuncCols := t.firstFunctional()
	if len(columns) != nonFuncCols || nonFuncCols == 0 {
		return false
	}
	seen := make([]bool, nonFuncCols)
	for _, c := range columns {
		if c < 0 || c >= nonFuncCols || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}

// lookup is key_indexer::get_matching_offsets' effect on the table: a
// full_signature_key_indexer writes the key fact into the reserve (a
// general_key_indexer touches only its own storage).
func (k *sparseKeyIndexer) lookup(key Fact) {
	if !k.fullSignature {
		return
	}
	for i, c := range k.columns {
		k.keyFact[c] = key[i]
	}
	k.table.writeIntoReserve(k.keyFact)
}

// sparseKeyTracker is the key_value / key_modified pair of the source's
// probing loops: the key starts as zeros, modified.
type sparseKeyTracker struct {
	key      Fact
	modified bool
}

func newSparseKeyTracker(n int) *sparseKeyTracker {
	return &sparseKeyTracker{key: make(Fact, n), modified: true}
}

// update copies key in and reports whether the lookup must run again
// (key_modified), clearing the flag.
func (k *sparseKeyTracker) update(key Fact) bool {
	for i, v := range key {
		if k.key[i] != v {
			k.key[i] = v
			k.modified = true
		}
	}
	modified := k.modified
	k.modified = false
	return modified
}

// ContainsFact is sparse_table::contains_fact with its verbose_action
// _va("contains_fact", 2), for the source call sites of contains_fact
// (default_table_negation_filter_fn::should_remove).
func (t *SparseTable) ContainsFact(f Fact) (_ bool, result error) {
	action, err := base.StartVerboseAction("contains_fact", 2)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, action.End()) }()
	return t.Contains(f)
}
