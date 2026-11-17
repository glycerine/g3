package smtcore

// Source: src/smt/cached_var_subst.h and src/smt/cached_var_subst.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type smtCachedSubstitutionEntry struct {
	quantifier ast.ASTHandle
	bindings   []ast.ASTHandle
	result     ast.ASTHandle
	// The source expr_ref_vector increments AST ownership directly. Do not
	// manufacture a manager ASTRef slot for each retained handle.
	owned, released int
}

type SMTCachedVarSubstitution struct {
	self         *SMTCachedVarSubstitution
	manager      *ast.ASTManager
	substitution *rewriter.VarSubstitution
	// entries is cached_var_subst::m_instances, a map keyed by (qa,
	// bindings) under key_hash_proc (a hash of the whole binding tuple and
	// qa's id). Buckets hold only hash collisions.
	entries map[uint64][]*smtCachedSubstitutionEntry
	ordered []*smtCachedSubstitutionEntry
	closed  bool
}

func NewSMTCachedVarSubstitution(m *ast.ASTManager) (*SMTCachedVarSubstitution, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	c := &SMTCachedVarSubstitution{manager: m, entries: make(map[uint64][]*smtCachedSubstitutionEntry)}
	c.self = c
	var err error
	c.substitution, err = rewriter.NewVarSubstitution(m, true)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *SMTCachedVarSubstitution) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil || c.substitution == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// smtHandleMix folds one AST handle into a tuple hash.
func smtHandleMix(h uint64, x ast.ASTHandle) uint64 {
	for _, v := range [3]uint64{uint64(x.Slot()), uint64(x.Generation()), x.Origin()} {
		h ^= v
		h *= 0x100000001b3
		h ^= h >> 29
	}
	return h
}

// smtSubstitutionKeyHash is key_hash_proc: the hash of the quantifier and
// every binding.
func smtSubstitutionKeyHash(quantifier ast.ASTHandle, bindings []ast.ASTHandle) uint64 {
	h := smtHandleMix(0xcbf29ce484222325^uint64(len(bindings)), quantifier)
	for _, b := range bindings {
		h = smtHandleMix(h, b)
	}
	return h
}

func (c *SMTCachedVarSubstitution) find(hash uint64, quantifier ast.ASTHandle, bindings []ast.ASTHandle) *smtCachedSubstitutionEntry {
	for _, entry := range c.entries[hash] {
		if entry.quantifier != quantifier || len(entry.bindings) != len(bindings) {
			continue
		}
		equal := true
		for i := range bindings {
			if entry.bindings[i] != bindings[i] {
				equal = false
				break
			}
		}
		if equal {
			return entry
		}
	}
	return nil
}

func (c *SMTCachedVarSubstitution) Instantiate(quantifier ast.ASTHandle, bindings []*SMTEnode, out ast.ASTRef) (cached bool, result error) {
	if err := c.open(); err != nil {
		return false, err
	}
	kind, err := c.manager.GetKind(quantifier)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTQuantifier {
		return false, ast.ErrASTDomain
	}
	owners := make([]ast.ASTHandle, len(bindings))
	for i, binding := range bindings {
		if binding == nil || binding.owner.IsNull() {
			return false, ast.ErrASTDomain
		}
		owners[i] = binding.owner
	}
	hash := smtSubstitutionKeyHash(quantifier, owners)
	if entry := c.find(hash, quantifier, owners); entry != nil {
		return true, c.manager.SetRef(out, entry.result)
	}
	body, err := c.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return false, err
	}
	if err = c.substitution.Substitute(body, owners, out); err != nil {
		return false, err
	}
	value, err := c.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	// owners was created for this call and is not borrowed from the caller.
	// Keep that tuple as the cache key and retain qa, bindings, result in the
	// exact m_refs.push_back order, with no extra API-reference objects.
	entry := &smtCachedSubstitutionEntry{quantifier: quantifier, bindings: owners, result: value}
	for entry.owned < len(owners)+2 {
		if err := c.manager.IncRef(entry.ownedHandle(entry.owned)); err != nil {
			for entry.owned > 0 {
				entry.owned--
				err = errors.Join(err, c.manager.DecRef(entry.ownedHandle(entry.owned)))
			}
			return false, err
		}
		entry.owned++
	}
	c.entries[hash] = append(c.entries[hash], entry)
	c.ordered = append(c.ordered, entry)
	return false, nil
}

func (c *SMTCachedVarSubstitution) Size() uint32 {
	if c == nil {
		return 0
	}
	return uint32(len(c.ordered))
}

func (c *SMTCachedVarSubstitution) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	// cached_var_subst::reset releases m_refs and m_instances; m_proc
	// (var_subst) keeps its reducer cache until its next call resets it.
	return c.clearEntries()
}

func (e *smtCachedSubstitutionEntry) ownedHandle(index int) ast.ASTHandle {
	if index == 0 {
		return e.quantifier
	}
	if index <= len(e.bindings) {
		return e.bindings[index-1]
	}
	return e.result
}

func (c *SMTCachedVarSubstitution) clearEntries() error {
	// m_refs releases qa, bindings, result front to back. Record consumed
	// references before returning a later destructor error so Reset/Close
	// can resume without decrementing any expression twice.
	for len(c.ordered) != 0 {
		entry := c.ordered[0]
		for entry.released < entry.owned {
			consumed, err := c.manager.DecRefConsumed(entry.ownedHandle(entry.released))
			if consumed {
				entry.released++
			}
			if err != nil {
				return err
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
		c.ordered[0] = nil
		c.ordered = c.ordered[1:]
	}
	c.ordered = nil
	c.entries = make(map[uint64][]*smtCachedSubstitutionEntry)
	return nil
}

func (c *SMTCachedVarSubstitution) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.clearEntries(); err != nil {
		return err
	}
	if c.substitution != nil {
		if err := c.substitution.Close(); err != nil {
			return err
		}
		c.substitution = nil
	}
	c.closed = true
	return nil
}
