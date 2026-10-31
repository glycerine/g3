package ddnf

// Source: src/test/ddnf.cpp tst_ddnf1.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDDNFSourceTwoBitInsertion(t *testing.T) {
	core := testCore(t, 2)
	var vectors []*rel.TBV
	for _, pattern := range []string{"xx", "11", "x1", "1x"} {
		vector := testPattern(t, core, pattern)
		vectors = append(vectors, vector)
		if _, err := core.Insert(vector); err != nil {
			t.Fatal(err)
		}
	}
	if well, err := core.WellFormed(); err != nil || !well {
		t.Fatal("DDNF invariant", well, err)
	}
	for _, vector := range vectors {
		if found, err := core.Contains(vector); err != nil || !found {
			t.Fatal("missing source vector", found, err)
		}
	}
	if size := core.Size(); size != 4 {
		t.Fatal("source insertion node count", size)
	}
}

func TestDDNFSourceForwardingRefinement(t *testing.T) {
	// Two routing tables, each ending with the source catch-all rule.
	const input = "2 2\n3 2\n1 1x\n2 x1\n0 xx\n3 2\n1 x1\n2 1x\n0 xx\n"
	core := testCore(t, 2)
	parse := func(consume func(uint32, *rel.TBV) error, reset func()) {
		t.Helper()
		reader := strings.NewReader(input)
		var width, routers uint32
		if _, err := fmt.Fscan(reader, &width, &routers); err != nil || width != 2 || routers != 2 {
			t.Fatal("routing header", width, routers, err)
		}
		for router := uint32(0); router < routers; router++ {
			var count, maxPort uint32
			if _, err := fmt.Fscan(reader, &count, &maxPort); err != nil {
				t.Fatal(err)
			}
			reset()
			for i := uint32(0); i < count; i++ {
				var port uint32
				var pattern string
				if _, err := fmt.Fscan(reader, &port, &pattern); err != nil || port > maxPort {
					t.Fatal("routing row", port, pattern, err)
				}
				vector := testPattern(t, core, pattern)
				if err := consume(port, vector); err != nil {
					t.Fatal(err)
				}
				if port == 0 && pattern == "xx" {
					break
				}
			}
		}
	}
	parse(func(_ uint32, v *rel.TBV) error { _, err := core.Insert(v); return err }, func() {})
	var indices [][]uint32
	parse(func(port uint32, v *rel.TBV) error {
		labels, err := core.Accumulate(v, nil)
		if err != nil {
			return err
		}
		for _, label := range labels {
			indices[len(indices)-1][label] = port
		}
		return nil
	}, func() { core.ResetAccumulate(); indices = append(indices, make([]uint32, core.Size())) })
	classes := make([]uint32, core.Size())
	count := uint32(1)
	for _, forwarding := range indices {
		type pair struct{ prior, next uint32 }
		seen := make(map[pair]uint32)
		count = 0
		for i := range classes {
			key := pair{classes[i], forwarding[i]}
			value, ok := seen[key]
			if !ok {
				value = count
				count++
				seen[key] = value
			}
			classes[i] = value
		}
	}
	if count != 4 {
		t.Fatal("forwarding class count", count, indices, classes)
	}
	for _, tc := range []struct {
		pattern       string
		first, second uint32
	}{{"xx", 0, 0}, {"1x", 1, 2}, {"x1", 2, 1}, {"11", 1, 1}} {
		vector := testPattern(t, core, tc.pattern)
		node, err := core.find(vector)
		if err != nil {
			t.Fatal(err)
		}
		if indices[0][node.ID()] != tc.first || indices[1][node.ID()] != tc.second {
			t.Fatal("forwarding", tc, indices)
		}
	}
}
