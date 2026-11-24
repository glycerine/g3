package ast

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTSymbolPayloadCodecAndPhysicalCharges(t *testing.T) {
	m := astTestManager(t)
	text := astTestSymbol(t, m, "codec-spelling")
	marked, e := text.Mark()
	astTestMust(t, e)
	numerical := usymbol.Numerical(math.MaxUint32)
	markedNumber, e := numerical.Mark()
	astTestMust(t, e)
	values := []usymbol.Symbol{{}, usymbol.Numerical(0), numerical, text, marked, markedNumber, usymbol.Dummy()}
	before := m.allocator.Stats()
	p, e := m.allocator.Allocate(uint32(len(values)) * 24)
	astTestMust(t, e)
	for i, s := range values {
		astTestMust(t, m.writeSymbol(p, uint32(i), s))
		got, e := m.readSymbol(p, uint32(i))
		astTestMust(t, e)
		if got != s {
			t.Fatal("scalar token roundtrip", i, got, s)
		}
	}
	after := m.allocator.Stats()
	if after.RequestedBytes-before.RequestedBytes != uint64(len(values))*24 {
		t.Fatal("actual encoded payload length", before, after)
	}
	if reflect.TypeOf(text.Generation).Size() != 4 || reflect.TypeOf(text.Tag).Size() != 4 {
		t.Fatal("codec field widths changed")
	}
	if after.OwnerBytes+after.PayloadBytes+after.MetadataBytes+after.DirectoryBytes <= after.RequestedBytes {
		t.Fatal("allocator overhead was not charged", after)
	}
	// 2^29*24 wraps a uint32 byte offset to zero. This must fail before writing
	// element zero, using bounded live storage rather than a huge allocation.
	if e = m.writeSymbol(p, 1<<29, text); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("overflowed codec offset", e)
	}
	got, e := m.readSymbol(p, 0)
	astTestMust(t, e)
	if got != (usymbol.Symbol{}) {
		t.Fatal("overflow mutated element zero")
	}
	if _, e = m.readSymbol(p, 1<<29); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("overflowed reader offset", e)
	}
	other, e := usymbol.NewManager(usymbol.Config{})
	astTestMust(t, e)
	foreign, e := other.Intern("foreign")
	astTestMust(t, e)
	if e = m.writeSymbol(p, 0, foreign); !errors.Is(e, usymbol.ErrOwner) {
		t.Fatal("foreign spelling accepted", e)
	}
	astTestMust(t, other.Close())
	astTestMust(t, m.allocator.Deallocate(uint32(len(values))*24, p))
	if _, e = m.readSymbol(p, 0); e == nil {
		t.Fatal("dead payload accepted")
	}
}

func TestASTSymbolPayloadShortFinalRecord(t *testing.T) {
	m := astTestManager(t)
	s := astTestSymbol(t, m, "short-symbol")
	var encoded [astSymbolBytes]byte
	for i, word := range s.SymbolWords() {
		binary.LittleEndian.PutUint32(encoded[4*i:], word)
	}
	for length := uint32(0); length <= astSymbolBytes; length++ {
		size := astSymbolBytes + length
		p, err := m.allocator.Allocate(size)
		astTestMust(t, err)
		data, err := m.allocator.Borrow(p)
		astTestMust(t, err)
		for i := range data {
			data[i] = 0xff
		}
		err = m.writeSymbol(p, 1, s)
		if length < astSymbolBytes && !errors.Is(err, arena.ErrBounds) || length == astSymbolBytes && err != nil {
			t.Fatalf("write with %d bytes left: %v", length, err)
		}
		// The scalar writer commits each complete word, including padding.
		want := bytes.Repeat([]byte{0xff}, int(size))
		copy(want[astSymbolBytes:], encoded[:length/4*4])
		if !bytes.Equal(data, want) {
			t.Fatalf("write with %d bytes left: %x, want %x", length, data, want)
		}
		got, err := m.readSymbol(p, 1)
		if length >= 20 {
			if err != nil || got != s {
				t.Fatalf("read with %d bytes left: %v, %v", length, got, err)
			}
		} else if !errors.Is(err, arena.ErrBounds) {
			t.Fatalf("read with %d bytes left: %v", length, err)
		}
		astTestMust(t, m.allocator.Deallocate(size, p))
	}
}

func TestCopiedQuantifierPayloadOwnershipAndRollback(t *testing.T) {
	m := astTestManager(t)
	for _, size := range []uint32{24, 512} {
		source, err := m.allocator.Allocate(size)
		astTestMust(t, err)
		want := make([]byte, size)
		for i := range want {
			want[i] = byte(i + 1)
			astTestMust(t, m.allocator.Write(source, uint32(i), want[i]))
		}
		before := m.allocator.Stats().RequestedBytes
		if p, err := m.copyQuantifierPayload(source, size+1, 1); !errors.Is(err, arena.ErrBounds) || p != (arena.SmallBytesRef{}) {
			t.Fatalf("short source of %d bytes returned %v, %v", size, p, err)
		}
		if got := m.allocator.Stats().RequestedBytes; got != before {
			t.Fatalf("failed copy retained %d bytes, previously %d", got, before)
		}
		p, err := m.copyQuantifierPayload(source, size, 1)
		astTestMust(t, err)
		astTestMust(t, m.allocator.Write(source, size-1, 17))
		astTestMust(t, m.allocator.Deallocate(size, source))
		data, err := m.allocator.View(p)
		astTestMust(t, err)
		if !bytes.Equal(data, want) {
			t.Fatalf("copy of %d bytes changed with its source", size)
		}
		if dead, err := m.copyQuantifierPayload(source, size, 1); err == nil || dead != (arena.SmallBytesRef{}) {
			t.Fatalf("stale source of %d bytes returned %v, %v", size, dead, err)
		}
		if got := m.allocator.Stats().RequestedBytes; got != before {
			t.Fatalf("stale source left %d bytes, expected %d", got, before)
		}
		astTestMust(t, m.allocator.Deallocate(size, p))
	}
}

func TestASTPayloadArithmeticFiniteBoundaries(t *testing.T) {
	limit := uint64(math.MaxUint32)
	if host := uint64(^uint(0) >> 1); host < limit {
		limit = host
	}
	count := uint32(limit / 24)
	if n, e := astPayloadBytes(count, 24); e != nil || n != count*24 {
		t.Fatal(n, e)
	}
	if _, e := astPayloadBytes(count+1, 24); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	if _, e := astPayloadOffset(1<<29, 24); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
}
