package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// A failed guarded agreement supplies a small set of ground constants for
// instantiating universal premises. Check the resulting ground problem only
// after the ordinary batch projections have been inconclusive.
func provePredicateAgreement(src *APISimpleSolver, m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (bool, error) {
	if !guardedPredicateAgreement(m, assertions[len(assertions)-1]) {
		return false, nil
	}
	proofs, err := m.ProofsEnabled()
	if err != nil || proofs {
		return false, err
	}
	var private groundInstanceSolver
	proved, err := private.proveInstances(src, m, assertions, min(steps, 2_000_000), 1, predicateAgreementSearch)
	err = errors.Join(err, private.close())
	return proved && err == nil && !m.ResourceLimit().CancelFlag(), err
}

// Recognize the negation of guard => (P(constants) = Q(constants)).
// The shape schedules a proof attempt. Only UNSAT of the ground instances
// establishes that the source counterexample is impossible.
func guardedPredicateAgreement(m *ast.ASTManager, goal ast.ASTHandle) bool {
	remaining := 4
	root, h, ok := (signedRefutationGoal{term: goal}).unwrap(m, &remaining)
	if !ok || !root.negated || h.Family != m.BasicFamilyID() || h.DeclKind != ast.ASTOpImplies || h.NumArgs != 2 {
		return false
	}
	eq, err := m.GetArg(root.term, 1)
	if err != nil {
		return false
	}
	h, err = m.NodeHeader(eq)
	if err != nil || h.Family != m.BasicFamilyID() || h.NumArgs != 2 || (h.DeclKind != ast.ASTOpEq && h.DeclKind != ast.ASTOpIff) {
		return false
	}
	var operands [2]ast.ASTHandle
	var arity uint32
	for i := range operands {
		operands[i], err = m.GetArg(eq, uint32(i))
		if err != nil {
			return false
		}
		h, err = m.NodeHeader(operands[i])
		if err != nil || h.Kind != ast.ASTApp || h.Family != ast.NullFamilyID || h.NumArgs == 0 || h.NumArgs > 4 {
			return false
		}
		if i == 0 {
			arity = h.NumArgs
		} else if h.NumArgs != arity {
			return false
		}
	}
	boolean, err := m.HasBoolSort(operands[0])
	if err != nil || !boolean {
		return false
	}
	for i := range arity {
		left, err := m.GetArg(operands[0], i)
		if err != nil {
			return false
		}
		right, err := m.GetArg(operands[1], i)
		if err != nil || left != right {
			return false
		}
		h, err := m.NodeHeader(left)
		if err != nil || h.Kind != ast.ASTApp || h.Family != ast.NullFamilyID || h.NumArgs != 0 {
			return false
		}
	}
	return true
}
