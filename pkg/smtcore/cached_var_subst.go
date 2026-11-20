package smtcore

// Source: src/smt/cached_var_subst.h and src/smt/cached_var_subst.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// smtCachedSubstitutionEntry is an m_instances entry. Its key (qa and the
// bindings) and value (the result) are the entry's run of m_refs, which
// pushes exactly qa, the bindings and the result: refs[start] is qa,
// refs[start+1:start+1+n] the bindings, refs[start+1+n] the result (z3's
// region-allocated key holds the same pointers). next chains entries
// whose hashes collide.
type smtCachedSubstitutionEntry struct {
	start, count, next uint32
}

type SMTCachedVarSubstitution struct {
	self         *SMTCachedVarSubstitution
	manager      *ast.ASTManager
	substitution *rewriter.VarSubstitution
	// entries is cached_var_subst::m_instances, a map keyed by (qa,
	// bindings) under key_hash_proc (a hash of the whole binding tuple and
	// qa's id). Buckets hold only hash collisions.
	entries map[uint64]uint32 // hash -> 1 + index of its newest entry
	list    []smtCachedSubstitutionEntry
	// scratch is m_new_keys: the probe key, reused until an insertion.
	scratch []ast.ASTHandle
	// refs is m_refs, an expr_ref_vector: qa, the bindings and the result
	// of each entry, in insertion order (inc_ref only, no reference slots).
	refs   []ast.ASTHandle
	closed bool
}

func NewSMTCachedVarSubstitution(m *ast.ASTManager) (*SMTCachedVarSubstitution, error) {
	if m == nil {
		return nil, ast.ErrASTDomain
	}
	c := &SMTCachedVarSubstitution{manager: m, entries: make(map[uint64]uint32)}
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

// smtHandleMix folds one AST handle into a tuple hash. Each table checks the
// complete tuple for equality on a hash match.
func smtHandleMix(h uint64, x ast.ASTHandle) uint64 {
	slotGeneration := uint64(x.Generation())<<32 | uint64(x.Slot())
	h = (h ^ slotGeneration ^ x.Origin()) * 0x9e3779b185ebca87
	return h ^ (h >> 32)
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

func (c *SMTCachedVarSubstitution) find(hash uint64, quantifier ast.ASTHandle, bindings []ast.ASTHandle) (ast.ASTHandle, bool) {
	for i := c.entries[hash]; i != 0; i = c.list[i-1].next {
		e := c.list[i-1]
		if int(e.count) != len(bindings) || c.refs[e.start] != quantifier {
			continue
		}
		key := c.refs[e.start+1 : e.start+1+e.count]
		equal := true
		for k := range bindings {
			if key[k] != bindings[k] {
				equal = false
				break
			}
		}
		if equal {
			return c.refs[e.start+1+e.count], true
		}
	}
	return ast.ASTHandle{}, false
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
	owners := c.scratch[:0]
	for _, binding := range bindings {
		if binding == nil || binding.owner.IsNull() {
			return false, ast.ErrASTDomain
		}
		owners = append(owners, binding.owner)
	}
	c.scratch = owners
	hash := smtSubstitutionKeyHash(quantifier, owners)
	if result, found := c.find(hash, quantifier, owners); found {
		return true, c.manager.SetRef(out, result)
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
	start := len(c.refs)
	c.refs = append(c.refs, quantifier)
	c.refs = append(c.refs, owners...)
	c.refs = append(c.refs, value)
	for i, h := range c.refs[start:] {
		if err := c.manager.IncRef(h); err != nil {
			for k := start + i - 1; k >= start; k-- {
				err = errors.Join(err, c.manager.DecRef(c.refs[k]))
			}
			c.refs = c.refs[:start]
			return false, err
		}
	}
	c.list = append(c.list, smtCachedSubstitutionEntry{start: uint32(start), count: uint32(len(owners)), next: c.entries[hash]})
	c.entries[hash] = uint32(len(c.list))
	return false, nil
}

func (c *SMTCachedVarSubstitution) Size() uint32 {
	if c == nil {
		return 0
	}
	return uint32(len(c.list))
}

func (c *SMTCachedVarSubstitution) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	// cached_var_subst::reset releases m_refs and m_instances; m_proc
	// (var_subst) keeps its reducer cache until its next call resets it.
	return c.clearEntries()
}

func (c *SMTCachedVarSubstitution) clearEntries() error {
	// m_instances goes first: its entries index m_refs, which is then
	// released front to back (an expr_ref_vector).
	c.list = nil
	c.entries = make(map[uint64]uint32)
	for len(c.refs) != 0 {
		consumed, err := c.manager.DecRefConsumed(c.refs[0])
		if consumed {
			c.refs = c.refs[1:]
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	c.refs = nil
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
