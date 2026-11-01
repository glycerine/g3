package model

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A real shared arithmetic subexpression leaves 17 in the evaluator cache.
// The debug deleted-ID guard forces its consumed final release to stop.
func TestModelImplicantOwnsInterruptedTemporaryEvaluator(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("requires source deleted-ID assertion")
	}
	m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
	arrayTableMust(t, m.EnableDebugRefCount())
	var pins []ast.ASTRef
	defer func() {
		for n := range pins {
			arrayTableMust(t, m.DelRef(&pins[n]))
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		arrayTableMust(t, err)
		ref, err := m.NewRef(h)
		arrayTableMust(t, err)
		pins = append(pins, ref)
		return h
	}
	eight := own(m.ArithmeticIntegerNumeralText("8"))
	nine := own(m.ArithmeticIntegerNumeralText("9"))
	sixteen := own(m.ArithmeticIntegerNumeralText("16"))
	eighteen := own(m.ArithmeticIntegerNumeralText("18"))
	family, err := m.ArithmeticFamilyID()
	arrayTableMust(t, err)
	sum := own(m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{eight, nine}))
	lower := own(m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{sum, sixteen}))
	upper := own(m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{sum, eighteen}))
	formula := own(m.MkAnd(lower, upper))
	// Warm the manager's small numeral/rewriter caches with a complete
	// control evaluation before taking exact lifetime snapshots.
	control, err := NewModelEvaluator(md.Core(), implicant.services)
	arrayTableMust(t, err)
	controlResult, err := control.EvaluateValue(formula)
	arrayTableMust(t, err)
	arrayTableMust(t, m.DelRef(&controlResult))
	arrayTableMust(t, control.Close())
	before := m.Stats()
	arrayTableMust(t, implicant.setupModel(md))
	evaluator, err := NewModelEvaluator(md.Core(), implicant.services)
	arrayTableMust(t, err)
	defer func() { arrayTableMust(t, evaluator.Close()) }()
	result, err := evaluator.EvaluateValue(formula)
	arrayTableMust(t, err)
	h, err := m.RefGet(result)
	arrayTableMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("arithmetic fixture did not evaluate true")
	}
	arrayTableMust(t, m.DelRef(&result))
	intermediate, err := m.ArithmeticIntegerNumeralText("17")
	arrayTableMust(t, err)
	count, err := m.GetRefCount(intermediate)
	arrayTableMust(t, err)
	if count == 0 {
		t.Fatal("real evaluator did not own its arithmetic intermediate")
	}
	intermediateDeclaration, err := m.GetDecl(intermediate)
	arrayTableMust(t, err)
	declarationCount, err := m.GetRefCount(intermediateDeclaration)
	arrayTableMust(t, err)
	if declarationCount != 1 {
		t.Fatal("intermediate declaration has another owner", declarationCount)
	}
	id, err := m.GetASTID(intermediate)
	arrayTableMust(t, err)
	m.DebugFreeIndices[id] = struct{}{}
	defer delete(m.DebugFreeIndices, id)
	err = implicant.closeTemporaryEvaluator(evaluator)
	if !errors.Is(err, ast.ErrASTDomain) || evaluator.closed || evaluator.header == 0 {
		t.Fatal("evaluator did not preserve its interrupted owner", err, evaluator.closed, evaluator.header)
	}
	delete(m.DebugFreeIndices, id)
	arrayTableMust(t, implicant.reset())
	if !evaluator.closed || evaluator.header != 0 || len(implicant.pendingEvaluators) != 0 {
		t.Fatal("implicant lost its interrupted temporary evaluator")
	}
	after := m.Stats()
	if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes+2 {
		t.Fatal("evaluator retry lost reference ownership", after, before)
	}
	// The guard stops before freeing the numeral and decrementing its
	// private declaration. Both native nodes belong to manager teardown.
	count, err = m.GetRefCount(intermediate)
	arrayTableMust(t, err)
	if count != 0 {
		t.Fatal("guard-stopped intermediate still has an owner", count)
	}
	declarationCount, err = m.GetRefCount(intermediateDeclaration)
	arrayTableMust(t, err)
	if declarationCount != 1 {
		t.Fatal("stopped numeral's declaration lifetime changed", declarationCount)
	}

}
