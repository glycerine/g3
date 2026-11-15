package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
)

func lpLUCheckText(matrix [][]float64, basis []uint32) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ignored header\nrows %d\ncolumns 27\n", len(matrix))
	for i, row := range matrix {
		fmt.Fprintf(&b, "row %d\n", i)
		for j, v := range row {
			if v != 0 {
				fmt.Fprintf(&b, "column %d value %.17g\n", j, v)
			}
		}
		b.WriteString("row_end\n")
	}
	b.WriteString("matrix_end\nbasis_start\n")
	for _, j := range basis {
		fmt.Fprintln(&b, j)
	}
	b.WriteString("basis_end\nvector_start\ninvalid ignored vector bytes\n")
	return b.String()
}

func TestLPLUCheckFileIndependentSolutions(t *testing.T) {
	rng := rand.New(rand.NewSource(281091))
	for world := 0; world < 100; world++ {
		n := 1 + world%8
		perm := rng.Perm(n)
		basis := make([]uint32, n)
		for j := range basis {
			basis[j] = uint32(perm[j])
		}
		a := make([][]float64, n)
		x := make([]float64, n)
		for j := range x {
			x[j] = float64(rng.Intn(19)-9) / 8
		}
		for i := range a {
			a[i] = make([]float64, 27)
			// Strict diagonal dominance provides independently nonsingular bases;
			// permutations ensure the driver cannot substitute an identity basis.
			for j := range basis {
				v := float64(rng.Intn(7) - 3)
				if i == j {
					v = float64(8*n + 1)
				}
				a[i][basis[j]] = v
				a[i][26] += v * x[j]
			}
		}
		name := lpMPSDriverFile(t, lpLUCheckText(a, basis))
		var out bytes.Buffer
		d := NewLUCheckFileDriver(name, &out)
		if err := d.Run(); err != nil {
			t.Fatal(world, err)
		}
		if !d.closed || d.file != nil || d.LU != nil || !reflect.DeepEqual(d.Basis, basis) || d.Settings.Stats.Factorizations != 1 {
			t.Fatal("source lifecycle", world, d)
		}
		for j, want := range x {
			if math.Abs(d.Column.Data[j]-want) > 1e-10 {
				t.Fatal(world, j, d.Column.Data, want)
			}
		}
		want := fmt.Sprintf("init matrix %d by 27\nreading basis\n", n)
		if out.String() != want {
			t.Fatal(world, out.String())
		}
		if d.Run() != ErrExactVectorPending || d.Close() != nil {
			t.Fatal("single use")
		}
	}
}

func TestLPLUCheckFileLiteralReadOrderAndAssertions(t *testing.T) {
	base := "ignored\nnot_rows 1\nnot_columns 27\nrow 0\ncolumn 0 value 2\ncolumn 26 value 6\nrow_end\nconsumed separator\nbasis_start\n0\nbasis_end" // no final newline
	for _, tc := range []struct {
		old, next string
		checked   bool
		report    string
	}{
		{"row 0", "arrow 0", false, "wrong row linearrow 0\n"},
		{"column 0 value 2", "column 0 value 2 trailing", true, ""},
		{"basis_start", "different marker", true, ""},
		{"consumed separator\n", "", true, ""},
	} {
		name := lpMPSDriverFile(t, strings.Replace(base, tc.old, tc.next, 1))
		var out bytes.Buffer
		d := NewLUCheckFileDriver(name, &out)
		err := d.Run()
		if tc.checked && sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(tc, err, out.String())
			}
		} else if tc.old == "consumed separator\n" {
			// read_rows consumes basis_start; read_basis then discards the first ID.
			if err == nil {
				t.Fatal("missing basis must not silently reconstruct IDs")
			}
		} else {
			if err != nil || d.Column.Data[0] != 3 {
				t.Fatal(tc, err, d.Column)
			}
		}
		if tc.report != "" && !strings.Contains(out.String(), tc.report) {
			t.Fatal(out.String())
		}
		if err = d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	d := NewLUCheckFileDriver(lpMPSDriverFile(t, base), io.Discard)
	if err := d.Run(); err != nil || d.Column.Data[0] != 3 {
		t.Fatal(err)
	}
}

func TestLPLUCheckFileMalformedAndTruncatedPrefixes(t *testing.T) {
	base := "header\nrows 1\ncolumns 27\nrow 0\ncolumn 0 value 2\ncolumn 26 value 6\nrow_end\nmatrix_end\nbasis_start\n0\nbasis_end\n"
	for _, text := range []string{
		"", "header\n", "header\nrows\n", "header\nrows bad\n", "header\nrows 2147483648\n",
		strings.Replace(base, "columns 27", "columns wrong", 1),
		strings.Replace(base, "row 0", "row bad", 1),
		strings.Replace(base, "column 0 value 2", "column bad value 2", 1),
		strings.Replace(base, "column 26 value 6", "column 27 value 6", 1),
		strings.Replace(base, "column 26 value 6", "bad", 1),
		strings.Replace(base, "\n0\nbasis_end", "\nbad\nbasis_end", 1),
		strings.Replace(base, "columns 27", "columns 26", 1),
	} {
		d := NewLUCheckFileDriver(lpMPSDriverFile(t, text), io.Discard)
		if err := d.Run(); err == nil {
			t.Fatal("accepted malformed input", text)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// The source reads values with atof, so a non-numeric value is 0
	// (AUDIT.md util_lp_04-F1).
	d := NewLUCheckFileDriver(lpMPSDriverFile(t, strings.Replace(base, "column 26 value 6", "column 26 value bad", 1)), io.Discard)
	if err := d.Run(); err != nil {
		t.Fatal("atof value", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	// Every complete-line prefix lacks an eventual required terminator and must
	// stop rather than reproduce source EOF loops or invalid token access.
	for at := 0; at < len(base); at++ {
		if at != 0 && base[at-1] != '\n' {
			continue
		}
		d := NewLUCheckFileDriver(lpMPSDriverFile(t, base[:at]), io.Discard)
		if err := d.Run(); err == nil {
			t.Fatal("accepted truncated input", at)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// Signed atoi-to-unsigned conversion is retained for valid negative integers.
	if got, err := luCheckIndex("-1"); err != nil || got != ^uint32(0) {
		t.Fatal(got, err)
	}
}

func TestLPLUCheckFileReportAndFileFailures(t *testing.T) {
	a := make([][]float64, 1)
	a[0] = make([]float64, 27)
	a[0][0] = 2
	a[0][26] = 6
	name := lpMPSDriverFile(t, lpLUCheckText(a, []uint32{0}))
	report := "init matrix 1 by 27\nreading basis\n"
	for at := 0; at < len(report); at++ {
		w := &lpResidualFailWriter{remaining: at}
		d := NewLUCheckFileDriver(name, lpResidualWriterOnly{w})
		if err := d.Run(); err == nil {
			t.Fatal(at)
		}
		if w.String() != report[:at] {
			t.Fatal(at, w.String())
		}
		if d.file == nil || d.LU != nil {
			t.Fatal("retained report prefix", at)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	missing := name + ".missing"
	var out bytes.Buffer
	d := NewLUCheckFileDriver(missing, &out)
	if err := d.Run(); !errors.Is(err, os.ErrNotExist) || out.String() != "cannot open file "+missing+"\n" {
		t.Fatal(err, out.String())
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	// A real closed file at the first report remains owned after read failure.
	d = NewLUCheckFileDriver(name, nil)
	w := &lpMPSDriverActionWriter{action: func(string) error { return d.file.Close() }}
	d.out = w
	if err := d.Run(); err == nil {
		t.Fatal("closed file read accepted")
	}
	// bufio may have already read the entire small file; either an actual read
	// or the mandatory explicit close reports the underlying closed descriptor.
	if d.file == nil {
		t.Fatal("closed descriptor owner lost")
	}
	if err := d.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	d.file = nil
	d.input = nil
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLPLUCheckFileFlushAndMissingReportFailures(t *testing.T) {
	a := make([][]float64, 1)
	a[0] = make([]float64, 27)
	a[0][0] = 2
	a[0][26] = 6
	name := lpMPSDriverFile(t, lpLUCheckText(a, []uint32{0}))
	for flush := 1; flush <= 2; flush++ {
		w := &lpMPSDriverActionWriter{failFlush: flush}
		d := NewLUCheckFileDriver(name, w)
		if err := d.Run(); err == nil || w.flushes != flush || d.file == nil || d.LU != nil {
			t.Fatal(flush, err, w.flushes)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	wrongName := lpMPSDriverFile(t, strings.Replace(lpLUCheckText(a, []uint32{0}), "row 0", "arrow 0", 1))
	wrongReport := "init matrix 1 by 27\nwrong row linearrow 0\nreading basis\n"
	for at := 0; at < len(wrongReport); at++ {
		w := &lpResidualFailWriter{remaining: at}
		d := NewLUCheckFileDriver(wrongName, lpResidualWriterOnly{w})
		if err := d.Run(); err != lpErrResidualWrite || w.String() != wrongReport[:at] {
			t.Fatal(at, err, w.String())
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
	missing := name + ".missing"
	report := "cannot open file " + missing + "\n"
	for at := 0; at < len(report); at++ {
		w := &lpResidualFailWriter{remaining: at}
		d := NewLUCheckFileDriver(missing, lpResidualWriterOnly{w})
		if err := d.Run(); err != lpErrResidualWrite || w.String() != report[:at] {
			t.Fatal(at, err, w.String())
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
