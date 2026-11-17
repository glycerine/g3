package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (t *SparseTable) SelectEqualAndProject(value Element, column int) (*SparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= t.signature.Len() {
		return nil, ErrTable
	}
	firstFunctional := t.signature.Len() - t.functional
	if t.signature.Len() > 1 && column < firstFunctional {
		plan, err := NewSparseIndexedSelectPlan(t, value, column)
		if err != nil {
			return nil, err
		}
		action, err := base.StartVerboseAction("select_equal_and_project", base.VerboseActionDefaultLevel)
		if err != nil {
			return nil, err
		}
		result, _, err := plan.Execute(t)
		return result, errors.Join(err, action.End())
	}
	filtered, err := t.Clone()
	if err != nil {
		return nil, err
	}
	defer filtered.Close()
	if err := filtered.FilterEqual(value, column); err != nil {
		return nil, err
	}
	return filtered.Project([]int{column})
}
func (t *SparseTable) FilterNegation(negative *SparseTable, tCols, nCols []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if err := validateColumns(t.signature, negative.signature, tCols, nCols); err != nil {
		return err
	}
	if sparseJoinInvolvesFunctional(t, negative, tCols, nCols) {
		// The sparse plugin refuses a join on functional columns, so the
		// relation manager falls back to default_table_negation_filter_fn
		// (contains_fact of the bound projection when every negated column
		// is bound once).
		return defaultTableNegationFilter(t, negative, negative.signature.Len(), tCols, nCols)
	}
	plan, err := NewSparseIndexedNegationPlan(t, negative, tCols, nCols)
	if err != nil {
		return err
	}
	// filter_by_negation_fn::operator()'s verbose_action.
	action, err := base.StartVerboseAction("filter_by_negation", base.VerboseActionDefaultLevel)
	if err != nil {
		return err
	}
	_, err = plan.Execute(t, negative)
	return errors.Join(err, action.End())
}
