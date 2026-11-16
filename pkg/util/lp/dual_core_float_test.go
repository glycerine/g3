package lp

import (
	"math"
	"reflect"
	"testing"
)

func TestLPFloatDualCoreIndependentDualVectors(t *testing.T) {
	for world := 0; world < 100; world++ {
		in := lpFloatCoreInput(t, 3, 6)
		in.Settings.SimplexStrategy = StrategyLU
		basis := []uint32{2, 0, 4}
		*in.Basis = basis
		diagonals := []float64{2, -4, 8}
		y := []float64{float64(world-7) / 4, float64(world+1) / 8, -3}
		for i, j := range basis {
			lpStaticMust(t, in.A.Set(uint32(i), j, diagonals[i]))
			(*in.Costs)[j] = y[i] * diagonals[i]
		}
		for _, j := range []uint32{1, 3, 5} {
			for i := 0; i < 3; i++ {
				lpStaticMust(t, in.A.Set(uint32(i), j, float64(i+int(j)+1)/4))
			}
			(*in.Costs)[j] = float64(j)
		}
		can := []bool{false, world%2 == 0, false, true, false, false}
		m, err := NewFloatDualCoreSolver(in, &can)
		lpStaticMust(t, err)
		t.Cleanup(func() { m.Close() })
		for i, v := range m.Y {
			if math.Abs(v-y[i]) > 1e-12 {
				t.Fatal(world, i, v, y[i])
			}
		}
		lpStaticMust(t, m.RecalculateD())
		for _, j := range []uint32{1, 3, 5} {
			want := float64(j)
			for i, v := range y {
				want -= float64(v * float64(i+int(j)+1) / 4)
			}
			if math.Abs(m.D[j]-want) > 1e-12 {
				t.Fatal(world, j, m.D[j], want)
			}
		}
		if !reflect.DeepEqual(*in.NBasis, []uint32{3, 1}) && world%2 == 0 || !reflect.DeepEqual(*in.NBasis, []uint32{3}) && world%2 != 0 {
			t.Fatal(*in.NBasis)
		}
		// Source leaves excluded negative headings unchanged when rebuilding nbasis.
		old := (*in.Heading)[5]
		can = []bool{false, false, false, false, false, true}
		lpStaticMust(t, m.RestoreNonBasis())
		if !reflect.DeepEqual(*in.NBasis, []uint32{5}) || (*in.Heading)[5] != -1 || old >= 0 {
			t.Fatal(*in.NBasis)
		}
		m.AWave = []float64{7, 8, 9, 10}
		m.Betas = []float64{0, 0, 0, 10}
		lpStaticMust(t, m.InitAWaveByZeros())
		lpStaticMust(t, m.InitBetas())
		if !reflect.DeepEqual(m.AWave, []float64{0, 0, 0, 10}) || !reflect.DeepEqual(m.Betas, []float64{1, 1, 1, 10}) {
			t.Fatal(m.AWave, m.Betas)
		}
	}
}
func TestLPFloatDualCorePartialOwnersAndGuards(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 2)
	in.Settings.SimplexStrategy = StrategyLU
	lpStaticMust(t, in.A.Set(0, 0, 1))
	can := []bool{false, true}
	if m, err := NewFloatDualCoreSolver(in, nil); m != nil || err != ErrBounds {
		t.Fatal(m, err)
	}
	broken := in
	broken.A = nil
	if m, err := NewFloatDualCoreSolver(broken, &can); m != nil || err != ErrBounds {
		t.Fatal(m, err)
	}
	can = nil
	m, err := NewFloatDualCoreSolver(in, &can)
	if m == nil || err != ErrBounds || m.Factorization == nil || len(*m.NBasis) != 0 {
		t.Fatal(m, err)
	}
	m.Close()
	can = []bool{false, true}
	in.Settings.SimplexStrategy = StrategyTableauRows
	m, err = NewFloatDualCoreSolver(in, &can)
	if m == nil || err != ErrBounds || m.AWave == nil || m.Factorization != nil {
		t.Fatal(m, err)
	}
	m.Close()
	in.Settings.SimplexStrategy = StrategyLU
	*in.Costs = nil
	m, err = NewFloatDualCoreSolver(in, &can)
	if m == nil || err != ErrBounds || m.Factorization == nil {
		t.Fatal(m, err)
	}
	m.Close()
	*in.Costs = []float64{0, 0}
	*in.Basis = []uint32{9}
	m, err = NewFloatDualCoreSolver(in, &can)
	if m == nil || err != ErrBounds || m.AWave != nil {
		t.Fatal(m, err)
	}
	m.Close()
	*in.Basis = []uint32{0}
	m, err = NewFloatDualCoreSolver(in, &can)
	lpStaticMust(t, err)
	defer m.Close()
	*in.Heading = nil
	if err := m.RestoreNonBasis(); err != ErrBounds {
		t.Fatal(err)
	}
	m.AWave = nil
	if err := m.InitAWaveByZeros(); err != ErrBounds {
		t.Fatal(err)
	}
	m.Betas = nil
	if err := m.InitBetas(); err != ErrBounds {
		t.Fatal(err)
	}
	m.Factorization.Close()
	if err := m.RecalculateD(); err == nil {
		t.Fatal("closed factorization accepted")
	}
	if err := m.RecalculateXBAndD(); err != ErrBounds {
		t.Fatal(err)
	} // heading traversal precedes child solve
	if err := m.AdjustXBForChangedXNAndInitBetas(); err != ErrBounds {
		t.Fatal(err)
	}
}
