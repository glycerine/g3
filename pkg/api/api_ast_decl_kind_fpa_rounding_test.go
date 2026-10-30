package api

import "testing"

func TestAPIFPARoundingDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationFPARMNearestTiesToEven,
		APIDeclarationFPARMNearestTiesToAway,
		APIDeclarationFPARMTowardPositive,
		APIDeclarationFPARMTowardNegative,
		APIDeclarationFPARMTowardZero,
	}
	for operation, want := range wants {
		if got := apiFPARoundingDeclarationKind(int32(operation)); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiFPARoundingDeclarationKind(5); got != APIDeclarationInternal {
		t.Fatal("non-rounding operation", got)
	}
}
