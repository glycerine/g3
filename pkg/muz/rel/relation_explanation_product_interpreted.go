package rel

// Source: src/muz/rel/dl_product_relation.cpp filter_interpreted_fn,
// src/muz/rel/dl_sieve_relation.cpp mk_filter_interpreted_fn and
// src/muz/rel/dl_mk_explanations.cpp assignment_filter_fn.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted applies guards independently to the finite and
// explanation sieves. As in the source sieve plugin, a condition spanning
// both components is deliberately an identity because neither inner relation
// owns every referenced column.
func (p *RelationExplanationProduct) FilterInterpreted(theory *astrewriter.TheoryRewriter, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if theory == nil || theory.Manager() != p.manager.manager || len(conditions) == 0 {
		return ErrTableRelation
	}
	explanationSorts, err := p.explanation.signature()
	if err != nil {
		return err
	}
	for _, condition := range conditions {
		used, err := ast.NewASTUsedVars(p.manager.manager)
		if err != nil {
			return err
		}
		if err = used.Collect(condition); err != nil {
			return err
		}
		finiteVariables := make(map[uint32]ast.ASTHandle)
		explanationVariables := make(map[uint32]ast.ASTHandle)
		for variable, declaredSort := range sorts {
			actualSort := used.Contains(variable)
			if actualSort.IsNull() {
				continue
			}
			column, found := columns[variable]
			if !found || actualSort != declaredSort {
				return ErrTableRelation
			}
			switch {
			case column >= 0 && column < len(p.finiteSorts):
				if p.finiteSorts[column] != declaredSort {
					return ErrTableRelation
				}
				finiteVariables[variable] = declaredSort
			case column >= len(p.finiteSorts) && column-len(p.finiteSorts) < len(explanationSorts):
				if explanationSorts[column-len(p.finiteSorts)] != declaredSort {
					return ErrTableRelation
				}
				explanationVariables[variable] = declaredSort
			default:
				return ErrTableRelation
			}
		}
		if uint32(len(finiteVariables)+len(explanationVariables)) != used.NumVars() {
			return ErrTableRelation
		}
		if len(finiteVariables) != 0 && len(explanationVariables) != 0 {
			continue
		}
		if len(explanationVariables) != 0 {
			if err = p.assignExplanationCondition(condition, columns, explanationVariables); err != nil {
				return err
			}
			continue
		}
		finiteColumns := make(map[uint32]int, len(finiteVariables))
		for variable := range finiteVariables {
			finiteColumns[variable] = columns[variable]
		}
		relation := &TableRelation{manager: p.manager, sorts: p.finiteSorts, table: p.table}
		if err = relation.FilterInterpreted(theory, []ast.ASTHandle{condition}, finiteColumns, finiteVariables); err != nil {
			return err
		}
	}
	return nil
}

func (p *RelationExplanationProduct) assignExplanationCondition(condition ast.ASTHandle, columns map[uint32]int, variables map[uint32]ast.ASTHandle) (resultErr error) {
	var maximum uint32
	for variable := range variables {
		if variable > maximum {
			maximum = variable
		}
	}
	bindings := make([]ast.ASTHandle, uint64(maximum)+1)
	var references []ast.ASTRef
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.manager.DelRef(&references[index]))
		}
	}()
	for index := range bindings {
		variable := uint32(index)
		sort, found := variables[variable]
		if !found {
			continue
		}
		column := columns[variable] - len(p.finiteSorts)
		value, err := p.manager.manager.MkVar(uint32(column), sort)
		if err != nil {
			return err
		}
		reference, err := p.manager.manager.NewRef(value)
		if err != nil {
			return err
		}
		references = append(references, reference)
		bindings[variable] = value
	}
	substitution, err := astrewriter.NewVarSubstitution(p.manager.manager, false)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	out, err := p.manager.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	references = append(references, out)
	if err = substitution.Substitute(condition, bindings, out); err != nil {
		return err
	}
	mapped, err := p.manager.manager.RefGet(out)
	if err != nil {
		return err
	}
	matched, err := p.explanation.FilterInterpreted(mapped)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("%w: unsupported explanation assignment", ErrTableRelation)
	}
	return nil
}
