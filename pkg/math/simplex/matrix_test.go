package simplex

import (
	"bytes"
	"fmt"
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
	// math_misc_02-F3): sparse_matrix::add iterates [v2, v1].
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

// The expected texts below are what a harness linked against the reference
// build's libz3.a printed for sparse_matrix<mpq_ext> on the same operations
// (row_iterator and col_iterator order, column_size). AUDIT.md
// math_misc_02-F3: deleted row and column entries are reused LIFO and rows
// and columns compress only when fewer than half their slots are live.
func TestSparseMatrixFreeSlotReuseOrder(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	row := func(m *Matrix, r Row) string {
		t.Helper()
		e, err := m.Entries(r)
		must(err)
		var b bytes.Buffer
		for _, x := range e {
			fmt.Fprintf(&b, " v%d=%s", x.Variable, x.Coefficient.RatString())
		}
		return b.String()
	}
	col := func(m *Matrix, v uint32) string {
		var b bytes.Buffer
		for _, r := range m.ColumnRows(v) {
			fmt.Fprintf(&b, " r%d", r.ID())
		}
		fmt.Fprintf(&b, " size=%d", m.ColumnSize(v))
		return b.String()
	}
	check := func(m *Matrix, tag, got, want string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: got %q want %q", tag, got, want)
		}
		if !m.WellFormed() {
			t.Fatalf("%s: matrix not well formed", tag)
		}
	}
	{
		var m Matrix
		m.EnsureVariable(7)
		r0, r1, r2 := m.NewRow(), m.NewRow(), m.NewRow()
		must(m.AddRational(r0, rat(1, 1), 0))
		must(m.AddRational(r0, rat(1, 1), 1))
		must(m.AddRational(r0, rat(1, 1), 2))
		must(m.AddRational(r1, rat(1, 1), 1))
		must(m.AddRational(r2, rat(1, 1), 3))
		must(m.Add(r0, rat(-1, 1), r1))
		check(&m, "A1", row(&m, r0), " v0=1 v2=1")
		must(m.Add(r0, rat(1, 1), r2))
		check(&m, "A2", row(&m, r0), " v0=1 v3=1 v2=1")
	}
	{
		var m Matrix
		m.EnsureVariable(7)
		a, b, c, d := m.NewRow(), m.NewRow(), m.NewRow(), m.NewRow()
		must(m.AddRational(a, rat(1, 1), 0))
		must(m.AddRational(b, rat(2, 1), 0))
		must(m.AddRational(c, rat(3, 1), 0))
		must(m.AddRational(a, rat(1, 1), 1))
		must(m.AddRational(b, rat(1, 1), 1))
		must(m.AddRational(c, rat(1, 1), 1))
		check(&m, "B0", col(&m, 0), " r0 r1 r2 size=3")
		must(m.Delete(b))
		check(&m, "B1", col(&m, 0), " r0 r2 size=2")
		e := m.NewRow()
		if e.ID() != 1 {
			t.Fatal("B reuse row id", e.ID())
		}
		must(m.AddRational(d, rat(5, 1), 0))
		check(&m, "B2", col(&m, 0), " r0 r3 r2 size=3")
		must(m.AddRational(e, rat(7, 1), 1))
		must(m.AddRational(e, rat(8, 1), 2))
		must(m.AddRational(e, rat(9, 1), 0))
		check(&m, "B3", row(&m, e), " v2=8 v1=7 v0=9")
		check(&m, "B3 col v0", col(&m, 0), " r0 r3 r2 r1 size=4")
		check(&m, "B3 col v1", col(&m, 1), " r0 r1 r2 size=3")
	}
	{
		var m Matrix
		m.EnsureVariable(7)
		r, s := m.NewRow(), m.NewRow()
		for v := uint32(0); v < 4; v++ {
			must(m.AddRational(r, rat(1, 1), v))
		}
		must(m.AddRational(s, rat(-1, 1), 0))
		must(m.AddRational(s, rat(-1, 1), 1))
		must(m.AddRational(s, rat(-1, 1), 2))
		must(m.AddRational(s, rat(1, 1), 5))
		must(m.Add(r, rat(1, 1), s))
		check(&m, "C1", row(&m, r), " v5=1 v3=1")
		must(m.AddRational(r, rat(4, 1), 6))
		check(&m, "C2", row(&m, r), " v6=4 v5=1 v3=1")
		check(&m, "C2 col v5", col(&m, 5), " r1 r0 size=2")
		tr, u, w := m.NewRow(), m.NewRow(), m.NewRow()
		must(m.AddRational(tr, rat(1, 1), 7))
		must(m.AddRational(u, rat(1, 1), 7))
		must(m.AddRational(w, rat(1, 1), 7))
		must(m.Delete(tr))
		must(m.Delete(u))
		check(&m, "C3", col(&m, 7), " r4 size=1")
		x := m.NewRow()
		must(m.AddRational(x, rat(2, 1), 7))
		check(&m, "C4", col(&m, 7), " r4 r3 size=2")
		if x.ID() != 3 {
			t.Fatal("C4 x id", x.ID())
		}
	}
}
