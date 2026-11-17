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
	return source.CloneFinite()
}
