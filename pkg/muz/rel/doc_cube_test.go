package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func newTestDocManager(t *testing.T, width uint32) (*DocManager, *arena.Budget) {
	t.Helper()
	budget := arena.NewBudget(1 << 22)
	manager, err := NewDocManager(width, arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		manager.Close()
		if used := budget.Used(); used != 0 {
			t.Errorf("DOC manager retained %d bytes", used)
		}
	})
	return manager, budget
}

func docPatternVector(t *testing.T, manager *DocManager, display string) *TBV {
	t.Helper()
	if uint32(len(display)) != manager.NumTBits() {
		t.Fatal("pattern width", display)
	}
	vector, err := manager.vectors.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	for offset, char := range display {
		value := TBitX
		switch char {
		case '0':
			value = TBit0
		case '1':
			value = TBit1
		case 'x':
		default:
			t.Fatal("pattern character", display)
		}
		if err = manager.vectors.Set(vector, uint32(len(display)-offset-1), value); err != nil {
			t.Fatal(err)
		}
	}
	return vector
}

func docPattern(t *testing.T, manager *DocManager, positive string, negatives ...string) *Doc {
	t.Helper()
	document, err := manager.AdoptTBV(docPatternVector(t, manager, positive))
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range negatives {
		if err = manager.AddNegativeOwned(document, docPatternVector(t, manager, pattern)); err != nil {
			t.Fatal(err)
		}
	}
	return document
}

func independentCubeMatches(t *testing.T, manager *DocManager, vector *TBV, assignment uint32) bool {
	t.Helper()
	for bit := uint32(0); bit < manager.NumTBits(); bit++ {
		value, err := manager.vectors.Get(vector, bit)
		if err != nil {
			t.Fatal(err)
		}
		switch value {
		case TBitX:
		case TBit0:
			if assignment&(1<<bit) != 0 {
				return false
			}
		case TBit1:
			if assignment&(1<<bit) == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func independentDocMatches(t *testing.T, manager *DocManager, document *Doc, assignment uint32) bool {
	t.Helper()
	if !independentCubeMatches(t, manager, document.positive, assignment) {
		return false
	}
	for _, negative := range document.negative.values {
		if independentCubeMatches(t, manager, negative, assignment) {
			return false
		}
	}
	return true
}

func independentUnionMatches(t *testing.T, union *DocUnion, assignment uint32) bool {
	t.Helper()
	for _, document := range union.values {
		if independentDocMatches(t, union.owner, document, assignment) {
			return true
		}
	}
	return false
}

func assertDocTruth(t *testing.T, manager *DocManager, document *Doc, want func(uint32) bool) {
	t.Helper()
	for assignment := uint32(0); assignment < 1<<manager.NumTBits(); assignment++ {
		if got := independentDocMatches(t, manager, document, assignment); got != want(assignment) {
			t.Fatalf("assignment %03b got %v want %v", assignment, got, want(assignment))
		}
	}
}

func assertUnionTruth(t *testing.T, union *DocUnion, want func(uint32) bool) {
	t.Helper()
	for assignment := uint32(0); assignment < 1<<union.owner.NumTBits(); assignment++ {
		if got := independentUnionMatches(t, union, assignment); got != want(assignment) {
			t.Fatalf("assignment %03b got %v want %v", assignment, got, want(assignment))
		}
	}
}

func TestDOCSourceIntersectionAndFoldTruthTables(t *testing.T) {
	manager, _ := newTestDocManager(t, 3)
	left := docPattern(t, manager, "xxx", "0xx")
	right := docPattern(t, manager, "x1x", "x11")
	leftTruth := func(value uint32) bool { return value&4 != 0 }
	rightTruth := func(value uint32) bool { return value&2 != 0 && value&1 == 0 }
	assertDocTruth(t, manager, left, leftTruth)
	assertDocTruth(t, manager, right, rightTruth)
	result, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	valid, err := manager.Intersect(left, right, result)
	if err != nil || !valid {
		t.Fatal("intersection", valid, err)
	}
	assertDocTruth(t, manager, result, func(value uint32) bool { return leftTruth(value) && rightTruth(value) })
	wellFormed, err := manager.WellFormed(result)
	if err != nil || !wellFormed {
		t.Fatal("intersection well formed", wellFormed, err)
	}

	oneDifference := docPattern(t, manager, "xxx", "xx0")
	valid, err = manager.FoldNeg(oneDifference)
	if err != nil || !valid {
		t.Fatal("one difference", valid, err)
	}
	if text, _ := manager.String(oneDifference); text != "xx1" {
		t.Fatal("one-difference fold", text)
	}
	assertDocTruth(t, manager, oneDifference, func(value uint32) bool { return value&1 != 0 })

	disjoint := docPattern(t, manager, "xx0", "xx1")
	valid, err = manager.FoldNeg(disjoint)
	if err != nil || !valid || disjoint.NumNegative() != 0 {
		t.Fatal("source count-three removal", valid, disjoint.NumNegative(), err)
	}
	twoDifferences := docPattern(t, manager, "xxx", "x00")
	valid, err = manager.FoldNeg(twoDifferences)
	if err != nil || !valid || twoDifferences.NumNegative() != 1 {
		t.Fatal("source count-two retention", valid, twoDifferences.NumNegative(), err)
	}
	empty := docPattern(t, manager, "xxx", "xxx")
	valid, err = manager.FoldNeg(empty)
	if err != nil || valid {
		t.Fatal("source covered positive is empty", valid, err)
	}
}

func TestDOCSourceComplementSubtractAndUnionConsumers(t *testing.T) {
	manager, _ := newTestDocManager(t, 3)
	left := docPattern(t, manager, "xxx", "0xx")
	right := docPattern(t, manager, "x1x", "x11")
	leftTruth := func(value uint32) bool { return value&4 != 0 }
	rightTruth := func(value uint32) bool { return value&2 != 0 && value&1 == 0 }

	complement, _ := NewDocUnion(manager)
	if err := manager.Complement(left, complement); err != nil {
		t.Fatal(err)
	}
	assertUnionTruth(t, complement, func(value uint32) bool { return !leftTruth(value) })
	difference, _ := NewDocUnion(manager)
	if err := manager.Subtract(left, right, difference); err != nil {
		t.Fatal(err)
	}
	assertUnionTruth(t, difference, func(value uint32) bool { return leftTruth(value) && !rightTruth(value) })

	union, _ := NewDocUnion(manager)
	leftCopy, _ := manager.AllocateCopy(left)
	rightCopy, _ := manager.AllocateCopy(right)
	if err := union.PushOwned(leftCopy); err != nil {
		t.Fatal(err)
	}
	if err := union.PushOwned(rightCopy); err != nil {
		t.Fatal(err)
	}
	assertUnionTruth(t, union, func(value uint32) bool { return leftTruth(value) || rightTruth(value) })
	unionComplement, _ := NewDocUnion(manager)
	if err := union.Complement(unionComplement); err != nil {
		t.Fatal(err)
	}
	assertUnionTruth(t, unionComplement, func(value uint32) bool { return !(leftTruth(value) || rightTruth(value)) })
	if valid, err := union.WellFormed(); err != nil || !valid {
		t.Fatal("union well formed", valid, err)
	}
	if bytes, err := union.SizeEstimateBytes(); err != nil || bytes == 0 {
		t.Fatal("union size", bytes, err)
	}
	if err := complement.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := difference.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := union.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := unionComplement.Reset(); err != nil {
		t.Fatal(err)
	}
}

func TestDOCSourceCopyPermutationContainmentAndDisplay(t *testing.T) {
	manager, _ := newTestDocManager(t, 3)
	source := docPattern(t, manager, "10x", "100", "101")
	copy, err := manager.AllocateCopy(source)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := manager.Equal(source, copy)
	if err != nil || !equal {
		t.Fatal("copy", equal, err)
	}
	first, _ := copy.Negative(0)
	if err = manager.vectors.Set(first, 0, TBitX); err != nil {
		t.Fatal(err)
	}
	equal, err = manager.Equal(source, copy)
	if err != nil || equal {
		t.Fatal("copy owns negative vectors independently", equal, err)
	}
	permuted, err := manager.AllocatePermutation(source, []uint32{2, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := manager.String(permuted); text != "x01 \\ {001, 101}" {
		t.Fatal("permutation/display", text)
	}
	container := docPattern(t, manager, "xxx")
	contains, err := manager.Contains(container, source)
	if err != nil || !contains {
		t.Fatal("full contains source", contains, err)
	}
	if hash, err := manager.Hash(source); err != nil || hash == 0 {
		t.Fatal("hash", hash, err)
	}
	if err := manager.Set(source, 0, TBit1); err != nil {
		t.Fatal(err)
	}
	if source.NumNegative() != 1 {
		t.Fatal("set removes contradictory negative", source.NumNegative())
	}
}

func TestDOCAllocationFailureAndForeignOwnershipPrefixes(t *testing.T) {
	manager, budget := newTestDocManager(t, 2)
	warm, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Deallocate(warm); err != nil {
		t.Fatal(err)
	}
	used := budget.Used()
	budget.FailAfter(0)
	if _, err = manager.AllocateX(); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("header allocation failure", err)
	}
	if budget.Used() != used {
		t.Fatal("failed header adopted vector", used, budget.Used())
	}
	budget.FailAfter(-1)
	document, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	foreign, _ := newTestDocManager(t, 2)
	foreignVector := docPatternVector(t, foreign, "00")
	if err = manager.AddNegativeOwned(document, foreignVector); err == nil {
		t.Fatal("foreign vector accepted")
	}
	if err = manager.Deallocate(document); err != nil {
		t.Fatal(err)
	}
	if err = manager.Deallocate(document); !errors.Is(err, ErrDOC) {
		t.Fatal("double deallocation", err)
	}
}
