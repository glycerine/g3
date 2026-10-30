package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIParserSuppliedDeclarationsPreserveASTIdentity(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	constant := f.pin(f.c.Constant("native-c", boolean))
	predicate := f.pin(f.c.Function("native-P", []ast.ASTHandle{boolean}, boolean))
	bindings := SMT2Bindings{Declarations: []SMT2DeclarationBinding{
		{Name: "c", Declaration: f.pin(f.m.GetDecl(constant))},
		{Name: "P", Declaration: predicate},
	}}
	parsed, err := f.c.ParseSMT2String("(assert (P c))", bindings)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(formula)
	astTestMust(t, err)
	argument, err := f.m.GetArg(formula, 0)
	astTestMust(t, err)
	if declaration != predicate || argument != constant {
		t.Fatal("supplied declaration identity", declaration, predicate, argument, constant)
	}
}

func TestAPIParserSuppliedSortIsUsedByNewDeclaration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.UninterpretedSort("native-S"))
	parsed, err := f.c.ParseSMT2String("(declare-const a S)(assert (= a a))", SMT2Bindings{Sorts: []SMT2SortBinding{{Name: "S", Sort: sort}}})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&parsed)) }()
	formula, err := f.m.RefGet(parsed)
	astTestMust(t, err)
	left, err := f.m.GetArg(formula, 0)
	astTestMust(t, err)
	actual, err := f.m.GetSort(left)
	astTestMust(t, err)
	if actual != sort {
		t.Fatal("supplied sort identity", actual, sort)
	}
}
