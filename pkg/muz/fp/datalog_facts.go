package fp

// Source: dl_context.cpp add_fact/add_table_fact and rel_context.cpp add_fact.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) selectFactEngine() error {
	if e.engineSelectionSet {
		return nil
	}
	if e.params == nil || e.globals == nil {
		return e.SelectEngine("auto-config")
	}
	return e.SelectConfiguredEngine()
}

func (e *PropositionalEngine) addDatalogFact(declaration ast.ASTHandle, state *predicateState, fact rel.Fact) error {
	if state.table == nil {
		return ErrPropositionalHorn
	}
	if err := e.relations.ResetSaturatedMarks(); err != nil {
		return err
	}
	// rel_context::add_fact: get_relation(pred).add_fact(fact).
	if err := e.relations.MaterializeFiniteRelation(declaration); err != nil {
		return err
	}
	if _, err := state.table.Add(fact); err != nil {
		return err
	}
	e.invalidateQueryForMutation()
	e.advanceRuleRevision()
	e.compiled = false
	return nil
}

// replaceDatalogTable is installDatalogTable for a table the source has not
// created yet: a placeholder stays a placeholder.
func (e *PropositionalEngine) replaceDatalogTable(d ast.ASTHandle, table rel.FiniteTable) error {
	state := e.predicates[d]
	if state == nil {
		return errors.Join(ErrPropositionalEngine, table.Close())
	}
	if err := e.relations.ReplaceFiniteRelation(d, table); err != nil {
		return errors.Join(err, table.Close())
	}
	state.table = table
	return nil
}

// installDatalogTable consumes the incoming clone on every exit. The prior
// borrowed state table is retired only after relation-manager publication.
func (e *PropositionalEngine) installDatalogTable(d ast.ASTHandle, table rel.FiniteTable) error {
	state := e.predicates[d]
	if state == nil {
		return errors.Join(ErrPropositionalEngine, table.Close())
	}
	if err := e.relations.StoreFiniteRelation(d, table); err != nil {
		return errors.Join(err, table.Close())
	}
	state.table = table
	return nil
}
