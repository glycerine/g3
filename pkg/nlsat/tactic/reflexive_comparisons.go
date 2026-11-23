package tactic

import "github.com/glycerine/g3/pkg/ast"

type componentComparison struct {
	form, left, right ast.ASTHandle
}

// A model can give identically constrained variables the same value even
// when they are linked by a non-strict comparison of identical expressions.
// Try this only when ordinary independent-component grouping found little.
func (p *polynomialComponents) groupReflexiveComparisons(forms []ast.ASTHandle) (*polynomialComponents, error) {
	var comparisons []componentComparison
	var rest []ast.ASTHandle
	for i, form := range forms {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return nil, nil
		}
		if p.terms[form].variable == mixedComponent {
			cmp, ok, err := p.reflexiveComparison(form)
			if err != nil {
				return nil, err
			}
			if ok {
				if len(comparisons) == 0 {
					rest = make([]ast.ASTHandle, 0, len(forms))
					rest = append(rest, forms[:i]...)
				}
				comparisons = append(comparisons, cmp)
				continue
			}
		}
		if len(comparisons) != 0 {
			rest = append(rest, form)
		}
	}
	if len(comparisons) < 3 {
		return nil, nil
	}
	// Scan the remaining assertions from scratch. In particular, a comparison
	// used again inside a Boolean formula still couples its operands there.
	q, err := scanPolynomialComponents(p.manager, p.arithmetic, rest, p.parameters)
	if err != nil || q == nil || q.group() < 4 {
		return nil, err
	}
	q.reflexive = make(map[ast.ASTHandle]bool, len(comparisons))
	for i, cmp := range comparisons {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return nil, nil
		}
		left := q.comparisonRepresentative(cmp.left)
		if left == 0 || left != q.comparisonRepresentative(cmp.right) {
			return nil, nil
		}
		q.reflexive[cmp.form] = true
	}
	return q, nil
}

func (p *polynomialComponents) reflexiveComparison(form ast.ASTHandle) (componentComparison, bool, error) {
	m := p.manager
	var cmp componentComparison
	h, err := m.NodeHeader(form)
	if err != nil || h.NumArgs != 2 {
		return cmp, false, err
	}
	if !(h.Family == p.arithmetic && (h.DeclKind == candidateLE || h.DeclKind == candidateGE) ||
		h.Family == m.BasicFamilyID() && h.DeclKind == ast.ASTOpEq) {
		return cmp, false, nil
	}
	left, err := m.GetArg(form, 0)
	if err != nil {
		return cmp, false, err
	}
	right, err := m.GetArg(form, 1)
	if err != nil {
		return cmp, false, err
	}
	a, b := p.terms[left], p.terms[right]
	if a.shape != b.shape || a.variable == 0 || b.variable == 0 ||
		a.variable == mixedComponent || b.variable == mixedComponent {
		return cmp, false, nil
	}
	// The compound operands might occur only in this comparison. Their
	// underlying constants identify the variables across the two scans.
	return componentComparison{form, p.variables[a.variable-1].expression, p.variables[b.variable-1].expression}, true, nil
}

func (p *polynomialComponents) comparisonRepresentative(x ast.ASTHandle) uint32 {
	id := p.terms[x].variable
	if id == 0 {
		return 0
	}
	v := &p.variables[id-1]
	if v.mixedNodes != 0 || len(v.constraints) == 0 {
		return 0
	}
	if v.representative != 0 {
		return v.representative
	}
	return id
}
