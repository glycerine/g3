package rel

// Source: src/muz/rel/dl_relation_manager.cpp
// relation_manager::display_output_tables.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
)

// DisplayOutputTables writes each requested output relation in rule-set order.
// Missing relations still receive the source tuple heading.
func (m *RelationManager) DisplayOutputTables(context *base.DatalogContext, rules *base.RuleSet, out io.Writer) error {
	if err := m.open(); err != nil {
		return err
	}
	if context == nil || rules == nil || out == nil {
		return ErrTableRelation
	}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	if manager != m.manager {
		return ErrTableRelation
	}
	outputs, err := rules.OutputPredicates()
	if err != nil {
		return err
	}
	for _, predicate := range outputs {
		if product := m.explanations[predicate]; product != nil {
			if err = product.DisplayTuples(context, predicate, out); err != nil {
				return err
			}
			continue
		}
		table := m.relations[predicate]
		if table == nil {
			name, getErr := manager.GetASTName(predicate)
			if getErr != nil {
				return getErr
			}
			nameText, getErr := manager.Symbols.Display(name)
			if getErr != nil {
				return getErr
			}
			if _, getErr = fmt.Fprintf(out, "Tuples in %s: \n", nameText); getErr != nil {
				return getErr
			}
			continue
		}
		sorts, _, getErr := m.PredicateSignature(predicate)
		if getErr != nil {
			return getErr
		}
		copy, getErr := table.Clone()
		if getErr != nil {
			return getErr
		}
		relation, getErr := AdoptTableRelation(m, sorts, copy)
		if getErr == nil {
			getErr = relation.DisplayTuples(context, predicate, out)
		}
		getErr = errors.Join(getErr, relation.Close(), copy.Close())
		if getErr != nil {
			return getErr
		}
	}
	return nil
}
