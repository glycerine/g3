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
	scopes         []datalogContextScope
	pendingScope   *RuleSet
	scopeRecovery  bool
	background     []ast.ASTRef
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
	c.rules.contextOwner = c
	c.binder, err = NewVariableBinder(manager)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *DatalogContext) open() error {
	if c != nil && c.scopeRecovery {
		return ErrDatalogContextOwner
	}
	if c == nil {
		return ErrDatalogContextOwner
	}
	c.scopeRecovery = true
	err := c.openForScopeCleanup()
	c.scopeRecovery = false
	if err != nil {
		return err
	}
	if len(c.scopes) != 0 && c.scopes[len(c.scopes)-1].popping {
		return ErrDatalogContextOwner
	}
	return nil
}

func (c *DatalogContext) openForScopeCleanup() error {
	if c == nil || c.closed || c.manager == nil {
		return ErrDatalogContextOwner
	}
	if c.pendingScope != nil {
		if err := c.pendingScope.Close(); err != nil {
			return err
		}
		c.pendingScope = nil
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
	if c.scopeRecovery {
		return ErrDatalogContextOwner
	}
	c.scopeRecovery = true
	defer func() { c.scopeRecovery = false }()
	if c.pendingScope != nil {
		if err := c.pendingScope.Close(); err != nil {
			return err
		}
		c.pendingScope = nil
	}
	for len(c.scopes) > 0 {
		last := len(c.scopes) - 1
		if err := c.scopes[last].rules.Close(); err != nil {
			return err
		}
		c.scopes = c.scopes[:last]
	}
	c.closed = true
	for i := len(c.background) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.background[i]))
	}
	c.background = nil
	if c.rules != nil {
		result = errors.Join(result, c.rules.closeContents())
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
