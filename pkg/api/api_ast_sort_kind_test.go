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
	finite := f.pin(f.c.FiniteDomainSort("sort-kind-finite", 7))
	relation := f.pin(f.m.MkDatalogRelationSort([]ast.ASTHandle{finite}))
	floating := f.pin(f.c.FloatingPointSingleSort())
	rounding := f.pin(f.c.FloatingPointRoundingModeSort())
	enum, err := f.c.EnumerationSort("sort-kind-enum", []string{"only"})
	astTestMust(t, err)
	datatype := f.pin(enum.Sort, nil)
	family, err := f.m.EnsureFamilyID("sort-kind-custom")
	astTestMust(t, err)
	info, err := f.m.NewSortInfo(family, 17, nil, ast.InfiniteSortSize(), false)
	astTestMust(t, err)
	name, err := f.c.StringSymbol("custom")
	astTestMust(t, err)
	unknown := f.pin(f.m.MkSort(name, info))
	astTestMust(t, f.m.DelSortInfo(&info))
	for _, test := range []struct {
		sort ast.ASTHandle
		want APISortKind
	}{{boolean, APISortBool}, {integer, APISortInt}, {real, APISortReal}, {bits, APISortBitVector}, {uninterpreted, APISortUninterpreted}, {array, APISortArray}, {sequence, APISortSequence}, {regex, APISortRegex}, {finite, APISortFiniteDomain}, {relation, APISortRelation}, {floating, APISortFloatingPoint}, {rounding, APISortRoundingMode}, {datatype, APISortDatatype}, {unknown, APISortUnknown}} {
		if got, err := f.c.SortKind(test.sort); err != nil || got != test.want {
			t.Fatal("sort kind", got, test.want, err)
		}
	}
	if _, err := f.c.SortKind(f.pin(f.c.Bool(true))); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-sort", err)
	}
}
