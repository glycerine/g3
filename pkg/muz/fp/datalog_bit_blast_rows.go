package fp

// Source: dl_mk_bit_blast.cpp:94-112 assigns bits from low to high within
// each original predicate column. The fixed fork migrates the stored facts
// into the blasted predicates (rel_context::add_bit_blasted_facts, FORK FIX
// #120 and #121); these concrete tuple conversions do it for the Go finite
// relation API.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func datalogBitBlastFactWidth(widths []uint32) (int, error) {
	count := 0
	maxInt := int(^uint(0) >> 1)
	for column, width := range widths {
		// Finite table signatures store the domain cardinality in uint64,
		// so a 64-bit original domain cannot be represented by this path.
		if width == 0 || width > 63 {
			return 0, fmt.Errorf("%w: bit-blasted column %d has unsupported width %d", rel.ErrTable, column, width)
		}
		if count > maxInt-int(width) {
			return 0, fmt.Errorf("%w: bit-blasted tuple arity overflows", rel.ErrTable)
		}
		count += int(width)
	}
	return count, nil
}

// encodeDatalogBitBlastFact expands an all-bit-vector finite tuple into Boolean
// columns. The result owns its slice and does not borrow the input tuple.
func encodeDatalogBitBlastFact(widths []uint32, fact rel.Fact) (rel.Fact, error) {
	count, err := datalogBitBlastFactWidth(widths)
	if err != nil {
		return nil, err
	}
	if len(fact) != len(widths) {
		return nil, fmt.Errorf("%w: bit-vector tuple arity %d, expected %d", rel.ErrTable, len(fact), len(widths))
	}
	for column, width := range widths {
		if uint64(fact[column]) >= uint64(1)<<width {
			return nil, fmt.Errorf("%w: bit-vector tuple column %d exceeds width %d", rel.ErrTable, column, width)
		}
	}
	result := make(rel.Fact, count)
	offset := 0
	for column, width := range widths {
		value := uint64(fact[column])
		for bit := uint32(0); bit < width; bit++ {
			result[offset] = rel.Element((value >> bit) & 1)
			offset++
		}
	}
	return result, nil
}

// decodeDatalogBitBlastFact reconstructs the original finite tuple from its
// Boolean columns, rejecting malformed rows before publishing a result.
func decodeDatalogBitBlastFact(widths []uint32, fact rel.Fact) (rel.Fact, error) {
	count, err := datalogBitBlastFactWidth(widths)
	if err != nil {
		return nil, err
	}
	if len(fact) != count {
		return nil, fmt.Errorf("%w: Boolean tuple arity %d, expected %d", rel.ErrTable, len(fact), count)
	}
	for column, bit := range fact {
		if bit > 1 {
			return nil, fmt.Errorf("%w: bit-blasted tuple column %d is not Boolean", rel.ErrTable, column)
		}
	}
	result := make(rel.Fact, len(widths))
	offset := 0
	for column, width := range widths {
		value := uint64(0)
		for bit := uint32(0); bit < width; bit++ {
			value |= uint64(fact[offset]) << bit
			offset++
		}
		result[column] = rel.Element(value)
	}
	return result, nil
}

// datalogBitBlastRelationTuples is FORK FIX #121 of
// rel_context::add_bit_blasted_facts for a relation that is not a table:
// every tuple of the columns' domains, first column slowest, whose
// contains_fact holds. Each tuple's relation_fact numerals are made before
// the call and released, first column first, when it returns.
func datalogBitBlastRelationTuples(manager *ast.ASTManager, relation rel.RelationBase, sorts []ast.ASTHandle, widths []uint32) ([]rel.Fact, error) {
	bits := uint32(0)
	for _, width := range widths {
		bits += width
	}
	if bits > 24 {
		return nil, z3exception.NewDefaultException("bit-blast: relation of more than 24 bits cannot be enumerated")
	}
	var facts []rel.Fact
	for n := uint64(0); n < uint64(1)<<bits; n++ {
		fact := make(rel.Fact, len(widths))
		v := n
		for i := len(widths) - 1; i >= 0; i-- {
			fact[i] = rel.Element(v & (uint64(1)<<widths[i] - 1))
			v >>= widths[i]
		}
		numerals := make([]ast.ASTRef, 0, len(fact))
		var err error
		for i, value := range fact {
			reference, cause := manager.NewRef(ast.ASTHandle{})
			if cause != nil {
				err = cause
				break
			}
			numeral, cause := manager.MkDatalogNumeral(uint64(value), sorts[i])
			cause = errors.Join(cause, manager.SetRef(reference, numeral))
			numerals = append(numerals, reference)
			if cause != nil {
				err = cause
				break
			}
		}
		found := false
		if err == nil {
			found, err = relation.Contains(fact)
		}
		for i := range numerals {
			err = errors.Join(err, manager.DelRef(&numerals[i]))
		}
		if err != nil {
			return nil, err
		}
		if found {
			facts = append(facts, fact)
		}
	}
	return facts, nil
}
