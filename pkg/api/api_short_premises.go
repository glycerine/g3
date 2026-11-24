package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
)

// source includes the omitted conjuncts and goal so validation can reject
// escaped bound variables and charge the full input to the child limit.
type shortPremisePlan struct {
	source, selected []ast.ASTHandle
	conjuncts        []ast.ASTHandle // All positive conjuncts for the ground projection.
}

// constantTrueGoal accepts singleton Boolean wrappers around true or not false.
func constantTrueGoal(m *ast.ASTManager, goal ast.ASTHandle) (bool, error) {
	negative := false
	for depth := 0; depth < 12; depth++ {
		hd, e := m.NodeHeader(goal)
		if e != nil {
			return false, e
		}
		if hd.Kind != ast.ASTApp || hd.Family != m.BasicFamilyID() {
			return false, nil
		}
		switch hd.DeclKind {
		case ast.ASTOpTrue:
			return !negative, nil
		case ast.ASTOpFalse:
			return negative, nil
		case ast.ASTOpNot:
			negative = !negative
		case ast.ASTOpAnd, ast.ASTOpOr:
		default:
			return false, nil
		}
		if hd.NumArgs != 1 {
			return false, nil
		}
		goal, e = m.GetArg(goal, 0)
		if e != nil {
			return false, e
		}
	}
	return false, nil
}

// For a constant true query, the premises themselves must be inconsistent.
// Only positive top-level conjuncts can be asserted independently. Keep the
// source order and try a bounded check of ground facts and short universals.
func (p *orderProofState) shortPremises(m *ast.ASTManager, assertions []ast.ASTHandle) (*shortPremisePlan, error) {
	if len(assertions) < 2 || len(assertions) > 8192 {
		return nil, nil
	}
	base, goal := assertions[:len(assertions)-1], assertions[len(assertions)-1]
	yes, err := constantTrueGoal(m, goal)
	if err != nil || !yes {
		return nil, err
	}
	index, err := p.indexedPremises(m, base)
	if err != nil || index == nil {
		return nil, err
	}
	var selected []ast.ASTHandle
	var facts, universals int
	for _, row := range index.rows {
		switch {
		case row.ground:
			facts++
		case row.shortUniversal:
			universals++
		default:
			continue
		}
		selected = append(selected, row.root)
	}
	if facts < 8 || universals < 8 {
		return nil, nil
	}
	plan := &shortPremisePlan{source: assertions, selected: selected}
	if len(selected) >= 64 {
		plan.conjuncts = selected
		if len(index.rows) != len(selected) {
			plan.conjuncts = make([]ast.ASTHandle, len(index.rows))
			for i, row := range index.rows {
				plan.conjuncts[i] = row.root
			}
		}
	}
	return plan, nil
}

// For larger constant-true queries, first look for a contradiction among
// ground facts and instances of universal premises. If the mixed-search
// allowance reaches half its cap, or the subset still contains most of the
// conjuncts, leave the mixed search to the ordinary solver.
func (p *orderProofState) checkShortPremises(m *ast.ASTManager, plan *shortPremisePlan, maxSteps uint32) (bool, error) {
	const mixedLimit = 80_000
	steps := min(maxSteps, mixedLimit, 192*uint32(len(plan.selected)))
	if len(plan.conjuncts) != 0 {
		groundSteps := min(maxSteps, 400_000)
		bits, err := order.ProjectGroundEqualities(m, plan.conjuncts, groundSteps)
		if err != nil || m.ResourceLimit().CancelFlag() {
			return false, err
		}
		if bits != nil {
			if p.ctx != nil {
				if err := p.release(); err != nil {
					return false, err
				}
			}
			yes, err := p.checkBooleanInput(m, bits, groundSteps, plan.source)
			if yes || err != nil || m.ResourceLimit().CancelFlag() {
				return yes, err
			}
			// A failed Boolean check can leave assumptions in its kernel.
			// Release it before a different proof can succeed on this query.
			if err := p.release(); err != nil {
				return false, err
			}
		}
		if steps >= mixedLimit/2 || len(plan.selected)*4 >= len(plan.conjuncts)*3 {
			return false, nil
		}
	}
	return p.checkProofPlan(m, plan, steps)
}

func (p *shortPremisePlan) assertIn(parent, target *ast.ASTManager, child *APISimpleSolver) (complete bool, err error) {
	input := witnessClauseSet{m: target}
	if ok, err := input.acceptsInput(parent, p.source); err != nil || !ok {
		return false, err
	}
	tr, err := ast.NewASTTranslation(parent, target, true)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, tr.Close()) }()
	return assertTranslatedPremises(tr, target, child, p.selected)
}
