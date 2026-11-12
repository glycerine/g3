package rel

// Source: src/muz/rel/dl_relation_manager.cpp
// default_table_filter_not_equal_fn (1337-1383) and
// src/ast/dl_decl_plugin.cpp dl_decl_util::is_numeral_ext (696-716).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// tableNotEqualFilter is default_table_filter_not_equal_fn: a fact is
// removed exactly when its column holds value.
type tableNotEqualFilter struct {
	variable uint32
	value    Element
}

// notEqualFilter is default_table_filter_not_equal_fn::mk: it recognizes
// (not (= x c)) for a variable x and an is_numeral_ext constant c.
func (m *RelationManager) notEqualFilter(condition ast.ASTHandle) (tableNotEqualFilter, bool, error) {
	manager := m.manager
	negated, equality, err := manager.IsNot(condition)
	if err != nil || !negated {
		return tableNotEqualFilter{}, false, err
	}
	family, op, x, y, binary, err := intervalParts(manager, equality)
	if err != nil || !binary || family != manager.BasicFamilyID() || op != ast.ASTOpEq {
		return tableNotEqualFilter{}, false, err
	}
	kind, err := manager.GetKind(x)
	if err != nil {
		return tableNotEqualFilter{}, false, err
	}
	if kind != ast.ASTVar {
		x, y = y, x
		if kind, err = manager.GetKind(x); err != nil {
			return tableNotEqualFilter{}, false, err
		}
		if kind != ast.ASTVar {
			return tableNotEqualFilter{}, false, nil
		}
	}
	value, numeral, err := m.numeralExt(y)
	if err != nil || !numeral {
		return tableNotEqualFilter{}, false, err
	}
	variable, err := manager.GetVarIndex(x)
	if err != nil {
		return tableNotEqualFilter{}, false, err
	}
	return tableNotEqualFilter{variable: variable, value: Element(value)}, true, nil
}

// numeralExt is dl_decl_util::is_numeral_ext(e, v).
func (m *RelationManager) numeralExt(expression ast.ASTHandle) (value uint64, matched bool, resultErr error) {
	manager := m.manager
	value, matched, err := manager.DatalogNumeralValue(expression)
	if err != nil || matched {
		return value, matched, err
	}
	service := manager.NumeralService()
	numeral, err := service.NewValue()
	if err != nil {
		return 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&numeral)) }()
	var width uint32
	matched, err = manager.BitVectorNumeralValue(expression, &numeral, &width)
	if err != nil {
		return 0, false, err
	}
	if matched && width < 64 {
		value, err = service.Uint64(&numeral)
		return value, err == nil, err
	}
	if manager.IsTrue(expression) {
		return 1, true, nil
	}
	if manager.IsFalse(expression) {
		return 0, true, nil
	}
	return 0, false, nil
}

// keeps is !should_remove for a fact whose condition variables are mapped to
// columns.
func (f tableNotEqualFilter) keeps(columns map[uint32]int, fact Fact) (bool, error) {
	column, found := columns[f.variable]
	if !found || column < 0 || column >= len(fact) {
		return false, ErrTable
	}
	return fact[column] != f.value, nil
}
