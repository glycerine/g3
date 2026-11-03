package fp

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestDatalogBitBlastFactColumnAndBitOrder(t *testing.T) {
	widths := []uint32{2, 3, 1}
	original := rel.Fact{2, 5, 1}
	wantBits := rel.Fact{0, 1, 1, 0, 1, 1}
	bits, err := encodeDatalogBitBlastFact(widths, original)
	if err != nil || !reflect.DeepEqual(bits, wantBits) {
		t.Fatal("column-major low-bit-first encoding", bits, err)
	}
	decoded, err := decodeDatalogBitBlastFact(widths, wantBits)
	if err != nil || !reflect.DeepEqual(decoded, rel.Fact{2, 5, 1}) {
		t.Fatal("independent literal Boolean tuple decoding", decoded, err)
	}
	// Results may outlive or be modified independently of both caller inputs.
	original[0], wantBits[0] = 3, 1
	if bits[0] != 0 || decoded[0] != 2 {
		t.Fatal("conversion result aliases an input", bits, decoded)
	}
	bits[1], decoded[1] = 0, 0
	if original[0] != 3 || wantBits[1] != 1 || !reflect.DeepEqual(widths, []uint32{2, 3, 1}) {
		t.Fatal("conversion changed caller-owned data", original, wantBits, widths)
	}
}

func TestDatalogBitBlastFactFiniteWidthEdges(t *testing.T) {
	sparseBits := make(rel.Fact, 63)
	sparseBits[0], sparseBits[62] = 1, 1
	fullBits := make(rel.Fact, 63)
	for i := range fullBits {
		fullBits[i] = 1
	}
	for _, tc := range []struct {
		name   string
		widths []uint32
		value  rel.Fact
		bits   rel.Fact
	}{
		{name: "zero-columns", widths: []uint32{}, value: rel.Fact{}, bits: rel.Fact{}},
		{name: "one-bit-zero", widths: []uint32{1}, value: rel.Fact{0}, bits: rel.Fact{0}},
		{name: "one-bit-one", widths: []uint32{1}, value: rel.Fact{1}, bits: rel.Fact{1}},
		{name: "highest-finite-bit", widths: []uint32{63}, value: rel.Fact{0x4000000000000001}, bits: sparseBits},
		{name: "largest-finite-value", widths: []uint32{63}, value: rel.Fact{0x7fffffffffffffff}, bits: fullBits},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := encodeDatalogBitBlastFact(tc.widths, tc.value)
			if err != nil || !reflect.DeepEqual(encoded, tc.bits) {
				t.Fatal("encoding boundary", encoded, err)
			}
			decoded, err := decodeDatalogBitBlastFact(tc.widths, tc.bits)
			if err != nil || !reflect.DeepEqual(decoded, tc.value) {
				t.Fatal("decoding boundary", decoded, err)
			}
		})
	}
}

func TestDatalogBitBlastFactRejectsMalformedRows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		widths []uint32
		fact   rel.Fact
		decode bool
	}{
		{name: "encode-zero-width", widths: []uint32{0}, fact: rel.Fact{0}},
		{name: "encode-wide-domain", widths: []uint32{64}, fact: rel.Fact{0}},
		{name: "encode-overflowing-width", widths: []uint32{^uint32(0)}, fact: rel.Fact{0}},
		{name: "encode-missing-column", widths: []uint32{2, 3}, fact: rel.Fact{1}},
		{name: "encode-extra-column", widths: []uint32{2}, fact: rel.Fact{1, 0}},
		{name: "encode-column-range", widths: []uint32{2, 1}, fact: rel.Fact{3, 2}},
		{name: "encode-63-bit-overflow", widths: []uint32{63}, fact: rel.Fact{0x8000000000000000}},
		{name: "decode-zero-width", widths: []uint32{0}, decode: true},
		{name: "decode-wide-domain", widths: []uint32{64}, decode: true},
		{name: "decode-missing-bit", widths: []uint32{2, 3}, fact: rel.Fact{0, 1, 1, 0}, decode: true},
		{name: "decode-extra-bit", widths: []uint32{2}, fact: rel.Fact{0, 1, 0}, decode: true},
		{name: "decode-non-Boolean", widths: []uint32{2}, fact: rel.Fact{1, 2}, decode: true},
		{name: "decode-large-non-Boolean", widths: []uint32{1}, fact: rel.Fact{^rel.Element(0)}, decode: true},
		{name: "encode-unexpected-nullary-column", fact: rel.Fact{0}},
		{name: "decode-unexpected-nullary-column", fact: rel.Fact{0}, decode: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result rel.Fact
			var err error
			if tc.decode {
				result, err = decodeDatalogBitBlastFact(tc.widths, tc.fact)
			} else {
				result, err = encodeDatalogBitBlastFact(tc.widths, tc.fact)
			}
			if !errors.Is(err, rel.ErrTable) || result != nil {
				t.Fatal("malformed tuple published a result", result, err)
			}
		})
	}
}
