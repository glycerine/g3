package realclosure

import "testing"

func TestSignDeterminationSourceMatrices(t *testing.T) {
	cases := []struct {
		name                      string
		equal, positive, negative int
		rows                      []string
	}{
		{"zero-positive", 1, 2, 0, []string{"1,1", "0,1"}},
		{"zero-negative", 1, 0, 2, []string{"1,1", "0,-1"}},
		{"positive-negative", 0, 2, 1, []string{"1,1", "1,-1"}},
		{"all-signs", 1, 2, 3, []string{"1,1,1", "0,1,-1", "0,1,1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := testSignDetMatrix(tc.equal, tc.positive, tc.negative)
			if !ok || m.rows != len(tc.rows) || m.cols != len(tc.rows) {
				t.Fatalf("unexpected matrix shape: %dx%d, ok=%v", m.rows, m.cols, ok)
			}
			for i, want := range tc.rows {
				got := ""
				for j := 0; j < m.cols; j++ {
					if j != 0 {
						got += ","
					}
					got += m.cellString(i, j)
				}
				if got != want {
					t.Fatalf("row %d: got %s, want %s", i, got, want)
				}
			}
		})
	}
	for _, counts := range [][3]int{{0, 0, 0}, {4, 0, 0}, {0, 4, 0}, {0, 0, 4}, {-1, 2, 0}} {
		if _, ok := testSignDetMatrix(counts[0], counts[1], counts[2]); ok {
			t.Fatalf("nondiscriminating counts %v produced a matrix", counts)
		}
	}
}

func TestSignMatrixTensorAndCopyOwnCells(t *testing.T) {
	a, _ := testSignDetMatrix(1, 2, 0)
	b, _ := testSignDetMatrix(0, 2, 1)
	a.setCellPow2(0, 0, 70)
	var copied signMatrix
	mustSign(copied.copyFrom(&a))
	copied.setCell(0, 0, 7)
	if !a.isPow2Cell(0, 0, 70) {
		t.Fatal("matrix copy shared a big integer cell")
	}
	var product signMatrix
	mustSign(product.tensorProduct(&a, &b))
	if product.rows != 4 || product.cols != 4 || !cellsEqual(&product, 0, 0, &a, 0, 0) || product.cellInt64(1, 3) != -1 || product.cellInt64(3, 3) != -1 {
		t.Fatal("tensor product has incorrect row or column layout")
	}
	mustSign(a.tensorProduct(&a, &b))
	for i := range product.cells {
		if !cellsEqual(&a, i/a.cols, i%a.cols, &product, i/product.cols, i%product.cols) {
			t.Fatalf("aliased tensor product differs at cell %d", i)
		}
	}
	product.setCell(0, 0, 0)
	if !a.isPow2Cell(0, 0, 70) {
		t.Fatal("tensor products share a cell")
	}
}
