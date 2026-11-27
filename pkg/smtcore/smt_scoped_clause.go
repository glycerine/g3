package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The lower width bound amortizes the premise scan over a broad disjunction.
const minScopedClauseWidth = 16

// Keep the public assertion and the row used by commitAssertedTo intact.
// Only the clause entering search can omit literals that are already false
// under the previously committed rows. All those rows outlive this clause.
func (k *SMTKernel) internalizePendingRow(head int) (next int, err error) {
	a := k.assertions[head]
	formula := a.Formula
	if k.reduceScopedClauses && k.assertedHead > 0 && len(k.scopes) > 0 && k.assertedHasQuantifiers && a.Proof.IsNull() {
		remaining, changed, matchErr := k.remainingClauseLiterals(formula)
		if matchErr != nil {
			return head, matchErr
		}
		if changed {
			scope := &smtAssertionScope{manager: k.manager}
			defer func() { err = scope.finish(err) }()
			scope.inputs[0], err = k.manager.NewRef(ast.ASTHandle{})
			if err != nil {
				return head, err
			}
			formula, err = k.manager.MkOrArray(remaining)
			if err != nil {
				return head, err
			}
			if err = k.manager.SetRef(scope.inputs[0], formula); err != nil {
				return head, err
			}
		}
	}
	if err := k.internalizer.Assert(formula, a.Proof, 0); err != nil {
		return head, err
	}
	// A refused release still leaves a successfully asserted row in search.
	return head + 1, nil
}

func (k *SMTKernel) remainingClauseLiterals(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	m := k.manager
	hd, err := m.NodeHeader(h)
	if err != nil || hd.Kind != ast.ASTApp || hd.Family != m.BasicFamilyID() ||
		hd.DeclKind != ast.ASTOpOr || hd.NumArgs < minScopedClauseWidth || hd.NumArgs > 128 || k.assertedHead > 8192 {
		return nil, false, err
	}
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() || m.TraceStream != nil ||
		m.Budget().InjectionArmed() || m.ResourceLimit().Limit != 0 || k.smtState.MaxConflicts != ^uint32(0) ||
		m.NumeralService().RuntimeOwner().AllocationLimited() {
		return nil, false, nil
	}
	ground, err := m.IsGround(h)
	if err != nil || !ground {
		return nil, false, err
	}
	proofs, err := m.ProofsEnabled()
	if err != nil || proofs || m.ResourceLimit().CancelFlag() {
		return nil, false, err
	}
	// Build the index from this kernel's live prefix for this call. Pending
	// rows, including the query itself, cannot justify dropping a literal.
	index := UniversalPremises{manager: m}
	work := 18_750
	for _, row := range k.assertions[:k.assertedHead] {
		if work == 0 {
			break
		}
		if err := index.collect(row.Formula, true, universalRule{}, 0, &work); err != nil {
			return nil, false, err
		}
	}
	if len(index.rules) == 0 {
		return nil, false, nil
	}
	work = 100_000
	remaining := make([]ast.ASTHandle, 0, hd.NumArgs)
	for i := uint32(0); i < hd.NumArgs; i++ {
		child, err := m.GetArg(h, i)
		if err != nil {
			return nil, false, err
		}
		if !index.entails(child, false, 0, &work) {
			remaining = append(remaining, child)
		}
	}
	return remaining, len(remaining) < int(hd.NumArgs) && !m.ResourceLimit().CancelFlag(), nil
}
