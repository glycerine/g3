package base

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) RegisterPredicate(declaration ast.ASTHandle, named bool) error {
	if err := c.open(); err != nil {
		return err
	}
	if c.predicates[declaration] {
		return nil
	}
	var name usymbol.Symbol
	if named {
		var err error
		name, err = c.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
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
	// Prepare restored declarations before changing membership, so an invalid
	// declaration or a refused reference cannot leave a partially restored set.
	var restored []ast.ASTHandle
	for d, present := range keep {
		if present && !c.predicates[d] {
			if err := c.ruleManager.RegisterPredicate(d); err != nil {
				return err
			}
			if err := c.retain(d); err != nil {
				return err
			}
			restored = append(restored, d)
		}
	}
	order := c.predicateOrder[:0]
	for _, d := range c.predicateOrder {
		if keep[d] {
			order = append(order, d)
			continue
		}
		delete(c.predicates, d)
	}
	c.predicateOrder = order
	// Scoped queries restore a previously saved set, which can contain
	// declarations no longer in the current membership set. The name table
	// is independent and remains unchanged by this operation.
	for _, d := range restored {
		c.predicates[d] = true
		c.predicateOrder = append(c.predicateOrder, d)
	}
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
	if err = c.RegisterPredicate(d, true); err != nil {
		return ast.ASTHandle{}, err
	}
	return d, nil
}
