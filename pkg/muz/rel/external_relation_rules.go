package rel

// Source: src/muz/rel/dl_compiler.cpp fact and single-tail rule compilation,
// specialized for src/muz/rel/dl_external_relation.cpp operations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type externalFactRule struct {
	head, tail  ast.ASTHandle
	tail2       ast.ASTHandle
	values      []ast.ASTHandle
	joinLeft    []uint32
	joinRight   []uint32
	headColumns []externalHeadColumn
	filters     [2]externalTailFilter
}

type externalTailFilter struct {
	equalColumns []uint32
	equalValues  []ast.ASTHandle
	identical    [][]uint32
}

func prepareExternalRuleTail(source *ExternalRelation, filter externalTailFilter) (_ *ExternalRelation, resultErr error) {
	value, err := source.Clone()
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, value.Close())
		}
	}()
	for index, column := range filter.equalColumns {
		if err = value.FilterEqual(column, filter.equalValues[index]); err != nil {
			return nil, err
		}
	}
	for _, columns := range filter.identical {
		if err = value.FilterIdentical(columns); err != nil {
			return nil, err
		}
	}
	published = true
	return value, nil
}

func classifyExternalFactRule(manager *ast.ASTManager, rule *base.Rule) (externalFactRule, error) {
	var result externalFactRule
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	result.head, err = manager.GetDecl(head)
	if err != nil {
		return result, err
	}
	positive, err := rule.PositiveTailSize()
	if err != nil {
		return result, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return result, err
	}
	tails, err := rule.TailSize()
	if err != nil || positive < 0 || positive > 2 || uninterpreted != positive || tails != positive {
		return result, errors.Join(err, ErrCompilerUnsupported)
	}
	headCount, err := manager.GetNumArgs(head)
	if err != nil {
		return result, err
	}
	headArgs := make([]ast.ASTHandle, headCount)
	for index := uint32(0); index < headCount; index++ {
		headArgs[index], err = manager.GetArg(head, index)
		if err != nil {
			return result, err
		}
	}
	if positive == 0 {
		for _, value := range headArgs {
			valid, cause := manager.IsValue(value)
			if cause != nil || !valid {
				return result, errors.Join(cause, ErrCompilerUnsupported)
			}
		}
		result.values = headArgs
		return result, nil
	}
	positions := make(map[ast.ASTHandle]uint32)
	firstTail := make(map[ast.ASTHandle]uint32)
	columns := make([]ast.ASTHandle, 0)
	for tailIndex := 0; tailIndex < positive; tailIndex++ {
		tail, negated, cause := rule.Tail(tailIndex)
		if cause != nil || negated {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		declaration, cause := manager.GetDecl(tail)
		if cause != nil {
			return result, cause
		}
		if tailIndex == 0 {
			result.tail = declaration
		} else {
			result.tail2 = declaration
		}
		count, cause := manager.GetNumArgs(tail)
		if cause != nil {
			return result, cause
		}
		local := make(map[ast.ASTHandle][]uint32, count)
		localOrder := make([]ast.ASTHandle, 0, count)
		for index := uint32(0); index < count; index++ {
			argument, argErr := manager.GetArg(tail, index)
			if argErr != nil {
				return result, argErr
			}
			combined := uint32(len(columns))
			columns = append(columns, argument)
			kind, argErr := manager.GetKind(argument)
			if argErr != nil {
				return result, argErr
			}
			if kind != ast.ASTVar {
				value, valueErr := manager.IsValue(argument)
				if valueErr != nil || !value {
					return result, errors.Join(valueErr, ErrCompilerUnsupported)
				}
				result.filters[tailIndex].equalColumns = append(result.filters[tailIndex].equalColumns, index)
				result.filters[tailIndex].equalValues = append(result.filters[tailIndex].equalValues, argument)
				continue
			}
			local[argument] = append(local[argument], index)
			if len(local[argument]) == 1 {
				localOrder = append(localOrder, argument)
				if tailIndex == 0 {
					firstTail[argument] = index
				} else if left, shared := firstTail[argument]; shared {
					result.joinLeft = append(result.joinLeft, left)
					result.joinRight = append(result.joinRight, index)
				}
				if _, found := positions[argument]; !found {
					positions[argument] = combined
				}
			}
		}
		for _, variable := range localOrder {
			group := local[variable]
			if len(group) > 1 {
				result.filters[tailIndex].identical = append(result.filters[tailIndex].identical, group)
			}
		}
	}
	result.headColumns = make([]externalHeadColumn, 0, len(headArgs))
	for _, argument := range headArgs {
		kind, cause := manager.GetKind(argument)
		if cause != nil {
			return result, cause
		}
		if kind == ast.ASTVar {
			position, found := positions[argument]
			if !found {
				return result, ErrCompilerUnsupported
			}
			result.headColumns = append(result.headColumns, externalSourceHeadColumn(position))
			continue
		}
		value, cause := manager.IsValue(argument)
		if cause != nil || !value {
			return result, errors.Join(cause, ErrCompilerUnsupported)
		}
		result.headColumns = append(result.headColumns, externalConstantHeadColumn(argument))
	}
	return result, nil
}

// ExecuteExternalFactRules runs the concrete source-reached external relation
// slice: ground facts, single positive-tail transformations, and two-tail
// equality joins with constant and repeated-variable tail filters. It validates the whole set
// before mutation.
func ExecuteExternalFactRules(plugin *ExternalRelationPlugin, set *base.RuleSet) (resultErr error) {
	if plugin == nil || plugin.closed || plugin.ast == nil || set == nil {
		return ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	compiled := make([]externalFactRule, 0, count)
	for index := 0; index < count; index++ {
		rule, cause := set.Rule(index)
		if cause != nil {
			return cause
		}
		entry, cause := classifyExternalFactRule(plugin.ast, rule)
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return cause
		}
		compiled = append(compiled, entry)
	}
	boolean, err := plugin.ast.MkBoolSort()
	if err != nil {
		return err
	}
	validated := make(map[ast.ASTHandle]bool)
	predicateOrder := make([]ast.ASTHandle, 0)
	for _, entry := range compiled {
		for _, predicate := range []ast.ASTHandle{entry.head, entry.tail, entry.tail2} {
			if predicate.IsNull() || validated[predicate] {
				continue
			}
			if _, _, err = plugin.manager.PredicateSignature(predicate); err != nil {
				return err
			}
			rangeSort, cause := plugin.ast.GetRange(predicate)
			if cause != nil || rangeSort != boolean {
				return errors.Join(cause, ErrCompilerUnsupported)
			}
			validated[predicate] = true
			predicateOrder = append(predicateOrder, predicate)
		}
	}
	// Relation allocation belongs to setup, not logical saturation. Complete
	// every source manager lookup and OP_RA_EMPTY callback before the first fact
	// is published so a later constructor failure cannot leave a rule prefix.
	for _, predicate := range predicateOrder {
		if _, err = plugin.PredicateRelation(predicate); err != nil {
			return err
		}
	}
	// Source make_add_constant_column caches unary singleton tables in top-level
	// setup. Construct every required singleton before publishing ground facts,
	// preserving whole-program preflight on callback failure.
	constantRelations := make(map[ast.ASTHandle]*ExternalRelation)
	constantOwners := make([]*ExternalRelation, 0)
	defer func() {
		for index := len(constantOwners) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, constantOwners[index].Close())
		}
	}()
	for _, entry := range compiled {
		for _, column := range entry.headColumns {
			if column.constant.IsNull() || constantRelations[column.constant] != nil {
				continue
			}
			sort, cause := plugin.ast.GetSort(column.constant)
			if cause != nil {
				return cause
			}
			singleton, cause := plugin.NewEmpty([]ast.ASTHandle{sort})
			if cause == nil {
				cause = singleton.AddFact([]ast.ASTHandle{column.constant})
			}
			if cause != nil {
				if singleton != nil {
					cause = errors.Join(cause, singleton.Close())
				}
				return cause
			}
			constantRelations[column.constant] = singleton
			constantOwners = append(constantOwners, singleton)
		}
	}
	checkpoint := func() error {
		canceled, cause := plugin.ast.Canceled()
		if cause != nil {
			return cause
		}
		if canceled {
			return fmt.Errorf("muz/rel: %s", plugin.ast.ResourceLimit().CancelMessage())
		}
		return nil
	}
	if err = checkpoint(); err != nil {
		return err
	}
	for _, entry := range compiled {
		if !entry.tail.IsNull() {
			continue
		}
		target, cause := plugin.PredicateRelation(entry.head)
		if cause != nil {
			return cause
		}
		if cause = target.AddFact(entry.values); cause != nil {
			return cause
		}
	}
	for {
		changed := false
		if err = checkpoint(); err != nil {
			return err
		}
		for _, entry := range compiled {
			if entry.tail.IsNull() {
				continue
			}
			source, cause := plugin.PredicateRelation(entry.tail)
			if cause != nil {
				return cause
			}
			value, cause := prepareExternalRuleTail(source, entry.filters[0])
			if cause != nil {
				return cause
			}
			if !entry.tail2.IsNull() {
				second, secondErr := plugin.PredicateRelation(entry.tail2)
				if secondErr != nil {
					return errors.Join(secondErr, value.Close())
				}
				secondValue, secondErr := prepareExternalRuleTail(second, entry.filters[1])
				if secondErr != nil {
					return errors.Join(secondErr, value.Close())
				}
				joined, joinErr := JoinExternalRelations(value, secondValue, entry.joinLeft, entry.joinRight)
				joinErr = errors.Join(joinErr, value.Close(), secondValue.Close())
				if joinErr != nil {
					if joined != nil {
						joinErr = errors.Join(joinErr, joined.Close())
					}
					return joinErr
				}
				value = joined
			}
			assembled, assembleErr := assembleExternalRuleHead(value, entry.headColumns, constantRelations)
			assembleErr = errors.Join(assembleErr, value.Close())
			if assembleErr != nil {
				if assembled != nil {
					assembleErr = errors.Join(assembleErr, assembled.Close())
				}
				return assembleErr
			}
			value = assembled
			target, cause := plugin.PredicateRelation(entry.head)
			var delta *ExternalRelation
			if cause == nil {
				sorts, sortErr := target.Sorts()
				if sortErr != nil {
					cause = sortErr
				} else {
					delta, cause = plugin.NewEmpty(sorts)
				}
			}
			if cause == nil {
				cause = target.Union(value, delta, false)
			}
			if cause == nil {
				empty, emptyErr := delta.Empty()
				cause = emptyErr
				changed = changed || !empty
			}
			if delta != nil {
				cause = errors.Join(cause, delta.Close())
			}
			cause = errors.Join(cause, value.Close())
			if cause != nil {
				return cause
			}
		}
		if !changed {
			break
		}
	}
	return nil
}
