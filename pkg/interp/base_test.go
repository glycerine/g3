package interp

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestInterpolationBaseRangesScopesFramesAndOptions(t *testing.T) {
	m, manager := interpManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("base-p", boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("base-q", boolSort)
	interpMust(t, err)
	q = interpPin(t, m, q)
	theory, err := manager.MakeVariable("base-theory", boolSort)
	interpMust(t, err)
	theory = interpPin(t, m, theory)
	b, err := NewBase(manager, []ast.ASTHandle{p, q}, nil, []ast.ASTHandle{theory})
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, b.Close()) })

	if r, e := b.ASTRange(p); e != nil || r != (ScopeRange{Lo: 0, Hi: 0}) {
		t.Fatal("p range", r, e)
	}
	if r, e := b.ASTRange(theory); e != nil || !r.IsFull() {
		t.Fatal("theory range", r, e)
	}
	if r, e := b.ASTScope(p); e != nil || r != (ScopeRange{Lo: 0, Hi: 0}) {
		t.Fatal("p scope", r, e)
	}
	if local, e := b.IsLocal(p); e != nil || !local {
		t.Fatal("p local", local, e)
	}
	conjunction, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	if local, e := b.IsLocal(conjunction); e != nil || local {
		t.Fatal("cross-frame conjunction", local, e)
	}
	if frame, e := b.FrameOfAssertion(q); e != nil || frame != 1 {
		t.Fatal("q frame", frame, e)
	}
	if frame, e := b.FrameOfAssertion(theory); e != nil || frame <= 1 {
		t.Fatal("theory frame", frame, e)
	}
	if _, e := b.FrameOfAssertion(conjunction); e == nil || e.Error() != "unknown assertion" {
		t.Fatal("unknown assertion", e)
	}
	interpMust(t, b.SetOption("weak", "1"))
	if weak, e := b.WeakMode(); e != nil || !weak {
		t.Fatal("weak mode", weak, e)
	}
	assumption, err := b.ProofCheckAssumption(ScopeRange{Lo: 0, Hi: 1})
	interpMust(t, err)
	if found, e := manager.Occurs(theory, assumption); e != nil || !found {
		t.Fatal("proof assumption theory", found, e)
	}
	var out bytes.Buffer
	interpMust(t, b.Print(&out))
	if out.Len() == 0 {
		t.Fatal("empty base print")
	}
}

func TestInterpolationBaseSimplification(t *testing.T) {
	m, manager := interpManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("base-simplify-p", boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("base-simplify-q", boolSort)
	interpMust(t, err)
	q = interpPin(t, m, q)
	b, err := NewBase(manager, []ast.ASTHandle{p}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, b.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	nested, err := manager.Make(OperatorAnd, p, truth, p)
	interpMust(t, err)
	simplified, err := b.Simplify(nested)
	interpMust(t, err)
	if op, e := manager.Operator(simplified); e != nil || op != OperatorAnd {
		t.Fatal("flatten/deduplicate operator", op, e)
	}
	if count, e := manager.NumArgs(simplified); e != nil || count != 1 {
		t.Fatal("flatten/deduplicate count", count, e)
	}
	if only, e := manager.Arg(simplified, 0); e != nil || only != p {
		t.Fatal("flatten/deduplicate argument", only, e)
	}
	contradiction, err := manager.Make(OperatorAnd, p, notP)
	interpMust(t, err)
	simplified, err = b.Simplify(contradiction)
	interpMust(t, err)
	if no, e := manager.IsFalse(simplified); e != nil || !no {
		t.Fatal("contradiction", simplified, e)
	}

	equality, err := manager.Make(OperatorEqual, q, p)
	interpMust(t, err)
	canonical, err := b.Simplify(equality)
	interpMust(t, err)
	left, err := manager.Arg(canonical, 0)
	interpMust(t, err)
	right, err := manager.Arg(canonical, 1)
	interpMust(t, err)
	leftID, err := manager.ASTID(left)
	interpMust(t, err)
	rightID, err := manager.ASTID(right)
	interpMust(t, err)
	if leftID > rightID {
		t.Fatal("noncanonical equality", leftID, rightID)
	}
	if again, e := b.Simplify(equality); e != nil || again != canonical {
		t.Fatal("simplification memo", again, e)
	}
}

func TestInterpolationBaseCopyCloseAndGuards(t *testing.T) {
	_, manager := interpManagerFixture(t)
	b, err := NewBase(manager, nil, nil, nil)
	interpMust(t, err)
	copyBase := *b
	if _, err = copyBase.Manager(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied base", err)
	}
	interpMust(t, b.Close())
	if _, err = b.Manager(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed base", err)
	}
}

func TestInterpolationBaseTreeToParentsRepresentation(t *testing.T) {
	m, manager := interpManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	makeConstant := func(name string) ast.ASTHandle {
		t.Helper()
		h, e := manager.MakeVariable(name, boolSort)
		interpMust(t, e)
		return interpPin(t, m, h)
	}
	p, q, background := makeConstant("tree-p"), makeConstant("tree-q"), makeConstant("tree-background")
	markedQ, err := manager.Make(OperatorInterp, q)
	interpMust(t, err)
	tree, err := manager.Make(OperatorAnd, p, markedQ)
	interpMust(t, err)
	b, err := NewBase(manager, nil, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, b.Close()) })

	constraints, parents, theory, positions, err := b.ToParentsRepresentation([]ast.ASTHandle{p, q, background}, tree, false)
	interpMust(t, err)
	if len(constraints) != 2 || constraints[0] != q || constraints[1] != p {
		t.Fatal("tree constraints", constraints)
	}
	if len(parents) != 2 || parents[0] != 1 || parents[1] != ScopeTop {
		t.Fatal("tree parents", parents)
	}
	if len(theory) != 1 || theory[0] != background {
		t.Fatal("tree theory", theory)
	}
	if len(positions) != 1 || positions[0] != 0 {
		t.Fatal("tree position map", positions)
	}
}

// interp_01-F13: simplify_and advances past the element swapped in from the
// back, so that element is never examined.
func TestInterpolationBaseSimplifyConjunctionSkipsSwappedElement(t *testing.T) {
	m, manager := interpManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("base-simplify-swap-p", boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	b, err := NewBase(manager, []ast.ASTHandle{p}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, b.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	result, err := b.simplifyConjunction([]ast.ASTHandle{truth, p, truth})
	interpMust(t, err)
	if count, e := manager.NumArgs(result); e != nil || count != 2 {
		t.Fatal("[true p true] count", count, e)
	}
	if first, e := manager.Arg(result, 0); e != nil || first != truth {
		t.Fatal("[true p true] first argument", first, e)
	}
	if second, e := manager.Arg(result, 1); e != nil || second != p {
		t.Fatal("[true p true] second argument", second, e)
	}
	result, err = b.simplifyConjunction([]ast.ASTHandle{p, p, notP})
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorAnd {
		t.Fatal("[p p !p] is not an and", op, e)
	}
	if count, e := manager.NumArgs(result); e != nil || count != 2 {
		t.Fatal("[p p !p] count", count, e)
	}
	if first, e := manager.Arg(result, 0); e != nil || first != p {
		t.Fatal("[p p !p] first argument", first, e)
	}
	if second, e := manager.Arg(result, 1); e != nil || second != notP {
		t.Fatal("[p p !p] second argument", second, e)
	}
}
