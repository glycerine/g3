package spacer

// Source: src/muz/spacer/spacer_util.cpp expand_literals arithmetic branch.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestExpandInductiveArithmeticEquality(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	owner, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := owner.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	integer, err := manager.GetSort(zero)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("expand-inductive-equality")
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	transformer, err := NewRulePredicateTransformer(owner, predicate, PropSolverConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := transformer.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	xName, err := manager.Symbols.Intern("expand-x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkConst(xName, integer)
	if err != nil {
		t.Fatal(err)
	}
	yName, err := manager.Symbols.Intern("expand-y")
	if err != nil {
		t.Fatal(err)
	}
	y, err := manager.MkConst(yName, integer)
	if err != nil {
		t.Fatal(err)
	}
	equality, err := manager.MkEq(x, y)
	if err != nil {
		t.Fatal(err)
	}
	expandedOwner, err := transformer.expandInductiveLiteral(equality)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := expandedOwner.Close(); err != nil {
			t.Error(err)
		}
	}()
	expanded := expandedOwner.literals
	if len(expanded) != 2 {
		t.Fatal("expanded equality size", len(expanded))
	}
	for index, expected := range []int32{2, 3} {
		kind, kindErr := manager.GetDeclKind(expanded[index])
		if kindErr != nil || kind != expected {
			t.Fatal("expanded equality operator", index, kind, kindErr)
		}
	}
	zName, err := manager.Symbols.Intern("expand-z")
	if err != nil {
		t.Fatal(err)
	}
	z, err := manager.MkConst(zName, integer)
	if err != nil {
		t.Fatal(err)
	}
	yz, err := manager.MkEq(y, z)
	if err != nil {
		t.Fatal(err)
	}
	classes, dirty, err := transformer.expandEqualityClasses([]ast.ASTHandle{equality, yz})
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || len(classes) != 3 {
		t.Fatal("full equivalence-class expansion", dirty, len(classes))
	}
	defer func() {
		if err := releaseNormalizeRefs(manager, classes); err != nil {
			t.Error(err)
		}
	}()
	for _, ref := range classes {
		h, err := manager.RefGet(ref)
		if err != nil {
			t.Fatal(err)
		}
		kind, err := manager.GetDeclKind(h)
		if err != nil || kind != ast.ASTOpEq {
			t.Fatal("expanded equality lifetime", kind, err)
		}
	}
}

func TestExpandInductiveDatatypeAndBitVectorEqualities(t *testing.T) {
	manager, globals := newSpacerTestServices(t)
	owner, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := owner.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	predicateName, err := manager.Symbols.Intern("expand-other-equalities")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(predicateName, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	transformer, err := NewRulePredicateTransformer(owner, predicate, PropSolverConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := transformer.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	integer, err := manager.GetSort(zero)
	if err != nil {
		t.Fatal(err)
	}
	datatypeName, err := manager.Symbols.Intern("expand-box")
	if err != nil {
		t.Fatal(err)
	}
	constructorName, err := manager.Symbols.Intern("expand-mk-box")
	if err != nil {
		t.Fatal(err)
	}
	recognizerName, err := manager.Symbols.Intern("expand-is-box")
	if err != nil {
		t.Fatal(err)
	}
	accessorName, err := manager.Symbols.Intern("expand-unbox")
	if err != nil {
		t.Fatal(err)
	}
	otherAccessorName, err := manager.Symbols.Intern("expand-unbox-other")
	if err != nil {
		t.Fatal(err)
	}
	sorts, err := manager.MkDatatypes([]ast.DatatypeDefinition{{
		Name: datatypeName,
		Constructors: []ast.DatatypeConstructorDefinition{{
			Name: constructorName, Recognizer: recognizerName,
			Accessors: []ast.DatatypeAccessorDefinition{{Name: accessorName, Range: integer}, {Name: otherAccessorName, Range: integer}},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	constructors, err := manager.DatatypeConstructors(sorts[0])
	if err != nil {
		t.Fatal(err)
	}
	boxValue, err := manager.MkApp(constructors[0], []ast.ASTHandle{zero, zero})
	if err != nil {
		t.Fatal(err)
	}
	boxName, err := manager.Symbols.Intern("expand-box-value")
	if err != nil {
		t.Fatal(err)
	}
	box, err := manager.MkConst(boxName, sorts[0])
	if err != nil {
		t.Fatal(err)
	}
	boxEquality, err := manager.MkEq(box, boxValue)
	if err != nil {
		t.Fatal(err)
	}
	datatypeExpandedOwner, err := transformer.expandInductiveLiteral(boxEquality)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := datatypeExpandedOwner.Close(); err != nil {
			t.Error(err)
		}
	}()
	datatypeExpanded := datatypeExpandedOwner.literals
	if len(datatypeExpanded) != 5 {
		t.Fatal("expanded datatype equality size", len(datatypeExpanded))
	}
	if kind, kindErr := manager.GetDeclKind(datatypeExpanded[0]); kindErr != nil || kind != ast.ASTDatatypeRecognizer {
		t.Fatal("datatype recognizer literal", kind, kindErr)
	}
	for index, expected := range []int32{2, 3, 2, 3} {
		if kind, kindErr := manager.GetDeclKind(datatypeExpanded[index+1]); kindErr != nil || kind != expected {
			t.Fatal("datatype integer field bound", index, kind, kindErr)
		}
	}
	accessors, err := manager.DatatypeConstructorAccessors(constructors[0])
	if err != nil {
		t.Fatal(err)
	}
	for index, accessor := range accessors {
		for _, bound := range datatypeExpanded[1+2*index : 3+2*index] {
			term, err := manager.GetArg(bound, 0)
			if err != nil {
				t.Fatal(err)
			}
			decl, err := manager.GetDecl(term)
			if err != nil || decl != accessor {
				t.Fatal("datatype field bound order", index, decl, accessor, err)
			}
		}
	}
	bvValue, err := manager.BitVectorNumeralUnsigned(5, 3)
	if err != nil {
		t.Fatal(err)
	}
	bvSort, err := manager.GetSort(bvValue)
	if err != nil {
		t.Fatal(err)
	}
	bvName, err := manager.Symbols.Intern("expand-bv")
	if err != nil {
		t.Fatal(err)
	}
	bv, err := manager.MkConst(bvName, bvSort)
	if err != nil {
		t.Fatal(err)
	}
	bvEquality, err := manager.MkEq(bv, bvValue)
	if err != nil {
		t.Fatal(err)
	}
	bvExpandedOwner, err := transformer.expandInductiveLiteral(bvEquality)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := bvExpandedOwner.Close(); err != nil {
			t.Error(err)
		}
	}()
	bvExpanded := bvExpandedOwner.literals
	if len(bvExpanded) != 3 {
		t.Fatal("expanded bit-vector equality size", len(bvExpanded))
	}
	for index, wantNegated := range []bool{false, true, false} {
		negated, _, notErr := manager.IsNot(bvExpanded[index])
		if notErr != nil || negated != wantNegated {
			t.Fatal("bit-vector expanded bit", index, negated, notErr)
		}
	}
}
