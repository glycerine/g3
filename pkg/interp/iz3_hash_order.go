package interp

// Source: src/interp/iz3hash.h hashtable, as instantiated by the
// hash_map<ast,...> and hash_set<ast> members of iz3base,
// iz3translation_full and iz3proof_itp_impl.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// iz3Primes is hash_space::primes, the bucket counts of a growing table.
var iz3Primes = [...]uint64{
	7, 53, 97, 193, 389, 769, 1543, 3079, 6151, 12289, 24593, 49157, 98317,
	196613, 393241, 786433, 1572869, 3145739, 6291469, 12582917, 25165843,
	50331653, 100663319, 201326611, 402653189, 805306457, 1610612741,
	3221225473, 4294967291,
}

func iz3NextPrime(n uint64) uint64 {
	for _, p := range iz3Primes {
		if p >= n {
			return p
		}
	}
	return iz3Primes[len(iz3Primes)-1]
}

type iz3RefEntry struct {
	next *iz3RefEntry
	id   uint32
	key  ast.ASTRef
	// values are the entry's AST-valued fields in declaration order: a
	// pair<ast,ast> value is (first, second). ~pair destroys the value
	// before the key, and a pair's second before its first.
	values []ast.ASTRef
}

// iz3RefTable holds the references of a source hash table keyed by an
// ast_r (hash<ast_r> is the AST id), with the source's bucket layout:
// lookup grows the table to next_prime(entries+1) before it inserts, a
// rehash moves each bucket's chain head-first onto the new buckets'
// heads, a new entry becomes its bucket's head, and clear (and the
// destructor) deletes the buckets in order, each chain from its head.
// The table only orders reference releases; callers keep their own maps.
type iz3RefTable struct {
	buckets []*iz3RefEntry
	entries int
	index   map[ast.ASTHandle]*iz3RefEntry
}

func (t *iz3RefTable) resize(newSize int) {
	oldN := len(t.buckets)
	if newSize <= oldN {
		return
	}
	n := int(iz3NextPrime(uint64(newSize)))
	if n <= oldN {
		return
	}
	table := make([]*iz3RefEntry, n)
	for i := 0; i < oldN; i++ {
		for entry := t.buckets[i]; entry != nil; entry = t.buckets[i] {
			bucket := int(uint64(entry.id) % uint64(n))
			t.buckets[i] = entry.next
			entry.next = table[bucket]
			table[bucket] = entry
		}
	}
	t.buckets = table
}

// lookup is hashtable::lookup(val, true) (operator[] and insert): the entry
// for key, made and holding a reference to key when absent.
func (t *iz3RefTable) lookup(manager *ast.ASTManager, key ast.ASTHandle) (*iz3RefEntry, error) {
	if t.buckets == nil {
		t.buckets = make([]*iz3RefEntry, 7)
		t.index = make(map[ast.ASTHandle]*iz3RefEntry)
	}
	t.resize(t.entries + 1)
	if entry := t.index[key]; entry != nil {
		return entry, nil
	}
	id, err := manager.GetASTID(key)
	if err != nil {
		return nil, err
	}
	ref, err := manager.NewRef(key)
	if err != nil {
		return nil, err
	}
	bucket := int(uint64(id) % uint64(len(t.buckets)))
	entry := &iz3RefEntry{next: t.buckets[bucket], id: id, key: ref}
	t.buckets[bucket] = entry
	t.index[key] = entry
	t.entries++
	return entry, nil
}

// insert is lookup for a table whose entries hold only their key.
func (t *iz3RefTable) insert(manager *ast.ASTManager, key ast.ASTHandle) error {
	_, err := t.lookup(manager, key)
	return err
}

// setValue assigns the entry's i-th AST-valued field (ast_r::operator=).
func (t *iz3RefTable) setValue(manager *ast.ASTManager, entry *iz3RefEntry, i int, value ast.ASTHandle) error {
	for len(entry.values) <= i {
		entry.values = append(entry.values, ast.ASTRef{})
	}
	var ref ast.ASTRef
	if !value.IsNull() {
		var err error
		if ref, err = manager.NewRef(value); err != nil {
			return err
		}
	}
	old := entry.values[i]
	entry.values[i] = ref
	if old != (ast.ASTRef{}) {
		return manager.DelRef(&old)
	}
	return nil
}

func (t *iz3RefTable) clear(manager *ast.ASTManager) (result error) {
	for i := range t.buckets {
		for entry := t.buckets[i]; entry != nil; {
			next := entry.next
			for j := len(entry.values) - 1; j >= 0; j-- {
				if entry.values[j] != (ast.ASTRef{}) {
					result = errors.Join(result, manager.DelRef(&entry.values[j]))
				}
			}
			result = errors.Join(result, manager.DelRef(&entry.key))
			entry = next
		}
		t.buckets[i] = nil
	}
	t.entries = 0
	if t.index != nil {
		clear(t.index)
	}
	return result
}

// iz3RefVector is a std::vector<ast> member: its destructor (and clear)
// releases the elements first to last.
type iz3RefVector []ast.ASTRef

func (v *iz3RefVector) push(manager *ast.ASTManager, handle ast.ASTHandle) error {
	ref, err := manager.NewRef(handle)
	if err != nil {
		return err
	}
	*v = append(*v, ref)
	return nil
}

func (v *iz3RefVector) clear(manager *ast.ASTManager) (result error) {
	for i := range *v {
		result = errors.Join(result, manager.DelRef(&(*v)[i]))
	}
	*v = nil
	return result
}

// iz3LocMaps is iz3proof_itp_impl::locmaps; ~locmaps destroys
// localization_pf_map, then localization_map.
type iz3LocMaps struct {
	terms  iz3RefTable // localization_map
	proofs iz3RefTable // localization_pf_map
}

type iz3RangeEntry struct {
	next  *iz3RangeEntry
	key   ScopeRange
	value *iz3LocMaps
}

// iz3RangeTable is localization_maps_per_range, a hash_map<prover::range,
// locmaps> with hash<scopes::range> = (size_t)lo + (size_t)hi; it only
// orders the releases of its locmaps.
type iz3RangeTable struct {
	buckets []*iz3RangeEntry
	entries int
	index   map[ScopeRange]*iz3RangeEntry
}

func iz3RangeHash(r ScopeRange) uint64 {
	return uint64(int64(int16(r.Lo))) + uint64(int64(int16(r.Hi)))
}

func (t *iz3RangeTable) resize(newSize int) {
	oldN := len(t.buckets)
	if newSize <= oldN {
		return
	}
	n := int(iz3NextPrime(uint64(newSize)))
	if n <= oldN {
		return
	}
	table := make([]*iz3RangeEntry, n)
	for i := 0; i < oldN; i++ {
		for entry := t.buckets[i]; entry != nil; entry = t.buckets[i] {
			bucket := int(iz3RangeHash(entry.key) % uint64(n))
			t.buckets[i] = entry.next
			entry.next = table[bucket]
			table[bucket] = entry
		}
	}
	t.buckets = table
}

// lookup is localization_maps_per_range[r].
func (t *iz3RangeTable) lookup(r ScopeRange) *iz3LocMaps {
	if t.buckets == nil {
		t.buckets = make([]*iz3RangeEntry, 7)
		t.index = make(map[ScopeRange]*iz3RangeEntry)
	}
	t.resize(t.entries + 1)
	if entry := t.index[r]; entry != nil {
		return entry.value
	}
	bucket := int(iz3RangeHash(r) % uint64(len(t.buckets)))
	entry := &iz3RangeEntry{next: t.buckets[bucket], key: r, value: &iz3LocMaps{}}
	t.buckets[bucket] = entry
	t.index[r] = entry
	t.entries++
	return entry.value
}

func (t *iz3RangeTable) clear(manager *ast.ASTManager) (result error) {
	for i := range t.buckets {
		for entry := t.buckets[i]; entry != nil; entry = entry.next {
			result = errors.Join(result, entry.value.proofs.clear(manager))
			result = errors.Join(result, entry.value.terms.clear(manager))
		}
		t.buckets[i] = nil
	}
	t.entries = 0
	if t.index != nil {
		clear(t.index)
	}
	return result
}

// set is map[key] = value on a hash_map<ast,ast>.
func (t *iz3RefTable) set(manager *ast.ASTManager, key, value ast.ASTHandle) error {
	entry, err := t.lookup(manager, key)
	if err != nil {
		return err
	}
	return t.setValue(manager, entry, 0, value)
}
