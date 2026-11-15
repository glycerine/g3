package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func isSparseIntervalRepresentation(kinds []string) bool {
	return len(kinds) == 2 && (kinds[0] == "tr_sparse" && kinds[1] == "interval_relation" || kinds[1] == "tr_sparse" && kinds[0] == "interval_relation")
}

func (e *PropositionalEngine) selectSparseIntervalRepresentation(declaration ast.ASTHandle, kinds []string) error {
	state := e.predicates[declaration]
	if state == nil || state.external != nil {
		return ErrPropositionalHorn
	}
	supported, err := rel.CanSelectSparseIntervalSieveProduct(e.relations, state.sorts, kinds)
	if err != nil {
		return err
	}
	if !supported {
		return ErrSparseIntervalDatalogFragment
	}
	pureFinite, err := rel.SparseIntervalSignatureIsPureFinite(e.relations, state.sorts)
	if err != nil {
		return err
	}
	if pureFinite {
		// The interval component of the product is the full zero-column
		// sieve, contributing true to product_relation::to_formula; the
		// product evaluates exactly as its sparse table.
		if state.table == nil {
			return ErrSparseIntervalDatalogFragment
		}
		if err := e.selectFiniteRepresentation(declaration, "tr_sparse"); err != nil {
			return err
		}
		state.explicitFiniteRepresentation = true
		state.requestedRepresentations = []string{"tr_sparse"}
		e.revision++
		return nil
	}
	state.requestedRepresentations = append([]string(nil), kinds...)
	state.explicitFiniteRepresentation = false
	e.revision++
	return nil
}

// A requested interval kind is recorded independently of the engine favourite.
// Its signature preflight is allocation-free, so a failed request leaves the
// previously selected predicate kind intact.
func (e *PropositionalEngine) selectIntervalRepresentation(declaration ast.ASTHandle, kinds []string) error {
	state := e.predicates[declaration]
	if state == nil || state.external != nil || !rel.CanHandleIntervalSorts(e.manager, state.sorts) {
		return ErrIntervalDatalogFragment
	}
	state.requestedRepresentations = append([]string(nil), kinds...)
	state.explicitFiniteRepresentation = false
	e.revision++
	return nil
}
