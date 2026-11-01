package rel

import (
	"fmt"
	"testing"
)

func TestSparseJoinProjectMatchesFiniteOracle(t *testing.T) {
	signature, err := NewSignature([]uint64{2, 2})
	must(t, err)
	allFacts := []Fact{{0, 0}, {0, 1}, {1, 0}, {1, 1}}

	for leftMask := 0; leftMask < 16; leftMask++ {
		left, err := NewSparseTable(signature, 0)
		must(t, err)
		for index, fact := range allFacts {
			if leftMask&(1<<index) != 0 {
				if _, err = left.Add(fact); err != nil {
					t.Fatal(err)
				}
			}
		}
		for rightMask := 0; rightMask < 16; rightMask++ {
			right, err := NewSparseTable(signature, 0)
			must(t, err)
			for index, fact := range allFacts {
				if rightMask&(1<<index) != 0 {
					if _, err = right.Add(fact); err != nil {
						t.Fatal(err)
					}
				}
			}
			for removedMask := 0; removedMask < 15; removedMask++ {
				removed := make([]int, 0, 3)
				for column := 0; column < 4; column++ {
					if removedMask&(1<<column) != 0 {
						removed = append(removed, column)
					}
				}
				projected, err := left.JoinProject(right, []int{0}, []int{0}, removed)
				if err != nil {
					t.Fatalf("left=%04b right=%04b removed=%v: %v", leftMask, rightMask, removed, err)
				}
				gotFacts, err := projected.Facts()
				if err != nil {
					t.Fatal(err)
				}
				want := make(map[string]bool)
				for leftIndex, leftFact := range allFacts {
					if leftMask&(1<<leftIndex) == 0 {
						continue
					}
					for rightIndex, rightFact := range allFacts {
						if rightMask&(1<<rightIndex) == 0 || leftFact[0] != rightFact[0] {
							continue
						}
						joined := Fact{leftFact[0], leftFact[1], rightFact[0], rightFact[1]}
						row := make(Fact, 0, 4-len(removed))
						nextRemoved := 0
						for column, value := range joined {
							if nextRemoved < len(removed) && removed[nextRemoved] == column {
								nextRemoved++
								continue
							}
							row = append(row, value)
						}
						want[fmt.Sprint(row)] = true
					}
				}
				if len(gotFacts) != len(want) {
					t.Fatalf("left=%04b right=%04b removed=%v: got %v, want %v", leftMask, rightMask, removed, gotFacts, want)
				}
				for _, fact := range gotFacts {
					if !want[fmt.Sprint(fact)] {
						t.Fatalf("left=%04b right=%04b removed=%v: unexpected row %v, want %v", leftMask, rightMask, removed, fact, want)
					}
				}
				if err = projected.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err = right.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if err = left.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
