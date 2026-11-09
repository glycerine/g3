package qe

// Source: src/qe/qe_arith_plugin.cpp subst_cache keyed by branch_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticBranchCache struct {
	manager *ast.ASTManager
	entries map[ArithmeticBranchKey]*ArithmeticBranchFormula
	// order is add_cache's m_trail order, released forward.
	order []*ArithmeticBranchFormula
}

func NewArithmeticBranchCache(manager *ast.ASTManager) (*ArithmeticBranchCache, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &ArithmeticBranchCache{manager: manager, entries: make(map[ArithmeticBranchKey]*ArithmeticBranchFormula)}, nil
}

func (c *ArithmeticBranchCache) Insert(record *ArithmeticBranchFormula) error {
	if c == nil || c.manager == nil || record == nil || record.manager != c.manager {
		return ast.ErrASTHandle
	}
	if c.entries[record.Key] != nil {
		return ast.ErrASTDomain
	}
	c.entries[record.Key] = record
	c.order = append(c.order, record)
	return nil
}

func (c *ArithmeticBranchCache) Find(key ArithmeticBranchKey) (*ArithmeticBranchFormula, bool, error) {
	if c == nil || c.manager == nil {
		return nil, false, ast.ErrASTHandle
	}
	record := c.entries[key]
	return record, record != nil, nil
}

func (c *ArithmeticBranchCache) Close() (result error) {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	kept := c.order[:0]
	for _, record := range c.order {
		cause := record.Close()
		result = errors.Join(result, cause)
		if cause == nil {
			delete(c.entries, record.Key)
		} else {
			kept = append(kept, record)
		}
	}
	c.order = kept
	if result == nil {
		c.manager = nil
	}
	return result
}
