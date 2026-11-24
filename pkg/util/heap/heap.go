package heap

// Concrete port of src/util/heap.h; src/util/vector.h:102-129,191-202.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrValue = errors.New("g3: invalid, absent, or duplicate heap value")
var ErrOwner = errors.New("g3: invalid heap owner")

// Config borrows Less, its environment and an optional explicit DebugContext
// for the heap's lifetime. A nil Debug reads the current process owner.
// Less must be a strict ordering, must not panic or reenter the heap, and may
// change a present value's priority only together with Increased/Decreased.
// A nil Less compares the integer values themselves.
type Config struct {
	Budget *arena.Budget
	Less   func(int32, int32) bool
	Debug  *diagnostic.DebugContext
	// Activity, when non-nil, is the source's bool_var_act_lt functor
	// (smt_case_split_queue.cpp): less_than(v1, v2) is
	// (*Activity)[v1] > (*Activity)[v2]. It is read through the pointer
	// at every comparison, as the functor reads its svector<double>
	// reference, and replaces Less.
	Activity *[]float64
}

// Int stores source integer values and inverse positions in pointer-free
// arrays. It is owner-local; callers serialize operations and borrowed priorities.
// Do not copy an owning Int value; use Clone or Assign for source deep copies.
type Int struct {
	self            *Int
	values, indices []int32
	budget          *arena.Budget
	less            func(int32, int32) bool
	activity        *[]float64
	debug           *diagnostic.DebugContext
	epoch           uint64
	closed          bool
}

var ownerBytes = uint64(reflect.TypeOf(Int{}).Size())

type Stats struct {
	OwnerBytes, ValuesBytes, IndicesBytes uint64
}

func New(bounds int32, c Config) (*Int, error) {
	if bounds < 0 {
		return nil, arena.ErrBounds
	}
	if e := c.Budget.Reserve(ownerBytes); e != nil {
		return nil, e
	}
	h := &Int{budget: c.Budget, less: c.Less, activity: c.Activity, debug: c.Debug, epoch: 1}
	if c.Activity != nil {
		h.less = nil
	}
	h.self = h
	h.values = append(h.values, -1)
	if e := h.SetBounds(bounds); e != nil {
		h.Close()
		return nil, e
	}
	if e := h.cassertInvariant(); e != nil {
		h.Close()
		return nil, e
	}
	return h, nil
}

// Gate source assertions before resolving the debug context. This lets release
// builds inline the empty checks even in the heap invariant walker.
func (h *Int) debugContext() *diagnostic.DebugContext {
	if h.debug != nil {
		return h.debug
	}
	return diagnostic.BorrowProcessDebugContext()
}

// CASSERT first tests the process assertion switch and the "heap" tag. The
// current process owner is read at each gate unless an explicit one was lent.
func (h *Int) cassert(line int32, condition string, test func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := h.debugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	on, err = d.IsDebugEnabled("heap")
	if err != nil || !on {
		return err
	}
	if test() {
		return nil
	}
	return h.assertFailure(d, line, condition)
}

func (h *Int) assertFailure(d *diagnostic.DebugContext, line int32, condition string) error {
	if err := d.NotifyAssertionViolation("src/util/heap.h", line, condition); err != nil {
		return err
	}
	return d.InvokeAssertionDebugger()
}

// insert's outer SASSERT calls is_valid_value, whose own assertion reports
// line 57. A continuing debugger action returns to the outer assertion before
// the source's inverse-index write; the Go bounds guard below protects that
// source-undefined write.
func (h *Int) sassertInsertValue(v int32) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := h.debugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	if v < 0 || int64(v) >= int64(len(h.indices)) {
		return h.assertFailure(d, 57, "v >= 0 && v < static_cast<int>(m_value2indices.size())")
	}
	return nil
}

func (h *Int) sassert(line int32, condition string, test func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := h.debugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	if !test() {
		return h.assertFailure(d, line, condition)
	}
	return nil
}

func (h *Int) cassertInvariant() error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := h.debugContext()
	on, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !on {
		return err
	}
	on, err = d.IsDebugEnabled("heap")
	if err != nil || !on {
		return err
	}
	// check_invariant_core reports failures through nested SASSERTs. Its
	// Boolean return is always true once those actions continue.
	var walk func(int) error
	walk = func(i int) error {
		if i >= len(h.values) {
			return nil
		}
		v := h.values[i]
		if v < 0 || int64(v) >= int64(len(h.indices)) || h.indices[v] != int32(i) {
			if err := h.assertFailure(d, 63, "m_value2indices[m_values[idx]] == idx"); err != nil {
				return err
			}
		}
		if i > 1 && h.lt(v, h.values[i/2]) {
			if err := h.assertFailure(d, 64, "parent(idx) == 0 || !less_than(m_values[idx], m_values[parent(idx)])"); err != nil {
				return err
			}
		}
		if err := walk(i * 2); err != nil {
			return err
		}
		return walk(i*2 + 1)
	}
	return walk(1)
}

func (h *Int) alive() error {
	if h == nil || h.self != h {
		return ErrOwner
	}
	if h.closed {
		return arena.ErrClosed
	}
	return nil
}
func (h *Int) canChange() error {
	if e := h.alive(); e != nil {
		return e
	}
	if h.epoch == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	return nil
}
func (h *Int) lt(a, b int32) bool {
	if h.activity != nil {
		act := *h.activity
		return act[a] > act[b]
	}
	if h.less == nil {
		return a < b
	}
	return h.less(a, b)
}
func (h *Int) Empty() bool {
	return h.Size() == 0
}
func (h *Int) Size() uint32 {
	if h == nil || h.self != h || h.closed {
		return 0
	}
	return uint32(len(h.values) - 1)
}
func (h *Int) Bounds() int32 {
	if h == nil || h.self != h || h.closed {
		return 0
	}
	return int32(len(h.indices))
}
func (h *Int) Contains(v int32) bool {
	return h.position(v) != 0
}

func (h *Int) position(v int32) int32 {
	if h == nil || h.self != h || h.closed || v < 0 || int64(v) >= int64(len(h.indices)) {
		return 0
	}
	return h.indices[v]
}

// CheckInvariant checks live values, as the source traversal does.
func (h *Int) CheckInvariant() bool {
	if h == nil || h.self != h || h.closed || len(h.values) == 0 || h.values[0] != -1 {
		return false
	}
	for i := 1; i < len(h.values); i++ {
		v := h.values[i]
		if v < 0 || int64(v) >= int64(len(h.indices)) || h.indices[v] != int32(i) {
			return false
		}
		if i > 1 && h.lt(v, h.values[i/2]) {
			return false
		}
	}
	return true
}
func (h *Int) up(i int32) error {
	if h.activity != nil {
		h.placeActivityUp(i, h.values[i])
		return h.cassertInvariant()
	}
	v := h.values[i]
	for p := i / 2; p != 0 && h.lt(v, h.values[p]); p = i / 2 {
		h.values[i] = h.values[p]
		h.indices[h.values[i]] = i
		i = p
	}
	h.values[i] = v
	h.indices[v] = i
	return h.cassertInvariant()
}

func (h *Int) placeActivityUp(i, v int32) {
	values, indices, activity := h.values, h.indices, *h.activity
	score := activity[v]
	parent := i / 2
	for parent != 0 && score > activity[values[parent]] {
		moved := values[parent]
		values[i] = moved
		indices[moved] = i
		i = parent
		parent = i / 2
	}
	values[i] = v
	indices[v] = i
}

func (h *Int) placeActivityDown(i, v int32) {
	values, indices, activity := h.values, h.indices, *h.activity
	score := activity[v]
	for left := uint64(i) * 2; left < uint64(len(values)); left = uint64(i) * 2 {
		child := int32(left)
		if left+1 < uint64(len(values)) && activity[values[child+1]] > activity[values[child]] {
			child++
		}
		moved := values[child]
		if !(activity[moved] > score) {
			break
		}
		values[i] = moved
		indices[moved] = i
		i = child
	}
	values[i] = v
	indices[v] = i
}

// When erasing an interior variable, its replacement can move in either direction.
func (h *Int) replaceActivity(i, v int32) {
	activity := *h.activity
	if parent := i / 2; parent != 0 && activity[v] > activity[h.values[parent]] {
		h.placeActivityUp(i, v)
		return
	}
	h.placeActivityDown(i, v)
}

func (h *Int) down(i int32) error {
	if h.activity != nil {
		h.placeActivityDown(i, h.values[i])
		return h.cassertInvariant()
	}
	v := h.values[i]
	for {
		left := uint64(i) * 2
		if left >= uint64(len(h.values)) {
			break
		}
		child := int32(left)
		if left+1 < uint64(len(h.values)) && h.lt(h.values[child+1], h.values[child]) {
			child++
		}
		if !h.lt(h.values[child], v) {
			break
		}
		h.values[i] = h.values[child]
		h.indices[h.values[i]] = i
		i = child
	}
	h.values[i] = v
	h.indices[v] = i
	return h.cassertInvariant()
}

// TryInsertReservedActivity inserts v using the heap's existing storage.
// A false return leaves the heap untouched so the caller can use Reserve and Insert.
func (h *Int) TryInsertReservedActivity(v int32) bool {
	if diagnostic.DebugBuild || h == nil || h.self != h || h.closed || h.activity == nil || h.epoch == ^uint64(0) {
		return false
	}
	if uint(v) >= uint(len(h.indices)) || h.indices[v] != 0 || uint(v) >= uint(len(*h.activity)) {
		return false
	}
	if len(h.values) == cap(h.values) || len(h.values) >= 1<<31-1 {
		return false
	}
	i := len(h.values)
	h.values = h.values[:i+1]
	h.placeActivityUp(int32(i), v)
	h.epoch++
	return true
}

func (h *Int) Insert(v int32) error {
	if e := h.canChange(); e != nil {
		return e
	}
	if e := h.cassertInvariant(); e != nil {
		return e
	}
	if e := h.cassert(232, "!contains(val)", func() bool { return !h.Contains(v) }); e != nil {
		return e
	}
	if e := h.sassertInsertValue(v); e != nil {
		return e
	}
	if v < 0 || int64(v) >= int64(len(h.indices)) || h.indices[v] != 0 {
		return ErrValue
	}
	if len(h.values) == 1<<31-1 {
		return arena.ErrSlotLimit
	}
	i := int32(len(h.values))
	// Source publishes membership before appending the value.
	h.indices[v] = i
	h.epoch++
	h.values = append(h.values, v)
	if e := h.up(i); e != nil {
		return e
	}
	return h.cassertInvariant()
}
func (h *Int) Min() (int32, error) {
	if e := h.alive(); e != nil {
		return 0, e
	}
	if e := h.sassert(169, "!empty()", func() bool { return !h.Empty() }); e != nil {
		return 0, e
	}
	if h.Empty() {
		return 0, ErrValue
	}
	return h.values[1], nil
}
func (h *Int) EraseMin() (int32, error) {
	if e := h.alive(); e != nil {
		return 0, e
	}
	if e := h.cassertInvariant(); e != nil {
		return 0, e
	}
	if e := h.sassert(175, "!empty()", func() bool { return !h.Empty() }); e != nil {
		return 0, e
	}
	if e := h.sassert(176, "m_values.size() >= 2", func() bool { return len(h.values) >= 2 }); e != nil {
		return 0, e
	}
	if h.Empty() {
		return 0, ErrValue
	}
	v := h.values[1]
	h.indices[v] = 0
	last := h.values[len(h.values)-1]
	h.values = h.values[:len(h.values)-1]
	h.epoch++
	if !h.Empty() {
		h.values[1] = last
		h.indices[last] = 1
		if e := h.down(1); e != nil {
			return 0, e
		}
	} else if e := h.sassert(181, "empty()", h.Empty); e != nil {
		return 0, e
	}
	return v, h.cassertInvariant()
}
func (h *Int) Erase(v int32) error {
	if e := h.canChange(); e != nil {
		return e
	}
	if e := h.cassertInvariant(); e != nil {
		return e
	}
	if e := h.sassert(197, "contains(val)", func() bool { return h.Contains(v) }); e != nil {
		return e
	}
	return h.eraseAt(v, h.position(v))
}

// EraseIfPresent applies Contains followed by Erase. An absent value leaves
// the heap and its iterators untouched.
func (h *Int) EraseIfPresent(v int32) error {
	i := h.position(v)
	if i == 0 {
		return nil
	}
	if diagnostic.DebugBuild {
		return h.Erase(v)
	}
	if h.epoch == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	return h.eraseAt(v, i)
}

func (h *Int) eraseAt(v, i int32) error {
	if i < 1 || int64(i) >= int64(len(h.values)) {
		return ErrValue
	}
	last := h.takeLastForErase(v)
	if int(i) < len(h.values) {
		if h.activity != nil && !diagnostic.DebugBuild {
			h.replaceActivity(i, last)
			return nil
		}
		h.values[i] = last
		h.indices[last] = i
		if i/2 != 0 && h.lt(last, h.values[i/2]) {
			if e := h.up(i); e != nil {
				return e
			}
		} else {
			if e := h.down(i); e != nil {
				return e
			}
		}
	}
	return h.cassertInvariant()
}

// Remove v and return the former last value, which fills v's hole if the
// hole is still within the live heap.
func (h *Int) takeLastForErase(v int32) int32 {
	last := h.values[len(h.values)-1]
	h.indices[v] = 0
	h.values = h.values[:len(h.values)-1]
	h.epoch++
	return last
}

// A prepared erase borrows the heap until EraseSuffix. It has no allocation
// and permits a caller to validate two queues before changing either owner.
type activityErase struct {
	heap      *Int
	low, high int
	epoch     uint64
}

// PrepareActivityErase checks whether removing any suffix of [low, high) can
// proceed without callbacks or epoch overflow. Nil optional queues are empty.
// The owner must keep the heap and its borrowed activity slice unchanged until
// EraseSuffix. A false result leaves all membership and iterator state intact.
func (h *Int) PrepareActivityErase(scores *[]float64, low, high int) (activityErase, bool) {
	if low < 0 || high < low || scores == nil || high > len(*scores) {
		return activityErase{}, false
	}
	if h == nil {
		return activityErase{low: low, high: high}, true
	}
	if diagnostic.DebugBuild || h.self != h || h.closed || h.activity != scores || uint64(high-low) > ^uint64(0)-h.epoch {
		return activityErase{}, false
	}
	return activityErase{heap: h, low: low, high: high, epoch: h.epoch}, true
}

// EraseSuffix erases the last count IDs in descending order. As with a slice
// bound, count must fit the prepared range. An invalid count or mutation
// of the prepared heap causes a panic before any entries are removed.
func (r activityErase) EraseSuffix(count int) {
	if count < 0 || count > r.high-r.low {
		panic("heap: count exceeds prepared activity range")
	}
	h := r.heap
	if h == nil {
		return
	}
	if h.epoch != r.epoch || h.closed {
		panic("heap: prepared activity range was invalidated")
	}
	low, high := r.high-count, min(r.high, len(h.indices))
	if low >= high || len(h.values) <= 1 {
		return
	}
	// With at most one member outside the range, there is only one possible
	// final layout. Otherwise replay the exact descending erases, including ties.
	if len(h.values)-2 <= high-low && h.eraseAllButOne(low, high) {
		return
	}
	for v := high - 1; v >= low; v-- {
		if pos := h.indices[v]; pos != 0 {
			last := h.takeLastForErase(int32(v))
			if int(pos) < len(h.values) {
				h.replaceActivity(pos, last)
			}
		}
	}
}

func (h *Int) eraseAllButOne(low, high int) bool {
	var survivor int32 = -1
	for _, v := range h.values[1:] {
		if int(v) >= low && int(v) < high {
			continue
		}
		if survivor >= 0 {
			return false
		}
		survivor = v
	}
	end := 1
	if survivor >= 0 {
		h.values[1] = survivor
		h.indices[survivor] = 1
		end = 2
	}
	h.epoch += uint64(len(h.values) - end)
	h.values = h.values[:end]
	clear(h.indices[low:high])
	return true
}

func (h *Int) Increased(v int32) error {
	if e := h.canChange(); e != nil {
		return e
	}
	if e := h.sassert(226, "contains(val)", func() bool { return h.Contains(v) }); e != nil {
		return e
	}
	if !h.Contains(v) || h.indices[v] < 1 || int64(h.indices[v]) >= int64(len(h.values)) {
		// Guard an invalid inverse position before indexing the native slice.
		return ErrValue
	}
	h.epoch++
	if e := h.down(h.indices[v]); e != nil {
		return e
	}
	return nil
}
func (h *Int) Decreased(v int32) error {
	if e := h.canChange(); e != nil {
		return e
	}
	if e := h.sassert(221, "contains(val)", func() bool { return h.Contains(v) }); e != nil {
		return e
	}
	if !h.Contains(v) || h.indices[v] < 1 || int64(h.indices[v]) >= int64(len(h.values)) {
		// Guard an invalid inverse position before indexing the native slice.
		return ErrValue
	}
	h.epoch++
	if e := h.up(h.indices[v]); e != nil {
		return e
	}
	return nil
}
func (h *Int) SetBounds(n int32) error {
	if e := h.alive(); e != nil {
		return e
	}
	if n < 0 {
		return arena.ErrBounds
	}
	if int(n) == len(h.indices) {
		return h.cassertInvariant()
	}
	if e := h.canChange(); e != nil {
		return e
	}
	if int(n) < len(h.indices) {
		for _, v := range h.values[1:] {
			if v >= n {
				return arena.ErrBounds
			}
		}
	}
	old := len(h.indices)
	if int(n) > old {
		h.indices = append(h.indices, make([]int32, int(n)-old)...)
	} else {
		h.indices = h.indices[:n]
	}
	h.epoch++
	return h.cassertInvariant()
}
func (h *Int) Reserve(n int32) error {
	if e := h.alive(); e != nil {
		return e
	}
	if e := h.cassertInvariant(); e != nil {
		return e
	}
	if int64(n) > int64(len(h.indices)) {
		if e := h.SetBounds(n); e != nil {
			return e
		}
	}
	return h.cassertInvariant()
}

func (h *Int) Reset() error {
	if e := h.alive(); e != nil {
		return e
	}
	if e := h.cassertInvariant(); e != nil {
		return e
	}
	if h.Empty() {
		return nil
	}
	if e := h.canChange(); e != nil {
		return e
	}
	clear(h.indices)
	h.values = h.values[:1]
	h.values[0] = -1
	h.epoch++
	return h.cassertInvariant()
}
func (h *Int) Clear() error { return h.Reset() }
func (h *Int) AppendValues(dst []int32) ([]int32, error) {
	if e := h.alive(); e != nil {
		return dst, e
	}
	return append(dst, h.values[1:]...), nil
}

// Swap exchanges only the source value and inverse-index vectors, leaving
// comparators and temporary workspaces in place. Comparators must be compatible
// with each incoming heap, as required by the source invariant assertions.
func (h *Int) Swap(other *Int) error {
	if e := h.alive(); e != nil {
		return e
	}
	if e := other.alive(); e != nil {
		return e
	}
	if h == other {
		return nil
	}
	if e := other.cassertInvariant(); e != nil {
		return e
	}
	if e := h.cassertInvariant(); e != nil {
		return e
	}
	if e := h.canChange(); e != nil {
		return e
	}
	if e := other.canChange(); e != nil {
		return e
	}
	h.values, other.values = other.values, h.values
	h.indices, other.indices = other.indices, h.indices
	h.epoch++
	other.epoch++
	if e := other.cassertInvariant(); e != nil {
		return e
	}
	return h.cassertInvariant()
}

// Clone implements the source implicit heap copy constructor with native slices.
func (h *Int) Clone(budget *arena.Budget) (*Int, error) {
	if e := h.alive(); e != nil {
		return nil, e
	}
	if e := budget.Reserve(ownerBytes); e != nil {
		return nil, e
	}
	c := &Int{budget: budget, less: h.less, activity: h.activity, debug: h.debug, epoch: 1}
	c.self = c
	c.values = append([]int32(nil), h.values...)
	c.indices = append([]int32(nil), h.indices...)
	return c, nil
}

// Assign copies the comparator and both concrete value slices in source order.
func (h *Int) Assign(other *Int) error {
	if e := h.alive(); e != nil {
		return e
	}
	if e := other.alive(); e != nil {
		return e
	}
	if h == other {
		return nil
	}
	if e := h.canChange(); e != nil {
		return e
	}
	h.less = other.less
	h.activity = other.activity
	h.values = append(h.values[:0], other.values...)
	h.indices = append(h.indices[:0], other.indices...)
	h.epoch++
	return nil
}

func (h *Int) Stats() Stats {
	if h == nil || h.self != h || h.closed {
		return Stats{}
	}
	return Stats{OwnerBytes: ownerBytes}
}
func (h *Int) Close() error {
	if h != nil && h.self != h {
		return ErrOwner
	}
	if h == nil || h.closed {
		return nil
	}
	s := h.Stats()
	h.budget.Release(s.OwnerBytes)
	h.values = nil
	h.indices = nil
	h.less = nil
	h.activity = nil
	h.debug = nil
	h.closed = true
	return nil
}

// Iterator is cold checked metadata, never a raw internal slice.
type Iterator struct {
	owner *Int
	epoch uint64
	index int
}

func (h *Int) Begin() Iterator {
	if h == nil {
		return Iterator{}
	}
	return Iterator{owner: h, epoch: h.epoch, index: 1}
}
func (h *Int) End() Iterator {
	if h == nil {
		return Iterator{}
	}
	return Iterator{owner: h, epoch: h.epoch, index: len(h.values)}
}
func (it Iterator) valid() error {
	if e := it.owner.alive(); e != nil {
		return e
	}
	if it.epoch != it.owner.epoch || it.index < 1 || it.index > len(it.owner.values) {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (it Iterator) Value() (int32, error) {
	if e := it.valid(); e != nil {
		return 0, e
	}
	if it.index == len(it.owner.values) {
		return 0, arena.ErrBounds
	}
	return it.owner.values[it.index], nil
}
func (it *Iterator) Next() error {
	if it == nil {
		return arena.ErrInvalidSlot
	}
	if e := it.valid(); e != nil {
		return e
	}
	if it.index == len(it.owner.values) {
		return arena.ErrBounds
	}
	it.index++
	return nil
}
func (it Iterator) Equal(other Iterator) bool { return it == other }
