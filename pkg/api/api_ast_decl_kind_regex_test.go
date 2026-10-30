package api

import "testing"

func TestAPIRegexDeclarationKindRetainedSourceCases(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{13, APIDeclarationRegexPlus},
		{14, APIDeclarationRegexStar},
		{15, APIDeclarationRegexOption},
		{17, APIDeclarationRegexConcat},
		{18, APIDeclarationRegexUnion},
		{19, APIDeclarationRegexIntersect},
		{20, APIDeclarationRegexLoop},
		{22, APIDeclarationRegexEmptySet},
		{24, APIDeclarationRegexFullSet},
	}
	for _, test := range tests {
		if got := apiRegexDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d, want %d", test.operation, got, test.want)
		}
	}
	for _, operation := range []int32{16, 21, 23, 25} {
		if got := apiRegexDeclarationKind(operation); got != APIDeclarationInternal {
			t.Fatalf("source-internal operation %d: %d", operation, got)
		}
	}
}
