package qe

// Source: src/muz/spacer/spacer_qe_project.cpp array_project_eqs_util.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArrayStoreConstructionFailuresAndRetry(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 2048 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			helperStart := m.Stats().LiveReferences
			p, err := newLegacyArrayProjection(m, f.r.config)
			fmMust(t, err)
			helperRefs := m.Stats().LiveReferences - helperStart
			arraySort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a := f.constant("legacy-partial-select-a", arraySort)
			b := f.constant("legacy-partial-select-b", arraySort)
			k := f.constant("legacy-partial-select-k", f.integer)
			zero := f.own(m.ArithmeticIntegerNumeralText("0"))
			one := f.own(m.ArithmeticIntegerNumeralText("1"))
			seven := f.own(m.ArithmeticIntegerNumeralText("7"))
			index := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, k}))
			storedA := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, index, seven}))
			formula := f.own(m.MkEq(storedA, b))
			base := f.own(m.MkConstArray(arraySort, zero))
			storedBase := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, seven}))
			expected := m.Stats().LiveReferences - helperRefs
			md, err := model.NewModel(m)
			fmMust(t, err)
			t.Cleanup(func() {
				fmMust(t, md.Close())
				fmMust(t, p.Close())
				if got := m.Stats().LiveReferences; got != expected {
					t.Errorf("temporary projection roots survived service disposal: got %d want %d", got, expected)
				}
			})
			for n, variable := range []ast.ASTHandle{a, b, k} {
				value := []ast.ASTHandle{base, storedBase, one}[n]
				decl, err := m.GetDecl(variable)
				fmMust(t, err)
				fmMust(t, md.Core().RegisterConstant(decl, value))
			}

			m.Budget().FailAfter(stop)
			result, eliminated, cause := p.projectStoreEquality(md.Core(), a, formula)
			m.Budget().FailAfter(-1)
			if cause != nil {
				if result != nil || eliminated {
					t.Fatal("failed projection published a result")
				}
				var construction memory.ConstructionFailure
				if errors.As(cause, &construction) {
					fmMust(t, construction.Cleanup())
				}
				var memout memory.Failure
				if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
					t.Fatal(cause)
				}
				failures++
				result, eliminated, err = p.projectStoreEquality(md.Core(), a, formula)
				fmMust(t, err)
			} else {
				success = true
			}
			if !eliminated || result == nil {
				t.Fatal("store projection did not eliminate array")
			}
			defer func() { fmMust(t, result.Close()) }()
			if len(result.variables) != 2 {
				t.Fatal("select/diff variable count", len(result.variables))
			}
			root, err := m.RefGet(result.formula)
			fmMust(t, err)
			contains, err := ast.NewContainsApp(m, a)
			fmMust(t, err)
			found, err := contains.Check(root)
			fmMust(t, err)
			fmMust(t, contains.Close())
			if found {
				t.Fatal("projected array survives retry")
			}
			evaluator, err := newLegacyModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
			fmMust(t, err)
			value, err := evaluator.EvaluateValue(root)
			fmMust(t, err)
			handle, err := m.RefGet(value)
			fmMust(t, err)
			if !m.IsTrue(handle) {
				t.Fatal("projection does not hold in model")
			}
			fmMust(t, m.DelRef(&value))
			fmMust(t, evaluator.Close())
		})
		if t.Failed() {
			t.Fatal("store construction allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("missing failure/success frontier", failures, success)
	}
	t.Log("first successful allowance", failures)
}

func TestLegacyPartialSelectConstructionFailuresAndRetry(t *testing.T) {
	for _, count := range []int{8, 64} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 2048 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newFMFixture(t)
					m := f.r.manager
					start := m.Stats().LiveReferences
					p, err := newLegacyArrayProjection(m, f.r.config)
					fmMust(t, err)
					helperRefs := m.Stats().LiveReferences - start
					sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
					a, b := f.constant("factor-a", sort), f.constant("factor-b", sort)
					zero := f.number("0")
					array := f.own(m.MkConstArray(sort, zero))
					indices := make([]ast.ASTHandle, count)
					for n := range indices {
						number := f.number(fmt.Sprint(n))
						array = f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, number, number}))
						indices[n] = f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, number}))
					}
					expected := m.Stats().LiveReferences - helperRefs
					partial, err := newLegacyPartialEquality(m, p.syntax.family, a, b, indices)
					fmMust(t, err)
					md, err := model.NewModel(m)
					fmMust(t, err)
					for _, variable := range []ast.ASTHandle{a, b} {
						decl, err := m.GetDecl(variable)
						fmMust(t, err)
						fmMust(t, md.Core().RegisterConstant(decl, array))
					}
					evaluator, err := newLegacyModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
					fmMust(t, err)
					fmMust(t, evaluator.SetModelCompletion(true))
					t.Cleanup(func() {
						fmMust(t, evaluator.Close())
						fmMust(t, md.Close())
						fmMust(t, partial.Close())
						fmMust(t, p.Close())
						if got := m.Stats().LiveReferences; got != expected {
							t.Errorf("factoring roots survived cleanup: got %d want %d", got, expected)
						}
					})
					m.Budget().FailAfter(stop)
					result, cause := p.factorPartialSelects(md.Core(), evaluator, a, partial)
					m.Budget().FailAfter(-1)
					if cause != nil {
						if result != nil {
							t.Fatal("failed factoring published a result")
						}
						var cleanup memory.ConstructionFailure
						if errors.As(cause, &cleanup) {
							fmMust(t, cleanup.Cleanup())
						}
						var memout memory.Failure
						if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
							t.Fatal(cause)
						}
						failures++
						// projectStoreEquality constructs a new evaluator for
						// each call; interrupted traversals are not reusable
						// without explicit cleanup/reset.
						fmMust(t, evaluator.Close())
						evaluator, err = newLegacyModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
						fmMust(t, err)
						fmMust(t, evaluator.SetModelCompletion(true))
						result, err = p.factorPartialSelects(md.Core(), evaluator, a, partial)
						fmMust(t, err)
					} else {
						success = true
					}
					defer func() { fmMust(t, result.Close()) }()
					if result.partial == nil || len(result.variables) != count || len(result.equalities) != count {
						t.Fatal("factoring output shape", len(result.variables), len(result.equalities))
					}
					for _, equality := range result.equalities {
						root, err := m.RefGet(equality)
						fmMust(t, err)
						value, err := evaluator.EvaluateValue(root)
						fmMust(t, err)
						handle, err := m.RefGet(value)
						fmMust(t, err)
						if !m.IsTrue(handle) {
							t.Fatal("factored definition false in model")
						}
						fmMust(t, m.DelRef(&value))
					}
				})
				if t.Failed() {
					t.Fatal("partial select construction frontier", stop)
				}
			}
			if !success || failures == 0 {
				t.Fatal("missing failure/success frontier", failures, success)
			}
			t.Log("first successful allowance", failures)
		})
	}
}

func TestLegacyArraySelectAndFormulaConstructionFailures(t *testing.T) {
	for _, kind := range []string{"selects", "formula"} {
		t.Run(kind, func(t *testing.T) {
			for _, count := range []int{8, 64} {
				t.Run(fmt.Sprint(count), func(t *testing.T) {
					success, failures := false, 0
					for stop := int64(0); stop < 2048 && !success; stop++ {
						t.Run(fmt.Sprint(stop), func(t *testing.T) {
							f := newFMFixture(t)
							m := f.r.manager
							start := m.Stats().LiveReferences
							p, err := newLegacyArrayProjection(m, f.r.config)
							fmMust(t, err)
							helperRefs := m.Stats().LiveReferences - start
							sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
							a := f.constant("project-many-selects", sort)
							array := f.own(m.MkConstArray(sort, f.number("0")))
							literals := make([]ast.ASTHandle, count)
							for n := range literals {
								num := f.number(fmt.Sprint(n))
								array = f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, num, num}))
								selected := f.own(m.MkFamilyApp(p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, num}))
								literals[n] = f.own(m.MkEq(selected, num))
							}
							formula := f.own(m.MkAndArray(literals))
							expected := m.Stats().LiveReferences - helperRefs
							md, err := model.NewModel(m)
							fmMust(t, err)
							decl, err := m.GetDecl(a)
							fmMust(t, err)
							fmMust(t, md.Core().RegisterConstant(decl, array))
							t.Cleanup(func() {
								fmMust(t, md.Close())
								fmMust(t, p.Close())
								if got := m.Stats().LiveReferences; got != expected {
									t.Errorf("projection roots survived cleanup: got %d want %d", got, expected)
								}
							})
							var selects *legacyArraySelectProjection
							var projected *legacyArrayProjectionResult
							run := func() error {
								var cause error
								if kind == "selects" {
									selects, cause = p.projectSelects(md.Core(), []ast.ASTHandle{a}, formula)
								} else {
									projected, cause = p.projectFormula(md.Core(), []ast.ASTHandle{a}, formula, false)
								}
								return cause
							}
							m.Budget().FailAfter(stop)
							cause := run()
							m.Budget().FailAfter(-1)
							if cause != nil {
								if selects != nil || projected != nil {
									t.Fatal("failed projection published result")
								}
								var cleanup memory.ConstructionFailure
								if errors.As(cause, &cleanup) {
									fmMust(t, cleanup.Cleanup())
								}
								var memout memory.Failure
								if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &memout) || memout.Kind != memory.FailureMemout) {
									t.Fatal(cause)
								}
								failures++
								fmMust(t, run())
							} else {
								success = true
							}
							var root ast.ASTRef
							var vars []ast.ASTRef
							if kind == "selects" {
								root, vars = selects.formula, selects.variables
								defer func() { fmMust(t, selects.Close()) }()
							} else {
								root, vars = projected.formula, projected.variables
								defer func() { fmMust(t, projected.Close()) }()
							}
							if len(vars) != count {
								t.Fatal("model value class count", len(vars), count)
							}
							handle, err := m.RefGet(root)
							fmMust(t, err)
							contains, err := ast.NewContainsApp(m, a)
							fmMust(t, err)
							found, err := contains.Check(handle)
							fmMust(t, err)
							fmMust(t, contains.Close())
							if found {
								t.Fatal("projected array survives")
							}
							evaluator, err := newLegacyModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation})
							fmMust(t, err)
							value, err := evaluator.EvaluateValue(handle)
							fmMust(t, err)
							actual, err := m.RefGet(value)
							fmMust(t, err)
							if !m.IsTrue(actual) {
								t.Fatal("projected formula false in model")
							}
							fmMust(t, m.DelRef(&value))
							fmMust(t, evaluator.Close())
						})
						if t.Failed() {
							t.Fatal("projection construction frontier", stop)
						}
					}
					if !success || failures == 0 {
						t.Fatal("missing failure/success frontier", failures, success)
					}
					t.Log("first successful allowance", failures)
				})
			}
		})
	}
}
