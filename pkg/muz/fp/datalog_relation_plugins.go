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

// registerRelContextPlugins is rel_context::rel_context's relation plugin
// registration, once per relation manager, in its order: bound_relation,
// interval_relation (karr_relation when context::karr()), udoc, then
// check_relation.
func (e *PropositionalEngine) registerRelContextPlugins() error {
	if e.relations.GetRelationPlugin("bound_relation") != nil {
		return nil
	}
	if err := e.relations.RegisterRelationPlugin(rel.NewBoundRelationPluginRel(e.relations)); err != nil {
		return err
	}
	if err := e.relations.RegisterRelationPlugin(rel.NewIntervalRelationPlugin(e.relations)); err != nil {
		return err
	}
	if karr, err := e.datalogKarrEnabled(); err != nil {
		return err
	} else if karr {
		manager := e.manager
		checkpoint := func() bool {
			canceled, err := manager.Canceled()
			return err == nil && !canceled
		}
		if err = e.relations.RegisterRelationPlugin(rel.NewKarrRelationPlugin(e.relations, checkpoint)); err != nil {
			return err
		}
	}
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
	return e.relations.RegisterRelationPlugin(check)
}

// datalogRelationPlugins registers the relation plugins once
// (rel_context::rel_context) and records the default relation.
func (e *PropositionalEngine) datalogRelationPlugins(checkedDoc bool) error {
	if err := e.registerRelContextPlugins(); err != nil {
		return err
	}
	if checkedDoc {
		check := e.relations.GetRelationPlugin("check_relation").(*rel.CheckRelationPlugin)
		check.SetPlugin(e.relations.GetRelationPlugin("doc"))
		return e.relations.SetDefaultRelation("check_relation")
	}
	return e.relations.SetDefaultRelation(e.datalogRelationFavourite)
}

// datalogRelationKind classifies a predicate's relation the way
// relation_manager::mk_empty_relation chooses it: "doc" for udoc_plugin
// (requested, or the favourite handling the signature), "table" for a
// table_relation, "" when neither applies here (product relations,
// other plugins, external relations).
func (e *PropositionalEngine) datalogRelationKind(probe *rel.UDocPlugin, state *predicateState, docFavourite bool) (string, error) {
	if state == nil || state.external != nil || state.checked != nil || state.sparse != nil {
		return "", nil
	}
	if state.table == nil {
		// get_relation: an Int/Real signature no table handles gets the
		// first relation plugin that does (bound_relation, interval), or
		// the empty product relation.
		if len(state.requestedRepresentations) == 0 && rel.CanHandleIntervalSorts(e.manager, state.sorts) {
			return "plugin", nil
		}
		return "", nil
	}
	udoc, err := probe.CanHandleSignature(state.sorts)
	if err != nil {
		return "", err
	}
	_, tableErr := state.table.Signature()
	table := tableErr == nil
	switch {
	case len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc":
		if udoc {
			return "doc", nil
		}
	case state.explicitFiniteRepresentation && len(state.requestedRepresentations) == 1:
		// A requested table plugin (tr_*) that rejects the signature
		// leaves the choice to the favourite.
		if table && e.relations.RequestedFiniteKindHandles(state.sorts, state.requestedRepresentations[0]) {
			return "table", nil
		}
	case len(state.requestedRepresentations) != 0:
		return "", nil
	}
	if docFavourite && udoc {
		return "doc", nil
	}
	if table {
		return "table", nil
	}
	return "", nil
}

// selectCompiledRelationFavourite reports whether the query runs on the
// relation plugins' relations through the compiled pipeline. Every
// predicate's relation must be udoc's or a table's, every rule must use
// relations of one kind (relation_manager joins a table_relation with a
// udoc relation only through product relations), and a doc rule's
// variables must be udoc columns. Checkers other than check_relation(doc),
// explanations and default_table_checked keep their evaluators.
func (e *PropositionalEngine) selectCompiledRelationFavourite() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if e.compiledRelationPlugins {
		// Already selected by an earlier query.
		return true, nil
	}
	if e.udocDatalog != nil {
		return false, nil
	}
	checkName, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	// rel_context::updt_params: a check_relation names the checked plugin
	// and check_relation becomes the default relation.
	checkedDoc := checkEnabled && checkName == "doc"
	if checkEnabled && !checkedDoc {
		return false, nil
	}
	docFavourite := checkedDoc || e.datalogRelationFavourite == "doc"
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	probe, err := rel.NewUDocPlugin(e.relations)
	if err != nil {
		return false, err
	}
	defer probe.Close()
	kinds := make(map[ast.ASTHandle]string, len(e.predicateOrder))
	anyDoc := false
	for _, d := range e.predicateOrder {
		kind, err := e.datalogRelationKind(probe, e.predicates[d], docFavourite)
		if err != nil || kind == "" {
			return false, err
		}
		kinds[d] = kind
		anyDoc = anyDoc || kind == "doc"
	}
	if !anyDoc {
		return false, nil
	}
	count, err := e.set.Len()
	if err != nil {
		return false, err
	}
	// A rule joining a table_relation with a udoc relation needs
	// finite_product/sieve relations, not ported yet: such programs keep
	// the previous evaluator.
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return false, err
		}
		n, err := rule.TailSize()
		if err != nil {
			return false, err
		}
		head, err := rule.Head()
		if err != nil {
			return false, err
		}
		headDecl, err := rule.HeadDeclaration()
		if err != nil {
			return false, err
		}
		kind := kinds[headDecl]
		roots := []ast.ASTHandle{head}
		uninterpreted, err := rule.UninterpretedTailSize()
		if err != nil {
			return false, err
		}
		for j := 0; j < n; j++ {
			tail, _, err := rule.Tail(j)
			if err != nil {
				return false, err
			}
			roots = append(roots, tail)
			if j < uninterpreted {
				decl, err := rule.TailDeclaration(j)
				if err != nil {
					return false, err
				}
				if kinds[decl] != kind {
					return false, nil
				}
			}
		}
		if kind != "doc" {
			continue
		}
		// An unbound variable of another sort makes a total relation of
		// that sort next to the udoc relations.
		for _, root := range roots {
			sorts, err := e.expressionVariableSorts(root)
			if err != nil {
				return false, err
			}
			for _, sort := range sorts {
				ok, err := probe.CanHandleSignature([]ast.ASTHandle{sort})
				if err != nil {
					return false, err
				}
				// A total over an Int/Real sort is a plugin relation
				// (mk_full_relation); the empty-signature join and the
				// product fallbacks combine it with the udoc relations.
				if _, table := e.relations.SortDomain(sort); !ok && !table && !rel.CanHandleIntervalSorts(e.manager, []ast.ASTHandle{sort}) {
					return false, nil
				}
			}
		}
	}
	if err = e.datalogRelationPlugins(checkedDoc); err != nil {
		return false, err
	}
	udoc := e.relations.GetRelationPlugin("doc")
	for _, d := range e.predicateOrder {
		if kinds[d] == "table" && e.predicates[d].explicitFiniteRepresentation {
			if err = e.relations.SetRequestedTableKind(d, e.predicates[d].requestedRepresentations[0]); err != nil {
				return false, err
			}
		}
		if kinds[d] == "plugin" {
			table, err := e.relations.ReserveFiniteRelation(d)
			if err != nil {
				return false, err
			}
			e.predicates[d].table = table
			continue
		}
		if kinds[d] != "doc" {
			continue
		}
		state := e.predicates[d]
		if len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc" {
			// rel_context::set_predicate_representation: set_predicate_kind.
			if err = e.relations.SetRequestedRelationKind(d, udoc); err != nil {
				return false, err
			}
		}
		if err = e.migrateToFavouriteRelation(d); err != nil {
			return false, err
		}
	}
	e.compiledRelationPlugins = true
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
