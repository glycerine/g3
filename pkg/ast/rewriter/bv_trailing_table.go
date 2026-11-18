package rewriter

// Source: src/ast/rewriter/bv_trailing.cpp:338-379.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvTrailingEntry struct {
	key      ast.ASTHandle
	min, max uint32
	owned    bool
}

// bvTrailingTable is the per-depth obj_map<expr, std::pair<unsigned,
// unsigned>>. Each entry owns one key reference. reset_cache releases the
// keys in obj_map iteration (table slot) order, so the table reproduces that
// layout with ast.ObjMap; Num packs min (high half) and max (low half).
// released records the keys a stopped cleanup already released, so a retry
// never decrements a key twice. Native storage growth has no
// numerical/object arena charge.
type bvTrailingTable struct {
	entries  *ast.ObjMap
	released map[ast.ASTHandle]bool
	header   uint64
}

func newBVTrailingTable(m *ast.ASTManager) (*bvTrailingTable, error) {
	header := uint64(reflect.TypeOf(bvTrailingTable{}).Size())
	if e := m.Budget().Reserve(header); e != nil {
		return nil, e
	}
	return &bvTrailingTable{entries: ast.NewObjMap(m), header: header}, nil
}

func bvTrailingUnpack(key ast.ASTHandle, num uint64, owned bool) bvTrailingEntry {
	return bvTrailingEntry{key: key, min: uint32(num >> 32), max: uint32(num), owned: owned}
}

func (t *bvTrailingTable) find(m *ast.ASTManager, key ast.ASTHandle) (bvTrailingEntry, bool, error) {
	if _, e := m.GetHash(key); e != nil {
		return bvTrailingEntry{}, false, e
	}
	num, found, e := t.entries.FindNum(key)
	if e != nil || !found {
		return bvTrailingEntry{}, false, e
	}
	return bvTrailingUnpack(key, num, !t.released[key]), true, nil
}

// insert is obj_map::insert(key, (min, max)); hash is the key's hash, read
// before the caller acquired the key reference.
func (t *bvTrailingTable) insert(key ast.ASTHandle, hash uint32, min, max uint32) {
	entry := t.entries.InsertIfNotThereHashed(key, hash, func(k ast.ASTHandle) bool { return k == key })
	entry.Num = uint64(min)<<32 | uint64(max)
}

func (t *bvTrailingTable) len() int { return t.entries.Len() }

// ordered lists the entries in obj_map iteration order.
func (t *bvTrailingTable) ordered() []bvTrailingEntry {
	all := t.entries.Entries()
	out := make([]bvTrailingEntry, len(all))
	for i, entry := range all {
		out[i] = bvTrailingUnpack(entry.Key, entry.Num, !t.released[entry.Key])
	}
	return out
}

func (t *bvTrailingTable) close(m *ast.ASTManager) error {
	for _, entry := range t.ordered() {
		if !entry.owned {
			continue
		}
		consumed, e := m.DecRefConsumed(entry.key)
		if consumed {
			if t.released == nil {
				t.released = map[ast.ASTHandle]bool{}
			}
			t.released[entry.key] = true
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	m.Budget().Release(t.header)
	t.entries = ast.NewObjMap(m)
	t.released = nil
	t.header = 0
	return nil
}
