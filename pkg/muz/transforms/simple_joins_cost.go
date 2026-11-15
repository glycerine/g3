package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp join_planner pair cost model.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/muz/base"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *SimpleJoinsPlugin) tracePairCost(left, right ast.ASTHandle, cost float32) error {
	return p.manager.EmitTrace("report_costs", "compute_cost", "src/muz/rel/dl_mk_simple_joins.cpp", 655, func(w io.Writer) error {
		for _, predicate := range []ast.ASTHandle{left, right} {
			if err := p.writeSimpleJoinPredicate(w, predicate); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "%g\n", cost)
		return err
	})
}

func (p *SimpleJoinsPlugin) writeSimpleJoinPredicate(w io.Writer, predicate ast.ASTHandle) error {
	declaration, err := p.manager.GetDecl(predicate)
	if err != nil {
		return err
	}
	name, err := p.manager.GetASTName(declaration)
	if err != nil {
		return err
	}
	text, err := p.manager.Symbols.String(name)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "%s(", text); err != nil {
		return err
	}
	count, err := p.manager.GetNumArgs(predicate)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		if index != 0 {
			if _, err = io.WriteString(w, ","); err != nil {
				return err
			}
		}
		argument, x := p.manager.GetArg(predicate, index)
		if x != nil {
			return x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return x
		}
		if kind != ast.ASTVar {
			return fmt.Errorf("muz/transforms: simple-join trace expects variable predicate arguments: %w", ast.ErrASTDomain)
		}
		variable, x := p.manager.GetVarIndex(argument)
		if x != nil {
			return x
		}
		if _, err = fmt.Fprintf(w, "#%d", variable); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, ")")
	return err
}

// writeRegisterPairTrace is the TRACE("dl") of join_planner::register_pair.
func (p *SimpleJoinsPlugin) writeRegisterPairTrace(w io.Writer, t1, t2 ast.ASTHandle, nonLocalVars []uint32, rule *base.Rule, inf *pairInfo) error {
	if err := p.writeSimpleJoinPredicate(w, t1); err != nil {
		return err
	}
	if _, err := io.WriteString(w, " "); err != nil {
		return err
	}
	if err := p.writeSimpleJoinPredicate(w, t2); err != nil {
		return err
	}
	for _, variable := range nonLocalVars {
		if _, err := fmt.Fprintf(w, " %d", variable); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	if err := p.rules.WriteRuleDisplay(w, rule); err != nil {
		return err
	}
	if inf.canBeJoined() {
		_, err := fmt.Fprintf(w, "cost: %g\n", inf.getCost())
		return err
	}
	return nil
}

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
		// context::get_sort_size_estimate returns uint64 maximum, which
		// join_planner then converts to its float32 cost before arithmetic.
		return float32(math.MaxUint64)
	}
	domain, err := size.Size()
	if err != nil {
		return float32(math.MaxUint64)
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
			rows, found, x := p.relations.RelationSizeEstimateRows(declaration)
			if x != nil {
				return 0, nil, x
			}
			if found {
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

// computeCost is join_planner::compute_cost.
func (p *SimpleJoinsPlugin) computeCost(t1, t2 ast.ASTHandle, nonLocalVars map[uint32]struct{}) (float32, error) {
	t1Pred, err := p.manager.GetDecl(t1)
	if err != nil {
		return 0, err
	}
	t2Pred, err := p.manager.GetDecl(t2)
	if err != nil {
		return 0, err
	}
	n1, err := p.manager.GetNumArgs(t1)
	if err != nil {
		return 0, err
	}
	n2, err := p.manager.GetNumArgs(t2)
	if err != nil {
		return 0, err
	}
	argVar := func(t ast.ASTHandle, i uint32) (uint32, bool, error) {
		argument, err := p.manager.GetArg(t, i)
		if err != nil {
			return 0, false, err
		}
		kind, err := p.manager.GetKind(argument)
		if err != nil || kind != ast.ASTVar {
			return 0, false, err
		}
		index, err := p.manager.GetVarIndex(argument)
		return index, err == nil, err
	}
	domainSize := func(pred ast.ASTHandle, i uint32) (float32, error) {
		s, err := p.manager.GetDomain(pred, i)
		if err != nil {
			return 0, err
		}
		return p.domainCost(s), nil
	}
	intersSize := float32(1)
	// variable_intersection vi; vi.populate(t1, t2): remove contributions
	// from joined columns.
	for i1 := uint32(0); i1 < n1; i1++ {
		v1, isVar1, err := argVar(t1, i1)
		if err != nil {
			return 0, err
		}
		if !isVar1 {
			continue
		}
		for i2 := uint32(0); i2 < n2; i2++ {
			v2, isVar2, err := argVar(t2, i2)
			if err != nil {
				return 0, err
			}
			if !isVar2 || v1 != v2 {
				continue
			}
			if _, found := nonLocalVars[v1]; found {
				d, err := domainSize(t1Pred, i1)
				if err != nil {
					return 0, err
				}
				intersSize *= d
			}
		}
	}
	// remove contributions from projected columns.
	for _, side := range []struct {
		t, pred ast.ASTHandle
		n       uint32
	}{{t1, t1Pred, n1}, {t2, t2Pred, n2}} {
		for i := uint32(0); i < side.n; i++ {
			v, isVar, err := argVar(side.t, i)
			if err != nil {
				return 0, err
			}
			if !isVar {
				continue
			}
			if _, found := nonLocalVars[v]; !found {
				d, err := domainSize(side.pred, i)
				if err != nil {
					return 0, err
				}
				intersSize *= d
			}
		}
	}
	s1, _, err := p.tailCost(t1)
	if err != nil {
		return 0, err
	}
	s2, _, err := p.tailCost(t2)
	if err != nil {
		return 0, err
	}
	res := s1 * s2 / intersSize
	if err = p.tracePairCost(t1, t2, res); err != nil {
		return 0, err
	}
	return res, nil
}
