package normal_forms

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func nnfOwner(t *testing.T, proofs bool, options NNFOptions) (*ast.ASTManager, *DefinedNames, *NNF) {
	t.Helper()
	m := definedNamesManager(t, proofs)
	names, e := NewDefinedNames(m)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, names.Close()) })
	n, e := NewNNF(m, names, options)
	definedNamesMust(t, e)
	t.Cleanup(func() { definedNamesMust(t, n.Close()) })
	return m, names, n
}

func TestNNFLabelsAndIntroducedDefinitions(t *testing.T) {
	options := DefaultNNFOptions()
	options.Mode = NNFFull
	m, _, n := nnfOwner(t, true, options)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	a := nameExprBoolConst(t, m, "label-nnf-a", b)
	labelName, e := m.Symbols.Intern("nnf-label")
	definedNamesMust(t, e)
	label, e := m.MkLabel(true, []usymbol.Symbol{labelName}, a)
	definedNamesMust(t, e)
	definedNamesPin(t, m, label)
	result, proof, _, _ := nnfTransform(t, m, n, label)
	if op, x := m.GetDeclKind(result); x != nil || op != ast.ASTOpAnd {
		t.Fatal("positive label expansion", op, x)
	}
	if proof.IsNull() {
		t.Fatal("label proof missing")
	}

	v0, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v0)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(7)}, Body: v0})
	definedNamesMust(t, e)
	definedNamesPin(t, m, q)
	fName, e := m.Symbols.Intern("nnf-uninterpreted")
	definedNamesMust(t, e)
	f, e := m.MkFuncDecl(fName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	definedNamesMust(t, e)
	definedNamesPin(t, m, f)
	root, e := m.MkApp(f, []ast.ASTHandle{q})
	definedNamesMust(t, e)
	definedNamesPin(t, m, root)
	result, proof, definitions, definitionProofs := nnfTransform(t, m, n, root)
	if result == root || proof.IsNull() || len(definitions) != 1 || len(definitionProofs) != 1 {
		t.Fatal("nested quantifier naming outputs", result, root, proof, len(definitions), len(definitionProofs))
	}
	definition, e := m.RefGet(definitions[0])
	definedNamesMust(t, e)
	if sort, x := m.GetSort(definition); x != nil || sort != b {
		t.Fatal("normalized definition sort", sort, x)
	}
}

func TestNNFMemoryLimitAndOptionGuards(t *testing.T) {
	options := DefaultNNFOptions()
	options.Mode = NNFFull
	m, _, n := nnfOwner(t, false, options)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	a := nameExprBoolConst(t, m, "memory-nnf", b)
	limited := options
	limited.MaxMemoryMB = 0
	definedNamesMust(t, n.UpdateOptions(limited))
	out, proof := nameExprOutputs(t, m)
	var definitions, definitionProofs []ast.ASTRef
	e = n.Transform(a, &definitions, &definitionProofs, out, proof)
	if e == nil || e.Error() != messages.MaxMemory {
		t.Fatal("NNF memory checkpoint", e)
	}
	if e = n.UpdateOptions(NNFOptions{Mode: NNFMode(17)}); !errors.Is(e, ast.ErrASTDomain) {
		t.Fatal("invalid mode", e)
	}
	definedNamesMust(t, n.UpdateOptions(options))
	m.Limit.Cancel()
	e = n.Transform(a, &definitions, &definitionProofs, out, proof)
	if e == nil || e.Error() != messages.Canceled {
		t.Fatal("NNF cancellation checkpoint", e)
	}
	m.Limit.ResetCancel()
	definedNamesMust(t, m.Limit.Push(1))
	if !m.Limit.Inc() {
		t.Fatal("NNF resource fixture exhausted its first step")
	}
	e = n.Transform(a, &definitions, &definitionProofs, out, proof)
	m.Limit.Pop()
	if e == nil || e.Error() != messages.MaxResource {
		t.Fatal("NNF resource-limit diagnostic", e)
	}
}

func nnfTransform(t *testing.T, m *ast.ASTManager, n *NNF, root ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, []ast.ASTRef, []ast.ASTRef) {
	t.Helper()
	out, proof := nameExprOutputs(t, m)
	var definitions, definitionProofs []ast.ASTRef
	definedNamesMust(t, n.Transform(root, &definitions, &definitionProofs, out, proof))
	t.Cleanup(func() {
		releaseNameExprRefs(t, m, definitionProofs)
		releaseNameExprRefs(t, m, definitions)
	})
	result, e := m.RefGet(out)
	definedNamesMust(t, e)
	p, e := m.RefGet(proof)
	definedNamesMust(t, e)
	return result, p, definitions, definitionProofs
}

func TestNNFFullConnectivesAndProof(t *testing.T) {
	options := DefaultNNFOptions()
	options.Mode = NNFFull
	m, _, n := nnfOwner(t, true, options)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	a := nameExprBoolConst(t, m, "nnf-a", b)
	bb := nameExprBoolConst(t, m, "nnf-b", b)
	notA, e := m.MkNot(a)
	definedNamesMust(t, e)
	definedNamesPin(t, m, notA)
	conjunction, e := m.MkAnd(bb, notA)
	definedNamesMust(t, e)
	definedNamesPin(t, m, conjunction)
	implication, e := m.MkImplies(a, conjunction)
	definedNamesMust(t, e)
	definedNamesPin(t, m, implication)
	root, e := m.MkNot(implication)
	definedNamesMust(t, e)
	definedNamesPin(t, m, root)
	notB, e := m.MkNot(bb)
	definedNamesMust(t, e)
	wantRight, e := m.MkOr(notB, a)
	definedNamesMust(t, e)
	want, e := m.MkAnd(a, wantRight)
	definedNamesMust(t, e)
	result, proof, definitions, definitionProofs := nnfTransform(t, m, n, root)
	if result != want || len(definitions) != 0 || len(definitionProofs) != 0 {
		t.Fatal("full connective NNF", result, want, len(definitions), len(definitionProofs))
	}
	if yes, x := m.IsProofKind(proof, ast.ASTPrNNFNeg); x != nil || !yes {
		t.Fatal("negative implication proof", yes, x)
	}
	fact, e := m.GetProofFact(proof)
	definedNamesMust(t, e)
	if right, x := m.GetArg(fact, 1); x != nil || right != want {
		t.Fatal("NNF proof result", right, x)
	}
}

func TestNNFModesIffXorITEAndGroundSkip(t *testing.T) {
	full := DefaultNNFOptions()
	full.Mode = NNFFull
	m, _, n := nnfOwner(t, false, full)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	a := nameExprBoolConst(t, m, "mode-a", b)
	bb := nameExprBoolConst(t, m, "mode-b", b)
	for _, makeRoot := range []func() (ast.ASTHandle, error){
		func() (ast.ASTHandle, error) { return m.MkIff(a, bb) },
		func() (ast.ASTHandle, error) { return m.MkXor(a, bb) },
		func() (ast.ASTHandle, error) { return m.MkIte(a, bb, a) },
	} {
		root, x := makeRoot()
		definedNamesMust(t, x)
		definedNamesPin(t, m, root)
		result, _, _, _ := nnfTransform(t, m, n, root)
		if op, x := m.GetDeclKind(result); x != nil || op != ast.ASTOpAnd {
			t.Fatal("expanded connective", op, x)
		}
	}
	skolem := DefaultNNFOptions()
	definedNamesMust(t, n.UpdateOptions(skolem))
	ground, e := m.MkImplies(a, bb)
	definedNamesMust(t, e)
	definedNamesPin(t, m, ground)
	result, _, _, _ := nnfTransform(t, m, n, ground)
	if result != ground {
		t.Fatal("skolem mode should skip ground formula", result, ground)
	}
}

func TestNNFQuantifierRetentionFlippingAndSkolemization(t *testing.T) {
	options := DefaultNNFOptions()
	options.Mode = NNFFull
	m, _, n := nnfOwner(t, true, options)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	v0, e := m.MkVar(0, b)
	definedNamesMust(t, e)
	definedNamesPin(t, m, v0)
	forall, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: v0})
	definedNamesMust(t, e)
	definedNamesPin(t, m, forall)
	result, _, _, _ := nnfTransform(t, m, n, forall)
	if kind, x := m.GetKind(result); x != nil || kind != ast.ASTQuantifier {
		t.Fatal("positive universal was not retained", kind, x)
	}
	if yes, x := m.IsForall(result); x != nil || !yes {
		t.Fatal("positive universal kind", yes, x)
	}

	exists, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: v0})
	definedNamesMust(t, e)
	definedNamesPin(t, m, exists)
	result, proof, _, _ := nnfTransform(t, m, n, exists)
	if kind, x := m.GetKind(result); x != nil || kind != ast.ASTApp {
		t.Fatal("positive existential was not skolemized", kind, x)
	}
	if yes, x := m.IsProofKind(proof, ast.ASTPrSkolemize); x != nil || !yes {
		t.Fatal("top-level skolemization proof", yes, x)
	}

	noSkolem := options
	noSkolem.Skolemize = false
	definedNamesMust(t, n.UpdateOptions(noSkolem))
	negated, e := m.MkNot(forall)
	definedNamesMust(t, e)
	definedNamesPin(t, m, negated)
	result, _, _, _ = nnfTransform(t, m, n, negated)
	if kind, x := m.GetKind(result); x != nil || kind != ast.ASTQuantifier {
		t.Fatal("disabled skolemization should retain quantifier", kind, x)
	}
	if yes, x := m.IsForall(result); x != nil || yes {
		t.Fatal("negative universal should flip to existential", yes, x)
	}
}

func TestNNFDeepTraversalCacheResetAndOwnerGuards(t *testing.T) {
	options := DefaultNNFOptions()
	options.Mode = NNFFull
	m, _, n := nnfOwner(t, false, options)
	b, e := m.MkBoolSort()
	definedNamesMust(t, e)
	base := nameExprBoolConst(t, m, "deep-nnf", b)
	root := base
	for i := 0; i < 4096; i++ {
		root, e = m.MkNot(root)
		definedNamesMust(t, e)
	}
	definedNamesPin(t, m, root)
	result, _, _, _ := nnfTransform(t, m, n, root)
	if result != base {
		t.Fatal("even deep negation chain", result, base)
	}
	shared, e := m.MkAnd(root, root)
	definedNamesMust(t, e)
	definedNamesPin(t, m, shared)
	nnfTransform(t, m, n, shared)
	populated := false
	for i := range n.caches {
		populated = populated || len(n.caches[i]) != 0
	}
	if !populated {
		t.Fatal("shared expression was not cached")
	}
	definedNamesMust(t, n.Reset())
	stillPopulated := false
	for i := range n.caches {
		stillPopulated = stillPopulated || len(n.caches[i]) != 0
	}
	if !stillPopulated {
		t.Fatal("Reset cleared persistent caches")
	}
	definedNamesMust(t, n.ResetCache())
	for i := range n.caches {
		if len(n.caches[i]) != 0 {
			t.Fatal("ResetCache retained entries", i, len(n.caches[i]))
		}
	}
	copyOwner := *n
	if e = copyOwner.Reset(); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("copied NNF owner accepted", e)
	}
	definedNamesMust(t, n.Close())
	definedNamesMust(t, n.Close())
	if e = n.Reset(); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("closed NNF owner accepted", e)
	}
}
