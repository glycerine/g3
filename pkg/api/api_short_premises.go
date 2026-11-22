package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// source includes the omitted conjuncts and goal so validation can reject
// escaped bound variables and charge the full input to the child limit.
type shortPremisePlan struct {
	source, selected []ast.ASTHandle
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
	return &shortPremisePlan{source: assertions, selected: selected}, nil
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
