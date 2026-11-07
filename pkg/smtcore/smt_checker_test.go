package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTCheckerBooleanAssignmentsAndStructure(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	p, err := m.MkConst(astTestSymbol(t, m, "checker-p"), boolean)
	astTestMust(t, err)
	q, err := m.MkConst(astTestSymbol(t, m, "checker-q"), boolean)
	astTestMust(t, err)
	disjunction, err := m.MkOr(p, q)
	astTestMust(t, err)
	conjunction, err := m.MkAnd(p, q)
	astTestMust(t, err)
	iff, err := m.MkIff(p, q)
	astTestMust(t, err)
	ite, err := m.MkIte(p, p, q)
	astTestMust(t, err)

	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{p, q} {
		astTestMust(t, context.SetRelevant(expression, true))
	}
	astTestMust(t, context.SetAssignment(p, lbool.True))
	astTestMust(t, context.SetAssignment(q, lbool.False))
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	for name, test := range map[string]struct {
		expression ast.ASTHandle
		unsat      bool
	}{
		"or-sat": {disjunction, false}, "and-unsat": {conjunction, true},
		"iff-unsat": {iff, true}, "ite-sat": {ite, false},
	} {
		t.Run(name, func(t *testing.T) {
			var got bool
			if test.unsat {
				got, err = checker.IsUnsatisfiable(test.expression, nil)
			} else {
				got, err = checker.IsSatisfiable(test.expression, nil)
			}
			if err != nil || !got {
				t.Fatal(got, err)
			}
		})
	}
	unknown, err := m.MkConst(astTestSymbol(t, m, "checker-unknown"), boolean)
	astTestMust(t, err)
	unknownITE, err := m.MkIte(unknown, p, p)
	astTestMust(t, err)
	if got, err := checker.IsSatisfiable(unknownITE, nil); err != nil || !got {
		t.Fatal("unassigned ITE checks both equal branches", got, err)
	}
}

func TestSMTCheckerEqualityDisequalityBindingsAndCongruence(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "CheckerSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	xOwner, x := smtEnodeConstant(t, m, 2070, sort, 0)
	yOwner, y := smtEnodeConstant(t, m, 2071, sort, 0)
	zOwner, z := smtEnodeConstant(t, m, 2072, sort, 0)
	y.root = x

	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	for _, node := range []*SMTEnode{x, y, z} {
		astTestMust(t, context.RegisterEnode(node, true))
	}
	astTestMust(t, context.SetDisequal(x, z, true))
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	equalXY, err := m.MkEq(xOwner, yOwner)
	astTestMust(t, err)
	equalXZ, err := m.MkEq(xOwner, zOwner)
	astTestMust(t, err)
	if got, err := checker.IsSatisfiable(equalXY, nil); err != nil || !got {
		t.Fatal("equality class", got, err)
	}
	if got, err := checker.IsUnsatisfiable(equalXZ, nil); err != nil || !got {
		t.Fatal("registered disequality", got, err)
	}

	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	declaration, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "checker-f"), []ast.ASTHandle{sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	fxOwner, err := m.MkApp(declaration, []ast.ASTHandle{xOwner})
	astTestMust(t, err)
	fx, err := NewSMTEnode(m, fxOwner, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{xOwner: x}), SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(fx, true))
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	fVariable, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	astTestMust(t, err)
	congruentEquality, err := m.MkEq(fVariable, fxOwner)
	astTestMust(t, err)
	if got, err := checker.IsSatisfiable(congruentEquality, []*SMTEnode{y}); err != nil || !got {
		t.Fatal("bound variable congruence lookup", got, err)
	}

	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicateInfo, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	predicate, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "checker-predicate"), []ast.ASTHandle{sort}, boolean, predicateInfo)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&predicateInfo))
	pxOwner, err := m.MkApp(predicate, []ast.ASTHandle{xOwner})
	astTestMust(t, err)
	px, err := NewSMTEnode(m, pxOwner, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{xOwner: x}), SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(px, true))
	astTestMust(t, context.SetAssignment(pxOwner, lbool.True))
	pVariable, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	if got, err := checker.IsSatisfiable(pVariable, []*SMTEnode{y}); err != nil || !got {
		t.Fatal("Boolean congruent enode assignment", got, err)
	}
}
