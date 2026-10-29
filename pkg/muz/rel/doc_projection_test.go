package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

func projectedAssignment(retained uint32, deleted []bool, deletedValue uint32) uint32 {
	var source, retainedIndex, deletedIndex uint32
	for bit, remove := range deleted {
		var value uint32
		if remove {
			value = deletedValue >> deletedIndex & 1
			deletedIndex++
		} else {
			value = retained >> retainedIndex & 1
			retainedIndex++
		}
		source |= value << uint32(bit)
	}
	return source
}

func TestDOCProjectionAllSourceActionsAgainstExistentialOracle(t *testing.T) {
	sourceManager, _ := newTestDocManager(t, 4)
	destinationManager, _ := newTestDocManager(t, 2)
	deleted := []bool{true, false, true, false}
	cases := []struct {
		name      string
		positive  string
		negatives []string
	}{
		{"no-negatives", "1xxx", nil},
		{"project-empty", "xxxx", []string{"xxxx"}},
		{"monolithic", "xxxx", []string{"xxx0", "x1x0"}},
		{"one-polarity", "xxxx", []string{"xxx0", "x1xx"}},
		{"resolution", "xxxx", []string{"x0x0", "xx11", "1x10"}},
		{"done", "xxxx", []string{"1x1x", "0x0x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := docPattern(t, sourceManager, tc.positive, tc.negatives...)
			projected, err := sourceManager.Project(destinationManager, deleted, source)
			if err != nil {
				t.Fatal(err)
			}
			for retained := uint32(0); retained < 4; retained++ {
				want := false
				for eliminated := uint32(0); eliminated < 4; eliminated++ {
					if independentDocMatches(t, sourceManager, source, projectedAssignment(retained, deleted, eliminated)) {
						want = true
						break
					}
				}
				if got := independentDocMatches(t, destinationManager, projected, retained); got != want {
					t.Fatalf("retained=%02b got %v want %v; projected=%v", retained, got, want, func() string { s, _ := destinationManager.String(projected); return s }())
				}
			}
			valid, err := destinationManager.WellFormed(projected)
			if err != nil || !valid {
				t.Fatal("projected well formed", valid, err)
			}
		})
	}
}

func TestDOCJoinAgainstConcatenationEqualityOracle(t *testing.T) {
	leftManager, _ := newTestDocManager(t, 2)
	rightManager, _ := newTestDocManager(t, 2)
	resultManager, _ := newTestDocManager(t, 4)
	left := docPattern(t, leftManager, "xx", "00")
	right := docPattern(t, rightManager, "1x", "11")
	joined, err := resultManager.Join(left, right, leftManager, []uint32{0}, []uint32{1})
	if err != nil || joined == nil {
		t.Fatal("join", joined, err)
	}
	for assignment := uint32(0); assignment < 16; assignment++ {
		leftValue := assignment & 3
		rightValue := assignment >> 2
		want := independentDocMatches(t, leftManager, left, leftValue) && independentDocMatches(t, rightManager, right, rightValue) && (leftValue&1) == ((rightValue>>1)&1)
		if got := independentDocMatches(t, resultManager, joined, assignment); got != want {
			t.Fatalf("assignment=%04b got %v want %v", assignment, got, want)
		}
	}
	conflictingLeft := docPattern(t, leftManager, "x0")
	conflictingRight := docPattern(t, rightManager, "1x")
	joined, err = resultManager.Join(conflictingLeft, conflictingRight, leftManager, []uint32{0}, []uint32{1})
	if err != nil || joined != nil {
		t.Fatal("fixed mismatch", joined, err)
	}
}

func TestDOCMergeEqualityClassesAgainstFiniteOracle(t *testing.T) {
	manager, _ := newTestDocManager(t, 3)
	classes := backtrack.NewDefaultUnionFind()
	for i := 0; i < 3; i++ {
		classes.MkVar()
	}
	classes.Merge(0, 2)
	document := docPattern(t, manager, "xxx")
	valid, err := manager.MergeRange(document, 0, 1, classes, []bool{false, false, false})
	if err != nil || !valid {
		t.Fatal("merge", valid, err)
	}
	assertDocTruth(t, manager, document, func(value uint32) bool { return value&1 == (value>>2)&1 })
	if document.NumNegative() != 2 {
		t.Fatal("two mismatch cubes", document.NumNegative())
	}
	fixed := docPattern(t, manager, "0xx")
	valid, err = manager.MergeRange(fixed, 0, 1, classes, []bool{false, false, false})
	if err != nil || !valid {
		t.Fatal("fixed merge", valid, err)
	}
	if text, _ := manager.String(fixed); text != "0x0" {
		t.Fatal("fixed value propagation", text)
	}
	conflict := docPattern(t, manager, "1x0")
	valid, err = manager.MergeRange(conflict, 0, 1, classes, []bool{false, false, false})
	if err != nil || valid {
		t.Fatal("conflicting class", valid, err)
	}
}

func TestDOCProjectionFailureReleasesTemporaryOwnership(t *testing.T) {
	sourceManager, _ := newTestDocManager(t, 3)
	destinationBudget := arena.NewBudget(1 << 20)
	destination, err := NewDocManager(2, arena.VectorConfig{PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: destinationBudget}})
	if err != nil {
		t.Fatal(err)
	}
	source := docPattern(t, sourceManager, "xxx", "xx0", "xx1")
	used := destinationBudget.Used()
	destinationBudget.FailAfter(0)
	if _, err = sourceManager.Project(destination, []bool{true, false, false}, source); err == nil {
		t.Fatal("expected destination allocation refusal")
	}
	if destinationBudget.Used() != used {
		t.Fatal("projection failure retained destination storage", used, destinationBudget.Used())
	}
	destinationBudget.FailAfter(-1)
	destination.Close()
	if destinationBudget.Used() != 0 {
		t.Fatal("destination close", destinationBudget.Used())
	}
}
