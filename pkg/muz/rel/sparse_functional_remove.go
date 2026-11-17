package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (table *SparseTable) RemoveFacts(facts []Fact) (int, error) {
	if err := table.open(); err != nil {
		return 0, err
	}
	removed := 0
	for _, fact := range facts {
		// table_base::remove_facts calls sparse_table::remove_fact per row,
		// whose verbose_action _va("remove_fact", 2) wraps the removal.
		action, err := base.StartVerboseAction("remove_fact", 2)
		if err != nil {
			return removed, err
		}
		var changed bool
		changed, err = table.Remove(fact)
		if err = errors.Join(err, action.End()); err != nil {
			return removed, err
		}
		if changed {
			removed++
		}
	}
	return removed, nil
}
