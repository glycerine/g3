package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type qeNNFFixture struct {
	*fmFixture
	context    *SolverContext
	adapter    *SolverNNFAdapter
	normalizer *QENNFNormalizer
	x, y       ast.ASTHandle
}

func newQENNFFixture(t *testing.T) *qeNNFFixture {
	f := &qeNNFFixture{fmFixture: newFMFixture(t)}
	f.x = f.constant("qe-nnf-x", f.boolean)
	f.y = f.constant("qe-nnf-y", f.boolean)
	var err error
	f.context, err = NewSolverContext(f.r.manager, []ast.ASTHandle{f.x})
	fmMust(t, err)
	f.adapter, err = NewSolverNNFAdapter(f.context)
	fmMust(t, err)
	f.normalizer, err = NewQENNFNormalizer(f.r.manager, f.adapter, f.adapter, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() {
		fmMust(t, f.normalizer.Close())
		fmMust(t, f.adapter.Close())
		fmMust(t, f.context.Close())
	})
	return f
}

func qeNNFRef(t *testing.T, f *qeNNFFixture, formula ast.ASTHandle) ast.ASTRef {
	t.Helper()
	ref, err := f.r.manager.NewRef(formula)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, f.r.manager.DelRef(&ref)) })
	return ref
}

func assertQENNFConnectives(t *testing.T, manager *ast.ASTManager, root ast.ASTHandle) {
	t.Helper()
	todo := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(todo) > 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := manager.GetKind(current)
		fmMust(t, err)
		if kind != ast.ASTApp {
			continue
		}
		family, err := manager.GetFamilyID(current)
		fmMust(t, err)
		op, err := manager.GetDeclKind(current)
		fmMust(t, err)
		if family == manager.BasicFamilyID() {
			switch op {
			case ast.ASTOpImplies, ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpIte:
				t.Fatal("non-NNF connective survived", op)
			case ast.ASTOpNot:
				argument, cause := manager.GetArg(current, 0)
				fmMust(t, cause)
				argumentFamily, cause := manager.GetFamilyID(argument)
				fmMust(t, cause)
				argumentOp, cause := manager.GetDeclKind(argument)
				fmMust(t, cause)
				if argumentFamily == manager.BasicFamilyID() && (argumentOp == ast.ASTOpAnd || argumentOp == ast.ASTOpOr || argumentOp == ast.ASTOpNot) {
					t.Fatal("negation is not on an atom", argumentOp)
				}
			}
		}
		count, err := manager.GetNumArgs(current)
		fmMust(t, err)
		for index := uint32(0); index < count; index++ {
			argument, cause := manager.GetArg(current, index)
			fmMust(t, cause)
			todo = append(todo, argument)
		}
	}
}

func TestQENNFRelevantConnectivesAtomPolarityAndReuse(t *testing.T) {
	f := newQENNFFixture(t)
	notX := f.own(f.r.manager.MkNot(f.x))
	conjunction := f.own(f.r.manager.MkAnd(f.y, notX))
	implication := f.own(f.r.manager.MkImplies(f.x, conjunction))
	formula := f.own(f.r.manager.MkNot(implication))
	ref := qeNNFRef(t, f, formula)
	atoms, err := f.normalizer.Normalize(ref)
	fmMust(t, err)
	h, err := f.r.manager.RefGet(ref)
	fmMust(t, err)
	assertQENNFConnectives(t, f.r.manager, h)
	if positive, cause := atoms.Positive(f.x); cause != nil || !positive {
		t.Fatal("positive x atom missing", positive, cause)
	}
	if negative, cause := atoms.Negative(f.x); cause != nil || negative {
		t.Fatal("unexpected negative x atom", negative, cause)
	}
	fmMust(t, atoms.Close())

	iff := f.own(f.r.manager.MkIff(f.x, f.y))
	second := qeNNFRef(t, f, iff)
	atoms, err = f.normalizer.Normalize(second)
	fmMust(t, err)
	if positive, cause := atoms.Positive(f.x); cause != nil || !positive {
		t.Fatal("reused normalizer lost positive x", positive, cause)
	}
	if negative, cause := atoms.Negative(f.x); cause != nil || !negative {
		t.Fatal("reused normalizer lost negative x", negative, cause)
	}
	fmMust(t, atoms.Close())
}

func TestQENNFLeavesWhollyIrrelevantFormulaUntouched(t *testing.T) {
	f := newQENNFFixture(t)
	q := f.constant("qe-nnf-ground", f.boolean)
	formula := f.own(f.r.manager.MkImplies(f.y, q))
	ref := qeNNFRef(t, f, formula)
	atoms, err := f.normalizer.Normalize(ref)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	result, err := f.r.manager.RefGet(ref)
	fmMust(t, err)
	if result != formula {
		t.Fatal("irrelevant formula was rewritten", result, formula)
	}
	if positive, cause := atoms.Positive(f.y); cause != nil || positive {
		t.Fatal("irrelevant atom collected", positive, cause)
	}
}

func TestQENNFAtomIndexFeedsBooleanPolarizedSolver(t *testing.T) {
	f := newQENNFFixture(t)
	plugin, err := NewBoolPlugin(f.context)
	fmMust(t, err)
	defer func() { fmMust(t, plugin.Close()) }()
	formula := f.own(f.r.manager.MkOr(f.x, f.y))
	ref := qeNNFRef(t, f, formula)
	atoms, err := f.normalizer.Normalize(ref)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	normalized, err := f.r.manager.RefGet(ref)
	fmMust(t, err)
	conjs, err := NewConjunctionEnum(f.r.manager, normalized)
	fmMust(t, err)
	defer func() { fmMust(t, conjs.Close()) }()
	result, solved, err := plugin.Solve(f.adapter.contains[0], normalized, conjs, atoms)
	fmMust(t, err)
	if !solved || result == nil {
		t.Fatal("normalized atom index did not drive Boolean solve")
	}
	defer func() { fmMust(t, result.Close()) }()
	h, err := f.r.manager.RefGet(result.Formula)
	fmMust(t, err)
	if found, cause := f.adapter.contains[0].Check(h); cause != nil || found {
		t.Fatal("Boolean solve retained normalized target", found, cause)
	}
}
