package api

import "testing"

func TestAPITypedASTIDsMatchManagerIDs(t *testing.T) {
	f := newAPITypedASTFixture(t)
	wantSort, err := f.manager.GetASTID(f.sort)
	if err != nil {
		t.Fatal(err)
	}
	gotSort, err := f.context.SortID(f.sort)
	if err != nil || gotSort != wantSort {
		t.Fatal(gotSort, wantSort, err)
	}
	wantDeclaration, err := f.manager.GetASTID(f.declaration)
	if err != nil {
		t.Fatal(err)
	}
	gotDeclaration, err := f.context.FunctionDeclarationID(f.declaration)
	if err != nil || gotDeclaration != wantDeclaration {
		t.Fatal(gotDeclaration, wantDeclaration, err)
	}
	if gotSort == gotDeclaration {
		t.Fatal("distinct source ASTs shared an identifier")
	}
}
