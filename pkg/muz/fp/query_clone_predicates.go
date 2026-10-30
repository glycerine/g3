package fp

// Source: local rule-set query copies preserve registered predicates.

func (e *PropositionalEngine) copyQueryPredicates(clone *PropositionalEngine) error {
	for _, declaration := range e.predicateOrder {
		if err := clone.RegisterPredicate(declaration); err != nil {
			return err
		}
		clone.auxiliary[declaration] = e.auxiliary[declaration]
	}
	return nil
}
