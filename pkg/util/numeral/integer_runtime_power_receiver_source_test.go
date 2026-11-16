package numeral

// Source: src/util/mpz.cpp:1453-1480; src/util/mpz.h:132-149; src/util/mpf.h:241-290.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"testing"
)

func TestIntegerPhysicalPowerTwoSoleRuntimeReceiver(t *testing.T) {
	m, b := integerManager(t)
	row, e := m.Memory.Allocate(16)
	serviceMust(t, e)
	out, base := NewInt(7), NewInt(2)
	serviceMust(t, m.Memory.WriteUint32(row, 0, 7))
	serviceMust(t, m.Memory.WriteUint32(row, 4, 0))
	serviceMust(t, m.Memory.WriteUint32(row, 8, 0))
	receiver := integerRuntimeValueReceiver{runtime: m.Memory, block: row}
	for _, p := range []uint32{0, 1, 30, 31, 100} {
		before := b.Used()
		if p < 31 {
			b.FailAfter(0)
		}
		e = m.powerWithPhysicalAllocationContext(&base, p, &out, nil, numeralContextRef(integerAllocationInvocationContext{}), &receiver)
		serviceMust(t, e)
		val, err := m.Memory.ReadUint32(row, 0)
		serviceMust(t, err)
		lo, err := m.Memory.ReadUint32(row, 4)
		serviceMust(t, err)
		hi, err := m.Memory.ReadUint32(row, 8)
		serviceMust(t, err)
		if int32(val) != out.Val || NumericCellRef(uint64(lo)|uint64(hi)<<32) != out.Cell {
			t.Fatal("sole reached NUM receiver differs", p, out, val, lo, hi)
		}
		if p < 31 && (out != NewInt(int32(uint32(1)<<p)) || b.Used() != before) {
			t.Fatal("inline power allocated or changed value", p, out, b.Used(), before)
		}
		integerPhysicalAuditWithRaw(t, m, []umemory.Block{row}, &out)
		if b.Used() != integerActualOwnedBytes(m) {
			t.Fatal("immediate physical/cold graph", p, b.Used())
		}
		b.FailAfter(-1)
	}
	serviceMust(t, m.Del(&out))
	serviceMust(t, m.Memory.Deallocate(&row))
	serviceMust(t, m.CloseChecked())
	serviceMust(t, m.Memory.Close())
	if b.Used() != 0 {
		t.Fatal("owning context end", b.Used())
	}
}
