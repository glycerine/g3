package fp

// Source: local rule-set query copies preserve registered predicates.

func (e *PropositionalEngine) copyQueryPredicates(clone *PropositionalEngine) error {
	for _, declaration := range e.predicateOrder {
		if err := clone.RegisterPredicate(declaration); err != nil {
			return err
		}
		clone.predicates[declaration].explicitFiniteRepresentation = e.predicates[declaration].explicitFiniteRepresentation
		if e.backend == "datalog" && e.predicates[declaration].table != nil {
			table, err := e.predicates[declaration].table.CloneFinite()
			if err != nil {
				return err
			}
			if err = clone.installDatalogTable(declaration, table); err != nil {
				return err
			}
		}
		if e.auxiliary[declaration] {
			clone.auxiliary[declaration] = true
		}
	}
	return nil
}
