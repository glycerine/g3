package fp

// Source: src/muz/base/dl_context.cpp get_model and
// src/muz/rel/dl_finite_product_relation.cpp finite relation interpretation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// datalogModel materializes the least fixed-point relation tables as Boolean
// predicate interpretations. The omitted declaration is the synthetic Horn
// tactic query predicate filtered by the native model converter.
func (e *PropositionalEngine) datalogModel(omitted ast.ASTHandle) (result *model.Model, resultErr error) {
	if e.backend != "datalog" || !e.queried || e.predicates[omitted] == nil {
		return nil, ErrPropositionalEngine
	}
	result, err := model.NewModel(e.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	truth, err := e.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := e.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	for _, declaration := range e.predicateOrder {
		if declaration == omitted {
			continue
		}
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			return nil, ErrPropositionalEngine
		}
		facts, x := state.table.Facts()
		if x != nil {
			return nil, x
		}
		if len(state.sorts) == 0 {
			value := falsity
			if len(facts) != 0 {
				value = truth
			}
			if x = result.Core().RegisterConstant(declaration, value); x != nil {
				return nil, x
			}
			continue
		}
		interpretation, x := model.NewFunctionInterpretation(e.manager, uint32(len(state.sorts)))
		if x != nil {
			return nil, x
		}
		registered := false
		defer func() {
			if !registered && interpretation != nil {
				resultErr = errors.Join(resultErr, interpretation.Close())
			}
		}()
		if x = interpretation.SetElse(falsity); x != nil {
			return nil, x
		}
		for _, fact := range facts {
			values, y := e.relations.DecodeFact(state.sorts, fact)
			if y != nil {
				return nil, y
			}
			handles := make([]ast.ASTHandle, len(values))
			for index := range values {
				handles[index], y = e.manager.RefGet(values[index])
				if y != nil {
					break
				}
			}
			if y == nil {
				y = interpretation.InsertNewEntry(handles, truth)
			}
			for index := len(values) - 1; index >= 0; index-- {
				y = errors.Join(y, e.manager.DelRef(&values[index]))
			}
			if y != nil {
				return nil, y
			}
		}
		if x = result.Core().RegisterFunction(declaration, interpretation); x != nil {
			return nil, x
		}
		registered = true
		interpretation = nil
	}
	return e.expandHornModel(result, e.datalogConversions)
}
