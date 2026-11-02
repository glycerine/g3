package rel

// Source: dl_instruction.cpp projection and union instructions, dispatching
// through the registered finite-product/explanation operations.

import "errors"

func explanationInstructionRegister(context *ExecutionContext, index uint32) (*FiniteExplanationProduct, error) {
	if index == VoidRegister {
		return nil, nil
	}
	return context.ExplanationRegister(index)
}
func hasExplanationInstructionRegister(context *ExecutionContext, index uint32) bool {
	return index != VoidRegister && uint64(index) < uint64(len(context.explanationRegisters)) && context.explanationRegisters[index] != nil
}

// projectExplanationColumns splits logical signature columns into the finite
// prefix and explanation suffix consumed by the two concrete projection paths.
func (p *FiniteExplanationProduct) projectExplanationColumns(removed []int) (result *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if _, err := tableRelationRemovedColumns(len(p.finiteSorts)+p.explanationColumns, removed); err != nil {
		return nil, err
	}
	var finite, explanations []int
	for _, column := range removed {
		if column < len(p.finiteSorts) {
			finite = append(finite, column)
		} else {
			explanations = append(explanations, column-len(p.finiteSorts))
		}
	}
	projected, err := p.Project(finite)
	if err != nil {
		return nil, err
	}
	if len(explanations) == 0 {
		return projected, nil
	}
	defer func() {
		resultErr = errors.Join(resultErr, projected.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	return projected.ProjectExplanation(explanations)
}

func (i *tableInstruction) performExplanation(context *ExecutionContext) (bool, error) {
	source, err := explanationInstructionRegister(context, i.first)
	if err != nil {
		return false, err
	}
	switch i.kind {
	case "join":
		context.stats.Join++
		right, err := explanationInstructionRegister(context, i.second)
		if err != nil {
			return false, err
		}
		if source == nil || right == nil {
			return true, context.MakeEmpty(i.result)
		}
		joined, err := source.Join(right, i.left, i.right)
		if err != nil {
			return false, err
		}
		return setExplanationInstructionResult(context, i.result, joined)
	case "rename":
		if source == nil {
			return true, context.MakeEmpty(i.result)
		}
		context.stats.ProjectRename++
		var renamed *FiniteExplanationProduct
		var err error
		finite := true
		for _, column := range i.left {
			finite = finite && column < len(source.finiteSorts)
		}
		if finite {
			renamed, err = source.RenameFinite(i.left)
		} else {
			cycle := make([]int, len(i.left))
			for index, column := range i.left {
				if column < len(source.finiteSorts) {
					return false, ErrInstruction
				}
				cycle[index] = column - len(source.finiteSorts)
			}
			renamed, err = source.RenameExplanation(cycle)
		}
		if err != nil {
			return false, err
		}
		return setExplanationInstructionResult(context, i.result, renamed)
	case "project":
		if source == nil {
			return true, context.MakeEmpty(i.result)
		}
		context.stats.ProjectRename++
		projected, err := source.projectExplanationColumns(i.removed)
		if err != nil {
			return false, err
		}
		count, err := projected.Len()
		if err != nil {
			return false, errors.Join(err, projected.Close())
		}
		if count == 0 {
			return true, errors.Join(projected.Close(), context.MakeEmpty(i.result))
		}
		if err = context.SetExplanationRegister(i.result, projected); err != nil {
			return false, errors.Join(err, projected.Close())
		}
		return true, nil
	case "union", "widen":
		if source == nil {
			return true, nil
		}
		context.stats.Union++
		if source.explanationColumns == 0 && !hasExplanationInstructionRegister(context, i.second) {
			return i.unionProjectedExplanationIntoTable(context, source)
		}
		target, err := explanationInstructionRegister(context, i.second)
		if err != nil {
			return false, err
		}
		if target == nil {
			target, err = newFiniteExplanationProduct(source.manager, source.finiteSorts, source.explanationColumns)
			if err != nil {
				return false, err
			}
			if err = context.SetExplanationRegister(i.second, target); err != nil {
				return false, errors.Join(err, target.Close())
			}
		}
		var delta *FiniteExplanationProduct
		if i.result != VoidRegister {
			delta, err = explanationInstructionRegister(context, i.result)
			if err != nil {
				return false, err
			}
			if delta == nil {
				delta, err = newFiniteExplanationProduct(source.manager, source.finiteSorts, source.explanationColumns)
				if err != nil {
					return false, err
				}
				if err = context.SetExplanationRegister(i.result, delta); err != nil {
					return false, errors.Join(err, delta.Close())
				}
			}
		}
		if err = target.Union(source, delta); err != nil {
			return false, err
		}
		if delta != nil {
			count, err := delta.Len()
			if err != nil {
				return false, err
			}
			if count == 0 {
				return true, context.MakeEmpty(i.result)
			}
		}
		return true, nil
	}
	return false, ErrInstruction
}

func (i *tableInstruction) unionProjectedExplanationIntoTable(context *ExecutionContext, source *FiniteExplanationProduct) (bool, error) {
	target, err := instructionRegister(context, i.second)
	if err != nil {
		return false, err
	}
	if target == nil {
		signature, err := tableRelationSignature(source.manager, source.finiteSorts)
		if err != nil {
			return false, err
		}
		target = NewHashTable(signature)
		if err = context.SetRegister(i.second, target); err != nil {
			return false, errors.Join(err, target.Close())
		}
	}
	var delta *HashTable
	if i.result != VoidRegister {
		delta, err = instructionRegister(context, i.result)
		if err != nil {
			return false, err
		}
		if delta == nil {
			delta, err = emptyLike(target)
			if err != nil {
				return false, err
			}
			if err = context.SetRegister(i.result, delta); err != nil {
				return false, errors.Join(err, delta.Close())
			}
		}
	}
	converted, err := source.ToTableRelation()
	if err != nil {
		return false, err
	}
	// The execution registers already own target and delta. The source converter
	// projects only its temporary; no owning table wrapper is manufactured here.
	err = errors.Join(target.Union(converted.table, delta), converted.Close())
	if err != nil {
		return false, err
	}
	if delta != nil {
		count, err := delta.Len()
		if err != nil {
			return false, err
		}
		if count == 0 {
			return true, context.MakeEmpty(i.result)
		}
	}
	return true, nil
}

func setExplanationInstructionResult(context *ExecutionContext, register uint32, product *FiniteExplanationProduct) (bool, error) {
	count, err := product.Len()
	if err != nil {
		return false, errors.Join(err, product.Close())
	}
	if count == 0 {
		return true, errors.Join(product.Close(), context.MakeEmpty(register))
	}
	if err = context.SetExplanationRegister(register, product); err != nil {
		return false, errors.Join(err, product.Close())
	}
	return true, nil
}
