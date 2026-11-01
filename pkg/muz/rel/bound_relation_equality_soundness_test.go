package rel

import (
	"math/big"
	"reflect"
	"testing"
)

func TestBoundRelationEqualityMatchesSourceRepresentativeCheck(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, strict := range []bool{false, true} {
			r := NewBoundRelation(2, false)
			var err error
			if strict {
				err = r.AddLess(0, 1)
			} else {
				err = r.AddLessEqual(0, 1)
			}
			astTestMust(t, err)
			if reverse {
				err = r.Equate(1, 0)
			} else {
				err = r.Equate(0, 1)
			}
			astTestMust(t, err)
			if r.Empty() != reverse {
				t.Fatalf("reverse=%t strict=%t: empty=%t", reverse, strict, r.Empty())
			}
			if !reverse {
				lt, le, err := r.Bounds(0)
				astTestMust(t, err)
				if strict && !reflect.DeepEqual(lt, []uint32{1}) || !strict && !reflect.DeepEqual(le, []uint32{1}) {
					t.Fatalf("source preserves the post-merge self bound: strict=%v weak=%v", lt, le)
				}
			}
		}
	}
}

func TestIntervalBoundProductIdenticalColumnsWithWeakOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		p := NewIntervalBoundProduct(2, false)
		astTestMust(t, p.bound.AddLessEqual(0, 1))
		if reverse {
			astTestMust(t, p.FilterIdentical(1, 0))
		} else {
			astTestMust(t, p.FilterIdentical(0, 1))
		}
		if p.Empty() != reverse {
			t.Fatalf("source first-representative check, reverse=%t empty=%t", reverse, p.Empty())
		}
		if p.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1)}) == reverse {
			t.Fatalf("membership did not follow empty state, reverse=%t", reverse)
		}
	}
}

func TestBoundRelationStrictCycleThroughMergedColumn(t *testing.T) {
	r := NewBoundRelation(3, false)
	astTestMust(t, r.AddLess(0, 1))
	astTestMust(t, r.Equate(1, 2))
	if r.Empty() {
		t.Fatal("unrelated equality made strict order empty")
	}
	astTestMust(t, r.AddLessEqual(2, 0))
	if !r.Empty() {
		t.Fatal("strict cycle through an equality class was not detected")
	}
}

func TestBoundRelationStrictCyclesAgainstGraphOracle(t *testing.T) {
	const n = 3
	// Each directed edge is absent, weak, or strict. Exhausting all 3^6
	// simple graphs checks transitive propagation and strict-cycle detection.
	edges := make([][2]int, 0, n*(n-1))
	for from := 0; from < n; from++ {
		for to := 0; to < n; to++ {
			if from != to {
				edges = append(edges, [2]int{from, to})
			}
		}
	}
	configs := 1
	for range edges {
		configs *= 3
	}
	for config := 0; config < configs; config++ {
		state := config
		closure := [n][n]uint8{}
		r := NewBoundRelation(n, false)
		for _, edge := range edges {
			kind := state % 3
			state /= 3
			if kind == 0 {
				continue
			}
			closure[edge[0]][edge[1]] = uint8(kind)
			var err error
			if kind == 1 {
				err = r.AddLessEqual(uint32(edge[0]), uint32(edge[1]))
			} else {
				err = r.AddLess(uint32(edge[0]), uint32(edge[1]))
			}
			astTestMust(t, err)
		}
		for via := 0; via < n; via++ {
			for from := 0; from < n; from++ {
				for to := 0; to < n; to++ {
					if closure[from][via] == 0 || closure[via][to] == 0 {
						continue
					}
					path := closure[from][via]
					if closure[via][to] > path {
						path = closure[via][to]
					}
					if path > closure[from][to] {
						closure[from][to] = path
					}
				}
			}
		}
		wantEmpty := false
		for i := 0; i < n; i++ {
			wantEmpty = wantEmpty || closure[i][i] == 2
		}
		if r.Empty() != wantEmpty {
			t.Fatalf("config %d: empty=%t want %t", config, r.Empty(), wantEmpty)
		}
	}
}

func TestIntervalBoundProductJoinCopiesRightBoundIndicesUnchanged(t *testing.T) {
	left, right := NewIntervalBoundProduct(1, false), NewIntervalBoundProduct(2, false)
	astTestMust(t, right.bound.AddLess(0, 1))
	joined, err := JoinIntervalBoundProducts(left, right, nil, nil)
	astTestMust(t, err)
	strict, weak, err := joined.bound.Bounds(1)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{1}) || len(weak) != 0 {
		t.Fatalf("source vector join leaves raw right index unshifted: strict=%v weak=%v", strict, weak)
	}
}
