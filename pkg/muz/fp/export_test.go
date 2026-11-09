package fp

// InstallContextModelConverter puts the engine in the state an earlier
// context::query leaves datalog::context in: m_mc is no longer null, so later
// relation queries record and apply their transformations' model converters.
func InstallContextModelConverter(e *PropositionalEngine) {
	e.contextModelConverter = true
}
