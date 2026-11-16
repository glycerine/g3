package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source chooses favourite plugins when constructing rel_context. Go eagerly
// creates tables at registration, so migrate those non-explicit owners once
// the configured Datalog engine first consumes facts or starts a query.
func (e *PropositionalEngine) applyDatalogDefaultFiniteTable() (resultErr error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return err
	}
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
		// relation_manager::register_plugin makes the favourite the table
		// plugin whose name equals default_table exactly (sparse, hashtable,
		// bitvector, lazy_sparse). Any other name, including the tr_ relation
		// spellings, sets no favourite, so try_get_appropriate_plugin takes the
		// first registered plugin that handles the signature: sparse
		// (registration order sparse, hashtable, bitvector, lazy_sparse).
		switch name {
		case "sparse", "hashtable", "bitvector", "lazy_sparse":
		default:
			name = "sparse"
		}
		e.datalogDefaultTable = name
	}
	if err := e.configureDatalogFiniteDefault(name); err != nil {
		return err
	}
	favouriteChanged := e.appliedDefaultTable != name
	e.appliedDefaultTable = name
	for _, decl := range e.predicateOrder {
		state := e.predicates[decl]
		if state == nil || state.table == nil || state.external != nil || state.explicitFiniteRepresentation {
			continue
		}
		if _, ok := rel.AsRelation(state.table); ok {
			// The favourite relation plugin's relation.
			continue
		}
		if !favouriteChanged && !e.relations.IsPlaceholderRelation(decl) {
			// relation_manager::get_relation created this relation with
			// the plugin chosen then (e.g. a union into an empty register
			// stores mk_empty of its source's plugin); the favourite only
			// selects the plugin of relations created later. A new
			// favourite (a recreated rel_context) applies to all.
			continue
		}
		kind := "tr_" + name
		if len(state.sorts) == 0 && name != "bitvector" {
			// Sparse and lazy_sparse reject the empty signature and
			// registration order reaches hashtable; bitvector handles it.
			kind = "tr_hashtable"
		}
		if err := e.selectFiniteRepresentation(decl, kind); err != nil {
			return err
		}
	}
	return nil
}

// Each context supplies its own lazy engine to its favourite plugin so
// predicates and compiler temporaries share that context's deferred owners.
func (e *PropositionalEngine) configureDatalogFiniteDefault(name string) error {
	if name == "lazy_sparse" {
		if e.lazyFinite == nil {
			e.lazyFinite = rel.NewLazySparseFiniteFactory()
		}
		if err := e.relations.SetLazySparseFiniteFactory(e.lazyFinite); err != nil {
			return err
		}
	}
	return e.relations.SetDefaultFiniteTable(name)
}
