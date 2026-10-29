package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointRoundingModes(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	roundingSort := f.pin(f.c.FloatingPointRoundingModeSort())
	tests := []struct {
		mode FloatingPointRoundingMode
		kind int32
	}{
		{FloatingPointRoundNearestTiesToEven, ast.FPAOpRMNearestEven},
		{FloatingPointRoundNearestTiesToAway, ast.FPAOpRMNearestAway},
		{FloatingPointRoundTowardPositive, ast.FPAOpRMTowardPositive},
		{FloatingPointRoundTowardNegative, ast.FPAOpRMTowardNegative},
		{FloatingPointRoundTowardZero, ast.FPAOpRMTowardZero},
	}
	for _, test := range tests {
		value := f.pin(f.c.FloatingPointRounding(test.mode))
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != test.kind {
			t.Fatalf("mode %d kind: %d, %v", test.mode, kind, err)
		}
		if sort := f.pin(f.c.SortOf(value)); sort != roundingSort {
			t.Fatalf("mode %d sort: %v != %v", test.mode, sort, roundingSort)
		}
	}
	aliases := []func() (ast.ASTHandle, error){
		f.c.FloatingPointRoundNearestEven,
		f.c.FloatingPointRoundNearestAway,
		f.c.FloatingPointRoundTowardPositiveValue,
		f.c.FloatingPointRoundTowardNegativeValue,
		f.c.FloatingPointRoundTowardZeroValue,
	}
	for i, make := range aliases {
		value := f.pin(make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != tests[i].kind {
			t.Fatal("rounding alias", i, kind, err)
		}
	}
	if _, err := f.c.FloatingPointRounding(FloatingPointRoundingMode(255)); err == nil {
		t.Fatal("invalid rounding mode accepted")
	}
}
