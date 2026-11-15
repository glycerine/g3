package rel

import (
	"reflect"
	"testing"
)

func bitvectorTestSignature(t *testing.T, domains ...uint64) Signature {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

// AUDIT.md muz_fp_03-F5: bitvector_table_plugin::can_handle_signature takes
// power-of-two columns whose bit counts (log2 s + 1 each) sum below 32.
func TestBitvectorCanHandleSignature(t *testing.T) {
	for _, tc := range []struct {
		domains    []uint64
		functional int
		want       bool
	}{
		{nil, 0, true},
		{[]uint64{2}, 0, true},
		{[]uint64{16, 16}, 0, true},
		{[]uint64{1 << 16}, 0, true},           // 17 bits
		{[]uint64{1 << 16, 8}, 0, true},        // 17 + 4 bits
		{[]uint64{1 << 16, 1 << 16}, 0, false}, // 34 bits
		{[]uint64{1 << 30}, 0, true},           // 31 bits
		{[]uint64{1 << 31}, 0, false},          // 32 bits
		{[]uint64{1 << 32}, 0, false},          // not an unsigned
		{[]uint64{3}, 0, false},                // not a power of two
		{[]uint64{4, 4}, 1, false},             // functional columns
	} {
		if got := BitvectorCanHandleSignature(Signature{domains: tc.domains}, tc.functional); got != tc.want {
			t.Fatal(tc.domains, tc.functional, got, tc.want)
		}
	}
	if _, err := NewBitvectorTable(Signature{domains: []uint64{3}}); err == nil || err.Error() != bitvectorTableSmallDomains {
		t.Fatal("constructor accepted a non-power-of-two domain", err)
	}
	if _, err := NewBitvectorTable(Signature{domains: []uint64{1 << 16, 1 << 16}}); err == nil || err.Error() != bitvectorTableSmallDomains {
		t.Fatal("constructor accepted a 34-bit offset", err)
	}
}

// bv_iterator visits increasing offsets: the last column is most
// significant, so rows are ordered by (column 1, column 0).
func TestBitvectorTableIterationOrderAndUpdates(t *testing.T) {
	table, err := NewBitvectorTable(bitvectorTestSignature(t, 16, 16))
	if err != nil {
		t.Fatal(err)
	}
	if table.bv.Size() != 1<<10 {
		t.Fatal("reserved bits", table.bv.Size())
	}
	for _, fact := range []Fact{{9, 3}, {3, 7}, {7, 1}, {1, 12}, {12, 5}, {9, 3}} {
		if _, err := table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{7, 1}, {9, 3}, {12, 5}, {3, 7}, {1, 12}}
	if !reflect.DeepEqual(facts, want) {
		t.Fatal(facts)
	}
	if n, _ := table.Len(); n != 5 {
		t.Fatal("size", n)
	}
	if removed, err := table.Remove(Fact{12, 5}); err != nil || !removed {
		t.Fatal(removed, err)
	}
	if removed, err := table.Remove(Fact{12, 5}); err != nil || removed {
		t.Fatal(removed, err)
	}
	if found, err := table.Contains(Fact{12, 5}); err != nil || found {
		t.Fatal(found, err)
	}
	if _, err := table.Add(Fact{16, 0}); err == nil {
		t.Fatal("accepted a value outside its column domain")
	}
	clone, err := table.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = table.Reset(); err != nil {
		t.Fatal(err)
	}
	if n, _ := table.Len(); n != 0 {
		t.Fatal("reset size", n)
	}
	facts, _ = clone.Facts()
	if !reflect.DeepEqual(facts, []Fact{{7, 1}, {9, 3}, {3, 7}, {1, 12}}) {
		t.Fatal("clone", facts)
	}
	renamed, err := clone.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	facts, _ = renamed.Facts()
	if !reflect.DeepEqual(facts, []Fact{{12, 1}, {7, 3}, {1, 7}, {3, 9}}) {
		t.Fatal("rename", facts)
	}
	projected, err := finiteTableProject(clone, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := projected.(*BitvectorTable); !ok {
		t.Fatalf("projection plugin %T", projected)
	}
	facts, _ = projected.Facts()
	if !reflect.DeepEqual(facts, []Fact{{1}, {3}, {7}, {9}}) {
		t.Fatal("project", facts)
	}
}

// FORK FIX #56: the nullary table reserves one bit.
func TestBitvectorTableNullary(t *testing.T) {
	table, err := NewBitvectorTable(bitvectorTestSignature(t))
	if err != nil {
		t.Fatal(err)
	}
	if table.bv.Size() != 1 {
		t.Fatal("nullary bits", table.bv.Size())
	}
	if added, err := table.Add(Fact{}); err != nil || !added {
		t.Fatal(added, err)
	}
	facts, _ := table.Facts()
	if len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatal(facts)
	}
}

// default_table_join_fn keeps the first operand's plugin when it handles the
// result, then the second's, then the manager's appropriate plugin (sparse).
func TestBitvectorJoinResultPlugin(t *testing.T) {
	small, err := NewBitvectorTable(bitvectorTestSignature(t, 16, 16))
	if err != nil {
		t.Fatal(err)
	}
	wide, err := NewBitvectorTable(bitvectorTestSignature(t, 1<<8, 1<<8))
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{2, 1}, {1, 3}} {
		if _, err = small.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, fact := range []Fact{{200, 3}, {3, 100}} {
		if _, err = wide.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := finiteTableJoin(small, small, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := joined.(*BitvectorTable); !ok {
		t.Fatalf("20-bit join result %T", joined)
	}
	joined, err = finiteTableJoin(wide, wide, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := joined.(*SparseTable); !ok {
		t.Fatalf("36-bit join result %T", joined)
	}
	facts, _ := joined.Facts()
	if !reflect.DeepEqual(facts, []Fact{{200, 3, 3, 100}}) {
		t.Fatal(facts)
	}
}

// AUDIT.md muz_fp_01-F6: check_table pairs with a bitvector side. The pair
// handles a signature only when both plugins do, and bitvector_table takes
// the nullary signature that sparse rejects.
func TestFiniteCheckedBitvectorPair(t *testing.T) {
	for _, tc := range []struct {
		primary, checker string
		nullary          bool
	}{
		{"bitvector", "sparse", false},
		{"sparse", "bitvector", false},
		{"bitvector", "hashtable", true},
		{"bitvector", "bitvector", true},
	} {
		t.Run(tc.primary+"-"+tc.checker, func(t *testing.T) {
			factory, err := NewFiniteCheckedFactory(tc.primary, tc.checker)
			if err != nil {
				t.Fatal(err)
			}
			if got := factory.CanHandleSignature(bitvectorTestSignature(t)); got != tc.nullary {
				t.Fatal("nullary", got)
			}
			if factory.CanHandleSignature(bitvectorTestSignature(t, 1<<16, 1<<16)) {
				t.Fatal("34-bit signature accepted")
			}
			pair, err := factory.New(bitvectorTestSignature(t, 4, 4))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = pair.Close() }()
			for _, fact := range []Fact{{3, 2}, {1, 0}, {2, 2}} {
				if _, err = pair.Add(fact); err != nil {
					t.Fatal(err)
				}
			}
			primary, checker, err := pair.ConcreteKinds()
			if err != nil || primary != tc.primary || checker != tc.checker {
				t.Fatal(primary, checker, err)
			}
			facts, err := pair.Facts()
			if err != nil {
				t.Fatal(err)
			}
			want := []Fact{{3, 2}, {1, 0}, {2, 2}}
			if tc.primary == "bitvector" {
				want = []Fact{{1, 0}, {2, 2}, {3, 2}}
			}
			if !reflect.DeepEqual(facts, want) {
				t.Fatal(facts)
			}
		})
	}
}
