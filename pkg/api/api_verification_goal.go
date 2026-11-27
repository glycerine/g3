package api

import "github.com/glycerine/g3/pkg/ast"

const (
	maxPrivateGoalWidth = 5
	maxNarrowPremises   = 32
	maxWidePremises     = 220
)

// A first push preprocesses the entire base. After proving a sibling goal,
// try a small set of original conjuncts in a separate bounded SMT context.
// A satisfiable or unknown subset leaves the full query undecided.
func (p *orderProofState) proveRelevantPremises(m *ast.ASTManager, assertions []ast.ASTHandle) (bool, error) {
	if len(assertions) < 2 {
		return false, nil
	}
	goal := assertions[len(assertions)-1]
	hd, err := m.NodeHeader(goal)
	if err != nil || hd.Kind != ast.ASTApp || hd.NumArgs == 0 || hd.NumArgs > maxPrivateGoalWidth {
		return false, err
	}
	// A unary Boolean wrapper can hide a wide goal. Use the wrapped
	// connective's width before building indices for a private solver.
	if hd.Family == m.BasicFamilyID() && hd.NumArgs == 1 {
		remaining := 32
		_, inner, ok := (signedRefutationGoal{term: goal}).unwrap(m, &remaining)
		if ok && inner.Kind == ast.ASTApp && inner.Family == m.BasicFamilyID() &&
			(inner.DeclKind == ast.ASTOpAnd || inner.DeclKind == ast.ASTOpOr) && inner.NumArgs > maxPrivateGoalWidth {
			return false, nil
		}
	}
	ground, err := m.IsGround(goal)
	if err != nil || !ground {
		return false, err
	}
	var walk witnessSymbolWalk
	relations, ok, err := walk.collect(m, goal)
	if err != nil || !ok || len(relations) == 0 || len(relations) > 10 {
		return false, err
	}
	base, err := p.indexedPremises(m, assertions[:len(assertions)-1])
	if err != nil || base == nil {
		return false, err
	}
	defs, err := p.indexDefinitions(m, base)
	if err != nil || defs == nil {
		return false, err
	}
	walk.includeConstants = true
	symbols, ok, err := walk.collect(m, goal)
	if err != nil || !ok {
		return false, err
	}
	// With a large base, very small definition and rare-symbol subsets
	// commonly admit models. In a sized batch their failed checks precede
	// the ordinary incremental search. Keep the wider selection eligible.
	minSelected := 1
	if p.expectedGoals > 0 && p.plans.reusable(m) && len(base.rows) > 2*maxNarrowPremises {
		minSelected = (len(base.rows) + 7) / 8
	}
	check := func(subset []ast.ASTHandle, minimum int, steps uint32) (bool, error) {
		if len(subset) < minimum {
			return false, nil
		}
		input := append(subset, goal)
		return p.checkProofPlan(m, &shortPremisePlan{source: assertions, selected: input}, steps)
	}
	narrow := defs.selectRows(m, symbols, false, maxNarrowPremises)
	if yes, err := check(narrow, minSelected, 6000); yes || err != nil || m.ResourceLimit().CancelFlag() {
		return yes, err
	}
	wideLimit := maxWidePremises
	if p.plans.reusable(m) {
		// An inconclusive search pays for both the private solver and the
		// source's first push. Require it to save a full batch of premises.
		wideLimit = min(wideLimit, len(base.rows)-maxWidePremises)
		// After the definition search has already paid for a solver check,
		// try additional constraints only if they still fit a small plan.
		if len(narrow) != 0 {
			wideLimit = min(wideLimit, maxNarrowPremises)
		}
	}
	if yes, err := check(defs.selectRows(m, symbols, true, wideLimit), 1, 100000); yes || err != nil || m.ResourceLimit().CancelFlag() {
		return yes, err
	}
	var subset []ast.ASTHandle
	for i, row := range base.rows {
		if i%128 == 0 && m.ResourceLimit().CancelFlag() {
			return false, nil
		}
		for _, h := range row.symbols {
			// The witness index stores twice the least frequency in each row.
			if 2*base.frequency[h] <= row.threshold && containsDeclaration(relations, h) {
				subset = append(subset, row.root)
				break
			}
		}
		if len(subset) > maxNarrowPremises {
			return false, nil
		}
	}
	if len(subset)*3 > len(base.rows) {
		return false, nil
	}
	return check(subset, minSelected, 5000)
}
