package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMPSLPSolverMapOrder: the owner's 2026-11-14 lp_solver task.
// mps_reader::fill_solver_on_rows walks m_rows and each row's
// m_row_columns, and lp_solver numbers core rows and columns by walking
// m_A_values and sums the cost over m_map_from_var_index_to_column_info,
// all in libstdc++ unordered_map order. On these degenerate LPs another
// order reaches a different optimal vertex (or rounds a value
// differently). The expected text is build/z3's output.
func TestMPSLPSolverMapOrder(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"lp_mps_solver_order_127.mps", "extended A[3,16]\nstatus is OPTIMAL\nobjective = 8\ncolumn values\nCBDB0 = 3\nCXAC12 = 0\nCYYZ1 = 0\nCDAC2 = 0\nCXYZ3 = 0\nCD4 = 0\nCCAD7 = 0\nCYX8 = 0\nCZ5 = 2\nCY6 = 0\nCZDY9 = 0\nCXAC11 = 0\nCAXA10 = 3\n"},
		{"lp_mps_solver_order_292.mps", "extended A[5,13]\nstatus is OPTIMAL\nobjective = 14.2222\ncolumn values\nCYD0 = 2\nCAY6 = 0\nCA1 = 1.66667\nCX2 = -3.73433e-16\nCAC5 = 5\nCBBYD3 = 0\nCZ4 = 2\nCBZB7 = 3.55556\n"},
		{"lp_mps_solver_order_336.mps", "extended A[3,15]\nstatus is OPTIMAL\nobjective = 16.5\ncolumn values\nCZD0 = 3\nCD1 = 0\nCAZD2 = 2\nCZYXA11 = 0\nCD3 = 0\nCXZCB4 = 4\nCCZB5 = 0\nCXXY6 = 1.5\nCD10 = 3\nCB7 = 0\nCDA8 = 0\nCDDBC9 = 3\n"},
	} {
		input := filepath.Join("test_vectors", tc.name)
		var out bytes.Buffer
		if err := runArguments([]string{input}, strings.NewReader(""), &out); err != nil {
			t.Fatal(tc.name, err, out.String())
		}
		if out.String() != tc.want {
			t.Fatalf("%s: g3 output\n%s\nwant\n%s", tc.name, out.String(), tc.want)
		}
		if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
			cpp, err := exec.Command(reference, input).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(cpp))
			}
			if !bytes.Equal(out.Bytes(), cpp) {
				t.Fatalf("%s: Go/C++ mismatch\nGo:\n%s\nC++:\n%s", tc.name, out.String(), cpp)
			}
		}
	}
}
