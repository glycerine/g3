package ddnf

// Source: src/test/ddnf.cpp tst_ddnf1.

import (
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
