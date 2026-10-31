package numeral

// Source: src/test/f2n.cpp tst1 and tst2.
import "testing"

func TestFloatNumeralCPPFloorCeilRegression(t *testing.T) {
	rows := []struct {
		n, d        int32
		floor, ceil float64
	}{
		{11, 3, 3, 4}, {-11, 3, -4, -3}, {11, 1, 11, 11},
	}
	t.Run("hardware", func(t *testing.T) {
		m, err := NewHardwareFloatNumeralManager(NewHardwareFloatManager(), 11, 53)
		if err != nil {
			t.Fatal(err)
		}
		var a, b HardwareFloat
		for _, row := range rows {
			if err := m.SetRatioInt32(&a, row.n, row.d); err != nil {
				t.Fatal(err)
			}
			if err := m.Floor(&a, &b); err != nil {
				t.Fatal(err)
			}
			if got := m.ToFloat64(&b); got != row.floor {
				t.Fatalf("floor(%d/%d) = %g, want %g", row.n, row.d, got, row.floor)
			}
			if err := m.Ceil(&a, &b); err != nil {
				t.Fatal(err)
			}
			if got := m.ToFloat64(&b); got != row.ceil {
				t.Fatalf("ceil(%d/%d) = %g, want %g", row.n, row.d, got, row.ceil)
			}
		}
	})
	t.Run("arbitrary", func(t *testing.T) {
		core, _, _ := mpfNumeralHelpersFixture(t)
		m, err := NewArbitraryFloatNumeralManager(core, 11, 53)
		if err != nil {
			t.Fatal(err)
		}
		var a, b ArbitraryFloat
		defer func() {
			if err := core.Del(&a); err != nil {
				t.Error(err)
			}
			if err := core.Del(&b); err != nil {
				t.Error(err)
			}
			if err := m.Close(); err != nil {
				t.Error(err)
			}
		}()
		for _, row := range rows {
			if err := m.SetFractionSigned(&a, row.n, row.d); err != nil {
				t.Fatal(err)
			}
			if err := m.Floor(&a, &b); err != nil {
				t.Fatal(err)
			}
			if got, err := core.ToDouble(&b); err != nil || got != row.floor {
				t.Fatalf("floor(%d/%d) = %g, %v; want %g", row.n, row.d, got, err, row.floor)
			}
			if err := m.Ceil(&a, &b); err != nil {
				t.Fatal(err)
			}
			if got, err := core.ToDouble(&b); err != nil || got != row.ceil {
				t.Fatalf("ceil(%d/%d) = %g, %v; want %g", row.n, row.d, got, err, row.ceil)
			}
		}
	})
}
