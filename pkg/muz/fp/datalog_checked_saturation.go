package fp

// saturateDatalogQuery runs the compiled program, else the interpreted
// evaluator.
func (e *PropositionalEngine) saturateDatalogQuery(round *datalogRestartRound) error {
	var err error
	e.compiled, err = e.tryCompiledSaturation(round)
	if err == nil && !e.compiled {
		err = e.saturate()
	}
	return err
}
