package ddnf

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func testCore(t *testing.T, width uint32) *Core {
	t.Helper()
	budget := arena.NewBudget(1 << 20)
	core, err := NewCore(width, arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		core.Close()
		if used := budget.Used(); used != 0 {
			t.Errorf("DDNF core retained %d bytes", used)
		}
	})
	return core
}

func testPattern(t *testing.T, core *Core, display string) *rel.TBV {
	t.Helper()
	manager := core.Manager()
	vector, err := manager.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	if uint32(len(display)) != manager.NumTBits() {
		t.Fatal("pattern width", display)
	}
	for offset, char := range display {
		var value rel.TBit
		switch char {
		case '0':
			value = rel.TBit0
		case '1':
			value = rel.TBit1
		case 'x':
			value = rel.TBitX
		default:
			t.Fatal("invalid pattern", display)
		}
		if err = manager.Set(vector, uint32(len(display)-offset-1), value); err != nil {
			t.Fatal(err)
		}
	}
	return vector
}

func TestDDNFCoreSourceIntersectionDAGAndAccumulate(t *testing.T) {
	core := testCore(t, 3)
	p := testPattern(t, core, "0xx")
	q := testPattern(t, core, "x0x")
	r := testPattern(t, core, "001")
	pNode, err := core.Insert(p)
	if err != nil {
		t.Fatal(err)
	}
	qNode, err := core.Insert(q)
	if err != nil {
		t.Fatal(err)
	}
	rNode, err := core.Insert(r)
	if err != nil {
		t.Fatal(err)
	}
	if pNode.ID() != 1 || qNode.ID() != 2 || rNode.ID() != 4 || core.Size() != 5 {
		t.Fatal("source node/intersection insertion order", pNode.ID(), qNode.ID(), rNode.ID(), core.Size())
	}
	for _, vector := range []*rel.TBV{p, q, r} {
		if found, containsErr := core.Contains(vector); containsErr != nil || !found {
			t.Fatal("inserted node lookup", found, containsErr)
		}
	}
	missing := testPattern(t, core, "111")
	if found, containsErr := core.Contains(missing); containsErr != nil || found {
		t.Fatal("uninserted vector lookup", found, containsErr)
	}
	intersection, err := pNode.Child(0)
	if err != nil || intersection.ID() != 3 || !qNode.ContainsChild(intersection) || !intersection.ContainsChild(rNode) {
		t.Fatal("source intersection DAG", intersection, err)
	}
	intersectionText, err := core.Manager().String(intersection.Vector())
	if err != nil || intersectionText != "00x" {
		t.Fatal("intersection vector", intersectionText, err)
	}
	wellFormed, err := core.WellFormed()
	if err != nil || !wellFormed {
		t.Fatal("well formed", wellFormed, err)
	}

	core.ResetAccumulate()
	labels, err := core.Accumulate(p, nil)
	if err != nil || len(labels) != 3 || labels[0] != 1 || labels[1] != 3 || labels[2] != 4 {
		t.Fatal("first source accumulation", labels, err)
	}
	labels, err = core.Accumulate(q, labels)
	if err != nil || len(labels) != 4 || labels[3] != 2 {
		t.Fatal("second source accumulation excludes covered labels", labels, err)
	}
	stats := core.Statistics()
	if stats.Insertions != 10 || stats.Comparisons != 10 || stats.Nodes != 5 {
		t.Fatal("source comparison accounting", stats)
	}
	display, err := core.Display()
	if err != nil {
		t.Fatal(err)
	}
	want := "node[0: xxx 1 2]\nnode[1: 0xx 3]\nnode[2: x0x 3]\nnode[3: 00x 4]\nnode[4: 001]\n"
	if display != want {
		t.Fatalf("display\n got %q\nwant %q", display, want)
	}
	if statistics, want := core.DisplayStatistics(), "Number of insertions:  10\nNumber of comparisons: 10\nNumber of nodes:       5\n"; statistics != want {
		t.Fatal("statistics display", statistics)
	}
}

func TestDDNFCoreInsertionAllocationRefusalCanRetry(t *testing.T) {
	for failure := int64(0); failure < 24; failure++ {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			budget := arena.NewBudget(1 << 20)
			config := arena.VectorConfig{PageSize: 1, Descriptors: arena.Config{PageSize: 1, Budget: budget}}
			core, err := NewCore(3, config)
			if err != nil {
				t.Fatal(err)
			}
			p, err := core.Manager().AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			q, err := core.Manager().AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			for i, vector := range []*rel.TBV{p, q} {
				for bit, value := range []rel.TBit{rel.TBit0, rel.TBitX, rel.TBitX} {
					if i == 1 && bit == 0 {
						value = rel.TBitX
					}
					if i == 1 && bit == 1 {
						value = rel.TBit0
					}
					if err = core.Manager().Set(vector, uint32(2-bit), value); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err = core.Insert(p); err != nil {
				t.Fatal(err)
			}
			if _, err = core.Insert(q); err != nil {
				t.Fatal(err)
			}
			r, err := core.Manager().AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			for bit, value := range []rel.TBit{rel.TBit0, rel.TBit0, rel.TBit1} {
				if err = core.Manager().Set(r, uint32(2-bit), value); err != nil {
					t.Fatal(err)
				}
			}
			budget.FailAfter(failure)
			_, insertErr := core.Insert(r)
			budget.FailAfter(-1)
			if failure == 0 && !errors.Is(insertErr, arena.ErrInjectedFailure) {
				t.Fatalf("insertion did not reach an injected allocation refusal: %v", insertErr)
			}
			if failure == 23 && insertErr != nil {
				t.Fatalf("high refusal threshold did not reach success: %v", insertErr)
			}
			if insertErr != nil && !errors.Is(insertErr, arena.ErrInjectedFailure) {
				t.Fatalf("unexpected insertion error: %v", insertErr)
			}
			if insertErr != nil {
				if _, err = core.Insert(r); err != nil {
					t.Fatalf("retry failed after %v: %v", insertErr, err)
				}
			}
			wellFormed, err := core.WellFormed()
			if err != nil || !wellFormed {
				t.Fatal("retry left an invalid intersection graph", wellFormed, err)
			}
			display, err := core.Display()
			if err != nil {
				t.Fatal(err)
			}
			want := "node[0: xxx 1 2]\nnode[1: 0xx 3]\nnode[2: x0x 3]\nnode[3: 00x 4]\nnode[4: 001]\n"
			if display != want {
				t.Fatalf("retry graph differs from uninterrupted insertion\n got %q\nwant %q", display, want)
			}
			if _, err = core.descendants(r); err != nil {
				t.Fatal("target missing after insertion", err)
			}
			core.Close()
			if used := budget.Used(); used != 0 {
				t.Fatal("insertion retry retained budget", used)
			}
		})
	}
}

func TestDDNFCoreDenseInsertionAllocationRefusalCanRetry(t *testing.T) {
	const budgetSize = uint64(1 << 20)
	patterns := []string{"0xxx", "x0xx", "xx0x", "xxx0", "0001"}
	build := func(t *testing.T, failure int64) (string, error) {
		t.Helper()
		budget := arena.NewBudget(budgetSize)
		config := arena.VectorConfig{PageSize: 1, Descriptors: arena.Config{PageSize: 1, Budget: budget}}
		core, err := NewCore(4, config)
		if err != nil {
			return "", err
		}
		defer core.Close()
		vectors := make([]*rel.TBV, len(patterns))
		for i, pattern := range patterns {
			vectors[i] = testPattern(t, core, pattern)
		}
		for i := 0; i < len(vectors)-1; i++ {
			if _, err = core.Insert(vectors[i]); err != nil {
				return "", err
			}
		}
		if failure >= 0 {
			budget.FailAfter(failure)
		}
		_, insertErr := core.Insert(vectors[len(vectors)-1])
		budget.FailAfter(-1)
		if insertErr != nil {
			if !errors.Is(insertErr, arena.ErrInjectedFailure) {
				return "", insertErr
			}
			if _, err = core.Insert(vectors[len(vectors)-1]); err != nil {
				return "", err
			}
		}
		return core.Display()
	}
	want, err := build(t, -1)
	if err != nil {
		t.Fatal(err)
	}
	for failure := int64(0); failure < 80; failure++ {
		got, buildErr := build(t, failure)
		if buildErr != nil {
			t.Fatal(failure, buildErr)
		}
		if got != want {
			t.Fatalf("failure %d retry graph differs\n got %q\nwant %q", failure, got, want)
		}
	}
}

func patternContains(parent, child string) bool {
	if len(parent) != len(child) {
		return false
	}
	for i := range parent {
		if parent[i] != 'x' && parent[i] != child[i] {
			return false
		}
	}
	return true
}

func TestDDNFCoreIndependentFiniteDescendantOracle(t *testing.T) {
	core := testCore(t, 2)
	patterns := []string{"xx", "00", "01", "0x", "10", "11", "1x", "x0", "x1"}
	nodes := make([]*Node, len(patterns))
	nodes[0] = core.root
	for i := 1; i < len(patterns); i++ {
		var err error
		nodes[i], err = core.Insert(testPattern(t, core, patterns[i]))
		if err != nil {
			t.Fatal(patterns[i], err)
		}
	}
	if core.Size() != uint32(len(patterns)) {
		t.Fatal("all ternary two-bit patterns should be materialized", core.Size())
	}
	wellFormed, err := core.WellFormed()
	if err != nil || !wellFormed {
		t.Fatal("finite graph", wellFormed, err)
	}
	for i, node := range nodes {
		descendants, lookupErr := core.descendants(node.vector)
		if lookupErr != nil {
			t.Fatal(patterns[i], lookupErr)
		}
		actual := make(map[uint32]bool, len(descendants))
		for _, descendant := range descendants {
			actual[descendant.id] = true
		}
		for j, candidate := range nodes {
			want := patternContains(patterns[i], patterns[j])
			if actual[candidate.id] != want {
				t.Fatalf("descendants of %s: candidate %s got %v want %v", patterns[i], patterns[j], actual[candidate.id], want)
			}
		}
	}
	if _, err = core.Insert(testPattern(t, core, "00")); !errors.Is(err, ErrCore) {
		t.Fatal("source lookup internalization freezes insertion", err)
	}
}

func TestDDNFCoreValidationAndClose(t *testing.T) {
	core := testCore(t, 1)
	vector := testPattern(t, core, "0")
	if _, err := core.Accumulate(vector, nil); !errors.Is(err, ErrCore) {
		t.Fatal("accumulation requires reset and an inserted vector", err)
	}
	if _, err := core.Insert(vector); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Accumulate(vector, nil); !errors.Is(err, ErrCore) {
		t.Fatal("accumulation requires ResetAccumulate", err)
	}
	core.Close()
	if _, err := core.Insert(vector); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed insertion", err)
	}
}

// Source: ddnf_mgr::insert, well_formed, and accumulate in
// src/muz/ddnf/ddnf.cpp. Reparenting preserves child order, while the
// accumulation worklist visits the last child first and shares marks with
// well_formed.
func TestDDNFCoreSourceReparentingAndAccumulationMarks(t *testing.T) {
	core := testCore(t, 2)
	var vectors []*rel.TBV
	for _, pattern := range []string{"00", "01", "10", "0x"} {
		vector := testPattern(t, core, pattern)
		vectors = append(vectors, vector)
		if _, err := core.Insert(vector); err != nil {
			t.Fatal(pattern, err)
		}
	}
	display, err := core.Display()
	if err != nil {
		t.Fatal(err)
	}
	const want = "node[0: xx 3 4]\nnode[1: 00]\nnode[2: 01]\nnode[3: 10]\nnode[4: 0x 1 2]\n"
	if display != want {
		t.Fatalf("reparented graph\n got %q\nwant %q", display, want)
	}
	core.ResetAccumulate()
	labels, err := core.Accumulate(core.root.vector, []uint32{99})
	if err != nil || fmt.Sprint(labels) != "[99 0 4 2 1 3]" {
		t.Fatal("source stack traversal", labels, err)
	}
	labels, err = core.Accumulate(vectors[3], labels)
	if err != nil || fmt.Sprint(labels) != "[99 0 4 2 1 3]" {
		t.Fatal("shared marks", labels, err)
	}
	core.ResetAccumulate()
	labels, err = core.Accumulate(vectors[3], nil)
	if err != nil || fmt.Sprint(labels) != "[4 2 1]" {
		t.Fatal("reset marks", labels, err)
	}
	well, err := core.WellFormed()
	if err != nil || !well {
		t.Fatal("well formed", well, err)
	}
	labels, err = core.Accumulate(core.root.vector, nil)
	if err != nil || len(labels) != 0 {
		t.Fatal("well_formed marks traversal", labels, err)
	}
}
