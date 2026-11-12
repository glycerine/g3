package cmd_context

// Source: src/cmd_context/cmd_util.cpp, named expression references used by
// the SMT-LIB debug commands.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// StoreDebugExpression owns an expression reference until replacement or reset.
func (c *Context) StoreDebugExpression(name string, expression ast.ASTHandle) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	kind, err := m.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
		return fmt.Errorf("global variable does not reference a term")
	}
	value, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	if c.debugExpressions == nil {
		c.debugExpressions = make(map[string]ast.ASTRef)
	}
	if previous, found := c.debugExpressions[name]; found {
		if err = m.DelRef(&previous); err != nil {
			if previous == (ast.ASTRef{}) {
				delete(c.debugExpressions, name)
			} else {
				c.debugExpressions[name] = previous
			}
			return errors.Join(err, m.DelRef(&value))
		}
	}
	c.debugExpressions[name] = value
	// m_object_refs.insert(s, r): the dictionary slot fixes the order
	// reset_object_refs releases in.
	if c.objectRefs == nil {
		c.objectRefs = newSymbolDictionary()
	}
	c.objectRefs.insertIfNotThere(name)
	return nil
}

// DebugExpression returns a borrowed expression, valid until replacement or reset.
func (c *Context) DebugExpression(name string) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, found := c.debugExpressions[name]
	if !found {
		return ast.ASTHandle{}, fmt.Errorf("unknown global variable %s", name)
	}
	return m.RefGet(value)
}

// clearDebugExpressions is reset_object_refs: dec_ref each value in
// m_object_refs (dictionary<object_ref*>) table order, then reset.
func (c *Context) clearDebugExpressions() error {
	var names []string
	if c.objectRefs != nil {
		for _, entry := range c.objectRefs.entries() {
			names = append(names, entry.name)
		}
	}
	for _, name := range names {
		value, found := c.debugExpressions[name]
		if !found {
			continue
		}
		if err := c.manager.DelRef(&value); err != nil {
			if value == (ast.ASTRef{}) {
				delete(c.debugExpressions, name)
			} else {
				c.debugExpressions[name] = value
			}
			return err
		}
		delete(c.debugExpressions, name)
	}
	if len(c.debugExpressions) != 0 {
		// Unreachable: every stored name is in objectRefs.
		return ast.ErrASTDomain
	}
	if c.objectRefs != nil {
		c.objectRefs.reset()
	}
	return nil
}
