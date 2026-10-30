package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp join_planner pair cost model.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *SimpleJoinsPlugin) variableSorts(expressions []ast.ASTHandle) (map[uint32]ast.ASTHandle, error) {
	result := make(map[uint32]ast.ASTHandle)
	for _, expression := range expressions {
		if err := p.variables(expression, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (p *SimpleJoinsPlugin) domainCost(sort ast.ASTHandle) float32 {
	size, err := p.manager.GetSortSize(sort)
	if err != nil || !size.IsFinite() {
		return math.MaxFloat32
	}
	domain, err := size.Size()
	if err != nil {
		return math.MaxFloat32
	}
	return float32(domain)
}

func (p *SimpleJoinsPlugin) tailCost(tail ast.ASTHandle) (float32, map[uint32]ast.ASTHandle, error) {
	variables, err := p.variableSorts([]ast.ASTHandle{tail})
	if err != nil {
		return 0, nil, err
	}
	count, err := p.manager.GetNumArgs(tail)
	if err != nil {
		return 0, nil, err
	}
	declaration, err := p.manager.GetDecl(tail)
	if err != nil {
		return 0, nil, err
	}
	if p.relations != nil {
		saturated, x := p.relations.IsSaturated(declaration)
		if x != nil {
			return 0, nil, x
		}
		if saturated {
			table, found, x := p.relations.TryGetRelation(declaration)
			if x != nil {
				return 0, nil, x
			}
			if found {
				rows, x := table.SizeEstimateRows()
				if x != nil {
					return 0, nil, x
				}
				if rows != 0 {
					result := float32(rows)
					for index := uint32(0); index < count; index++ {
						argument, y := p.manager.GetArg(tail, index)
						if y != nil {
							return 0, nil, y
						}
						kind, y := p.manager.GetKind(argument)
						if y != nil {
							return 0, nil, y
						}
						if kind != ast.ASTVar {
							sort, y := p.manager.GetDomain(declaration, index)
							if y != nil {
								return 0, nil, y
							}
							result /= p.domainCost(sort)
						}
					}
					return result, variables, nil
				}
			}
		}
	}
	result := float32(1)
	for index := uint32(0); index < count; index++ {
		argument, x := p.manager.GetArg(tail, index)
		if x != nil {
			return 0, nil, x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return 0, nil, x
		}
		if kind == ast.ASTVar {
			sort, x := p.manager.GetDomain(declaration, index)
			if x != nil {
				return 0, nil, x
			}
			result *= p.domainCost(sort)
		}
	}
	return result, variables, nil
}

func (p *SimpleJoinsPlugin) estimatedPairCost(tails []ast.ASTHandle, outside []ast.ASTHandle, leftIndex, rightIndex int) (float32, error) {
	left, _, err := p.tailCost(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	right, _, err := p.tailCost(tails[rightIndex])
	if err != nil {
		return 0, err
	}
	other := make([]ast.ASTHandle, 0, len(outside)+len(tails)-2)
	other = append(other, outside...)
	for index, tail := range tails {
		if index != leftIndex && index != rightIndex {
			other = append(other, tail)
		}
	}
	nonLocal, err := p.variableSorts(other)
	if err != nil {
		return 0, err
	}
	intersectionSize := float32(1)
	leftArity, err := p.manager.GetNumArgs(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	rightArity, err := p.manager.GetNumArgs(tails[rightIndex])
	if err != nil {
		return 0, err
	}
	leftDecl, err := p.manager.GetDecl(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	rightDecl, err := p.manager.GetDecl(tails[rightIndex])
	if err != nil {
		return 0, err
	}
	// The source variable_intersection records positional matches, including
	// repeated occurrences of one variable. Its projected-column loops are
	// positional too, so keep multiplicity here instead of collapsing to sets.
	leftIndices := make([]uint32, 0, leftArity)
	leftVariables := make([]uint32, 0, leftArity)
	for index := uint32(0); index < leftArity; index++ {
		argument, x := p.manager.GetArg(tails[leftIndex], index)
		if x != nil {
			return 0, x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return 0, x
		}
		if kind == ast.ASTVar {
			variable, x := p.manager.GetVarIndex(argument)
			if x != nil {
				return 0, x
			}
			leftIndices = append(leftIndices, index)
			leftVariables = append(leftVariables, variable)
			if _, live := nonLocal[variable]; !live {
				sort, x := p.manager.GetDomain(leftDecl, index)
				if x != nil {
					return 0, x
				}
				intersectionSize *= p.domainCost(sort)
			}
		}
	}
	for index := uint32(0); index < rightArity; index++ {
		argument, x := p.manager.GetArg(tails[rightIndex], index)
		if x != nil {
			return 0, x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return 0, x
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, x := p.manager.GetVarIndex(argument)
		if x != nil {
			return 0, x
		}
		if _, live := nonLocal[variable]; !live {
			sort, x := p.manager.GetDomain(rightDecl, index)
			if x != nil {
				return 0, x
			}
			intersectionSize *= p.domainCost(sort)
		}
		for occurrence, leftVariable := range leftVariables {
			if leftVariable != variable {
				continue
			}
			if _, live := nonLocal[variable]; !live {
				continue
			}
			leftSort, x := p.manager.GetDomain(leftDecl, leftIndices[occurrence])
			if x != nil {
				return 0, x
			}
			intersectionSize *= p.domainCost(leftSort)
		}
	}
	if intersectionSize == 0 {
		return 0, nil
	}
	cost := left * right / intersectionSize
	return cost, nil
}
