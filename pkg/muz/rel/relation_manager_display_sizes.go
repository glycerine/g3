package rel

// Source: src/muz/rel/dl_relation_manager.cpp
// relation_manager::display_relation_sizes.

import (
	"fmt"
	"io"
)

// WriteRelationSizes writes "Relation <name> has size <rows>" for each
// relation in m_relations' table order, where rows is
// get_size_estimate_rows() (product relations keep relation_base's
// UINT_MAX).
func (m *RelationManager) WriteRelationSizes(out io.Writer) error {
	if err := m.open(); err != nil {
		return err
	}
	for _, entry := range m.layout.Entries() {
		predicate := entry.Key
		name, err := m.manager.GetASTName(predicate)
		if err != nil {
			return err
		}
		text, err := m.manager.Symbols.Display(name)
		if err != nil {
			return err
		}
		var rows uint32
		if product, found := m.explanations[predicate]; found && product != nil {
			if rows, err = product.SizeEstimateRows(); err != nil {
				return err
			}
		} else if product, found := m.relationLevel[predicate]; found && product != nil {
			rows = ^uint32(0)
		} else if table, found := m.finite[predicate]; found && table != nil {
			if rows, _, err = finiteTableSizeEstimates(table); err != nil {
				return err
			}
		} else {
			continue
		}
		if _, err = fmt.Fprintf(out, "Relation %s has size %d\n", text, rows); err != nil {
			return err
		}
	}
	return nil
}
