package simplex

import (
	"bytes"
	"math/big"
	"testing"
)

func rat(n, d int64) *big.Rat { return big.NewRat(n, d) }

func TestSparseMatrixRowArithmeticColumnsAndReuse(t *testing.T) {
	var m Matrix
	m.EnsureVariable(3)
	r0, r1 := m.NewRow(), m.NewRow()
	if err := m.AddRational(r0, rat(2, 1), 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(r0, rat(3, 1), 1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(r1, rat(-2, 1), 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(r1, rat(5, 1), 2); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(r0, rat(1, 1), r1); err != nil {
		t.Fatal(err)
	}
	e, err := m.Entries(r0)
	if err != nil {
		t.Fatal(err)
	}
	// v0 cancels and frees slot 0, which v2 then reuses (AUDIT.md
	// math_misc_02-F3: sparse_matrix free-list slot order).
	if len(e) != 2 || e[0].Variable != 2 || e[0].Coefficient.Cmp(rat(5, 1)) != 0 || e[1].Variable != 1 || e[1].Coefficient.Cmp(rat(3, 1)) != 0 {
		t.Fatal("row addition/order", e)
	}
	if m.ColumnSize(0) != 1 || m.ColumnSize(1) != 1 || m.ColumnSize(2) != 2 || !m.WellFormed() {
		t.Fatal("column links", m.ColumnRows(2))
	}
	if err = m.Delete(r1); err != nil {
		t.Fatal(err)
	}
	if next := m.NewRow(); next != r1 {
		t.Fatal("row id was not recycled", next, r1)
	}
}

func TestSparseMatrixRationalGCDNormalizeMultiplyAndDisplay(t *testing.T) {
	var m Matrix
	r := m.NewRow()
	if err := m.AddRational(r, rat(6, 5), 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(r, rat(-9, 10), 1); err != nil {
		t.Fatal(err)
	}
	g, err := m.GCDNormalize(r)
	if err != nil {
		t.Fatal(err)
	}
	if g.Cmp(rat(3, 10)) != 0 {
		t.Fatal("gcd", g)
	}
	e, _ := m.Entries(r)
	if e[0].Coefficient.Cmp(rat(4, 1)) != 0 || e[1].Coefficient.Cmp(rat(-3, 1)) != 0 {
		t.Fatal("normalized", e)
	}
	if err = m.Multiply(r, rat(-2, 1)); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = m.Display(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "-8*v0 6*v1 \n" {
		t.Fatal("display", out.String())
	}
	if m.AddRowCount() != 0 || !m.WellFormed() {
		t.Fatal("matrix accounting")
	}
}

func TestSparseMatrixRationalGCDScansPastUnitPrefix(t *testing.T) {
	var m Matrix
	r := m.NewRow()
	if err := m.AddRational(r, rat(1, 1), 0); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(r, rat(1, 2), 1); err != nil {
		t.Fatal(err)
	}
	g, err := m.GCDNormalize(r)
	if err != nil {
		t.Fatal(err)
	}
	if g.Cmp(rat(1, 2)) != 0 {
		t.Fatal("gcd", g)
	}
	entries, _ := m.Entries(r)
	if entries[0].Coefficient.Cmp(rat(2, 1)) != 0 || entries[1].Coefficient.Cmp(rat(1, 1)) != 0 {
		t.Fatal("normalized", entries)
	}
}

func TestSparseMatrixIntegerEntryAndCancellation(t *testing.T) {
	var m Matrix
	r0, r1 := m.NewRow(), m.NewRow()
	if err := m.AddInteger(r0, big.NewInt(7), 4); err != nil {
		t.Fatal(err)
	}
	if err := m.AddInteger(r1, big.NewInt(7), 4); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(r0, rat(-1, 1), r1); err != nil {
		t.Fatal(err)
	}
	e, err := m.Entries(r0)
	if err != nil || len(e) != 0 {
		t.Fatal("cancellation", e, err)
	}
	if m.ColumnSize(4) != 1 || m.AddRowCount() != 1 || !m.WellFormed() {
		t.Fatal("post-cancellation links")
	}
}

// AUDIT.md math_misc_02-F3: a deleted row or column entry is reused LIFO by
// the next insertion, so iteration follows the source slot order.
func TestSparseMatrixFreeSlotReuseOrder(t *testing.T) {
	var m Matrix
	m.EnsureVariable(5)
	dst, minusV1, plusV3 := m.NewRow(), m.NewRow(), m.NewRow()
	for _, v := range []uint32{0, 1, 2} {
		if err := m.AddRational(dst, rat(1, 1), v); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.AddRational(minusV1, rat(1, 1), 1); err != nil {
		t.Fatal(err)
	}
	if err := m.AddRational(plusV3, rat(1, 1), 3); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(dst, rat(-1, 1), minusV1); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(dst, rat(1, 1), plusV3); err != nil {
		t.Fatal(err)
	}
	e, err := m.Entries(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 3 || e[0].Variable != 0 || e[1].Variable != 3 || e[2].Variable != 2 {
		t.Fatalf("row order = %v, want [v0 v3 v2]", e)
	}
	// Column v4 holds rows a, b, c; deleting b frees slot 1, which the
	// next row with v4 reuses.
	var c Matrix
	c.EnsureVariable(4)
	var rows []Row
	for i := 0; i < 3; i++ {
		r := c.NewRow()
		if err := c.AddRational(r, rat(1, 1), 4); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	if err := c.Delete(rows[1]); err != nil {
		t.Fatal(err)
	}
	extra := c.NewRow()
	if extra != rows[1] {
		t.Fatal("row id not recycled")
	}
	other := c.NewRow()
	if err := c.AddRational(other, rat(1, 1), 4); err != nil {
		t.Fatal(err)
	}
	got := c.ColumnRows(4)
	if len(got) != 3 || got[0] != rows[0] || got[1] != other || got[2] != rows[2] || !c.WellFormed() {
		t.Fatalf("column order = %v, want [%v %v %v]", got, rows[0], other, rows[2])
	}
}
