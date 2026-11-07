package smtcore

// Source: active implementation in src/smt/smt_cg_table.h and
// src/smt/smt_cg_table.cpp.

import (
	"encoding/binary"
	"github.com/glycerine/g3/pkg/ast"
)

// Native Go signature keys replace the source per-declaration unary, binary,
// commutative-binary and n-ary hash tables. Arity one and two key on root
// pointers, so one enode per key suffices. Wider keys encode root owners in a
// suffix, which distinct roots can share, so they keep collision chains whose
// entries still pass the exact source congruence comparison.
type smtCongruenceKey struct {
	first, second *SMTEnode
	tail          string
}
type smtCongruenceBucket struct {
	id          uint32
	declaration ast.ASTHandle
	small       map[[2]*SMTEnode]*SMTEnode // arity one and two, without a string key
	wide        map[smtCongruenceKey][]*SMTEnode
}

func smtCongruenceSmallKey(node *SMTEnode) [2]*SMTEnode {
	key := [2]*SMTEnode{node.arguments[0].root}
	if len(node.arguments) > 1 {
		key[1] = node.arguments[1].root
	}
	return key
}

func smtCongruenceSignature(node *SMTEnode) smtCongruenceKey {
	key := smtCongruenceKey{first: node.arguments[0].root}
	if len(node.arguments) > 1 {
		key.second = node.arguments[1].root
	}
	if len(node.arguments) > 2 {
		data := make([]byte, 0, 16*(len(node.arguments)-2))
		for _, arg := range node.arguments[2:] {
			h := arg.root.owner
			data = binary.LittleEndian.AppendUint64(data, h.Origin())
			data = binary.LittleEndian.AppendUint32(data, h.Slot())
			data = binary.LittleEndian.AppendUint32(data, h.Generation())
		}
		key.tail = string(data)
	}
	return key
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
	if candidate := b.small[key]; candidate != nil {
		congruent, commuted, err := SMTEnodesCongruent(candidate, node)
		if err != nil || congruent {
			return candidate, commuted, err
		}
	}
	if node.commutative && len(node.arguments) == 2 && key[0] != key[1] {
		if candidate := b.small[[2]*SMTEnode{key[1], key[0]}]; candidate != nil {
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
		b.small[smtCongruenceSmallKey(node)] = node
		return
	}
	key := smtCongruenceSignature(node)
	b.wide[key] = append(b.wide[key], node)
}

// remove deletes the stored node itself, under its own current signature.
func (b *smtCongruenceBucket) remove(node *SMTEnode) {
	if len(node.arguments) <= 2 {
		delete(b.small, smtCongruenceSmallKey(node))
		return
	}
	key := smtCongruenceSignature(node)
	chain := b.wide[key]
	for i, entry := range chain {
		if entry != node {
			continue
		}
		if len(chain) == 1 {
			delete(b.wide, key)
			return
		}
		copy(chain[i:], chain[i+1:])
		chain[len(chain)-1] = nil
		b.wide[key] = chain[:len(chain)-1]
		return
	}
}

type SMTCongruenceInsertResult struct {
	Node     *SMTEnode
	Commuted bool
}

type SMTCongruenceTable struct {
	self         *SMTCongruenceTable
	manager      *ast.ASTManager
	tables       map[ast.ASTHandle]*smtCongruenceBucket
	buckets      []*smtCongruenceBucket // by enode functionDeclID, to skip the map
	declarations []ast.ASTHandle
	resetIndex   int
	closed       bool
}

func NewSMTCongruenceTable(manager *ast.ASTManager) (*SMTCongruenceTable, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	table := &SMTCongruenceTable{manager: manager, tables: make(map[ast.ASTHandle]*smtCongruenceBucket)}
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
	bucket := t.tables[node.declaration]
	if bucket != nil {
		node.functionDeclID = bucket.id
	}
	return bucket
}

func (t *SMTCongruenceTable) ensureTable(node *SMTEnode) (*smtCongruenceBucket, error) {
	if bucket := t.bucketFor(node); bucket != nil {
		return bucket, nil
	}
	if err := t.manager.IncRef(node.declaration); err != nil {
		return nil, err
	}
	node.functionDeclID = uint32(len(t.declarations))
	t.declarations = append(t.declarations, node.declaration)
	bucket := &smtCongruenceBucket{
		id: node.functionDeclID, declaration: node.declaration,
		small: make(map[[2]*SMTEnode]*SMTEnode), wide: make(map[smtCongruenceKey][]*SMTEnode),
	}
	t.buckets = append(t.buckets, bucket)
	t.tables[node.declaration] = bucket
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
	bucket := t.bucketFor(node)
	if bucket == nil {
		return nil, false, nil
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

func (t *SMTCongruenceTable) Erase(node *SMTEnode) (bool, error) {
	candidate, _, err := t.Find(node)
	if err != nil || candidate == nil {
		return false, err
	}
	t.bucketFor(node).remove(candidate)
	return true, nil
}

// forEachEntry visits stored enodes, reporting whether each sits under its
// current signature.
func (t *SMTCongruenceTable) forEachEntry(visit func(node *SMTEnode, current bool)) {
	for _, bucket := range t.buckets {
		for key, node := range bucket.small {
			visit(node, key == smtCongruenceSmallKey(node))
		}
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
	if t.resetIndex == 0 {
		t.resetIndex = len(t.declarations)
	}
	for t.resetIndex > 0 {
		i := t.resetIndex - 1
		handle := t.declarations[i]
		consumed, err := t.manager.DecRefConsumed(handle)
		if consumed {
			t.declarations[i] = ast.ASTHandle{}
			t.resetIndex--
		}
		if err != nil {
			return err
		}
	}
	t.declarations, t.buckets = nil, nil
	t.tables = make(map[ast.ASTHandle]*smtCongruenceBucket)
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
