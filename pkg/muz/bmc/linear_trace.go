package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, linear::get_model rule selection.
// Rule substitution and hyper-resolution proof assembly remain separate.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

// LinearTrace follows model-selected rules from the query down to a fact.
// The caller owns the returned rule references.
func (c *Compiler) LinearTrace(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore) (result []*base.Rule, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeRuleOwners(c.manager, result))
			result = nil
		}
	}()
	predicate := query
	for {
		canceled, err := m.Canceled()
		if err != nil {
			return result, err
		}
		if canceled {
			return result, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		owned, _, err := c.selectedLinearRule(rules, predicate, level, md)
		if err != nil {
			return result, err
		}
		result = append(result, owned)
		tails, err := owned.UninterpretedTailSize()
		if err != nil {
			return result, err
		}
		if tails == 0 {
			return result, nil
		}
		if tails != 1 || level == 0 {
			return result, ast.ErrASTDomain
		}
		predicate, err = owned.TailDeclaration(0)
		if err != nil {
			return result, err
		}
		level--
	}
}

// selectedLinearRule returns an owning reference and its index in the group.
func (c *Compiler) selectedLinearRule(rules *base.RuleSet, predicate ast.ASTHandle, level uint32, md *model.ModelCore) (*base.Rule, uint32, error) {
	m := c.manager
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return nil, 0, err
	}
	var selected *base.Rule
	var selectedIndex uint32
	for i, rule := range group {
		ref, e := c.linearRule(predicate, uint32(i), level)
		if e != nil {
			err = e
			break
		}
		atom, e := m.RefGet(ref)
		if e == nil {
			var decl ast.ASTHandle
			decl, e = m.GetDecl(atom)
			if e == nil {
				var value ast.ASTHandle
				value, e = md.ConstantInterpretation(decl)
				if e == nil && !value.IsNull() && m.IsTrue(value) {
					selected = rule
					selectedIndex = uint32(i)
				}
			}
		}
		err = errors.Join(e, m.DelRef(&ref))
		if err != nil || selected != nil {
			break
		}
	}
	if err == nil && selected == nil {
		err = fmt.Errorf("bmc: model has no selected linear rule at level %d", level)
	}
	var owned *base.Rule
	if err == nil {
		owned, err = selected.Retain()
	}
	err = errors.Join(err, closeRuleOwners(m, group))
	if err != nil {
		if owned != nil {
			err = errors.Join(err, closeRuleOwners(m, []*base.Rule{owned}))
		}
		return nil, 0, err
	}
	return owned, selectedIndex, nil
}
