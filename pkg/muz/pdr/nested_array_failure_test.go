package pdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRNestedArrayEvaluationColdFailureCleanupAndRetry(t *testing.T) {
	for _, gap := range []int{8, 16, 32} {
		t.Run(fmt.Sprintf("gap=%d", gap), func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 2048 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, owner, transformer, _, closeBase := closureFixture(t)
					defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
					c, err := NewContext(m, nil, 2, ReachableNoCache, owner.contexts.config)
					closureMust(t, err)
					defer func() { closureMust(t, c.Close()) }()
					var pins []ast.ASTRef
					defer func() { closureMust(t, closeASTRefs(m, pins)) }()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						closureMust(t, err)
						ref, err := m.NewRef(h)
						closureMust(t, err)
						pins = append(pins, ref)
						return h
					}
					boolean := own(m.MkBoolSort())
					truth, falsity := own(m.MkTrue()), own(m.MkFalse())
					indices := make([]ast.ASTHandle, 16)
					for j := range indices {
						indices[j] = own(m.ArithmeticIntegerNumeralText(fmt.Sprint(j)))
					}
					integer, err := m.GetSort(indices[0])
					closureMust(t, err)
					name, err := m.Symbols.Intern("nested-failure-function")
					closureMust(t, err)
					function := own(m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{}))
					graph, err := model.NewFunctionInterpretation(m, 1)
					closureMust(t, err)
					closureMust(t, graph.SetElse(falsity))
					for j, index := range indices {
						value := falsity
						if j%2 == 0 {
							value = truth
						}
						closureMust(t, graph.InsertEntry([]ast.ASTHandle{index}, value))
					}
					md, err := model.NewModel(m)
					closureMust(t, err)
					defer func() { closureMust(t, md.Close()) }()
					closureMust(t, md.Core().RegisterFunction(function, graph))
					inner := own(m.MkAsArray(function))
					innerSort, err := m.GetSort(inner)
					closureMust(t, err)
					innerBase := own(m.MkConstArray(innerSort, falsity))
					outerSort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), innerSort))
					outerBase := own(m.MkConstArray(outerSort, innerBase))
					family, err := m.ArrayFamilyID()
					closureMust(t, err)
					input := outerBase
					for _, index := range indices {
						input = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{input, index, inner}))
					}
					variable := own(m.MkFreshConst("nested-failure-array", outerSort))
					decl, err := m.GetDecl(variable)
					closureMust(t, err)
					closureMust(t, md.Core().RegisterConstant(decl, input))
					// Force reference growth at different nested evaluator depths.
					var padding []ast.ASTRef
					for n := 0; n < 64; n++ {
						ref, err := m.NewRef(ast.ASTHandle{})
						closureMust(t, err)
						padding = append(padding, ref)
					}
					m.Budget().FailAfter(0)
					var pressureErr error
					for len(padding) < 16384 {
						ref, err := m.NewRef(ast.ASTHandle{})
						if err != nil {
							pressureErr = err
							break
						}
						padding = append(padding, ref)
					}
					m.Budget().FailAfter(-1)
					if !errors.Is(pressureErr, arena.ErrInjectedFailure) {
						t.Fatal("missing reference-arena pressure", pressureErr)
					}
					for n := 0; n < gap; n++ {
						last := len(padding) - 1
						closureMust(t, m.DelRef(&padding[last]))
						padding = padding[:last]
					}
					pins = append(pins, padding...)
					before := m.Stats().LiveReferences
					beforeNodes := m.Stats().LiveNodes
					innerCount, err := m.GetRefCount(inner)
					closureMust(t, err)
					inputCount, err := m.GetRefCount(input)
					closureMust(t, err)
					m.Budget().FailAfter(stop)
					result, err := c.implicant.EvalExpr(md, variable)
					m.Budget().FailAfter(-1)
					if result != (ast.ASTRef{}) {
						closureMust(t, m.DelRef(&result))
					}
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							closureMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected nested evaluation failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					check := func() {
						t.Helper()
						if got := m.Stats().LiveReferences; got != before {
							t.Fatal("nested evaluation leaked reference slots", got, before)
						}
						if got := m.Stats().LiveNodes; got != beforeNodes {
							t.Fatal("nested evaluation leaked AST nodes", got, beforeNodes)
						}
						for _, entry := range []struct {
							h     ast.ASTHandle
							count uint32
						}{{inner, innerCount}, {input, inputCount}} {
							count, err := m.GetRefCount(entry.h)
							closureMust(t, err)
							if count != entry.count {
								t.Fatal("nested evaluation leaked source ownership", count, entry.count)
							}
						}
					}
					check()
					retry, err := c.implicant.EvalExpr(md, variable)
					closureMust(t, err)
					defer func() {
						if retry != (ast.ASTRef{}) {
							closureMust(t, m.DelRef(&retry))
						}
					}()
					actual, err := m.RefGet(retry)
					closureMust(t, err)
					normalized := innerBase
					for j := 14; j >= 0; j -= 2 {
						normalized, err = m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{normalized, indices[j], truth})
						closureMust(t, err)
					}
					expected := outerBase
					for _, index := range indices {
						expected, err = m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{expected, index, normalized})
						closureMust(t, err)
					}
					if actual != expected {
						t.Fatal("retry lost nested rows or source store order")
					}
					closureMust(t, m.DelRef(&retry))
					check()
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("missing nested evaluation failure/success frontier", failures, succeeded)
			}
		})
	}
}
