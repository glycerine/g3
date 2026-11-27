package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Guarded agreements use a small private projection. A disjunction of cases
// can need additional witnesses from the premises. Reuse those instances and
// their SAT clauses across siblings with identical premises.
func (p *orderProofState) proveGuardedGoal(m *ast.ASTManager, assertions []ast.ASTHandle, steps uint32) (bool, error) {
	goal := assertions[len(assertions)-1]
	agreement := guardedPredicateAgreement(m, goal)
	if !agreement && !guardedEqualityCases(m, goal) {
		return false, nil
	}
	proofs, err := m.ProofsEnabled()
	if err != nil || proofs {
		return false, err
	}
	steps = min(steps, 2_000_000)
	if !agreement {
		if p.guardedCases == nil {
			p.guardedCases = new(groundInstanceSolver)
		}
		return p.guardedCases.proveInstances(p.source, m, assertions, steps, 1, guardedCaseSearch)
	}
	var private groundInstanceSolver
	proved, err := private.proveInstances(p.source, m, assertions, steps, 1, predicateAgreementSearch)
	err = errors.Join(err, private.close())
	return proved && err == nil && !m.ResourceLimit().CancelFlag(), err
}

// Recognize a negated guarded disjunction whose alternatives are conjunctions,
// at least one of which contains an equality. Both n-ary and binary trees occur.
func guardedEqualityCases(m *ast.ASTManager, goal ast.ASTHandle) bool {
	remaining := 4
	root, h, ok := (signedRefutationGoal{term: goal}).unwrap(m, &remaining)
	if !ok || !root.negated || h.Family != m.BasicFamilyID() || h.DeclKind != ast.ASTOpImplies || h.NumArgs != 2 {
		return false
	}
	guard, err := m.GetArg(root.term, 0)
	if err != nil {
		return false
	}
	gh, err := m.NodeHeader(guard)
	if err != nil || gh.Family != m.BasicFamilyID() || gh.DeclKind != ast.ASTOpAnd {
		return false
	}
	rhs, err := m.GetArg(root.term, 1)
	if err != nil {
		return false
	}
	rh, err := m.NodeHeader(rhs)
	if err != nil || rh.Family != m.BasicFamilyID() || rh.DeclKind != ast.ASTOpOr {
		return false
	}
	remaining = 96
	equal := false
	return visitGuardedCases(m, rhs, true, &remaining, &equal) && equal
}

func visitGuardedCases(m *ast.ASTManager, t ast.ASTHandle, alternatives bool, remaining *int, equal *bool) bool {
	if *remaining == 0 {
		return false
	}
	*remaining--
	h, err := m.NodeHeader(t)
	if err != nil {
		return false
	}
	if h.Family != m.BasicFamilyID() {
		return !alternatives
	}
	if h.DeclKind == ast.ASTOpAnd {
		alternatives = false
	} else if !alternatives || h.DeclKind != ast.ASTOpOr {
		if h.DeclKind == ast.ASTOpEq {
			*equal = true
		}
		return !alternatives
	}
	for i := range h.NumArgs {
		a, err := m.GetArg(t, i)
		if err != nil || !visitGuardedCases(m, a, alternatives, remaining, equal) {
			return false
		}
	}
	return true
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
