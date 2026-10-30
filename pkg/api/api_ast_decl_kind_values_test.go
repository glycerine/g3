package api

import "testing"

func TestAPIPublicDeclarationKindCategoriesDoNotAlias(t *testing.T) {
	seen := make(map[APIDeclarationKind]bool)
	values := []APIDeclarationKind{
		APIDeclarationUninterpreted, APIDeclarationInternal, APIDeclarationTrue,
		APIDeclarationFalse, APIDeclarationEqual, APIDeclarationDistinct,
		APIDeclarationIte, APIDeclarationAnd, APIDeclarationOr, APIDeclarationIff,
		APIDeclarationXor, APIDeclarationNot, APIDeclarationImplies,
		APIDeclarationOEq, APIDeclarationInterpolant, APIDeclarationPBLE,
		APIDeclarationPBGE, APIDeclarationPBEQ, APIDeclarationPBAtMost,
		APIDeclarationPBAtLeast, APIDeclarationProofUndefined,
		APIDeclarationProofHyperResolve,
	}
	for _, value := range values {
		if seen[value] {
			t.Fatal("aliased public declaration category", value)
		}
		seen[value] = true
	}
}
