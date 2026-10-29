package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTSortKindCoreFamilies(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	integer := f.pin(f.c.IntegerSort())
	real := f.pin(f.c.RealSort())
	bits := f.pin(f.c.BitVectorSort(9))
	uninterpreted := f.pin(f.c.UninterpretedSort("sort-kind-U"))
	array := f.pin(f.c.ArraySort([]ast.ASTHandle{integer}, boolean))
	sequence := f.pin(f.c.SequenceSort(integer))
	regex := f.pin(f.c.RegexSort(sequence))
	for _, test := range []struct {
		sort ast.ASTHandle
		want APISortKind
	}{{boolean, APISortBool}, {integer, APISortInt}, {real, APISortReal}, {bits, APISortBitVector}, {uninterpreted, APISortUninterpreted}, {array, APISortArray}, {sequence, APISortSequence}, {regex, APISortRegex}} {
		if got, err := f.c.SortKind(test.sort); err != nil || got != test.want {
			t.Fatal("sort kind", got, test.want, err)
		}
	}
	if _, err := f.c.SortKind(f.pin(f.c.Bool(true))); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-sort", err)
	}
}
