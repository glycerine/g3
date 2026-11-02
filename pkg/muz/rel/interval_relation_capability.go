package rel

// Source: src/muz/rel/dl_interval_relation.cpp
// interval_relation_plugin::can_handle_signature and filter_identical_fn.

import "github.com/glycerine/g3/pkg/ast"

// CanHandleIntervalSorts reports whether every sort is an integer or real
// arithmetic sort, matching interval_relation_plugin::can_handle_signature.
func CanHandleIntervalSorts(manager *ast.ASTManager, sorts []ast.ASTHandle) bool {
	if manager == nil {
		return false
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return false
	}
	for _, sort := range sorts {
		kind, err := manager.GetKind(sort)
		if err != nil || kind != ast.ASTSort {
			return false
		}
		actualFamily, err := manager.GetFamilyID(sort)
		if err != nil || actualFamily != family {
			return false
		}
		declKind, err := manager.GetDeclKind(sort)
		if err != nil || declKind < 0 || declKind > 1 {
			return false
		}
	}
	return true
}

// FilterIdenticalColumns intersects every listed column with the first one.
// Zero and one-column filters are source no-ops.
func (r *IntervalRelation) FilterIdenticalColumns(columns []uint32) error {
	if r == nil || r.values == nil {
		return ast.ErrASTDomain
	}
	if len(columns) < 2 {
		return nil
	}
	for _, column := range columns {
		if uint64(column) >= uint64(len(r.values.columns)) {
			return ast.ErrASTDomain
		}
	}
	for _, column := range columns[1:] {
		if err := r.values.equate(columns[0], column); err != nil {
			return err
		}
	}
	return nil
}
