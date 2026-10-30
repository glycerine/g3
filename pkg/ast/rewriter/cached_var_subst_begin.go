package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:47-54.

import "github.com/glycerine/g3/pkg/ast"

func (c *CachedVarSubstitution) Begin(quantifier ast.ASTHandle, numBindings uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	kind, err := c.manager.GetKind(quantifier)
	if err != nil {
		return err
	}
	if kind != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	c.current = &cachedVarSubstitutionKey{quantifier: quantifier, bindings: make([]ast.ASTHandle, numBindings)}
	return nil
}

func (c *CachedVarSubstitution) SetBinding(index uint32, value ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.current == nil || index >= uint32(len(c.current.bindings)) || value.IsNull() {
		return ast.ErrASTDomain
	}
	if _, err := c.manager.GetKind(value); err != nil {
		return err
	}
	c.current.bindings[index] = value
	return nil
}
