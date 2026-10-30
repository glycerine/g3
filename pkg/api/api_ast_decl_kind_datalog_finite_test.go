package api

import "testing"

func TestAPIDatalogFiniteDeclarationKindSourceTable(t *testing.T) {
	if got := apiDatalogFiniteDeclarationKind(13); got != APIDeclarationFiniteDomainConstant {
		t.Fatal("finite constant", got)
	}
	if got := apiDatalogFiniteDeclarationKind(14); got != APIDeclarationFiniteDomainLessThan {
		t.Fatal("finite less-than", got)
	}
	// OP_DL_REP and OP_DL_ABS are plugin operations but are absent from the
	// public declaration-kind switch.
	for _, operation := range []int32{15, 16, 17} {
		if got := apiDatalogDeclarationKind(operation); got != APIDeclarationInternal {
			t.Fatalf("source-internal operation %d: %d", operation, got)
		}
	}
}
