package fp

// Sources: src/muz/pdr/pdr_dl_interface.cpp and
// src/muz/spacer/spacer_dl_interface.cpp get_model.

import "github.com/glycerine/g3/pkg/model"

// ExpressionModel returns a caller-owned model for the latest expression query.
// The backend preserves its ordinary result-status restrictions.
func (e *PropositionalEngine) ExpressionModel() (*model.Model, error) {
	clone, kind, err := e.expressionQueryState()
	if err != nil {
		return nil, err
	}
	// The ddnf clone runs its compiler directly and records no result
	// snapshot; the parent holds it (and ddnf keeps engine_base::get_model).
	if kind == EngineDDNF {
		return e.Model(e.queryClonePredicate)
	}
	return clone.Model(e.queryClonePredicate)
}
