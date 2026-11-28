package smtcore

// Source: active implementation in src/smt/smt_cg_table.h and
// src/smt/smt_cg_table.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// Native Go signature keys replace the source per-declaration unary, binary,
// commutative-binary and n-ary hash tables. Arity one and two key on root
// pointers, so one enode per key suffices. Wider keys encode root owners in a
// suffix, which distinct roots can share, so they keep collision chains whose
// entries still pass the exact source congruence comparison.
type smtCongruenceBucket struct {
	id          uint32
	declaration ast.ASTHandle
	small       smtCgSmallTable // arity one and two, without a string key
	// wide holds the nodes of arity above two by smtCongruenceSignature;
	// a chain can mix signatures that share the hash, and lookups test
	// each candidate with SMTEnodesCongruent, so the first congruent
	// entry is the one an exact-signature chain would yield.
	wide map[uint64][]*SMTEnode
	// flatAssociative is mk_table_for's choice of the positional n-ary
	// table, not comm_table, for a flat-associative binary declaration.
	flatAssociative bool
}

func smtCongruenceSmallKey(node *SMTEnode) [2]*SMTEnode {
	key := [2]*SMTEnode{node.arguments[0].root}
	if len(node.arguments) > 1 {
		key[1] = node.arguments[1].root
	}
	return key
}

// smtCongruenceSignature hashes the owners of the argument roots.
func smtCongruenceSignature(node *SMTEnode) uint64 {
	h := uint64(0xcbf29ce484222325) ^ uint64(len(node.arguments))
	for _, arg := range node.arguments {
		h = smtHandleMix(h, arg.root.owner)
	}
	return h
}

// find returns the congruent entry for a node, if any.
func (b *smtCongruenceBucket) find(node *SMTEnode) (*SMTEnode, bool, error) {
	if len(node.arguments) > 2 {
		for _, candidate := range b.wide[smtCongruenceSignature(node)] {
			congruent, commuted, err := SMTEnodesCongruent(candidate, node)
			if err != nil {
				return nil, false, err
			}
			if congruent {
				return candidate, commuted, nil
			}
		}
		return nil, false, nil
	}
	key := smtCongruenceSmallKey(node)
	if candidate := b.small.get(key[0], key[1]); candidate != nil {
		congruent, commuted, err := SMTEnodesCongruent(candidate, node)
		if err != nil || congruent {
			return candidate, commuted, err
		}
	}
	if node.commutative && !b.flatAssociative && len(node.arguments) == 2 && key[0] != key[1] {
		if candidate := b.small.get(key[1], key[0]); candidate != nil {
			congruent, commuted, err := SMTEnodesCongruent(candidate, node)
			if err != nil || congruent {
				return candidate, commuted, err
			}
		}
	}
	return nil, false, nil
}

func (b *smtCongruenceBucket) store(node *SMTEnode) {
	if len(node.arguments) <= 2 {
		key := smtCongruenceSmallKey(node)
		b.small.set(key[0], key[1], node)
		return
	}
	if b.wide == nil {
		b.wide = make(map[uint64][]*SMTEnode)
	}
	key := smtCongruenceSignature(node)
	b.wide[key] = append(b.wide[key], node)
}

func (b *smtCongruenceBucket) eraseWide(node *SMTEnode) (bool, error) {
	key := smtCongruenceSignature(node)
	chain := b.wide[key]
	for i, entry := range chain {
		congruent, _, err := SMTEnodesCongruent(entry, node)
		if err != nil {
			return false, err
		}
		if !congruent {
			continue
		}
		if len(chain) == 1 {
			delete(b.wide, key)
		} else {
			copy(chain[i:], chain[i+1:])
			chain[len(chain)-1] = nil
			b.wide[key] = chain[:len(chain)-1]
		}
		return true, nil
	}
	return false, nil
}

type SMTCongruenceInsertResult struct {
	Node     *SMTEnode
	Commuted bool
}

type SMTCongruenceTable struct {
	// FindApp repeatedly visits the same declarations during candidate checks.
	appBuckets [8]*smtCongruenceBucket
	self       *SMTCongruenceTable
	manager    *ast.ASTManager
	tables     map[uint64]*smtCongruenceBucket // m_func_decl2table, keyed by smtHandleKey(decl)
	buckets    []*smtCongruenceBucket          // by enode functionDeclID, to skip the map
	// declarationOrder is m_func_decl2id (obj_map<func_decl, unsigned>),
	// whose table order reset() releases the declarations in;
	// resetOrder and resetIndex resume a refused release.
	declarationOrder *ast.ObjMap
	resetOrder       []ast.ASTHandle
	resetIndex       int
	closed           bool
}

func NewSMTCongruenceTable(manager *ast.ASTManager) (*SMTCongruenceTable, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	table := &SMTCongruenceTable{manager: manager, tables: make(map[uint64]*smtCongruenceBucket)}
	table.self = table
	return table, nil
}

func (t *SMTCongruenceTable) open() error {
	if t == nil || t.self != t || t.manager == nil || t.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (t *SMTCongruenceTable) validateNode(node *SMTEnode) error {
	if err := t.open(); err != nil {
		return err
	}
	if node == nil || node.manager != t.manager || len(node.arguments) == 0 {
		return ast.ErrASTDomain
	}
	return node.valid()
}

func (t *SMTCongruenceTable) bucketFor(node *SMTEnode) *smtCongruenceBucket {
	if id := node.functionDeclID; uint64(id) < uint64(len(t.buckets)) {
		if bucket := t.buckets[id]; bucket.declaration == node.declaration {
			return bucket
		}
	}
	bucket := t.tables[smtHandleKey(node.declaration)]
	if bucket != nil {
		node.functionDeclID = bucket.id
	}
	return bucket
}

func (t *SMTCongruenceTable) ensureTable(node *SMTEnode) (*smtCongruenceBucket, error) {
	if bucket := t.bucketFor(node); bucket != nil {
		return bucket, nil
	}
	flags, err := t.manager.GetFuncDeclFlags(node.declaration)
	if err != nil {
		return nil, err
	}
	if err := t.manager.IncRef(node.declaration); err != nil {
		return nil, err
	}
	node.functionDeclID = uint32(len(t.buckets))
	if t.declarationOrder == nil {
		t.declarationOrder = ast.NewObjMap(t.manager)
	}
	if err := t.declarationOrder.InsertNum(node.declaration, uint64(node.functionDeclID)); err != nil {
		return nil, err
	}
	bucket := &smtCongruenceBucket{
		id: node.functionDeclID, declaration: node.declaration,
		flatAssociative: flags.FlatAssociative,
	}
	t.buckets = append(t.buckets, bucket)
	t.tables[smtHandleKey(node.declaration)] = bucket
	return bucket, nil
}

func (t *SMTCongruenceTable) Insert(node *SMTEnode) (SMTCongruenceInsertResult, error) {
	if err := t.validateNode(node); err != nil {
		return SMTCongruenceInsertResult{}, err
	}
	bucket, err := t.ensureTable(node)
	if err != nil {
		return SMTCongruenceInsertResult{}, err
	}
	candidate, commuted, err := bucket.find(node)
	if err != nil {
		return SMTCongruenceInsertResult{}, err
	}
	if candidate != nil {
		return SMTCongruenceInsertResult{Node: candidate, Commuted: commuted}, nil
	}
	bucket.store(node)
	return SMTCongruenceInsertResult{Node: node}, nil
}

func (t *SMTCongruenceTable) Find(node *SMTEnode) (*SMTEnode, bool, error) {
	if err := t.validateNode(node); err != nil {
		return nil, false, err
	}
	bucket, err := t.ensureTable(node)
	if err != nil {
		return nil, false, err
	}
	return bucket.find(node)
}

func (t *SMTCongruenceTable) Contains(node *SMTEnode) (bool, error) {
	found, _, err := t.Find(node)
	return found != nil, err
}

func (t *SMTCongruenceTable) ContainsPointer(node *SMTEnode) (bool, error) {
	found, _, err := t.Find(node)
	return found == node, err
}

// Merge and undo usually remove the stored parent itself. Another table may
// have updated its slot hint, or a merge may have changed its argument roots.
func (t *SMTCongruenceTable) eraseParent(parent *SMTEnode) (bool, error) {
	if err := t.validateNode(parent); err != nil {
		return false, err
	}
	arity := len(parent.arguments)
	if arity <= 2 && (!parent.commutative || arity == 2) && uint(parent.functionDeclID) < uint(len(t.buckets)) {
		bucket := t.buckets[parent.functionDeclID]
		slot := parent.congruenceSlot
		if bucket.declaration == parent.declaration && uint(slot) < uint(len(bucket.small.entries)) {
			entry := &bucket.small.entries[slot]
			if entry.node == parent {
				key := smtCongruenceSmallKey(parent)
				if entry.k0 == key[0] && entry.k1 == key[1] {
					bucket.small.deleteSlot(int(slot))
					return true, nil
				}
			}
		}
	}
	return t.eraseValidated(parent)
}

func (t *SMTCongruenceTable) Erase(node *SMTEnode) (bool, error) {
	if err := t.validateNode(node); err != nil {
		return false, err
	}
	return t.eraseValidated(node)
}

func (t *SMTCongruenceTable) eraseValidated(node *SMTEnode) (bool, error) {
	bucket, err := t.ensureTable(node)
	if err != nil {
		return false, err
	}
	if len(node.arguments) > 2 {
		return bucket.eraseWide(node)
	}
	key := smtCongruenceSmallKey(node)
	erased, err := bucket.small.eraseCongruent(key, node)
	if err == nil && !erased && node.commutative && !bucket.flatAssociative && len(node.arguments) == 2 && key[0] != key[1] {
		return bucket.small.eraseCongruent([2]*SMTEnode{key[1], key[0]}, node)
	}
	return erased, err
}

// forEachEntry visits stored enodes, reporting whether each sits under its
// current signature.
func (t *SMTCongruenceTable) forEachEntry(visit func(node *SMTEnode, current bool)) {
	for _, bucket := range t.buckets {
		bucket.small.forEach(func(k0, k1, node *SMTEnode) {
			visit(node, [2]*SMTEnode{k0, k1} == smtCongruenceSmallKey(node))
		})
		for key, chain := range bucket.wide {
			for _, node := range chain {
				visit(node, key == smtCongruenceSignature(node))
			}
		}
	}
}

func (t *SMTCongruenceTable) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	// for (it2 : m_func_decl2id) m_manager.dec_ref(it2->m_key), in table order.
	if t.resetOrder == nil && t.declarationOrder != nil {
		for _, entry := range t.declarationOrder.Entries() {
			t.resetOrder = append(t.resetOrder, entry.Key)
		}
		t.resetIndex = 0
	}
	for t.resetIndex < len(t.resetOrder) {
		consumed, err := t.manager.DecRefConsumed(t.resetOrder[t.resetIndex])
		if consumed {
			t.resetIndex++
		}
		if err != nil {
			return err
		}
	}
	t.resetOrder, t.resetIndex = nil, 0
	if t.declarationOrder != nil {
		t.declarationOrder.Reset()
	}
	clear(t.appBuckets[:])
	t.buckets = nil
	t.tables = make(map[uint64]*smtCongruenceBucket)
	return nil
}

func (t *SMTCongruenceTable) CheckInvariant() error {
	if err := t.open(); err != nil {
		return err
	}
	var failure error
	t.forEachEntry(func(node *SMTEnode, current bool) {
		if failure != nil {
			return
		}
		contained, err := t.ContainsPointer(node)
		if err != nil {
			failure = err
		} else if !contained || !current {
			failure = ast.ErrASTDomain
		}
	})
	return failure
}

// Display and DisplayCompact are intentionally empty in the active source
// implementation.
func (t *SMTCongruenceTable) Display() error        { return t.open() }
func (t *SMTCongruenceTable) DisplayCompact() error { return t.open() }

func (t *SMTCongruenceTable) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.Reset(); err != nil {
		return err
	}
	t.manager = nil
	t.closed = true
	return nil
}

// FindApp is context::get_enode_eq_to(f, num_args, args): the table entry
// congruent to f(args), through a temporary enode as m_tmp_enode does.
func (t *SMTCongruenceTable) FindApp(decl ast.ASTHandle, args []*SMTEnode, commutative bool) (*SMTEnode, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, nil
	}
	// cg_table::get_table registers the declaration even when this lookup
	// misses. Its reference lives until Reset, which affects AST ID reuse.
	tmp := SMTEnode{manager: t.manager, declaration: decl, arguments: args, commutative: commutative && len(args) == 2}
	slot := &t.appBuckets[decl.Slot()%uint32(len(t.appBuckets))]
	bucket := *slot
	if bucket == nil || bucket.declaration != decl {
		var err error
		bucket, err = t.ensureTable(&tmp)
		if err != nil {
			return nil, err
		}
		*slot = bucket
	}
	found, _, err := bucket.find(&tmp)
	return found, err
}
