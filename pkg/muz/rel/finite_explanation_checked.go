package rel

import "github.com/glycerine/g3/pkg/ast"

// CheckedConcreteKinds reports the actual finite/index pair. Explanation
// AST ownership remains in the product's separately owned inner relations.
func (p *FiniteExplanationProduct) CheckedConcreteKinds() (primary, checker string, checked bool, err error) {
	if err = p.open(); err != nil {
		return "", "", false, err
	}
	index, ok := p.table.(*CheckedExplanationIndex)
	if !ok {
		return "", "", false, nil
	}
	primary, checker, err = index.ConcreteKinds()
	return primary, checker, true, err
}

// Derived products preserve an existing checked index's actual pair even
// after the manager restores its query-scoped factory. Plain products still
// follow an installed factory when they enter a checked query.
func (p *FiniteExplanationProduct) newProduct(sorts []ast.ASTHandle, explanations int) (*FiniteExplanationProduct, error) {
	primary, checker, checked, err := p.CheckedConcreteKinds()
	if err != nil {
		return nil, err
	}
	if !checked {
		return newFiniteExplanationProduct(p.manager, sorts, explanations)
	}
	factory, err := NewFiniteCheckedFactory(primary, checker)
	if err != nil {
		return nil, err
	}
	return newFiniteExplanationProductWithFactory(p.manager, sorts, explanations, factory)
}
