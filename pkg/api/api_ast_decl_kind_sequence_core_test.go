package api

import "testing"

func TestAPISequenceCoreDeclarationKindSourceTable(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{0, APIDeclarationSequenceUnit},
		{1, APIDeclarationSequenceEmpty},
		{2, APIDeclarationSequenceConcat},
		{11, APIDeclarationSequenceToRegex},
		{12, APIDeclarationSequenceInRegex},
	}
	for _, test := range tests {
		if got := apiSequenceCoreDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d, want %d", test.operation, got, test.want)
		}
	}
	if got := apiSequenceCoreDeclarationKind(3); got != APIDeclarationInternal {
		t.Fatal("non-core operation", got)
	}
}
