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

func TestPDRImplicantPartialOutputFailureCleanupAndRetry(t *testing.T) {
	for _, cone := range []bool{false, true} {
		t.Run(fmt.Sprintf("cone=%t", cone), func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 2048 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, owner, transformer, _, closeBase := closureFixture(t)
					config := owner.contexts.config
					closureMust(t, transformer.Close())
					closureMust(t, owner.Close())
					c, err := NewContext(m, nil, 2, ReachableNoCache, config)
					closureMust(t, err)
					defer func() { closureMust(t, c.Close()); closeBase() }()
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
					zero := own(m.ArithmeticIntegerNumeralText("0"))
					integer, err := m.GetSort(zero)
					closureMust(t, err)
					md, err := model.NewModel(m)
					closureMust(t, err)
					defer func() { closureMust(t, md.Close()) }()
					variables := make([]ast.ASTHandle, 32)
					formulas := make([]ast.ASTHandle, len(variables))
					for i := range variables {
						variables[i] = own(m.MkFreshConst("pdr-output-failure", integer))
						rhs := own(m.ArithmeticIntegerNumeralText(fmt.Sprint(i)))
						value := own(m.ArithmeticIntegerNumeralText(fmt.Sprint(i + 1)))
						decl, err := m.GetDecl(variables[i])
						closureMust(t, err)
						closureMust(t, md.Core().RegisterConstant(decl, value))
						formulas[i] = own(m.MkNot(own(m.MkEq(variables[i], rhs))))
					}
					// Leave a bounded reference-arena gap so setup can progress
					// into a partially published output before the next growth.
					var padding []ast.ASTRef
					for n := 0; n < 1024; n++ {
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
						t.Fatal("missing reference-arena pressure boundary", pressureErr)
					}
					for n := 0; n < 48; n++ {
						last := len(padding) - 1
						closureMust(t, m.DelRef(&padding[last]))
						padding = padding[:last]
					}
					pins = append(pins, padding...)
					counts := make([]uint32, len(variables))
					for i, variable := range variables {
						counts[i], err = m.GetRefCount(variable)
						closureMust(t, err)
					}
					before := m.Stats().LiveReferences
					minimize := func() ([]ast.ASTRef, error) {
						if cone {
							return c.implicant.MinimizeModel(formulas, md)
						}
						return c.implicant.MinimizeLiterals(formulas, md)
					}
					m.Budget().FailAfter(stop)
					result, err := minimize()
					m.Budget().FailAfter(-1)
					closureMust(t, model.CloseImplicantResults(m, &result))
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							closureMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected minimization failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					check := func() {
						t.Helper()
						if got := m.Stats().LiveReferences; got != before {
							t.Fatal("partial output leaked reference slots", got, before)
						}
						for i, variable := range variables {
							count, err := m.GetRefCount(variable)
							closureMust(t, err)
							if count != counts[i] {
								t.Fatal("partial output leaked variable ownership", i, count, counts[i])
							}
						}
					}
					check()
					retry, err := minimize()
					closureMust(t, err)
					defer func() { closureMust(t, model.CloseImplicantResults(m, &retry)) }()
					if len(retry) != len(variables) {
						t.Fatal("retry lost selected literals", len(retry), len(variables))
					}
					// Cone output follows model insertion order; literal collection pops roots.
					for i, ref := range retry {
						j := len(variables) - 1 - i
						if cone {
							j = i
						}
						root, err := m.RefGet(ref)
						closureMust(t, err)
						left, err := m.GetArg(root, 0)
						closureMust(t, err)
						right, err := m.GetArg(root, 1)
						closureMust(t, err)
						expected, err := m.ArithmeticIntegerNumeralText(fmt.Sprint(j))
						closureMust(t, err)
						if cone {
							expected, err = m.ArithmeticIntegerNumeralText(fmt.Sprint(j + 1))
							closureMust(t, err)
							if left != variables[j] || right != expected {
								t.Fatal("wrong cone equality", i)
							}
						} else if left != expected || right != variables[j] {
							t.Fatal("wrong strict disequality witness", i)
						}
						var expectedRoot ast.ASTHandle
						if cone {
							expectedRoot, err = m.MkEq(left, right)
						} else {
							expectedRoot, err = m.MkArithmeticLT(left, right)
						}
						closureMust(t, err)
						if root != expectedRoot {
							t.Fatal("wrong minimized relation", i)
						}
					}
					closureMust(t, model.CloseImplicantResults(m, &retry))
					check()
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("missing failure/success frontier", failures, succeeded)
			}
		})
	}
}
