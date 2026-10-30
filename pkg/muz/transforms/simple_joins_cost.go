package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp join_planner pair cost model.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
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

func (p *SimpleJoinsPlugin) estimatedPairCost(set *base.RuleSet, head ast.ASTHandle, tails []ast.ASTHandle, outside []ast.ASTHandle, leftIndex, rightIndex int) (float32, error) {
	left, leftVars, err := p.tailCost(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	right, rightVars, err := p.tailCost(tails[rightIndex])
	if err != nil {
		return 0, err
	}
	other := make([]ast.ASTHandle, 0, len(outside)+len(tails)-2)
	other = append(other, head)
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
	for variable, leftSort := range leftVars {
		if rightSort, common := rightVars[variable]; common {
			if _, live := nonLocal[variable]; live {
				intersectionSize *= p.domainCost(leftSort)
			}
			_ = rightSort // Well-typed predicate applications share the variable sort.
		}
	}
	for _, variables := range []map[uint32]ast.ASTHandle{leftVars, rightVars} {
		for variable, sort := range variables {
			if _, live := nonLocal[variable]; !live {
				intersectionSize *= p.domainCost(sort)
			}
		}
	}
	if intersectionSize == 0 {
		return 0, nil
	}
	cost := left * right / intersectionSize
	leftDecl, err := p.manager.GetDecl(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	rightDecl, err := p.manager.GetDecl(tails[rightIndex])
	if err != nil {
		return 0, err
	}
	leftStratum, leftErr := set.PredicateStratum(leftDecl)
	rightStratum, rightErr := set.PredicateStratum(rightDecl)
	headDecl, headErr := p.manager.GetDecl(head)
	headStratum, headStratumErr := set.PredicateStratum(headDecl)
	if leftErr == nil && rightErr == nil && headErr == nil && headStratumErr == nil {
		sourceStratum := leftStratum
		if rightStratum > sourceStratum {
			sourceStratum = rightStratum
		}
		if headStratum > sourceStratum && cost > 0 {
			cost *= 1.0 / 16.0
		}
	}
	return cost, nil
}

func (p *SimpleJoinsPlugin) bestPair(set *base.RuleSet, head ast.ASTHandle, tails, outside []ast.ASTHandle) (int, int, error) {
	if len(tails) < 2 {
		return -1, -1, nil
	}
	bestLeft, bestRight := 0, 1
	bestCost, err := p.estimatedPairCost(set, head, tails, outside, bestLeft, bestRight)
	if err != nil {
		return -1, -1, err
	}
	for left := 0; left < len(tails); left++ {
		for right := left + 1; right < len(tails); right++ {
			if left == bestLeft && right == bestRight {
				continue
			}
			cost, x := p.estimatedPairCost(set, head, tails, outside, left, right)
			if x != nil {
				return -1, -1, x
			}
			if cost < bestCost {
				bestLeft, bestRight, bestCost = left, right, cost
			}
		}
	}
	return bestLeft, bestRight, nil
}
