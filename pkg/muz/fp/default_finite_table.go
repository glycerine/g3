package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source chooses favourite plugins when constructing rel_context. Go eagerly
// creates tables at registration, so migrate those non-explicit owners once
// the configured Datalog engine first consumes facts or starts a query.
func (e *PropositionalEngine) applyDatalogDefaultFiniteTable() (resultErr error) {
	name := e.datalogDefaultTable
	if name == "" {
		name = "sparse"
		if e.params != nil && e.globals != nil {
			view, err := e.fixedpointParamView()
			if err != nil {
				return err
			}
			selected, err := view.Symbol("datalog.default_table", symbol.Symbol{})
			if err == nil && selected.Value != 0 {
				name, _, err = e.manager.Symbols.BareString(selected)
			}
			err = errors.Join(err, view.Close())
			if err != nil {
				return err
			}
		}
		switch name {
		case "sparse", "tr_sparse":
			name = "sparse"
		default:
			name = "hashtable" // No preferred supported plugin: source registration-order fallback.
		}
		e.datalogDefaultTable = name
	}
	if err := e.relations.SetDefaultFiniteTable(name); err != nil {
		return err
	}
	for _, decl := range e.predicateOrder {
		state := e.predicates[decl]
		if state == nil || state.table == nil || state.external != nil || state.explicitFiniteRepresentation {
			continue
		}
		kind := "tr_" + name
		if len(state.sorts) == 0 {
			kind = "tr_hashtable"
		}
		if err := e.selectFiniteRepresentation(decl, kind); err != nil {
			return err
		}
	}
	return nil
}
