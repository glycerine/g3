package normal_forms

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSkolemizerFreeVariablesProofAndCache(t *testing.T) {
	m := definedNamesManager(t, true)
	sortName, e := m.Symbols.Intern("skolem-sort")
	definedNamesMust(t, e)
	sort, e := m.MkSort(sortName, ast.ASTSortInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, sort)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	pName, e := m.Symbols.Intern("skolem-p")
	definedNamesMust(t, e)
	pDecl, e := m.MkFuncDecl(pName, []ast.ASTHandle{sort, sort}, b, ast.ASTFuncDeclInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, pDecl)
	v0, e := m.MkVar(0, sort)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v0)
	v2, e := m.MkVar(2, sort)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v2)
	body, e := m.MkApp(pDecl, []ast.ASTHandle{v0, v2})
	definedNamesMust(t, e)
	definedNamesPin(t, m, body)
	skID, e := m.Symbols.Intern("source-skid")
	definedNamesMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: false, Sorts: []ast.ASTHandle{sort, sort},
		Names: []usymbol.Symbol{usymbol.Numerical(10), usymbol.Numerical(11)},
		Body:  body, SkolemID: skID,
	})
	definedNamesMust(t, e)
	definedNamesPin(t, m, q)
	s, e := NewSkolemizer(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, s.Close()) })
	out, proof := nameExprOutputs(t, m)
	definedNamesMust(t, s.Apply(q, out, proof))
	result, e := m.RefGet(out)
	definedNamesMust(t, e)
	if decl, x := m.GetDecl(result); x != nil || decl != pDecl {
		t.Fatal("predicate declaration", decl, x)
	}
	skolem, e := m.GetArg(result, 0)
	definedNamesMust(t, e)
	outer, e := m.GetArg(result, 1)
	definedNamesMust(t, e)
	if index, x := m.GetVarIndex(outer); x != nil || index != 0 {
		t.Fatal("outer variable was not shifted below eliminated binders", index, x)
	}
	skolemDecl, e := m.GetDecl(skolem)
	definedNamesMust(t, e)
	if yes, x := m.IsSkolemDecl(skolemDecl); x != nil || !yes {
		t.Fatal("fresh declaration metadata", yes, x)
	}
	if arity, x := m.GetArity(skolemDecl); x != nil || arity != 1 {
		t.Fatal("free-variable Skolem domain", arity, x)
	}
	skolemArg, e := m.GetArg(skolem, 0)
	definedNamesMust(t, e)
	if index, x := m.GetVarIndex(skolemArg); x != nil || index != 0 {
		t.Fatal("Skolem free-variable argument", index, x)
	}
	p, e := m.RefGet(proof)
	definedNamesMust(t, e)
	if yes, x := m.IsProofKind(p, ast.ASTPrSkolemize); x != nil || !yes {
		t.Fatal("skolemization proof", yes, x)
	}
	first := result
	definedNamesMust(t, s.Apply(q, out, proof))
	result, e = m.RefGet(out)
	definedNamesMust(t, e)
	if result != first {
		t.Fatal("cached result identity", first, result)
	}
}

func TestSkolemizerHackPatternAugmentsExistentialBody(t *testing.T) {
	m := definedNamesManager(t, false)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	v0, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v0)
	hackName, e := m.Symbols.Intern("sk_hack")
	definedNamesMust(t, e)
	hackDecl, e := m.MkFuncDecl(hackName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, hackDecl)
	hack, e := m.MkApp(hackDecl, []ast.ASTHandle{v0})
	definedNamesMust(t, e)
	definedNamesPin(t, m, hack)
	pattern, e := m.MkPattern([]ast.ASTHandle{hack})
	definedNamesMust(t, e)
	definedNamesPin(t, m, pattern)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: false, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)},
		Body: v0, Patterns: []ast.ASTHandle{pattern},
	})
	definedNamesMust(t, e)
	definedNamesPin(t, m, q)
	s, e := NewSkolemizer(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, s.Close()) })
	if yes, x := s.IsSkolemHack(pattern); x != nil || !yes {
		t.Fatal("sk_hack recognition", yes, x)
	}
	definedNamesMust(t, s.SetSkolemHack(true))
	out, proof := nameExprOutputs(t, m)
	definedNamesMust(t, s.Apply(q, out, proof))
	result, e := m.RefGet(out)
	definedNamesMust(t, e)
	if op, x := m.GetDeclKind(result); x != nil || op != ast.ASTOpAnd {
		t.Fatal("existential sk_hack conjunction", op, x)
	}
	left, e := m.GetArg(result, 0)
	definedNamesMust(t, e)
	right, e := m.GetArg(result, 1)
	definedNamesMust(t, e)
	rightArg, e := m.GetArg(right, 0)
	definedNamesMust(t, e)
	if left != rightArg {
		t.Fatal("body and sk_hack did not receive the same Skolem", left, rightArg)
	}
	if p, x := m.RefGet(proof); x != nil || !p.IsNull() {
		t.Fatal("proof-disabled output", p, x)
	}
}

// skolemizer::process builds mk_skolemization(mk_not(q), mk_not(r)); GCC
// evaluates the arguments right to left, so (not r) is created first.
func TestSkolemizerForallProofNegationOrder(t *testing.T) {
	m := definedNamesManager(t, true)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	pName, e := m.Symbols.Intern("skolem-order-p")
	definedNamesMust(t, e)
	pDecl, e := m.MkFuncDecl(pName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, pDecl)
	v0, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	body, e := m.MkApp(pDecl, []ast.ASTHandle{v0})
	definedNamesMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(12)}, Body: body})
	definedNamesMust(t, e)
	definedNamesPin(t, m, q)
	s, e := NewSkolemizer(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, s.Close()) })
	out, proof := nameExprOutputs(t, m)
	definedNamesMust(t, s.Apply(q, out, proof))
	result, e := m.RefGet(out)
	definedNamesMust(t, e)
	notQ, e := m.MkNot(q)
	definedNamesMust(t, e)
	notR, e := m.MkNot(result)
	definedNamesMust(t, e)
	qID, e := m.GetASTID(notQ)
	definedNamesMust(t, e)
	rID, e := m.GetASTID(notR)
	definedNamesMust(t, e)
	if rID >= qID {
		t.Fatal("(not r) must be built before (not q)", rID, qID)
	}
}
