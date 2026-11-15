package tactic

// Source: src/tactic/tactical.cpp:462-884; src/tactic/tactical.h:55-61.

// NewParTactic selects the source _NO_OMP_ implementation for this
// single-goroutine port. It has the same ordered fallback behavior as or_else.
func NewParTactic(children ...Tactic) (Tactic, error) {
	return NewOrElseTactic(children...)
}

// NewParAndThenTactic runs the subgoals in order (the source's sequential
// composition) for this single-goroutine port; the second tactic's resource
// steps on several subgoals follow the reference build (see andThenTactic).
func NewParAndThenTactic(children ...Tactic) (Tactic, error) {
	result, err := NewAndThenTactic(children...)
	if err != nil {
		return nil, err
	}
	for t, ok := result.(*andThenTactic); ok; t, ok = t.second.(*andThenTactic) {
		t.parallel = true
		if t.second == children[len(children)-1] {
			break
		}
	}
	return result, nil
}
