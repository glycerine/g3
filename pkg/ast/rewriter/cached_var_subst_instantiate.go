package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:58-103.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *CachedVarSubstitution) Instantiate(out ast.ASTRef) (cached bool, result error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.current == nil {
		return false, ast.ErrASTDomain
	}
	for _, binding := range c.current.bindings {
		if binding.IsNull() {
			return false, ast.ErrASTDomain
		}
	}
	if entry := c.findCurrent(); entry != nil {
		value, err := c.manager.RefGet(entry.result)
		if err != nil {
			return false, err
		}
		if err = c.manager.SetRef(out, value); err != nil {
			return false, err
		}
		return true, c.traceCacheHit(c.current.bindings)
	}
	body, err := c.manager.GetQuantifierBody(c.current.quantifier)
	if err != nil {
		return false, err
	}
	if err = c.substitution.Substitute(body, c.current.bindings, out); err != nil {
		return false, err
	}
	value, err := c.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	entry := &cachedVarSubstitutionEntry{key: cachedVarSubstitutionKey{
		quantifier: c.current.quantifier,
		bindings:   append([]ast.ASTHandle(nil), c.current.bindings...),
	}}
	handles := make([]ast.ASTHandle, 0, len(entry.key.bindings)+2)
	handles = append(handles, entry.key.quantifier)
	handles = append(handles, entry.key.bindings...)
	handles = append(handles, value)
	for _, handle := range handles {
		ref, makeErr := c.manager.NewRef(handle)
		if makeErr != nil {
			for i := len(entry.owned) - 1; i >= 0; i-- {
				makeErr = errors.Join(makeErr, c.manager.DelRef(&entry.owned[i]))
			}
			return false, makeErr
		}
		entry.owned = append(entry.owned, ref)
	}
	entry.result = entry.owned[len(entry.owned)-1]
	byArity := c.entries[entry.key.quantifier]
	if byArity == nil {
		byArity = make(map[uint32][]*cachedVarSubstitutionEntry)
		c.entries[entry.key.quantifier] = byArity
	}
	arity := uint32(len(entry.key.bindings))
	byArity[arity] = append(byArity[arity], entry)
	c.ordered = append(c.ordered, entry)
	c.current = &entry.key
	return false, nil
}
