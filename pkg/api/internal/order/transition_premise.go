package order

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

// addInductionInstance uses a positive universal premise with four or more
// binders. Check for that shape before reading an uninterpreted-sort problem
// for this attempt. Exhausting the scan simply leaves the ordinary solver to
// handle it. The cap bounds work on a shared conjunction DAG.
func scanTransitionPremises(m *ast.ASTManager, base []ast.ASTHandle) (bool, error) {
	work := 512
	var visit func(ast.ASTHandle, int) (bool, error)
	visit = func(h ast.ASTHandle, depth int) (bool, error) {
		if work == 0 {
			return false, nil
		}
		work--
		if depth > 12 || m.ResourceLimit().CancelFlag() {
			return false, nil
		}
		hd, err := m.NodeHeader(h)
		if err != nil {
			return false, err
		}
		if hd.Kind == ast.ASTQuantifier {
			n, err := m.GetNumDeclarations(h)
			if err != nil || n < 4 {
				return false, err
			}
			return m.IsForall(h)
		}
		if hd.Kind != ast.ASTApp || hd.Family != m.BasicFamilyID() || hd.DeclKind != ast.ASTOpAnd {
			return false, nil
		}
		for j := uint32(0); j < hd.NumArgs && work > 0; j++ {
			child, err := m.GetArg(h, j)
			if err != nil {
				return false, err
			}
			found, err := visit(child, depth+1)
			if err != nil || found {
				return found, err
			}
		}
		return false, nil
	}
	for _, h := range base {
		found, err := visit(h, 0)
		if err != nil || found {
			return found, err
		}
		if work <= 0 {
			break
		}
	}
	return false, nil
}

// TransitionPremises remembers a batch's admission test for uninterpreted
// domains. A changed assertion, generation or manager requires a new scan.
type TransitionPremises struct {
	manager *ast.ASTManager
	base    []ast.ASTHandle
	wide    bool
}

func (p *TransitionPremises) eligible(m *ast.ASTManager, base []ast.ASTHandle) (bool, error) {
	if p.manager == m && slices.Equal(base, p.base) {
		return p.wide, nil
	}
	found, err := scanTransitionPremises(m, base)
	// Cap the handles retained by a long-lived batch. They are weak handles.
	if err == nil && !m.ResourceLimit().CancelFlag() && len(base) <= 1024 {
		p.manager = m
		p.base = append(p.base[:0], base...)
		p.wide = found
	}
	return found, err
}
