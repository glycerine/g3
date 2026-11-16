package numeral

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatRationalSetNormalizedSourceActions(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertion only")
	}
	for _, tc := range []struct {
		name string
		sig  Rational
	}{
		{"below one", Rational{Num: NewInt(1), Den: NewInt(2)}},
		{"at two", Rational{Num: NewInt(2), Den: NewInt(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, log := bindMPNDebugActions(t, "TC")
			b := arena.NewBudget(1 << 25)
			m, e := NewArbitraryFloatManager(serviceConfig(b).Integers)
			serviceMust(t, e)
			call := func() error {
				return m.assertRationalSetNormalized(&tc.sig, numeralContextRef(integerAllocationInvocationContext{}))
			}
			if e := call(); !isMPNVerificationThrow(e) {
				t.Fatal("Throw at normalized significand assertion", e)
			}
			if e := call(); e != nil {
				t.Fatal("Continue reaches following source code", e)
			}
			contents, e := os.ReadFile(log.Name())
			serviceMust(t, e)
			if n := strings.Count(string(contents), "File: src/util/mpf.cpp\nLine: 274\n(m_mpq_manager.le(1, sig) && m_mpq_manager.lt(sig, 2))\n"); n != 2 {
				t.Fatal("source assertion site and action count", n, string(contents))
			}
			serviceMust(t, d.EnableAssertions(false))
			if e := call(); e != nil {
				t.Fatal("disabled source assertion", e)
			}
			serviceMust(t, d.Close())
			valid := Rational{Num: NewInt(3), Den: NewInt(2)}
			if e := m.assertRationalSetNormalized(&valid, numeralContextRef(integerAllocationInvocationContext{})); !errors.Is(e, arena.ErrClosed) {
				t.Fatal("valid predicate reads closed assertion owner", e)
			}
			serviceMust(t, m.Close())
			if b.Used() != 0 {
				t.Fatal("unreleased normalization owner", b.Used())
			}
		})
	}
}
