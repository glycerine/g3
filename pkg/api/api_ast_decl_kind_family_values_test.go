package api

import "testing"

func TestAPIArithmeticAndArrayDeclarationCategoriesDoNotAlias(t *testing.T) {
	seen := make(map[APIDeclarationKind]bool)
	for value := APIDeclarationArithmeticNumeral; value <= APIDeclarationIsInteger; value++ {
		if seen[value] {
			t.Fatal(value)
		}
		seen[value] = true
	}
	for value := APIDeclarationArrayStore; value <= APIDeclarationArrayExtensionality; value++ {
		if seen[value] {
			t.Fatal(value)
		}
		seen[value] = true
	}
	for _, prior := range []APIDeclarationKind{APIDeclarationInternal, APIDeclarationPBAtLeast, APIDeclarationProofHyperResolve} {
		if seen[prior] {
			t.Fatal("new family category aliased prior batch", prior)
		}
	}
}
