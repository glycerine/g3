package numeral

// AUDIT.md util_numeral_03-F1: src/util/mpf.cpp keeps only scoped locals in
// set/round/to_rational, so a long-lived mpf_manager stays a constant size
// however many operations run. Clean invocation histories must be retired.

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/arena"
)

func TestArbitraryFloatHistoriesBounded(t *testing.T) {
	b := arena.NewBudget(1 << 40)
	m, e := NewArbitraryFloatManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	if e != nil {
		t.Fatal(e)
	}
	var x, y, out ArbitraryFloat
	serviceMust(t, m.SetDouble(&x, 11, 53, 1000.1))
	serviceMust(t, m.SetDouble(&y, 11, 53, 2.3))
	q := NewRational()
	lengths := func() (int, int, int, int, int) {
		r, rs, vc, fs, cv := 0, 0, 0, 0, 0
		for h := m.roundHistories; h != nil; h = h.next {
			r++
		}
		for h := m.RationalSetHistories; h != nil; h = h.Next {
			rs++
		}
		for h := m.ValueCalls; h != nil; h = h.Next {
			vc++
		}
		for h := m.FormattedSetHistories; h != nil; h = h.Next {
			fs++
		}
		for h := m.conversionHistory; h != nil; h = h.next {
			cv++
		}
		return r, rs, vc, fs, cv
	}
	for _, op := range []struct {
		name string
		f    func() error
	}{
		{"sqrt", func() error { return m.Sqrt(ArbitraryFloatNearestEven, &x, &out) }},
		{"rem", func() error { return m.Rem(&x, &y, &out) }},
		{"round-to-integral", func() error { return m.RoundToIntegral(ArbitraryFloatNearestEven, &x, &out) }},
		{"mul", func() error { return m.Mul(ArbitraryFloatNearestEven, &x, &y, &out) }},
		{"add", func() error { return m.Add(ArbitraryFloatNearestEven, &x, &y, &out) }},
		{"set-fraction", func() error { return m.SetFractionSigned(&out, 11, 53, ArbitraryFloatNearestEven, 1, 3) }},
		{"set-format", func() error { return m.SetFormat(&out, 8, 24, ArbitraryFloatNearestEven, &x) }},
		{"to-rational", func() error { return m.ToRational(&x, m.rational, &q) }},
	} {
		serviceMust(t, op.f()) // warm caches
		used := b.Used()
		for i := 0; i < 1000; i++ {
			if e := op.f(); e != nil {
				t.Fatal(op.name, e)
			}
		}
		if b.Used() != used {
			t.Fatalf("%s: budget grew from %d to %d over 1000 calls", op.name, used, b.Used())
		}
		if r, rs, vc, fs, cv := lengths(); r > 1 || rs > 1 || vc > 1 || fs > 1 || cv > 1 {
			t.Fatalf("%s: histories round=%d rational-set=%d value-copy=%d formatted-set=%d conversion=%d", op.name, r, rs, vc, fs, cv)
		}
	}
	serviceMust(t, m.rational.Del(&q))
	serviceMust(t, m.Del(&x))
	serviceMust(t, m.Del(&y))
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Close())
	if b.Used() != 0 {
		t.Fatal("retained bytes", b.Used())
	}
}

// Chain-held copy records stay until the chain owner ends; their cleanup must
// not rescan the manager-wide list (formerly O(n^2) at context close).
func TestArbitraryFloatValueCopyChainCleanupLinear(t *testing.T) {
	b := arena.NewBudget(1 << 40)
	m, e := NewArbitraryFloatManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	if e != nil {
		t.Fatal(e)
	}
	var x ArbitraryFloat
	serviceMust(t, m.SetDouble(&x, 11, 53, 1000.1))
	var chain ArbitraryFloatValueCopyChain
	serviceMust(t, chain.Bind(m))
	const n = 20000
	outs := make([]ArbitraryFloat, n)
	for i := range outs {
		var r ArbitraryFloatValueCopyReceipt
		serviceMust(t, chain.CopyWithReceipt(&outs[i], &x, &r))
	}
	start := time.Now()
	serviceMust(t, chain.Cleanup(true))
	serviceMust(t, chain.End())
	if d := time.Since(start); d > time.Second {
		t.Fatalf("chain cleanup of %d copies took %v", n, d)
	}
	for i := range outs {
		serviceMust(t, m.Del(&outs[i]))
	}
	serviceMust(t, m.Del(&x))
	serviceMust(t, m.Close())
	if b.Used() != 0 {
		t.Fatal("retained bytes", b.Used())
	}
}
