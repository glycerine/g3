package rel

import "testing"

// The source implements these operations on the two-bit possibility encoding;
// its column containment has a distinct rule for X and otherwise tests equality.
func TestTBVSourceSingleBitTruthTables(t *testing.T) {
	m, _ := newTestTBVManager(t, 1)
	left, err := m.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	right, err := m.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	out, err := m.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	for a := TBitZ; a <= TBitX; a++ {
		for b := TBitZ; b <= TBitX; b++ {
			if err := m.Set(left, 0, a); err != nil {
				t.Fatal(err)
			}
			if err := m.Set(right, 0, b); err != nil {
				t.Fatal(err)
			}
			if err := m.Copy(out, left); err != nil {
				t.Fatal(err)
			}
			valid, err := m.SetAnd(out, right)
			if err != nil {
				t.Fatal(err)
			}
			got, err := m.Get(out, 0)
			if err != nil || got != a&b || valid != (a&b != TBitZ) {
				t.Fatalf("%d & %d: %d valid=%v err=%v", a, b, got, valid, err)
			}
			if err := m.Copy(out, left); err != nil {
				t.Fatal(err)
			}
			if err := m.SetOr(out, right); err != nil {
				t.Fatal(err)
			}
			got, err = m.Get(out, 0)
			if err != nil || got != a|b {
				t.Fatalf("%d | %d: %d err=%v", a, b, got, err)
			}
			contains, err := m.Contains(left, right)
			if err != nil || contains != (a&b == b) {
				t.Fatalf("contains %d, %d: %v err=%v", a, b, contains, err)
			}
			columns, err := m.ContainsColumns(left, []uint32{0}, right, []uint32{0})
			if err != nil || columns != (a == TBitX || a == b) {
				t.Fatalf("column contains %d, %d: %v err=%v", a, b, columns, err)
			}
		}
	}
}

func TestTBVSourceComplementOrderAndUnknownBits(t *testing.T) {
	m, _ := newTestTBVManager(t, 4)
	v, err := m.AllocateString("01x0")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Set(v, 2, TBitZ); err != nil {
		t.Fatal(err)
	}
	result, err := m.Complement(v)
	if err != nil {
		t.Fatal(err)
	}
	// Definite positions 0, 1 and 3 are flipped in ascending index order;
	// the undefined position 2 is skipped, as in tbv_manager::complement.
	want := []string{"0z11", "0z00", "1z10"}
	if len(result) != len(want) {
		t.Fatalf("got %d complements", len(result))
	}
	for i, cube := range result {
		if got := tbvString(t, m, cube); got != want[i] {
			t.Fatalf("complement %d: got %q want %q", i, got, want[i])
		}
		if err := m.Deallocate(cube); err != nil {
			t.Fatal(err)
		}
	}
	if got := tbvString(t, m, v); got != "0z10" {
		t.Fatalf("source mutated: %q", got)
	}
}
