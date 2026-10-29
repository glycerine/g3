package base

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) RegisterPredicate(declaration ast.ASTHandle, named bool) error {
	if err := c.open(); err != nil {
		return err
	}
	var name usymbol.Symbol
	if named {
		var err error
		name, err = c.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
		if old, exists := c.named[name]; exists && old != declaration {
			return fmt.Errorf("muz/base: predicate name is already registered")
		}
	}
	if c.predicates[declaration] {
		if named {
			c.named[name] = declaration
		}
		return nil
	}
	if err := c.ruleManager.RegisterPredicate(declaration); err != nil {
		return err
	}
	if err := c.retain(declaration); err != nil {
		return err
	}
	c.predicates[declaration] = true
	c.predicateOrder = append(c.predicateOrder, declaration)
	if named {
		c.named[name] = declaration
	}
	return nil
}

func (c *DatalogContext) IsPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.predicates[declaration], nil
}
func (c *DatalogContext) Predicate(name usymbol.Symbol) (ast.ASTHandle, bool, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	d, ok := c.named[name]
	return d, ok, nil
}
func (c *DatalogContext) Predicates() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), c.predicateOrder...), nil
}

func (c *DatalogContext) RestrictPredicates(keep map[ast.ASTHandle]bool) error {
	if err := c.open(); err != nil {
		return err
	}
	order := c.predicateOrder[:0]
	for _, d := range c.predicateOrder {
		if keep[d] {
			order = append(order, d)
			continue
		}
		delete(c.predicates, d)
		name, err := c.manager.GetASTName(d)
		if err != nil {
			return err
		}
		if c.named[name] == d {
			delete(c.named, name)
		}
	}
	c.predicateOrder = order
	return nil
}

func (c *DatalogContext) FreshHeadPredicate(prefix, suffix usymbol.Symbol, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	b, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := c.manager.MkFreshFuncDecl(prefix, suffix, domain, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.RegisterPredicate(d, false); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}
