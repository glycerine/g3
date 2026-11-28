package smtcore

import "github.com/glycerine/g3/pkg/ast"

func skolemConstant(manager *ast.ASTManager, term ast.ASTHandle) (constant, skolem bool, err error) {
	header, err := manager.NodeHeader(term)
	if err != nil || header.Kind != ast.ASTApp || header.Family != ast.NullFamilyID || header.NumArgs != 0 {
		return false, false, err
	}
	skolem, err = manager.IsSkolemDecl(header.Decl)
	return true, skolem, err
}

// A pair of different fresh constants can give a binary instance little in
// common with the existing ground clauses. If the second model value also
// has a named representative, keep the first binding and try that pair.
// Preserve the inverse map's generation and use its completed evaluations.
func (c *SMTModelChecker) namedSecondBinding(q, first, second, value ast.ASTHandle, generation uint32) (ast.ASTHandle, error) {
	if first == second {
		return second, nil
	}
	_, fresh, err := skolemConstant(c.manager, second)
	if err != nil || !fresh {
		return second, err
	}
	_, fresh, err = skolemConstant(c.manager, first)
	if err != nil || !fresh {
		return second, err
	}
	firstSort, err := c.manager.GetSort(first)
	if err != nil {
		return second, err
	}
	secondSort, err := c.manager.GetSort(second)
	if err != nil || firstSort != secondSort {
		return second, err
	}
	set, err := c.modelFinder.instantiationSet(q, 1, false)
	if err != nil || set == nil {
		return second, err
	}
	evaluated := c.modelFinder.aufSolver.evalCache[1]
	for _, entry := range set.entries() {
		if uint32(entry.Num) != generation || entry.Key == second || evaluated[entry.Key] != value {
			continue
		}
		constant, skolem, err := skolemConstant(c.manager, entry.Key)
		if err != nil {
			return second, err
		}
		if constant && !skolem {
			return entry.Key, nil
		}
	}
	return second, nil
}
