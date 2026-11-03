package fp

// Source: dl_compiler.cpp compile_interpreted_tail translates rule variables
// to relation columns before invoking udoc_plugin's interpreted filter.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

type udocGuardAttempt struct {
	manager      *ast.ASTManager
	roots        []ast.ASTRef
	substitution *astrewriter.VarSubstitution
	mapped       ast.ASTRef
}

func (a *udocGuardAttempt) close() error {
	if a.mapped != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.mapped); err != nil {
			return err
		}
	}
	if a.substitution != nil {
		if err := a.substitution.Close(); err != nil {
			return err
		}
		a.substitution = nil
	}
	for len(a.roots) > 0 {
		index := len(a.roots) - 1
		if a.roots[index] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.roots[index]); err != nil {
				return err
			}
		}
		a.roots = a.roots[:index]
	}
	return nil
}

func (c *datalogUDocContext) clearGuardCleanup() error {
	for len(c.guardCleanup) > 0 {
		index := len(c.guardCleanup) - 1
		if err := c.guardCleanup[index].close(); err != nil {
			return err
		}
		c.guardCleanup = c.guardCleanup[:index]
	}
	return nil
}

func (c *datalogUDocContext) filterCondition(working *rel.TypedUDocRelation, conditions []ast.ASTHandle, variables map[uint32]uint32) (resultErr error) {
	attempt := &udocGuardAttempt{manager: c.manager}
	c.guardCleanup = append(c.guardCleanup, attempt)
	defer func() { resultErr = errors.Join(resultErr, c.clearGuardCleanup()) }()
	condition, err := c.manager.MkAndArgs(conditions)
	if err != nil {
		return err
	}
	conditionRoot, err := c.manager.NewRef(condition)
	if err != nil {
		return err
	}
	attempt.roots = append(attempt.roots, conditionRoot)
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	sorts := working.Sorts()
	bindings := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		column, found := variables[variable]
		if !found || int(column) >= len(sorts) || sort != sorts[column] {
			return fmt.Errorf("%w: unbound interpreted variable", ErrUDocDatalogFragment)
		}
	}
	for variable := uint32(0); variable < used.MaxFoundVarIndexPlusOne(); variable++ {
		sort := used.Contains(variable)
		if sort.IsNull() {
			continue
		}
		value, cause := c.manager.MkVar(variables[variable], sort)
		if cause != nil {
			return cause
		}
		root, cause := c.manager.NewRef(value)
		if cause != nil {
			return cause
		}
		attempt.roots = append(attempt.roots, root)
		bindings[variable] = value
	}
	attempt.substitution, err = astrewriter.NewVarSubstitution(c.manager, false)
	if err != nil {
		return err
	}
	attempt.mapped, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = attempt.substitution.Substitute(condition, bindings, attempt.mapped); err != nil {
		return err
	}
	mapped, err := c.manager.RefGet(attempt.mapped)
	if err != nil {
		return err
	}
	if err = working.FilterInterpreted(mapped); err != nil {
		return errors.Join(err, ErrUDocDatalogFragment)
	}
	return nil
}

func (c *datalogUDocContext) filterInterpretedTails(working *rel.TypedUDocRelation, rule *base.Rule, first, total int, variables map[uint32]uint32) error {
	if first == total {
		return nil
	}
	conditions := make([]ast.ASTHandle, 0, total-first)
	for i := first; i < total; i++ {
		if err := c.checkpoint(); err != nil {
			return err
		}
		condition, negated, err := rule.Tail(i)
		if err != nil {
			return err
		}
		if negated {
			return ErrUDocDatalogFragment
		}
		conditions = append(conditions, condition)
	}
	return c.filterCondition(working, conditions, variables)
}
