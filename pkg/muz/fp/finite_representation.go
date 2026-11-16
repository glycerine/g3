package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Source sets a requested plugin before creating the relation. Go creates an
// empty relation eagerly and historically accepts prequery changes after fact
// insertion, so preserve those tuples during this representation migration.
func (e *PropositionalEngine) selectFiniteRepresentation(declaration ast.ASTHandle, kind string) error {
	state := e.predicates[declaration]
	if state == nil || state.table == nil {
		return ErrPropositionalHorn
	}
	if kind == "tr_sparse" || kind == "tr_hashtable" || kind == "tr_bitvector" || kind == "tr_lazy_sparse" {
		if err := e.relations.SetRequestedFiniteKind(declaration, kind); err != nil {
			return err
		}
	}
	if kind == "tr_lazy_sparse" {
		if e.lazyFinite != nil && e.lazyFinite.Owns(state.table) {
			return nil
		}
		if e.lazyFinite == nil {
			e.lazyFinite = rel.NewLazySparseFiniteFactory()
		}
		replacement, err := e.lazyFinite.Migrate(state.table)
		if err != nil {
			return err
		}
		return e.replaceDatalogTable(declaration, replacement)
	}
	switch kind {
	case "tr_sparse":
		if _, ok := state.table.(*rel.SparseTable); ok {
			return nil
		}
	case "tr_hashtable":
		if _, ok := state.table.(*rel.HashTable); ok {
			return nil
		}
	case "tr_bitvector":
		if _, ok := state.table.(*rel.BitvectorTable); ok {
			return nil
		}
	default:
		return ErrPropositionalHorn
	}
	signature, err := state.table.Signature()
	if err != nil {
		return err
	}
	var replacement rel.FiniteTable
	switch {
	case kind == "tr_sparse":
		replacement, err = rel.NewSparseTable(signature, 0)
	case kind == "tr_bitvector" && rel.BitvectorCanHandleSignature(signature, 0):
		replacement, err = rel.NewBitvectorTable(signature)
	case kind == "tr_bitvector":
		// mk_empty_relation(s, kind): the bitvector plugin rejects the
		// signature, so the manager's appropriate plugin is used.
		replacement, err = e.relations.NewDefaultFiniteTable(signature)
	default:
		replacement = rel.NewHashTable(signature)
	}
	if err != nil {
		return err
	}
	if r, ok := rel.AsRelation(state.table); ok {
		// The favourite relation plugin's empty relation of a predicate
		// registered after the plugins were selected stands for one
		// get_relation has not created yet: set_predicate_kind decides
		// the kind it is created with.
		empty, err := r.Empty()
		if err != nil || !empty {
			return errors.Join(err, rel.ErrRelationNotTable, replacement.Close())
		}
		return e.replaceDatalogTable(declaration, replacement)
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
	return e.replaceDatalogTable(declaration, replacement)
}
