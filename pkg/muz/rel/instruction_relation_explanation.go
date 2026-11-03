package rel

// Source: dl_instruction.cpp delegates each concrete operation to the
// registered product/sieve plugins used for relation-level explanations.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type relationExplanationInstruction struct{ Instruction }

func relationExplanationInstructionFor(instruction Instruction) (Instruction, error) {
	switch instruction.(type) {
	case *registerInstruction, *relationIOInstruction, *explanationFiniteLoadInstruction, *constructInstruction, *tableInstruction, *interpretedFilterInstruction, *explanationNegationInstruction:
		return &relationExplanationInstruction{instruction}, nil
	default:
		return nil, ErrCompilerUnsupported
	}
}

func setRelationExplanationResult(context *ExecutionContext, target uint32, product *RelationExplanationProduct) (bool, error) {
	empty, err := product.Empty()
	if err != nil {
		return false, errors.Join(err, product.Close())
	}
	if empty {
		return true, errors.Join(product.Close(), context.MakeEmpty(target))
	}
	if err := context.SetRelationExplanationRegister(target, product); err != nil {
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}

func finishRelationExplanationMutation(context *ExecutionContext, target uint32, product *RelationExplanationProduct, resultErr error) (bool, error) {
	if resultErr != nil {
		return false, resultErr
	}
	empty, err := product.Empty()
	if err != nil {
		return false, err
	}
	if empty {
		return true, context.MakeEmpty(target)
	}
	return true, nil
}

func (i *relationExplanationInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := context.open(); err != nil {
		return false, err
	}
	switch instruction := i.Instruction.(type) {
	case *relationIOInstruction:
		return performRelationExplanationIO(instruction, context)
	case *explanationFiniteLoadInstruction:
		return performRelationExplanationFiniteLoad(instruction, context)
	case *constructInstruction:
		return performRelationExplanationConstruct(instruction, context)
	case *tableInstruction:
		return performRelationExplanationTable(instruction, context)
	case *interpretedFilterInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		product, err := context.RelationExplanationRegister(instruction.register)
		if err != nil || product == nil {
			return err == nil, err
		}
		conditions, err := instruction.conditionHandles()
		if err != nil {
			return false, err
		}
		context.stats.Filter++
		err = product.FilterInterpreted(instruction.rewriter, conditions, instruction.variableColumns, instruction.variableSorts)
		return finishRelationExplanationMutation(context, instruction.register, product, err)
	case *explanationNegationInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		product, err := context.RelationExplanationRegister(instruction.first)
		if err != nil || product == nil {
			return err == nil, err
		}
		negative, err := context.RelationExplanationRegister(instruction.second)
		if err != nil || negative == nil {
			return err == nil, err
		}
		context.stats.FilterByNegation++
		err = finiteTableFilterNegation(product.table, negative.table, instruction.left, instruction.right)
		return finishRelationExplanationMutation(context, instruction.first, product, err)
	case *registerInstruction:
		if err := instruction.open(); err != nil {
			return false, err
		}
		if instruction.kind == "dealloc" {
			return true, context.MakeEmpty(instruction.source)
		}
		product, err := context.RelationExplanationRegister(instruction.source)
		if err != nil {
			return false, err
		}
		if product == nil {
			return true, context.MakeEmpty(instruction.target)
		}
		if instruction.kind == "clone" {
			clone, err := product.Clone()
			if err != nil {
				return false, err
			}
			return setRelationExplanationResult(context, instruction.target, clone)
		}
		if instruction.kind == "move" {
			product, err = context.ReleaseRelationExplanationRegister(instruction.source)
			if err != nil {
				return false, err
			}
			if err := context.SetRelationExplanationRegister(instruction.target, product); err != nil {
				return false, errors.Join(err, context.SetRelationExplanationRegister(instruction.source, product))
			}
			return true, nil
		}
	}
	return false, ErrInstruction
}

func performRelationExplanationIO(i *relationIOInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if !i.explanation {
		return i.Perform(context)
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	if !i.store {
		product, err := relations.GetRelationExplanation(i.predicate)
		if err != nil {
			return false, err
		}
		empty, err := product.Empty()
		if err != nil {
			return false, err
		}
		if empty {
			return true, context.MakeEmpty(i.register)
		}
		clone, err := product.Clone()
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.register, clone)
	}
	product, err := context.RelationExplanationRegister(i.register)
	if err != nil {
		return false, err
	}
	transferred := product != nil
	if transferred {
		product, err = context.ReleaseRelationExplanationRegister(i.register)
	} else {
		sorts, _, _, signatureErr := relations.relationExplanationPredicateSignature(i.predicate)
		if signatureErr != nil {
			return false, signatureErr
		}
		product, err = NewRelationExplanationProduct(relations, sorts)
	}
	if err != nil {
		return false, err
	}
	if err = relations.StoreRelationExplanation(i.predicate, product); err != nil {
		if transferred {
			return false, errors.Join(err, context.SetRelationExplanationRegister(i.register, product))
		}
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}

func performRelationExplanationFiniteLoad(i *explanationFiniteLoadInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	sorts, _, err := relations.PredicateSignature(i.predicate)
	if err != nil {
		return false, err
	}
	var table FiniteTable
	if i.source == VoidRegister {
		table, err = relations.GetFiniteRelation(i.predicate)
	} else {
		table, err = context.FiniteRegister(i.source)
	}
	if err != nil {
		return false, err
	}
	if table == nil {
		return true, context.MakeEmpty(i.register)
	}
	rows, err := table.Len()
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return true, context.MakeEmpty(i.register)
	}
	product, err := RelationExplanationFromFinite(relations, sorts, table)
	if err != nil {
		return false, err
	}
	return setRelationExplanationResult(context, i.register, product)
}

func performRelationExplanationConstruct(i *constructInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if context.relations != nil && context.relations != i.relations {
		return false, ErrInstruction
	}
	if i.kind == "mk_explanation_total" {
		context.stats.Total++
		product, err := NewRelationExplanationTotal(i.relations, i.explanations)
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.target, product)
	}
	if i.kind != "mk_explanation_finite_total" && i.kind != "mk_explanation_finite_singleton" {
		return false, ErrCompilerUnsupported
	}
	sort, err := i.finiteManager.RefGet(i.finiteSort)
	if err != nil {
		return false, err
	}
	table := NewHashTable(i.signature)
	if i.kind == "mk_explanation_finite_total" {
		context.stats.Total++
		err = fillTotalTable(table, i.signature)
	} else {
		context.stats.UnarySingleton++
		_, err = table.Add(Fact{i.value})
	}
	if err != nil {
		return false, errors.Join(err, table.Close())
	}
	product, err := RelationExplanationFromFinite(i.relations, []ast.ASTHandle{sort}, table)
	err = errors.Join(err, table.Close())
	if err != nil {
		if product != nil {
			err = errors.Join(err, product.Close())
		}
		return false, err
	}
	return setRelationExplanationResult(context, i.target, product)
}

func performRelationExplanationTable(i *tableInstruction, context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	source, err := context.RelationExplanationRegister(i.first)
	if err != nil {
		return false, err
	}
	if source == nil {
		switch i.kind {
		case "join", "project", "rename":
			return true, context.MakeEmpty(i.result)
		}
		return true, nil
	}
	switch i.kind {
	case "filter_equal":
		context.stats.FilterEqual++
		err = finiteTableFilterEqual(source.table, i.value, i.column)
		return finishRelationExplanationMutation(context, i.first, source, err)
	case "filter_identical":
		context.stats.FilterIdentical++
		err = finiteTableFilterIdentical(source.table, i.left)
		return finishRelationExplanationMutation(context, i.first, source, err)
	case "join":
		context.stats.Join++
		right, err := context.RelationExplanationRegister(i.second)
		if err != nil {
			return false, err
		}
		if right == nil {
			return true, context.MakeEmpty(i.result)
		}
		product, err := source.Join(right, i.left, i.right)
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.result, product)
	case "project", "rename":
		context.stats.ProjectRename++
		var product *RelationExplanationProduct
		if i.kind == "project" {
			product, err = source.ProjectColumns(i.removed)
		} else {
			product, err = source.RenameFinite(i.left)
		}
		if err != nil {
			return false, err
		}
		return setRelationExplanationResult(context, i.result, product)
	case "union", "widen":
		context.stats.Union++
		proofSorts, err := source.explanation.signature()
		if err != nil {
			return false, err
		}
		if len(proofSorts) == 0 {
			return unionRelationExplanationIntoFinite(i, context, source)
		}
		target, err := context.RelationExplanationRegister(i.second)
		if err != nil {
			return false, err
		}
		if target == nil {
			target, err = source.EmptyLike()
			if err != nil {
				return false, err
			}
			if err = context.SetRelationExplanationRegister(i.second, target); err != nil {
				return false, errors.Join(err, target.Close())
			}
		}
		var delta *RelationExplanationProduct
		if i.result != VoidRegister {
			delta, err = context.RelationExplanationRegister(i.result)
			if err != nil {
				return false, err
			}
			if delta == nil {
				delta, err = source.EmptyLike()
				if err != nil {
					return false, err
				}
				if err = context.SetRelationExplanationRegister(i.result, delta); err != nil {
					return false, errors.Join(err, delta.Close())
				}
			}
		}
		err = target.Union(source, delta)
		return err == nil, err
	}
	return false, ErrCompilerUnsupported
}

func unionRelationExplanationIntoFinite(i *tableInstruction, context *ExecutionContext, source *RelationExplanationProduct) (bool, error) {
	target, err := context.FiniteRegister(i.second)
	if err != nil {
		return false, err
	}
	signature, err := source.table.Signature()
	if err != nil {
		return false, err
	}
	if target == nil {
		target, err = source.manager.NewDefaultFiniteTable(signature)
		if err != nil {
			return false, err
		}
		if err = context.SetFiniteRegister(i.second, target); err != nil {
			return false, errors.Join(err, target.Close())
		}
	}
	var delta FiniteTable
	if i.result != VoidRegister {
		delta, err = context.FiniteRegister(i.result)
		if err != nil {
			return false, err
		}
		if delta == nil {
			delta, err = source.manager.NewDefaultFiniteTable(signature)
			if err != nil {
				return false, err
			}
			if err = context.SetFiniteRegister(i.result, delta); err != nil {
				return false, errors.Join(err, delta.Close())
			}
		}
	}
	err = finiteTableUnion(target, source.table, delta)
	return err == nil, err
}
