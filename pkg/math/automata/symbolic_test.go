package automata

import "testing"

func acceptsBits(a *Automaton, symbols []uint32) bool {
	states := make(map[uint32]bool)
	for _, state := range a.EpsilonClosure(a.initial) {
		states[state] = true
	}
	for _, symbol := range symbols {
		next := make(map[uint32]bool)
		bit := uint32(1) << symbol
		for state := range states {
			for _, move := range a.out[state] {
				if !move.Epsilon && move.Condition&bit != 0 {
					for _, target := range a.EpsilonClosure(move.Destination) {
						next[target] = true
					}
				}
			}
		}
		states = next
	}
	return a.IsFinalConfiguration(states)
}

func TestSymbolicDeterminizeComplementAndTotal(t *testing.T) {
	symbolic, err := NewSymbolic(BitsetAlgebra{Universe: 3})
	if err != nil {
		t.Fatal(err)
	}
	nondeterministic := New(0, []uint32{1, 2}, []Move{
		ConditionMove(0, 1, 1),
		ConditionMove(0, 2, 3),
	})
	deterministic, err := symbolic.Determinize(nondeterministic)
	if err != nil {
		t.Fatal(err)
	}
	if !acceptsBits(deterministic, []uint32{0}) || !acceptsBits(deterministic, []uint32{1}) || acceptsBits(deterministic, nil) {
		t.Fatal("determinized language")
	}
	complement, err := symbolic.Complement(Singleton(1))
	if err != nil {
		t.Fatal(err)
	}
	if !acceptsBits(complement, nil) || acceptsBits(complement, []uint32{0}) || !acceptsBits(complement, []uint32{1}) || !acceptsBits(complement, []uint32{0, 0}) {
		t.Fatal("complement language")
	}
	total, err := symbolic.Total(Singleton(1))
	if err != nil {
		t.Fatal(err)
	}
	if total.NumStates() != 3 || total.IsFinal(2) || total.OutDegree(2) != 1 {
		t.Fatal("total dead state", total.NumStates(), total.FinalStates(), total.Moves())
	}
}

func TestSymbolicProductDifferenceAndEpsilonRemoval(t *testing.T) {
	symbolic, _ := NewSymbolic(BitsetAlgebra{Universe: 3})
	left := New(0, []uint32{2}, []Move{EpsilonMove(0, 1), ConditionMove(1, 2, 3)})
	right := Singleton(1)
	product, err := symbolic.Product(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !acceptsBits(product, []uint32{0}) || acceptsBits(product, []uint32{1}) {
		t.Fatal("product language")
	}
	difference, err := symbolic.Difference(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if acceptsBits(difference, []uint32{0}) || !acceptsBits(difference, []uint32{1}) {
		t.Fatal("difference language")
	}
	without := symbolic.RemoveEpsilons(left)
	if !without.IsEpsilonFree() || !acceptsBits(without, []uint32{0}) || !acceptsBits(without, []uint32{1}) {
		t.Fatal("epsilon removal", without.FinalStates(), without.Moves())
	}
}

func TestSymbolicMinimizeEquivalentFinalStates(t *testing.T) {
	symbolic, _ := NewSymbolic(BitsetAlgebra{Universe: 3})
	a := New(0, []uint32{1, 2}, []Move{
		ConditionMove(0, 1, 1), ConditionMove(0, 2, 2),
		ConditionMove(1, 1, 3), ConditionMove(2, 2, 3),
	})
	minimized, err := symbolic.Minimize(a)
	if err != nil {
		t.Fatal(err)
	}
	if minimized.NumStates() != 2 || !acceptsBits(minimized, []uint32{0}) || !acceptsBits(minimized, []uint32{1, 0}) || acceptsBits(minimized, nil) {
		t.Fatal("minimized automaton", minimized.NumStates(), minimized.FinalStates(), minimized.Moves())
	}
}

// symbolic_automata::mk_product in z3 4.7.1 tests only the initial states
// themselves for finality. Each operand of ("" | a) & ("" | b) reaches its
// final initial state through an epsilon move, so the product came out
// empty; upstream z3 606754c09 uses the epsilon closures.
func TestSymbolicProductEpsilonClosedInitial(t *testing.T) {
	symbolic, _ := NewSymbolic(BitsetAlgebra{Universe: 3})
	left := Union(Epsilon(), Singleton(1))
	right := Union(Epsilon(), Singleton(2))
	if left.IsFinal(left.Initial()) || right.IsFinal(right.Initial()) {
		t.Fatal("operands should reach a final state only through epsilon moves")
	}
	product, err := symbolic.Product(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !acceptsBits(product, nil) || acceptsBits(product, []uint32{0}) || acceptsBits(product, []uint32{1}) {
		t.Fatal("product language", product.FinalStates(), product.Moves())
	}
}
