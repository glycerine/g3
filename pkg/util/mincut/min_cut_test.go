package mincut

// Source: src/util/min_cut.h:25-59; src/util/min_cut.cpp:21-230.

import (
	"errors"
	"reflect"
	"testing"
)

func TestMinCutAugmentPathReverseEdgesFollowPredecessors(t *testing.T) {
	m := New()
	m.NewNode() // 2
	m.NewNode() // 3
	for _, edge := range []struct{ from, to, capacity uint32 }{
		{0, 2, 7}, {2, 3, 5}, {3, 1, 3},
	} {
		if err := m.AddEdgeCapacity(edge.from, edge.to, edge.capacity); err != nil {
			t.Fatal(err)
		}
	}
	m.predecessor = []uint32{0, 3, 0, 2}
	m.augmentPath()
	want := [][]minCutEdge{
		{{node: 2, weight: 4}},
		{{node: 3, weight: 3}},
		{{node: 3, weight: 2}, {node: 0, weight: 3}},
		{{node: 1, weight: 0}, {node: 2, weight: 3}},
	}
	if !reflect.DeepEqual(m.edges, want) {
		t.Fatalf("residual edges after a three-edge augment: got %v, want %v", m.edges, want)
	}
}

func TestMinCutNodeConstructionAndBounds(t *testing.T) {
	m := New()
	if got := m.NewNode(); got != 2 {
		t.Fatal("first ordinary node", got)
	}
	if got := m.NewNode(); got != 3 {
		t.Fatal("second ordinary node", got)
	}
	if err := m.AddEdge(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.AddEdgeCapacity(2, 1, 7); err != nil {
		t.Fatal(err)
	}
	for _, edge := range [][2]uint32{{4, 0}, {0, 4}} {
		if err := m.AddEdge(edge[0], edge[1]); !errors.Is(err, ErrNode) {
			t.Fatal("out-of-range edge", edge, err)
		}
	}
	var absent *MinCut
	if err := absent.AddEdge(0, 1); !errors.Is(err, ErrNode) {
		t.Fatal("nil graph edge", err)
	}
	if _, err := absent.ComputeMinCut(nil); !errors.Is(err, ErrNode) {
		t.Fatal("nil graph compute", err)
	}
}

func TestMinCutSourceSinkOnlyPreservesOutput(t *testing.T) {
	m := New()
	if err := m.AddEdgeCapacity(0, 1, 9); err != nil {
		t.Fatal(err)
	}
	seed := []uint32{99}
	got, err := m.ComputeMinCut(seed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, seed) {
		t.Fatal(got)
	}
}

func TestMinCutWeightedBranchesAndAppendOrder(t *testing.T) {
	m := New()
	a := m.NewNode()
	b := m.NewNode()
	for _, edge := range []struct {
		from, to, capacity uint32
	}{
		{0, a, 2},
		{0, b, 1},
		{a, 1, 1},
		{b, 1, 1},
	} {
		if err := m.AddEdgeCapacity(edge.from, edge.to, edge.capacity); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.ComputeMinCut([]uint32{77})
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{77, b, 1}; !reflect.DeepEqual(got, want) {
		t.Fatal("cut nodes", got, want)
	}
	if _, err := m.ComputeMinCut(nil); !errors.Is(err, ErrComputed) {
		t.Fatal("second compute", err)
	}
}

func TestMinCutZeroCapacityStillAppearsInCutEnumeration(t *testing.T) {
	m := New()
	a := m.NewNode()
	b := m.NewNode()
	if err := m.AddEdge(0, a); err != nil {
		t.Fatal(err)
	}
	if err := m.AddEdge(a, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddEdgeCapacity(0, b, 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddEdge(b, 1); err != nil {
		t.Fatal(err)
	}
	got, err := m.ComputeMinCut(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{a, b}; !reflect.DeepEqual(got, want) {
		t.Fatal("cut nodes", got, want)
	}
}

func TestMinCutRejectsSourceGraphsThatWouldNotTerminate(t *testing.T) {
	disconnected := New()
	a := disconnected.NewNode()
	if err := disconnected.AddEdge(0, a); err != nil {
		t.Fatal(err)
	}
	if _, err := disconnected.ComputeMinCut(nil); !errors.Is(err, ErrGraph) {
		t.Fatal("reached dead end", err)
	}

	cyclic := New()
	a = cyclic.NewNode()
	if err := cyclic.AddEdge(0, a); err != nil {
		t.Fatal(err)
	}
	if err := cyclic.AddEdge(a, 0); err != nil {
		t.Fatal(err)
	}
	if err := cyclic.AddEdge(a, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := cyclic.ComputeMinCut(nil); !errors.Is(err, ErrGraph) {
		t.Fatal("cycle", err)
	}
}

func TestMinCutExhaustiveFourNodeDAGAgainstPartitionOracle(t *testing.T) {
	candidates := [][2]uint32{
		{0, 2}, {0, 3}, {0, 1}, {2, 3}, {2, 1}, {3, 1},
	}
	for edgeMask := uint32(0); edgeMask < 1<<len(candidates); edgeMask++ {
		original := make([][2]uint32, 0, len(candidates))
		for i, edge := range candidates {
			if edgeMask&(1<<i) != 0 {
				original = append(original, edge)
			}
		}
		if !minCutTestAllReachedCanReachSink(original) {
			continue
		}

		m := New()
		m.NewNode()
		m.NewNode()
		for _, edge := range original {
			if err := m.AddEdge(edge[0], edge[1]); err != nil {
				t.Fatal(edgeMask, err)
			}
		}
		if _, err := m.ComputeMinCut(nil); err != nil {
			t.Fatal(edgeMask, err)
		}

		reachable := make([]bool, 4)
		m.computeReachableNodes(reachable)
		var residualCut uint32
		for _, edge := range original {
			if reachable[edge[0]] && !reachable[edge[1]] {
				residualCut++
			}
		}
		want := uint32(len(original) + 1)
		for partition := uint32(0); partition < 4; partition++ {
			inSource := [4]bool{true, false, partition&1 != 0, partition&2 != 0}
			var capacity uint32
			for _, edge := range original {
				if inSource[edge[0]] && !inSource[edge[1]] {
					capacity++
				}
			}
			if capacity < want {
				want = capacity
			}
		}
		if residualCut != want {
			t.Fatal("edge mask", edgeMask, "cut capacity", residualCut, "want", want, "reachable", reachable)
		}
	}
}

func minCutTestAllReachedCanReachSink(edges [][2]uint32) bool {
	fromSource := [4]bool{true}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if fromSource[edge[0]] && !fromSource[edge[1]] {
				fromSource[edge[1]] = true
				changed = true
			}
		}
	}
	toSink := [4]bool{false, true}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if toSink[edge[1]] && !toSink[edge[0]] {
				toSink[edge[0]] = true
				changed = true
			}
		}
	}
	for node := range fromSource {
		if fromSource[node] && !toSink[node] {
			return false
		}
	}
	return fromSource[1]
}

// z3 4.7.1 never returns on this diamond: both augmenting paths share node
// 4, and augment_path's sink-directed reverse arc keeps refilling capacity
// into the sink (AUDIT.md util_misc_01-F1, upstream 8150cd6e8). The cut is
// the edge 4->1, reported as node 1, as in the fixed C++.
func TestMinCutSharedNodeDiamondTerminates(t *testing.T) {
	m := New()
	a := m.NewNode()
	b := m.NewNode()
	c := m.NewNode()
	for _, edge := range [][2]uint32{{0, a}, {0, b}, {a, c}, {b, c}, {c, 1}} {
		if err := m.AddEdge(edge[0], edge[1]); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.ComputeMinCut(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []uint32{1}; !reflect.DeepEqual(got, want) {
		t.Fatal("cut nodes", got, want)
	}
}
