package duality

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// iz3Table mirrors hash_space::hashtable (src/interp/iz3hash.h) as Duality
// uses it for its AST memo maps, so that the memo's references live as long
// and are released in the same order as in the source. Entries hash by AST
// id (hash<ast>) or by an int key, buckets have a prime count (initially 7),
// every inserting lookup first runs resize(entries + 1), new entries go to
// the head of their bucket, and clear() walks buckets in order, each chain
// from its head, destroying each pair's value before its key.
type iz3Table struct {
	buckets [][]*iz3Entry
	entries int
}

type iz3Entry struct {
	hash       uint64
	key, value ast.ASTRef
	inner      *iz3Table // the value of a hash_map<int, hash_map<ast,Term>>
}

// iz3hashPrimes is hash_space::primes (src/interp/iz3hash.h).
var iz3hashPrimes = [...]uint64{7, 53, 97, 193, 389, 769, 1543, 3079, 6151, 12289, 24593, 49157, 98317, 196613, 393241, 786433, 1572869, 3145739, 6291469, 12582917, 25165843, 50331653, 100663319, 201326611, 402653189, 805306457, 1610612741, 3221225473, 4294967291}

func newIZ3Table() *iz3Table {
	return &iz3Table{buckets: make([][]*iz3Entry, 7)}
}

// resize is hashtable::resize(entries + 1).
func (t *iz3Table) resize() {
	want := uint64(t.entries + 1)
	old := uint64(len(t.buckets))
	if want <= old {
		return
	}
	n := iz3hashPrimes[len(iz3hashPrimes)-1]
	for _, p := range iz3hashPrimes {
		if p >= want {
			n = p
			break
		}
	}
	if n <= old {
		return
	}
	next := make([][]*iz3Entry, n)
	for _, chain := range t.buckets {
		// Entries leave the old chain from its head and are pushed onto the
		// head of their new chain.
		for _, e := range chain {
			b := e.hash % n
			next[b] = append([]*iz3Entry{e}, next[b]...)
		}
	}
	t.buckets = next
}

// find is lookup_key: no resize.
func (t *iz3Table) find(hash uint64) *iz3Entry {
	for _, e := range t.buckets[hash%uint64(len(t.buckets))] {
		if e.hash == hash {
			return e
		}
	}
	return nil
}

// lookup is lookup(val, true): resize, then find or insert at the head.
func (t *iz3Table) lookup(hash uint64) (*iz3Entry, bool) {
	t.resize()
	if e := t.find(hash); e != nil {
		return e, false
	}
	e := &iz3Entry{hash: hash}
	b := hash % uint64(len(t.buckets))
	t.buckets[b] = append([]*iz3Entry{e}, t.buckets[b]...)
	t.entries++
	return e, true
}

// clear is hashtable::clear (also ~hashtable).
func (t *iz3Table) clear(m *ast.ASTManager) (result error) {
	for i := range t.buckets {
		for len(t.buckets[i]) > 0 {
			e := t.buckets[i][0]
			if e.inner != nil {
				result = errors.Join(result, e.inner.clear(m))
				e.inner = nil
			}
			if e.value != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&e.value))
			}
			if e.key != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&e.key))
			}
			t.buckets[i] = t.buckets[i][1:]
		}
	}
	t.entries = 0
	return result
}
