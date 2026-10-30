package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:39-45.

import "github.com/glycerine/g3/pkg/ast"

func (c *CachedVarSubstitution) clearEntries() error {
	for len(c.ordered) != 0 {
		entry := c.ordered[len(c.ordered)-1]
		for len(entry.owned) != 0 {
			i := len(entry.owned) - 1
			if err := c.manager.DelRef(&entry.owned[i]); err != nil {
				return err
			}
			entry.owned = entry.owned[:i]
		}
		c.ordered = c.ordered[:len(c.ordered)-1]
	}
	c.entries = make(map[ast.ASTHandle]map[uint32][]*cachedVarSubstitutionEntry)
	c.current = nil
	return nil
}

func (c *CachedVarSubstitution) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.clearEntries(); err != nil {
		return err
	}
	return c.substitution.Reset()
}

func (c *CachedVarSubstitution) Close() (result error) {
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
	if c.header != 0 {
		c.manager.Budget().Release(c.header)
		c.header = 0
	}
	return result
}
