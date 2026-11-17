package fp

// A query with a checked scope (datalog.check_relation over a check table
// or a lazy table, formula-checked relations) keeps its interpreted
// evaluator.
func (e *PropositionalEngine) saturateDatalogQuery(check *datalogCheckRelationScope, round *datalogRestartRound) error {
	if check != nil {
		e.compiled = false
		return e.saturate()
	}
	var err error
	e.compiled, err = e.tryCompiledSaturation(round)
	if err == nil && !e.compiled {
		err = e.saturate()
	}
	return err
}
