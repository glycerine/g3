//go:build g3debug

package lp

import (
	"bytes"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

const lpSmallLUReport = " test_small_lu\n" +
	"  1 0 3.9  0 0 -3 \n  0 4   0  0 7  0 \n1.8 0   5 11 2  8 \n" +
	"entering 2, leaving 0\nentering 4, leaving 3\nwe were factoring \n" +
	"3.9 0 0 \n  0 4 7 \n  5 0 2 \n\n" +
	"entering 5, leaving 1\nwe were factoring \n3.9 -3 0 \n  0  0 7 \n  5  8 2 \n\n" +
	"entering 3, leaving 2\nwe were factoring \n 0 -3 0 \n 0  0 7 \n11  8 2 \n\nhere\n"

func TestLPSmallLUDriverLiteralScenario(t *testing.T) {
	s, err := NewSettings()
	lpStaticMust(t, err)
	var out bytes.Buffer
	d := NewSmallLUTestDriver(s, &out)
	t.Cleanup(func() { lpStaticMust(t, d.Close()) })
	err = d.Run()
	if err != nil {
		t.Fatalf("source scenario: %v; pivots=%d extended=%v basis=%v report=%q", err, d.CompletedPivots, d.Extended, d.Basis, out.String())
	}
	if !d.closed || d.CompletedPivots != 4 || !d.Extended || !reflect.DeepEqual(d.Basis, []uint32{3, 5, 4, 6, 7}) {
		t.Fatal("source final state", d.Basis, d.CompletedPivots, d.Extended)
	}
	if out.String() != lpSmallLUReport || !reflect.DeepEqual(d.NBasis, []uint32{1, 2, 0}) || !reflect.DeepEqual(d.Heading, []int32{-3, -1, -2, 0, 2, 1, 3, 4}) {
		t.Fatal("source report/headings", out.String(), d.NBasis, d.Heading)
	}
	for i, row := range [][]float64{{1, 0, 3.9, 0, 0, -3, 0, 0}, {0, 4, 0, 0, 7, 0, 0, 0}, {1.8, 0, 5, 11, 2, 8, 0, 0}, {0, 1, 2, 3, 4, 5, 6, 7}, {0, 1, 4, 9, 16, 25, 36, 49}} {
		for j, want := range row {
			got, err := d.Matrix.Element(uint32(i), uint32(j))
			lpStaticMust(t, err)
			if got != want {
				t.Fatal("literal extended matrix", i, j, got, want)
			}
		}
	}
	if d.Run() != ErrExactVectorPending {
		t.Fatal("repeated scenario accepted")
	}
}

func TestLPSmallLUDriverIndependentExtendedSolves(t *testing.T) {
	// Independently form B*x from the literal final columns, then solve using
	// the factorization immediately before the source final correctness check.
	b := [5][5]float64{{0, -3, 0, 0, 0}, {0, 0, 7, 0, 0}, {11, 8, 2, 0, 0}, {3, 5, 4, 6, 7}, {9, 25, 16, 36, 49}}
	for world := 0; world < 30; world++ {
		s, err := NewSettings()
		lpStaticMust(t, err)
		s.DensityThreshold = []float64{0, .7, math.Inf(1)}[world%3]
		var d *SmallLUTestDriver
		checked := false
		w := &lpMPSDriverActionWriter{action: func(text string) error {
			if text != "here\n" {
				return nil
			}
			var x, rhs [5]float64
			for j := range x {
				x[j] = float64((world+3*j)%13-6) / 4
			}
			for i := range rhs {
				for j := range x {
					rhs[i] += float64(b[i][j] * x[j])
				}
			}
			if err := d.LU.SolveBy(rhs[:]); err != nil {
				return err
			}
			for i := range x {
				if math.Abs(rhs[i]-x[i]) > 1e-9 {
					t.Fatal("independent final B solve", world, i, rhs[i], x[i])
				}
			}
			checked = true
			return nil
		}}
		d = NewSmallLUTestDriver(s, w)
		lpStaticMust(t, d.Run())
		lpStaticMust(t, d.Close())
		if !checked || w.buffer.String() != lpSmallLUReport || w.flushes != 24 {
			t.Fatal("actual extended check/report/endl", world, checked, w.flushes)
		}
	}
}

func TestLPSmallLUDriverReportFailurePrefixes(t *testing.T) {
	s, err := NewSettings()
	lpStaticMust(t, err)
	for at := 0; at < len(lpSmallLUReport); at++ {
		w := &larConstraintPrefixWriter{remaining: at, err: lpErrResidualWrite}
		d := NewSmallLUTestDriver(s, w)
		if err := d.Run(); err != lpErrResidualWrite || w.output.String() != lpSmallLUReport[:at] {
			t.Fatal("report prefix", at, err, w.output.String())
		}
		want := uint32(0)
		for _, line := range []string{"entering 2, leaving 0\n", "entering 4, leaving 3\n", "entering 5, leaving 1\n", "entering 3, leaving 2\n"} {
			if at >= strings.Index(lpSmallLUReport, line)+len(line) {
				want++
			}
		}
		if d.CompletedPivots != want || d.Extended != (at >= strings.Index(lpSmallLUReport, "here\n")) || d.closed {
			t.Fatal("algorithm publication before failed output", at, d.CompletedPivots, want, d.Extended)
		}
		lpStaticMust(t, d.Close())
		lpStaticMust(t, d.Close())
	}
	for at := 1; at <= 24; at++ {
		w := &lpDriverFlushWriter{failAt: at}
		d := NewSmallLUTestDriver(s, w)
		if err := d.Run(); err != lpErrResidualWrite || w.flushes != at || !strings.HasPrefix(lpSmallLUReport, w.String()) {
			t.Fatal("endl failure", at, err, w.flushes, w.String())
		}
		lpStaticMust(t, d.Close())
	}
	// Short nil-error writes must stop before constructing any numerical child.
	w := &larConstraintPrefixWriter{remaining: 3}
	d := NewSmallLUTestDriver(s, w)
	if err := d.Run(); err != io.ErrShortWrite || d.Matrix != nil || d.LU != nil {
		t.Fatal("short initial report", err)
	}
	lpStaticMust(t, d.Close())
	d = NewSmallLUTestDriver(nil, io.Discard)
	if err := d.Run(); err != ErrBounds || d.Matrix == nil || d.LU != nil {
		t.Fatal("nil settings guard", err)
	}
	lpStaticMust(t, d.Close())
}
