package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:22-31,60-72.

func cachedVarSubstitutionKeysEqual(left, right *cachedVarSubstitutionKey) bool {
	if left == nil || right == nil || left.quantifier != right.quantifier || len(left.bindings) != len(right.bindings) {
		return false
	}
	for i := range left.bindings {
		if left.bindings[i] != right.bindings[i] {
			return false
		}
	}
	return true
}

func (c *CachedVarSubstitution) findCurrent() *cachedVarSubstitutionEntry {
	byArity := c.entries[c.current.quantifier]
	if byArity == nil {
		return nil
	}
	for _, entry := range byArity[uint32(len(c.current.bindings))] {
		if cachedVarSubstitutionKeysEqual(&entry.key, c.current) {
			return entry
		}
	}
	return nil
}
