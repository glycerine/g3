package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (table *SparseTable) ContainsExact(fact Fact) (bool, error) {
	return table.Contains(fact)
}

// FetchFact is sparse_table::fetch_fact, timed by its
// verbose_action("fetch_fact", 2): without functional columns
// contains_fact, else the functional columns of the stored row with fact's
// key are copied into fact.
func (table *SparseTable) FetchFact(fact Fact) (_ bool, result error) {
	if err := table.open(); err != nil {
		return false, err
	}
	action, err := base.StartVerboseAction("fetch_fact", 2)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, action.End()) }()
	if table.functional == 0 {
		if err := table.layout.checkFact(table.signature, fact); err != nil {
			return false, err
		}
		return table.ContainsFact(fact)
	}
	return table.fetchFunctional(fact)
}

// fetchFunctional is fetch_fact's lookup of a row by its non-functional
// key, without the verbose action.
func (table *SparseTable) fetchFunctional(fact Fact) (bool, error) {
	if err := table.layout.checkFact(table.signature, fact); err != nil {
		return false, err
	}
	if table.functional == 0 {
		return table.Contains(fact)
	}
	table.writeIntoReserve(fact)
	ofs, found := table.data.findReserveContent()
	if !found {
		return false, nil
	}
	for i := table.firstFunctional(); i < table.signature.Len(); i++ {
		fact[i] = table.getCell(ofs, i)
	}
	return true, nil
}

// SuggestFact is table_base::suggest_fact (sparse_table does not override
// it): contains_fact without functional columns, else fetch_fact, and
// add_new_fact when the row is absent.
func (table *SparseTable) SuggestFact(fact Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	var found bool
	var err error
	if table.functional == 0 {
		found, err = table.ContainsFact(fact)
	} else {
		found, err = table.FetchFact(fact)
	}
	if err != nil || found {
		return false, err
	}
	return table.Add(fact)
}

// EnsureFact is sparse_table::ensure_fact, timed by its
// verbose_action("ensure_fact", 2): the functional columns of an existing
// row are overwritten in place.
func (table *SparseTable) EnsureFact(fact Fact) (result error) {
	if err := table.open(); err != nil {
		return err
	}
	action, err := base.StartVerboseAction("ensure_fact", 2)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, action.End()) }()
	if table.functional == 0 {
		_, err := table.Add(fact)
		return err
	}
	if err := table.layout.checkFact(table.signature, fact); err != nil {
		return err
	}
	table.writeIntoReserve(fact)
	ofs, found := table.data.findReserveContent()
	if !found {
		_, err := table.Add(fact)
		return err
	}
	for i := table.firstFunctional(); i < table.signature.Len(); i++ {
		table.setCell(ofs, i, fact[i])
	}
	return nil
}
