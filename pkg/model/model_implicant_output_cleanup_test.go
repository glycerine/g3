package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Real minimization outputs enter the same lexical-vector cleanup used for
// failed, unpublished results. The deleted-ID guard forces a consumed release
// error; this is Go owner recovery coverage, not a natural C++ failure claim.
func TestModelImplicantOwnsInterruptedOutputCleanup(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("requires source deleted-ID assertion")
	}
	for _, cone := range []bool{false, true} {
		for _, closing := range []bool{false, true} {
			for _, failedIndex := range []int{0, 1} {
				t.Run(fmt.Sprintf("cone=%t/close=%t/index=%d", cone, closing, failedIndex), func(t *testing.T) {
					m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
					arrayTableMust(t, m.EnableDebugRefCount())
					boolean, err := m.MkBoolSort()
					arrayTableMust(t, err)
					no, err := m.MkFalse()
					arrayTableMust(t, err)
					declarations := make([]ast.ASTHandle, 2)
					for n := range declarations {
						symbol, err := m.Symbols.Intern(fmt.Sprintf("output-stop-%d", n))
						arrayTableMust(t, err)
						declarations[n], err = m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
						arrayTableMust(t, err)
						arrayTableMust(t, md.Core().RegisterConstant(declarations[n], no))
					}
					before := m.Stats()
					var inputs []ast.ASTRef
					defer func() {
						for n := range inputs {
							if inputs[n] != (ast.ASTRef{}) {
								arrayTableMust(t, m.DelRef(&inputs[n]))
							}
						}
					}()
					formulas := make([]ast.ASTHandle, 2)
					for n, decl := range declarations {
						variable, err := m.MkApp(decl, nil)
						arrayTableMust(t, err)
						formulas[n], err = m.MkNot(variable)
						arrayTableMust(t, err)
						pin, err := m.NewRef(formulas[n])
						arrayTableMust(t, err)
						inputs = append(inputs, pin)
					}
					var results []ast.ASTRef
					if cone {
						results, err = implicant.MinimizeModel(formulas, md)
					} else {
						results, err = implicant.MinimizeLiterals(formulas, md)
					}
					arrayTableMust(t, err)
					defer func() { arrayTableMust(t, CloseImplicantResults(m, &results)) }()
					if len(results) != 2 {
						t.Fatal("expected two real minimization outputs", len(results))
					}
					roots := make([]ast.ASTHandle, 2)
					for n, ref := range results {
						roots[n], err = m.RefGet(ref)
						arrayTableMust(t, err)
					}
					for n := range inputs {
						arrayTableMust(t, m.DelRef(&inputs[n]))
					}
					for _, root := range roots {
						count, err := m.GetRefCount(root)
						arrayTableMust(t, err)
						if count != 1 {
							t.Fatal("output is not solely owned by its result vector", count)
						}
					}
					child, err := m.GetArg(roots[failedIndex], 0)
					arrayTableMust(t, err)
					id, err := m.GetASTID(roots[failedIndex])
					arrayTableMust(t, err)
					m.DebugFreeIndices[id] = struct{}{}
					defer delete(m.DebugFreeIndices, id)
					err = implicant.closeUnpublishedResults(&results)
					if !errors.Is(err, ast.ErrASTDomain) {
						t.Fatal("output release missed consumed-error guard", err)
					}
					if len(results) != 0 {
						t.Fatal("failed output scope did not transfer its remaining ownership")
					}
					interrupted := m.Stats()
					if interrupted.LiveReferences != before.LiveReferences+uint32(1-failedIndex) {
						t.Fatal("completed output prefix was not consumed", interrupted, before)
					}
					if failedIndex == 0 {
						count, err := m.GetRefCount(roots[1])
						arrayTableMust(t, err)
						if count != 1 {
							t.Fatal("unreached output was released", count)
						}
					}
					delete(m.DebugFreeIndices, id)
					if closing {
						arrayTableMust(t, implicant.Close())
					} else {
						arrayTableMust(t, implicant.reset())
					}
					count, err := m.GetRefCount(roots[failedIndex])
					arrayTableMust(t, err)
					childCount, err := m.GetRefCount(child)
					arrayTableMust(t, err)
					after := m.Stats()
					// The guard stops before freeing the root or decrementing its private
					// constant child; those two native nodes belong to manager teardown.
					if count != 0 || childCount != 1 || after.LiveNodes != before.LiveNodes+2 || after.LiveReferences != before.LiveReferences {
						t.Fatal("output cleanup retry lost owner bookkeeping", count, childCount, after, before)
					}
					if implicant.closed != closing || implicant.model != nil || implicant.evaluator != nil {
						t.Fatal("output retry did not finish implicant teardown")
					}
				})
			}
		}
	}
}

// The trace prefix occurs after the real output vector is built. Trigger the
// next operand-allocation refusal there and stop its first output's release.
type modelImplicantOutputStopWriter struct {
	manager         *ast.ASTManager
	variable, value ast.ASTHandle
	fired           bool
	guardID         uint32
	failure         error
}

func (w *modelImplicantOutputStopWriter) Write(data []byte) (int, error) {
	if !w.fired && strings.Contains(string(data), "pruned model:\n") {
		root, err := w.manager.MkEq(w.variable, w.value)
		if err == nil {
			var count uint32
			count, err = w.manager.GetRefCount(root)
			if err == nil && count != 1 {
				err = fmt.Errorf("output reference count = %d, want 1", count)
			}
		}
		if err == nil {
			w.guardID, err = w.manager.GetASTID(root)
		}
		if err != nil {
			w.failure = err
			return 0, err
		}
		w.fired = true
		w.manager.DebugFreeIndices[w.guardID] = struct{}{}
		w.manager.Budget().FailAfter(0)
	}
	return len(data), nil
}

func TestModelImplicantConeFailureKeepsUnpublishedOutputs(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("requires source deleted-ID assertion")
	}
	m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
	writer := &modelImplicantOutputStopWriter{manager: m}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("pdr_verbose")
	if !tags.ShouldTrace("pdr_verbose") {
		t.Skip("requires source TRACE")
	}
	implicant.services.Trace = tags
	arrayTableMust(t, m.EnableDebugRefCount())
	boolean, err := m.MkBoolSort()
	arrayTableMust(t, err)
	no, err := m.MkFalse()
	arrayTableMust(t, err)
	var pins []ast.ASTRef
	defer func() {
		for n := range pins {
			arrayTableMust(t, m.DelRef(&pins[n]))
		}
	}()
	formulas := make([]ast.ASTHandle, 2)
	for n := range formulas {
		symbol, err := m.Symbols.Intern(fmt.Sprintf("public-output-%d", n))
		arrayTableMust(t, err)
		variable, err := m.MkConst(symbol, boolean)
		arrayTableMust(t, err)
		if n == 0 {
			writer.variable = variable
			writer.value = no
		}
		decl, err := m.GetDecl(variable)
		arrayTableMust(t, err)
		arrayTableMust(t, md.Core().RegisterConstant(decl, no))
		formulas[n], err = m.MkNot(variable)
		arrayTableMust(t, err)
		pin, err := m.NewRef(formulas[n])
		arrayTableMust(t, err)
		pins = append(pins, pin)
	}
	before := m.Stats()
	result, err := implicant.MinimizeModel(formulas, md)
	m.Budget().FailAfter(-1)
	defer delete(m.DebugFreeIndices, writer.guardID)
	defer func() { arrayTableMust(t, CloseImplicantResults(m, &result)) }()
	if !writer.fired || writer.failure != nil {
		t.Fatal("did not reach the actual built-output trace", writer.fired, writer.failure)
	}
	if !errors.Is(err, arena.ErrInjectedFailure) || !errors.Is(err, ast.ErrASTDomain) || len(result) != 0 {
		t.Fatal("late failure did not preserve both errors and hide output", err, len(result))
	}
	if m.Stats().LiveReferences != before.LiveReferences+1 {
		t.Fatal("unreached output reference was not preserved", m.Stats(), before)
	}
	delete(m.DebugFreeIndices, writer.guardID)
	arrayTableMust(t, implicant.Close())
	after := m.Stats()
	if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes+1 {
		t.Fatal("public failure lost unpublished output ownership", after, before)
	}
}
