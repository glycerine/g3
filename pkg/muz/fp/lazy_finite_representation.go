package fp

import "github.com/glycerine/g3/pkg/muz/rel"

// Lazy clones normally share their deferred engine. Context copies instead
// migrate rows into their own engine so either context may be closed first.
func (e *PropositionalEngine) cloneFiniteForContext(source rel.FiniteTable) (rel.FiniteTable, error) {
	if external, ok := source.(*rel.ExternalRelation); ok && e.externalPluginRegistered() {
		// A query clone and its context share the external relations.
		return e.external.ShareRelation(external)
	}
	if _, lazy := source.(*rel.LazySparseTable); lazy {
		if e.lazyFinite == nil {
			e.lazyFinite = e.relations.NewLazySparseFiniteFactory()
		}
		return e.lazyFinite.Migrate(source)
	}
	if sparse, ok := source.(*rel.SparseTable); ok {
		// The source's one table: the copy belongs to this context's
		// sparse_table_plugin and keeps its capacities.
		return e.relations.TwinSparseTable(sparse)
	}
	if pair, ok := source.(*rel.FiniteCheckedPair); ok {
		return rel.CloneCheckTableForContext(pair, e.relations, e.cloneFiniteForContext)
	}
	if r, ok := rel.AsRelation(source); ok && e.relations != nil && r.RelationPlugin().Manager() != e.relations {
		// The source has one relation manager (rel_context's): a udoc
		// relation of the owner's plugin is copied into this context's
		// udoc plugin (rel_context::rel_context registers it), so the
		// compiled code's operations see one plugin per kind.
		if err := e.registerRelContextPlugins(); err != nil {
			return nil, err
		}
		if plugin, ok := e.relations.GetRelationPlugin(r.RelationPlugin().Name()).(*rel.UDocPlugin); ok {
			if u, ok := r.(*rel.UDocPluginRelation); ok {
				return u.CopyIntoPlugin(plugin)
			}
		}
	}
	return source.CloneFinite()
}
