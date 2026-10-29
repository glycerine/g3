package base

// Foundational port of src/muz/base/dl_context.{h,cpp}.  This file owns the
// base-layer state; relation and engine services are deliberately layered on
// top of it by later ports.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrDatalogContextOwner = errors.New("muz/base: invalid or closed datalog context")

type DatalogContext struct {
	manager        *ast.ASTManager
	ruleManager    *RuleManager
	rules          *RuleSet
	binder         *VariableBinder
	predicates     map[ast.ASTHandle]bool
	predicateOrder []ast.ASTHandle
	named          map[usymbol.Symbol]ast.ASTHandle
	references     []ast.ASTRef
	sorts          map[ast.ASTHandle]*datalogSortDomain
	argumentNames  map[ast.ASTHandle][]usymbol.Symbol
	scopes         []int
	bindVariables  bool
	closed         bool
}

func NewDatalogContext(manager *ast.ASTManager) (_ *DatalogContext, resultErr error) {
	if manager == nil {
		return nil, ErrDatalogContextOwner
	}
	rm, err := NewRuleManager(manager)
	if err != nil {
		return nil, err
	}
	c := &DatalogContext{manager: manager, ruleManager: rm, predicates: make(map[ast.ASTHandle]bool), named: make(map[usymbol.Symbol]ast.ASTHandle), sorts: make(map[ast.ASTHandle]*datalogSortDomain), argumentNames: make(map[ast.ASTHandle][]usymbol.Symbol), bindVariables: true}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.Close())
		}
	}()
	c.rules, err = NewRuleSet(rm)
	if err != nil {
		return nil, err
	}
	c.binder, err = NewVariableBinder(manager)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *DatalogContext) open() error {
	if c == nil || c.closed || c.manager == nil {
		return ErrDatalogContextOwner
	}
	return nil
}

func (c *DatalogContext) Manager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.manager, nil
}
func (c *DatalogContext) RuleManager() (*RuleManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.ruleManager, nil
}
func (c *DatalogContext) Rules() (*RuleSet, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.rules, nil
}

func (c *DatalogContext) retain(h ast.ASTHandle) error {
	r, err := c.manager.NewRef(h)
	if err != nil {
		return err
	}
	c.references = append(c.references, r)
	return nil
}

func (c *DatalogContext) Close() (result error) {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	if c.rules != nil {
		result = errors.Join(result, c.rules.Close())
	}
	if c.binder != nil {
		result = errors.Join(result, c.binder.Close())
	}
	if c.ruleManager != nil {
		result = errors.Join(result, c.ruleManager.Close())
	}
	for i := len(c.references) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.references[i]))
	}
	c.references = nil
	c.manager = nil
	if result != nil {
		return fmt.Errorf("muz/base: close datalog context: %w", result)
	}
	return nil
}
