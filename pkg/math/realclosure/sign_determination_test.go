package realclosure

import (
	"reflect"
	"testing"
)

func TestSignDeterminationRetainedColumnTransition(t *testing.T) {
	state := newTestSignDetermination(3)
	if kept, allOne := state.advanceT(2, 1, 0, []int{3, 1}); !kept || allOne {
		t.Fatalf("first derivative: kept=%v, allOne=%v", kept, allOne)
	}
	if state.matrix.rows != 2 || state.matrix.cols != 2 || !reflect.DeepEqual(state.tarski, []int{3, 1}) || len(state.conditions) != 2 || state.polynomials != 1 {
		t.Fatal("first derivative did not publish the two-column state")
	}
	if state.conditions[0].sign != 0 || state.conditions[1].sign != 1 || state.conditions[0].previous != nil || state.conditions[1].previous != nil {
		t.Fatal("first derivative sign assignment differs from source matrix")
	}
	firstZero, firstPositive := state.conditions[0], state.conditions[1]
	if kept, allOne := state.advanceT(3, 0, 0, nil); kept || allOne || state.polynomials != 1 {
		t.Fatal("derivative with one possible sign changed the state")
	}
	// Both old columns have a single surviving child. The derivative cannot
	// distinguish any roots within an old sign condition, so it is discarded.
	if kept, allOne := state.advanceT(2, 1, 0, []int{3, 1, 1, 1}); kept || allOne {
		t.Fatalf("nondiscriminating derivative: kept=%v, allOne=%v", kept, allOne)
	}
	if state.polynomials != 1 || state.conditions[0] != firstZero || state.conditions[1] != firstPositive || !reflect.DeepEqual(state.tarski, []int{3, 1}) {
		t.Fatal("discarded derivative changed published state")
	}
	// Cardinalities are [1,1,0, 0,0,1] in the tensor-product columns.
	// They split the first old condition and leave one child of the second.
	if kept, allOne := state.advanceT(1, 1, 1, []int{3, 0, 2, 1, -1, 1}); !kept || !allOne {
		t.Fatalf("final derivative: kept=%v, allOne=%v", kept, allOne)
	}
	if state.matrix.rows != 3 || state.matrix.cols != 3 || len(state.tarski) != 3 || len(state.conditions) != 3 || state.polynomials != 2 {
		t.Fatal("final derivative did not reduce to three roots")
	}
	wantSigns := []int{0, 1, -1}
	wantPrevious := []*signCondition{firstZero, firstZero, firstPositive}
	for i, sc := range state.conditions {
		if sc.polynomial != 1 || sc.sign != wantSigns[i] || sc.previous != wantPrevious[i] {
			t.Fatalf("condition %d has wrong sign chain", i)
		}
	}
	if result, ok := state.matrix.solveIntT(state.tarski); !ok || !reflect.DeepEqual(result, []int{1, 1, 1}) {
		t.Fatalf("published reduced system: result=%v, ok=%v", result, ok)
	}
}

func TestKeepSignAssignmentUsesOldConditionGroups(t *testing.T) {
	if !keepSignAssignment([]int{2, 0, 1, 1}, false) || keepSignAssignment([]int{2, 0, 0, 1}, false) {
		t.Fatal("two-way sign discrimination used the wrong grouping")
	}
	if !keepSignAssignment([]int{1, 0, 1, 3, 0, 0}, true) || keepSignAssignment([]int{1, 0, 0, 0, 2, 0}, true) {
		t.Fatal("three-way sign discrimination used the wrong grouping")
	}
}
