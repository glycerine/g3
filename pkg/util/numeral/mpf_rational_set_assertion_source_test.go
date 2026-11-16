package numeral

// Source: src/util/mpf.cpp:274; src/util/mpq.cpp:rat_lt.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// This is a represented helper-domain assertion prefix, not a reachable
// normalized public SET body failure. The deliberately below-one input makes
// the first conjunct false, so the upper comparison must remain unread.
func TestArbitraryFloatExponentRationalSetAssertionHelperPrefix(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertion only")
	}
	bindMPNDebugActions(t, "T")
	b := arena.NewBudget(1 << 25)
	m, e := NewArbitraryFloatManager(IntegerConfig{Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, e)
	q := NewRational()
	q.Num = NewInt(1)
	d := make([]uint32, 21)
	d[0], d[20] = 3, 1
	serviceMust(t, m.integer.SetLimbs(&q.Den, d))
	if e = m.assertRationalSetNormalized(&q, numeralContextRef(integerAllocationInvocationContext{})); !isMPNVerificationThrow(e) {
		t.Fatal(e)
	}
	cmp, e := m.integer.Compare(&m.rational.LtTmp1.Num, &q.Den)
	serviceMust(t, e)
	if cmp != 0 || m.rational.LtTmp2.Num != q.Num {
		t.Fatal("source le(1,sig) prefix changed or upper conjunct read")
	}
	integerPhysicalAudit(t, m.integer, &q.Num, &q.Den, &m.rational.LtTmp1.Num, &m.rational.LtTmp1.Den, &m.rational.LtTmp2.Num, &m.rational.LtTmp2.Den)
	if b.Used() != m.memory.Stats().TotalBytes+m.OwnedStorageBytes() {
		t.Fatal("helper immediate cold graph")
	}
	serviceMust(t, m.rational.Del(&q))
	serviceMust(t, m.SourceFinalize())
	serviceMust(t, m.Close())
	if b.Used() != 0 {
		t.Fatal(b.Used())
	}
}
