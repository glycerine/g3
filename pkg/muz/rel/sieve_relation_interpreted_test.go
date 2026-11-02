package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSieveInterpretedFilterSelectedAndIgnoredColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	sorts := []ast.ASTHandle{f.boolean, f.bits, f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	newRelation := func() *SieveRelation {
		t.Helper()
		r, newErr := NewSieveRelation(signature, []bool{true, false, true})
		if newErr != nil {
			t.Fatal(newErr)
		}
		for _, fact := range []Fact{{0, 0, 0}, {0, 0, 1}, {1, 0, 0}, {1, 0, 1}} {
			if _, newErr = r.AddFact(fact); newErr != nil {
				t.Fatal(newErr)
			}
		}
		return r
	}
	first, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	third, err := f.ast.MkVar(2, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.ast.MkEq(first, third)
	if err != nil {
		t.Fatal(err)
	}
	selected := newRelation()
	defer selected.Close()
	if err = selected.FilterInterpreted(f.relations, sorts, theory, equal); err != nil {
		t.Fatal(err)
	}
	if selected.NumInnerRows() != 2 {
		t.Fatal("selected filter rows", selected.NumInnerRows())
	}
	requireSieveFact(t, selected, Fact{0, 3, 0}, true)
	requireSieveFact(t, selected, Fact{1, 1, 1}, true)
	requireSieveFact(t, selected, Fact{0, 0, 1}, false)

	ignoredVar, err := f.ast.MkVar(1, f.bits)
	if err != nil {
		t.Fatal(err)
	}
	ignoredCondition, err := f.ast.MkEq(ignoredVar, f.bv(3))
	if err != nil {
		t.Fatal(err)
	}
	ignored := newRelation()
	defer ignored.Close()
	// Source returns the identity operation before asking the inner plugin for
	// an interpreted filter, so no rewriter is needed on this path.
	if err = ignored.FilterInterpreted(f.relations, sorts, nil, ignoredCondition); err != nil {
		t.Fatal(err)
	}
	if ignored.NumInnerRows() != 4 {
		t.Fatal("ignored filter changed inner rows", ignored.NumInnerRows())
	}
	mixed, err := f.ast.MkAnd(equal, ignoredCondition)
	if err != nil {
		t.Fatal(err)
	}
	if err = ignored.FilterInterpreted(f.relations, sorts, theory, mixed); err != nil || ignored.NumInnerRows() != 4 {
		t.Fatal("mixed ignored filter must be identity", ignored.NumInnerRows(), err)
	}
}

func TestSieveInterpretedFilterGroundFailureAndAtomicUnknown(t *testing.T) {
	f := newTableRelationFixture(t)
	theory := tableRelationTheoryRewriter(t, f.ast)
	sorts := []ast.ASTHandle{f.boolean}
	signature, err := tableRelationSignature(f.relations, sorts)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := NewSieveRelation(signature, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	defer relation.Close()
	if _, err = relation.AddFact(Fact{0}); err != nil {
		t.Fatal(err)
	}
	if _, err = relation.AddFact(Fact{1}); err != nil {
		t.Fatal(err)
	}
	name, err := f.ast.Symbols.Intern("sieve-interpreted-unknown")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := f.ast.MkApp(predicate, []ast.ASTHandle{variable})
	if err != nil {
		t.Fatal(err)
	}
	if err = relation.FilterInterpreted(f.relations, sorts, theory, unknown); !errors.Is(err, ErrSieve) {
		t.Fatal("unknown filter", err)
	}
	if relation.NumInnerRows() != 2 {
		t.Fatal("failed filter mutated rows", relation.NumInnerRows())
	}
	if err = relation.FilterInterpreted(f.relations, sorts, theory, f.falsehood); err != nil {
		t.Fatal(err)
	}
	if !relation.Empty() {
		t.Fatal("ground false filter left rows")
	}
}
