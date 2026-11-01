package rel

import "testing"

func TestSparseNegatedJoinIndexedPathMatchesExhaustiveOracle(t *testing.T) {
	signature, err := NewSignature([]uint64{2, 2})
	if err != nil {
		t.Fatal(err)
	}
	universe := []Fact{{0, 0}, {0, 1}, {1, 0}, {1, 1}}
	for firstMask := 0; firstMask < 1<<len(universe); firstMask++ {
		for secondMask := 0; secondMask < 1<<len(universe); secondMask++ {
			target, err := NewSparseTable(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			first, err := NewSparseTable(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			second, err := NewSparseTable(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range universe {
				if _, err = target.Add(row); err != nil {
					t.Fatal(err)
				}
			}
			firstFacts, secondFacts := make([]Fact, 0), make([]Fact, 0)
			for i, row := range universe {
				if firstMask&(1<<i) != 0 {
					if _, err = first.Add(row); err != nil {
						t.Fatal(err)
					}
					firstFacts = append(firstFacts, row)
				}
				if secondMask&(1<<i) != 0 {
					if _, err = second.Add(row); err != nil {
						t.Fatal(err)
					}
					secondFacts = append(secondFacts, row)
				}
			}
			plan, err := NewSparseNegatedJoinPlan(target, first, second, []int{0, 1}, []int{0, 2}, []int{1}, []int{1})
			if err != nil {
				t.Fatal(err)
			}
			if !sparseNegatedJoinIndexable(plan) {
				t.Fatal("nonfunctional key plan did not select indexed path")
			}
			if _, err = plan.Apply(target, first, second); err != nil {
				t.Fatal(err)
			}
			want := make([]Fact, 0)
			for _, candidate := range universe {
				matched := false
				for _, left := range firstFacts {
					if left[0] != candidate[0] {
						continue
					}
					for _, right := range secondFacts {
						if right[0] == candidate[1] && left[1] == right[1] {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
				if !matched {
					want = append(want, candidate)
				}
			}
			got, err := target.Facts()
			if err != nil || len(got) != len(want) {
				t.Fatalf("masks (%04b,%04b): facts=%v want=%v err=%v", firstMask, secondMask, got, want, err)
			}
			wantSet := make(map[string]bool, len(want))
			for _, fact := range want {
				wantSet[factKey(fact)] = true
			}
			for _, fact := range got {
				if !wantSet[factKey(fact)] {
					t.Fatalf("masks (%04b,%04b): unexpected fact %v in %v, want %v", firstMask, secondMask, fact, got, want)
				}
			}
			_ = target.Close()
			_ = first.Close()
			_ = second.Close()
		}
	}
}
