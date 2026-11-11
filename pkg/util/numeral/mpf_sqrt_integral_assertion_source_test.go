package numeral

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatSqrtIntegralEntrySourceAssertions(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertions only")
	}
	for _, tc := range []struct {
		name, line string
		call       func(*ArbitraryFloatManager, *ArbitraryFloat, *ArbitraryFloat) error
	}{
		{"sqrt", "1006", func(m *ArbitraryFloatManager, x, out *ArbitraryFloat) error {
			return m.Sqrt(ArbitraryFloatNearestEven, x, out)
		}},
		{"round integral", "1048", func(m *ArbitraryFloatManager, x, out *ArbitraryFloat) error {
			return m.RoundToIntegral(ArbitraryFloatNearestEven, x, out)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, log := bindMPNDebugActions(t, "T")
			m, _, _ := mpfNumeralHelpersFixture(t)
			var invalid, valid, out ArbitraryFloat
			serviceMust(t, m.SetDouble(&valid, 11, 53, 4))
			serviceMust(t, m.SetDouble(&out, 11, 53, 9))
			before := out
			if e := tc.call(m, &invalid, &out); !isMPNVerificationThrow(e) || out != before {
				t.Fatal("Throw before trace and output publication", e, out)
			}
			contents, e := os.ReadFile(log.Name())
			serviceMust(t, e)
			if n := strings.Count(string(contents), "File: src/util/mpf.cpp\nLine: "+tc.line+"\nx.ebits > 0 && x.sbits > 0\n"); n != 1 {
				t.Fatal("source entry assertion site", n, string(contents))
			}
			serviceMust(t, d.Close())
			if e := tc.call(m, &valid, &out); !errors.Is(e, arena.ErrClosed) || out != before {
				t.Fatal("valid input reads closed assertion owner", e, out)
			}
			serviceMust(t, m.Del(&valid))
			serviceMust(t, m.Del(&out))
		})
	}
}

func TestArbitraryFloatRoundIntegralModeSourceActions(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertion only")
	}
	d, log := bindMPNDebugActions(t, "TC")
	m, _, _ := mpfNumeralHelpersFixture(t)
	var x, out ArbitraryFloat
	serviceMust(t, m.SetDouble(&x, 11, 53, .75))
	serviceMust(t, m.SetDouble(&out, 11, 53, 9))
	before := out
	invalid := ArbitraryFloatRoundingMode(255)
	if e := m.RoundToIntegral(invalid, &x, &out); !isMPNVerificationThrow(e) || out != before {
		t.Fatal("Throw before small-magnitude mode body", e, out)
	}
	checkContinued := func(label string) {
		t.Helper()
		serviceMust(t, m.RoundToIntegral(invalid, &x, &out))
		v, e := m.ToDouble(&out)
		if e != nil || v != 1 {
			t.Fatal(label, "source default rounding branch", v, e)
		}
	}
	checkContinued("Continue")
	contents, e := os.ReadFile(log.Name())
	serviceMust(t, e)
	if n := strings.Count(string(contents), "File: src/util/mpf.cpp\nLine: 1074\nrm == MPF_ROUND_NEAREST_TEVEN || rm == MPF_ROUND_NEAREST_TAWAY\n"); n != 2 {
		t.Fatal("source mode assertion site and action count", n, string(contents))
	}
	serviceMust(t, d.EnableAssertions(false))
	checkContinued("disabled")
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Del(&x))
}

func TestArbitraryFloatRoundIntegralUnpackedBoundsSourceActions(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertions only")
	}
	for _, tc := range []struct {
		name       string
		sig        int32
		line       string
		expression string
	}{
		{"above width", 8, "1102", "m_mpz_manager.lt(a.significand(), m_powers2(x.sbits))"},
		{"below hidden bit", -1, "1103", "m_mpz_manager.ge(a.significand(), m_powers2(x.sbits - 1))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, log := bindMPNDebugActions(t, "T")
			m, _, _ := mpfNumeralHelpersFixture(t)
			var x, out ArbitraryFloat
			sig := NewInt(tc.sig)
			serviceMust(t, m.SetComponents(&x, 11, 4, false, 0, &sig))
			serviceMust(t, m.SetDouble(&out, 11, 4, 3))
			if e := m.RoundToIntegral(ArbitraryFloatNearestEven, &x, &out); !isMPNVerificationThrow(e) {
				t.Fatal("unpacked significand source Throw", e)
			}
			contents, e := os.ReadFile(log.Name())
			serviceMust(t, e)
			if n := strings.Count(string(contents), "File: src/util/mpf.cpp\nLine: "+tc.line+"\n"+tc.expression+"\n"); n != 1 {
				t.Fatal("source unpacked significand assertion site", n, string(contents))
			}
			serviceMust(t, m.Del(&out))
			serviceMust(t, m.Del(&x))
		})
	}
}
