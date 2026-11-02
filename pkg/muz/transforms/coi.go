package transforms

// Source: src/muz/transforms/dl_mk_coi_filter.cpp and .h. This file ports the
// retained rule-pruning core; fixedpoint-context model conversion remains with
// the future context integration.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/dataflow"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// ConeModelDefinition is one owning extension-model entry emitted by a cone
// pass. Formula uses the declaration's bound-variable argument convention.
type ConeModelDefinition struct {
	Declaration ast.ASTHandle
	Formula     ast.ASTRef
	manager     *ast.ASTManager
}

// ConeModelStep preserves one source extension_model_converter boundary.
// Entries and steps are replayed in reverse order by the consuming backend.
type ConeModelStep []ConeModelDefinition

func closeConeModelSteps(steps []ConeModelStep) (result error) {
	for i := len(steps) - 1; i >= 0; i-- {
		for j := len(steps[i]) - 1; j >= 0; j-- {
			if steps[i][j].Formula != (ast.ASTRef{}) {
				result = errors.Join(result, steps[i][j].manager.DelRef(&steps[i][j].Formula))
			}
		}
	}
	return result
}

// CloseConeModelSteps releases every owning formula reference in steps.
func CloseConeModelSteps(steps []ConeModelStep) error { return closeConeModelSteps(steps) }

func newConeResult(source *base.RuleSet) (*base.RuleSet, error) {
	manager, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	result, err := base.NewRuleSet(manager)
	if err != nil {
		return nil, err
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

func finishConeResult(result *base.RuleSet) error {
	valid, err := result.Finalize()
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("muz/transforms: cone filter produced an unstratified rule set")
	}
	return nil
}

func coneHeadDefinition(manager *ast.ASTManager, heads []ast.ASTHandle) (ast.ASTRef, error) {
	rows := make([]ast.ASTHandle, 0, len(heads))
	for _, head := range heads {
		arity, err := manager.GetNumArgs(head)
		if err != nil {
			return ast.ASTRef{}, err
		}
		conjuncts := make([]ast.ASTHandle, 0, arity)
		for index := uint32(0); index < arity; index++ {
			argument, getErr := manager.GetArg(head, index)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			kind, getErr := manager.GetKind(argument)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			if kind == ast.ASTVar {
				continue
			}
			sort, getErr := manager.GetSort(argument)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			variable, getErr := manager.MkVar(index, sort)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			equality, getErr := manager.MkEq(variable, argument)
			if getErr != nil {
				return ast.ASTRef{}, getErr
			}
			conjuncts = append(conjuncts, equality)
		}
		row, err := manager.MkAndArray(conjuncts)
		if err != nil {
			return ast.ASTRef{}, err
		}
		rows = append(rows, row)
	}
	formula, err := manager.MkOrArray(rows)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(formula)
}

func topDownCone(source *base.RuleSet, captureModel bool) (_ *base.RuleSet, step ConeModelStep, resultErr error) {
	reachable, err := dataflow.TopDown(source)
	if err != nil {
		return nil, nil, err
	}
	result, err := newConeResult(source)
	if err != nil {
		return nil, nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, nil, err
	}
	kept := 0
	pruned := make(map[ast.ASTHandle][]ast.ASTHandle)
	var prunedOrder []ast.ASTHandle
	var manager *ast.ASTManager
	for i := 0; i < count; i++ {
		rule, e := source.Rule(i)
		if e != nil {
			return nil, nil, e
		}
		head, e := rule.HeadDeclaration()
		if e == nil {
			if reachable.IsReachable(head) {
				e = result.Add(rule)
				if e == nil {
					kept++
				}
			} else if captureModel {
				var application ast.ASTHandle
				application, e = rule.Head()
				if e == nil {
					manager, e = rule.ASTManager()
				}
				if e == nil {
					if _, found := pruned[head]; !found {
						prunedOrder = append(prunedOrder, head)
					}
					pruned[head] = append(pruned[head], application)
				}
			}
		}
		if cleanup := rule.Close(); cleanup != nil {
			e = errors.Join(e, cleanup)
		}
		if e != nil {
			return nil, nil, e
		}
	}
	if kept == count {
		return nil, nil, nil
	}
	if err = finishConeResult(result); err != nil {
		return nil, nil, err
	}
	if captureModel {
		for _, declaration := range prunedOrder {
			heads := pruned[declaration]
			formula, makeErr := coneHeadDefinition(manager, heads)
			if makeErr != nil {
				return nil, nil, errors.Join(makeErr, closeConeModelSteps([]ConeModelStep{step}))
			}
			step = append(step, ConeModelDefinition{Declaration: declaration, Formula: formula, manager: manager})
		}
	}
	keepResult = true
	return result, step, nil
}

func filteredRule(manager *base.RuleManager, rule *base.Rule, reachable *dataflow.Reachability) (*base.Rule, bool, error) {
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return nil, false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	tailSize, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	removed := false
	for i := 0; i < positive; i++ {
		declaration, e := rule.TailDeclaration(i)
		if e != nil {
			return nil, false, e
		}
		if !reachable.IsReachable(declaration) {
			return nil, false, nil
		}
	}
	tail := make([]ast.ASTHandle, 0, tailSize)
	negated := make([]bool, 0, tailSize)
	for i := 0; i < tailSize; i++ {
		expression, negative, e := rule.Tail(i)
		if e != nil {
			return nil, false, e
		}
		if i < uninterpreted && negative {
			declaration, x := rule.TailDeclaration(i)
			if x != nil {
				return nil, false, x
			}
			if !reachable.IsReachable(declaration) {
				removed = true
				continue
			}
		}
		tail = append(tail, expression)
		negated = append(negated, negative)
	}
	if !removed {
		return nil, true, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	replacement, err := manager.NewRule(head, tail, negated, name)
	return replacement, true, err
}

func bottomUpCone(source *base.RuleSet, captureModel bool, relations *rel.RelationManager) (_ *base.RuleSet, step ConeModelStep, resultErr error) {
	reachable, err := dataflow.BottomUp(source)
	if err != nil {
		return nil, nil, err
	}
	manager, err := source.RuleManager()
	if err != nil {
		return nil, nil, err
	}
	result, err := newConeResult(source)
	if err != nil {
		return nil, nil, err
	}
	keepResult := false
	defer func() {
		if !keepResult || resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, nil, err
	}
	kept := 0
	declarations := make(map[ast.ASTHandle]bool)
	var declarationOrder []ast.ASTHandle
	var astManager *ast.ASTManager
	remember := func(declaration ast.ASTHandle) {
		if !declarations[declaration] {
			declarations[declaration] = true
			declarationOrder = append(declarationOrder, declaration)
		}
	}
	for i := 0; i < count; i++ {
		rule, e := source.Rule(i)
		if e != nil {
			return nil, nil, e
		}
		if relations != nil {
			var tails int
			tails, e = rule.UninterpretedTailSize()
			for j := 0; e == nil && j < tails; j++ {
				var declaration ast.ASTHandle
				declaration, e = rule.TailDeclaration(j)
				if e != nil {
					break
				}
				table, found, relationErr := relations.TryGetRelation(declaration)
				e = relationErr
				if e == nil && found {
					var rows int
					rows, e = table.Len()
					if e == nil && rows != 0 {
						return nil, nil, nil
					}
				}
			}
			if e != nil {
				return nil, nil, errors.Join(e, rule.Close())
			}
		}
		replacement, retain, e := filteredRule(manager, rule, reachable)
		if captureModel && e == nil {
			astManager, e = rule.ASTManager()
			if e == nil {
				var declaration ast.ASTHandle
				declaration, e = rule.HeadDeclaration()
				if e == nil {
					remember(declaration)
				}
			}
			if e == nil {
				var tails int
				tails, e = rule.UninterpretedTailSize()
				for j := 0; e == nil && j < tails; j++ {
					var declaration ast.ASTHandle
					declaration, e = rule.TailDeclaration(j)
					if e == nil {
						remember(declaration)
					}
				}
			}
		}
		if e == nil && retain {
			candidate := rule
			if replacement != nil {
				candidate = replacement
			}
			e = result.Add(candidate)
			if e == nil {
				kept++
			}
		}
		if replacement != nil {
			e = errors.Join(e, replacement.Close())
		}
		e = errors.Join(e, rule.Close())
		if e != nil {
			return nil, nil, e
		}
	}
	// Preserve the source transform's observable no-change test, which compares
	// rule counts even when an unreachable negative tail was simplified.
	if kept == count {
		return nil, nil, nil
	}
	if err = finishConeResult(result); err != nil {
		return nil, nil, err
	}
	if captureModel {
		falsity, makeErr := astManager.MkFalse()
		if makeErr != nil {
			return nil, nil, makeErr
		}
		for _, declaration := range declarationOrder {
			if reachable.IsReachable(declaration) {
				continue
			}
			formula, refErr := astManager.NewRef(falsity)
			if refErr != nil {
				return nil, nil, errors.Join(refErr, closeConeModelSteps([]ConeModelStep{step}))
			}
			step = append(step, ConeModelDefinition{Declaration: declaration, Formula: formula, manager: astManager})
		}
	}
	keepResult = true
	return result, step, nil
}

// ConeOfInfluence applies the source top-down then bottom-up passes. A nil
// result means neither pass changed the source. A non-nil result is owned by
// the caller and is already finalized.
func ConeOfInfluence(source *base.RuleSet) (*base.RuleSet, error) {
	result, _, err := coneOfInfluence(source, false, nil)
	return result, err
}

// ConeOfInfluenceWithRelations preserves the source context bailout when a
// bottom-up candidate references a predicate with materialized facts.
func ConeOfInfluenceWithRelations(source *base.RuleSet, relations *rel.RelationManager) (*base.RuleSet, error) {
	result, _, err := coneOfInfluence(source, false, relations)
	return result, err
}

func coneOfInfluence(source *base.RuleSet, captureModel bool, relations *rel.RelationManager) (_ *base.RuleSet, steps []ConeModelStep, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeConeModelSteps(steps))
			steps = nil
		}
	}()
	top, topStep, err := topDownCone(source, captureModel)
	if err != nil {
		return nil, nil, err
	}
	if len(topStep) != 0 {
		steps = append(steps, topStep)
	}
	basis := source
	if top != nil {
		basis = top
	}
	bottom, bottomStep, err := bottomUpCone(basis, captureModel, relations)
	if err != nil {
		if top != nil {
			err = errors.Join(err, top.Close())
		}
		return nil, nil, err
	}
	if len(bottomStep) != 0 {
		steps = append(steps, bottomStep)
	}
	if bottom != nil {
		if top != nil {
			err = top.Close()
			if err != nil {
				return nil, nil, errors.Join(err, bottom.Close())
			}
		}
		return bottom, steps, nil
	}
	return top, steps, nil
}
