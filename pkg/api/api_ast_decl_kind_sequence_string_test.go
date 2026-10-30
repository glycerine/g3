package api

import "testing"

func TestAPISequenceStringDeclarationKindSourceAliases(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{27, APIDeclarationIntegerToString},
		{28, APIDeclarationStringToInteger},
		{29, APIDeclarationSequenceReplace},
		{30, APIDeclarationSequenceConcat},
		{31, APIDeclarationSequenceLength},
		{32, APIDeclarationSequenceContains},
		{33, APIDeclarationSequencePrefix},
		{34, APIDeclarationSequenceSuffix},
		{35, APIDeclarationSequenceInRegex},
		{36, APIDeclarationSequenceToRegex},
		{37, APIDeclarationSequenceAt},
		{38, APIDeclarationSequenceExtract},
		{39, APIDeclarationSequenceIndex},
		{40, APIDeclarationRegexEmptySet},
		{41, APIDeclarationRegexFullSet},
	}
	for _, test := range tests {
		if got := apiSequenceStringDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d, want %d", test.operation, got, test.want)
		}
	}
	if got := apiSequenceStringDeclarationKind(26); got != APIDeclarationInternal {
		t.Fatal("string literal unexpectedly public", got)
	}
}
