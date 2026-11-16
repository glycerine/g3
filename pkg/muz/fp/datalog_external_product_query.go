package fp

// Source: rel_context::set_predicate_representation of an
// external_relation and interval_relation product. g3 does not run
// expression queries (query clones) over such products.

import "errors"

var ErrExternalIntervalDatalogFragment = errors.New("muz/fp: unsupported external/interval product Datalog fragment")

func isExternalIntervalRepresentation(kinds []string) bool {
	return len(kinds) == 2 && kinds[0] != kinds[1] &&
		(kinds[0] == "external_relation" && kinds[1] == "interval_relation" ||
			kinds[0] == "interval_relation" && kinds[1] == "external_relation")
}

func (e *PropositionalEngine) hasExternalIntervalRepresentation() bool {
	for _, state := range e.predicates {
		if state != nil && isExternalIntervalRepresentation(state.requestedRepresentations) {
			return true
		}
	}
	return false
}
