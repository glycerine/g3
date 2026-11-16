package fp

// Source: src/muz/rel/rel_context.cpp rel_context::rel_context's relation
// plugin registration (rm.register_plugin(alloc(udoc_plugin, rm)), ...) and
// relation_manager::register_relation_plugin_impl's favourite selection by
// context::default_relation(). The favourite relation plugin's relations
// are created by relation_manager::mk_empty_relation and run through the
// compiled instructions like table relations.
//
// Go creates every predicate's table at registration; once the relation
// favourite is in effect, the predicates' eagerly made tables are replaced
// by the favourite plugin's relations (keeping their facts), as
// applyDatalogDefaultFiniteTable does for the favourite table plugin.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// datalogRelationPlugins registers the relation plugins once
// (rel_context::rel_context) and records the default relation.
func (e *PropositionalEngine) datalogRelationPlugins(checkedDoc bool) error {
	if e.relations.GetRelationPlugin("doc") == nil {
		plugin, err := rel.NewUDocPlugin(e.relations)
		if err != nil {
			return err
		}
		if err = e.relations.RegisterRelationPlugin(plugin); err != nil {
			return errors.Join(err, plugin.Close())
		}
		check, err := rel.NewCheckRelationPlugin(e.relations)
		if err != nil {
			return err
		}
		if err = e.relations.RegisterRelationPlugin(check); err != nil {
			return err
		}
	}
	if checkedDoc {
		check := e.relations.GetRelationPlugin("check_relation").(*rel.CheckRelationPlugin)
		check.SetPlugin(e.relations.GetRelationPlugin("doc"))
		return e.relations.SetDefaultRelation("check_relation")
	}
	return e.relations.SetDefaultRelation(e.datalogRelationFavourite)
}

// selectCompiledRelationFavourite reports whether the query runs on the
// favourite relation plugin's relations through the compiled pipeline. It
// holds when the favourite is a ported relation plugin that handles every
// predicate's signature and no checker, explanation or explicit
// representation applies; those configurations keep their existing
// evaluators until their plugins are ported.
func (e *PropositionalEngine) selectCompiledRelationFavourite() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	checkName, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	// rel_context::updt_params: a check_relation names the checked plugin
	// and check_relation becomes the default relation.
	checkedDoc := checkEnabled && checkName == "doc"
	if checkEnabled && !checkedDoc || !checkedDoc && e.datalogRelationFavourite != "doc" || e.udocDatalog != nil {
		return false, nil
	}
	if fav := e.relations.FavouriteRelationPlugin(); fav != nil {
		// Already selected by an earlier query.
		return true, nil
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	if e.params != nil && e.globals != nil {
		// mk_bit_blast's Bool relations are table relations next to the
		// favourite's; the mixed operations are not ported.
		view, err := e.fixedpointParamView()
		if err != nil {
			return false, err
		}
		bitBlast, err := view.Bool("xform.bit_blast", false)
		if err = errors.Join(err, view.Close()); err != nil || bitBlast {
			return false, err
		}
	}
	probe, err := rel.NewUDocPlugin(e.relations)
	if err != nil {
		return false, err
	}
	defer probe.Close()
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state == nil || state.table == nil || state.external != nil || state.explicitFiniteRepresentation || len(state.requestedRepresentations) != 0 || state.checked != nil || state.sparse != nil {
			return false, nil
		}
		ok, err := probe.CanHandleSignature(state.sorts)
		if err != nil || !ok {
			return false, err
		}
		if _, err = state.table.Signature(); err != nil {
			return false, nil
		}
	}
	// Every rule variable's column must be the favourite's too: an unbound
	// variable of another sort makes a total relation of that sort (a
	// table or another plugin's relation next to the favourite's).
	count, err := e.set.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return false, err
		}
		n, err := rule.TailSize()
		if err != nil {
			return false, err
		}
		roots := make([]ast.ASTHandle, 0, n+1)
		head, err := rule.Head()
		if err != nil {
			return false, err
		}
		roots = append(roots, head)
		for j := 0; j < n; j++ {
			tail, _, err := rule.Tail(j)
			if err != nil {
				return false, err
			}
			roots = append(roots, tail)
		}
		for _, root := range roots {
			sorts, err := e.expressionVariableSorts(root)
			if err != nil {
				return false, err
			}
			for _, sort := range sorts {
				ok, err := probe.CanHandleSignature([]ast.ASTHandle{sort})
				if err != nil || !ok {
					return false, err
				}
			}
		}
	}
	if err = e.datalogRelationPlugins(checkedDoc); err != nil {
		return false, err
	}
	for _, d := range e.predicateOrder {
		if err = e.migrateToFavouriteRelation(d); err != nil {
			return false, err
		}
	}
	return true, nil
}

// migrateToFavouriteRelation replaces predicate d's eagerly created table
// by mk_empty_relation(sig, pred) of the favourite plugin, keeping its
// facts.
func (e *PropositionalEngine) migrateToFavouriteRelation(d ast.ASTHandle) error {
	state := e.predicates[d]
	if _, ok := rel.AsRelation(state.table); ok {
		return nil
	}
	replacement, err := e.relations.MkEmptyRelation(state.sorts, d)
	if err != nil {
		return err
	}
	if _, ok := rel.AsRelation(replacement); !ok {
		return errors.Join(ErrPropositionalEngine, replacement.Close())
	}
	facts, err := state.table.Facts()
	if err != nil {
		return errors.Join(err, replacement.Close())
	}
	for _, fact := range facts {
		if _, err = replacement.Add(fact); err != nil {
			return errors.Join(err, replacement.Close())
		}
	}
	return e.replaceDatalogTable(d, replacement)
}
