package fp

import (
	"errors"
)

// ErrKarrDatalogFragment reports a karr_relation configuration whose
// relation plugins are not ported.
var ErrKarrDatalogFragment = errors.New("muz/fp: unsupported Karr Datalog fragment")

func (e *PropositionalEngine) ensureKarrDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	_, checkOn, checkErr := e.datalogCheckRelationName()
	if checkErr != nil {
		return false, checkErr
	}
	// check_relation over karr_relation runs on the relation plugins.
	selected := e.datalogRelationFavourite == "karr_relation" && !checkOn
	for _, state := range e.predicates {
		for _, name := range state.requestedRepresentations {
			selected = selected || name == "karr_relation"
		}
	}
	if !selected {
		return false, nil
	}
	// rel_context registers karr_relation_plugin only when xform.karr is set.
	// Without it the favourite names no plugin and the checker cannot be
	// constructed, so evaluation continues with the ordinary plugins.
	enabled, err := e.datalogKarrEnabled()
	if err != nil {
		return true, err
	}
	if !enabled {
		return false, nil
	}
	if e.udocDatalog != nil || e.externalIntervalDatalog != nil {
		return true, ErrKarrDatalogFragment
	}
	for declaration, state := range e.predicates {
		if e.datalogRelationFavourite != "karr_relation" && !e.auxiliary[declaration] && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "karr_relation") {
			return true, ErrKarrDatalogFragment
		}
		if state.external != nil || state.explicitFiniteRepresentation {
			return true, ErrKarrDatalogFragment
		}
		if len(state.requestedRepresentations) > 0 && (len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != "karr_relation") {
			return true, ErrKarrDatalogFragment
		}
		if state.table != nil {
			n, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if n != 0 {
				return true, ErrKarrDatalogFragment
			}
		}
	}
	if on, err := e.datalogExplanationsEnabled(); err != nil || on {
		return true, errors.Join(err, ErrKarrDatalogFragment)
	}
	if checkOn {
		return true, ErrKarrDatalogFragment
	}
	if _, on, err := e.datalogDefaultTableCheck(); err != nil || on {
		return true, errors.Join(err, ErrKarrDatalogFragment)
	}
	// karr_relation predicates run on the relation plugins
	// (arithRelationsRoute); what reaches here is a configuration whose
	// plugins are not ported yet.
	return true, ErrKarrDatalogFragment
}

// datalogKarrEnabled reads context::karr() (fixedpoint.xform.karr).
func (e *PropositionalEngine) datalogKarrEnabled() (bool, error) {
	if e.params == nil || e.globals == nil {
		return false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return false, err
	}
	enabled, err := view.Bool("xform.karr", false)
	return enabled, errors.Join(err, view.Close())
}
