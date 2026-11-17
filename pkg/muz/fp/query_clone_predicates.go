package fp

import "github.com/glycerine/g3/pkg/ast"

// Source: local rule-set query copies preserve registered predicates.

func (e *PropositionalEngine) copyQueryPredicates(clone *PropositionalEngine) error {
	for _, declaration := range e.predicateOrder {
		if err := clone.RegisterPredicate(declaration); err != nil {
			return err
		}
		output, err := e.set.IsOutputPredicate(declaration)
		if err != nil {
			return err
		}
		if output {
			if err = clone.SetOutputPredicate(declaration); err != nil {
				return err
			}
		}
		clone.predicates[declaration].argumentNames = append(clone.predicates[declaration].argumentNames[:0], e.predicates[declaration].argumentNames...)
		clone.predicates[declaration].argumentNamesSet = e.predicates[declaration].argumentNamesSet
		clone.predicates[declaration].requestedRepresentations = append([]string(nil), e.predicates[declaration].requestedRepresentations...)
		clone.predicates[declaration].explicitFiniteRepresentation = e.predicates[declaration].explicitFiniteRepresentation
		// The query runs on the context's relation manager: its requested
		// predicate kinds (set_predicate_kind) are the context's.
		if err = clone.requestTableRelationKind(declaration); err != nil {
			return err
		}
		if e.backend == "datalog" && e.predicates[declaration].table != nil {
			table, err := clone.cloneFiniteForContext(e.predicates[declaration].table)
			if err != nil {
				return err
			}
			install := clone.installDatalogTable
			if e.relations.IsPlaceholderRelation(declaration) {
				install = clone.replaceDatalogTable
			}
			if err = install(declaration, table); err != nil {
				return err
			}
		}
		if e.auxiliary[declaration] {
			clone.auxiliary[declaration] = true
		}
	}
	return nil
}

// requestTableRelationKind is set_predicate_representation's
// set_predicate_kind of a requested table_relation plugin.
func (e *PropositionalEngine) requestTableRelationKind(declaration ast.ASTHandle) error {
	state := e.predicates[declaration]
	if state == nil || len(state.requestedRepresentations) != 1 {
		return nil
	}
	switch kind := state.requestedRepresentations[0]; kind {
	case "tr_hashtable", "tr_sparse", "tr_lazy_sparse", "tr_bitvector":
		return e.relations.SetRequestedTableRelation(declaration, kind[3:])
	}
	return nil
}
