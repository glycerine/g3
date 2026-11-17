package fp

// Source: src/muz/rel/rel_context.cpp checked favourite-plugin query scope.

import "fmt"

func (e *PropositionalEngine) datalogDefaultCheckedPrimary() (primary string, resultErr error) {
	if e.datalogDefaultTable == "" {
		if err := e.applyDatalogDefaultFiniteTable(); err != nil {
			return "", err
		}
	}
	primary = e.datalogDefaultTable
	switch primary {
	case "tr_hashtable":
		primary = "hashtable"
	case "tr_sparse":
		primary = "sparse"
	case "tr_bitvector":
		primary = "bitvector"
	case "tr_lazy_sparse":
		primary = "lazy_sparse"
	}
	if primary != "hashtable" && primary != "sparse" && primary != "bitvector" && primary != "lazy_sparse" {
		return "", fmt.Errorf("muz/fp: checked default table %s is not available", primary)
	}
	return primary, nil
}
