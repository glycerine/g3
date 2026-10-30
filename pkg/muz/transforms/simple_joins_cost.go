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
	left, leftVars, err := p.tailCost(tails[leftIndex])
	if err != nil {
		return 0, err
	}
	right, rightVars, err := p.tailCost(tails[rightIndex])
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
	return cost, nil
}

func (p *SimpleJoinsPlugin) bestPair(tails, outside []ast.ASTHandle, plan *simpleJoinPlan) (int, int, error) {
	if len(tails) < 2 {
		return -1, -1, nil
	}
	bestLeft, bestRight := 0, 1
	bestCost, err := p.estimatedPairCost(tails, outside, bestLeft, bestRight)
	if err != nil {
		return -1, -1, err
	}
	bestCost = p.amortizedPairCost(tails[bestLeft], tails[bestRight], bestCost, plan)
	for left := 0; left < len(tails); left++ {
		for right := left + 1; right < len(tails); right++ {
			if left == bestLeft && right == bestRight {
				continue
			}
			cost, x := p.estimatedPairCost(tails, outside, left, right)
			if x != nil {
				return -1, -1, x
			}
			cost = p.amortizedPairCost(tails[left], tails[right], cost, plan)
			if cost < bestCost {
				bestLeft, bestRight, bestCost = left, right, cost
			}
		}
	}
	return bestLeft, bestRight, nil
}

func (p *SimpleJoinsPlugin) amortizedPairCost(left, right ast.ASTHandle, fallback float32, plan *simpleJoinPlan) float32 {
	if plan == nil {
		return fallback
	}
	pair, err := p.normalizePair(left, right)
	if err != nil {
		return fallback
	}
	consumers := plan.consumers[pair.key]
	if consumers == 0 {
		return fallback
	}
	cost := plan.costs[pair.key] / float32(consumers)
	if plan.stratified[pair.key] && cost > 0 {
		cost /= 16
	}
	return cost
}
