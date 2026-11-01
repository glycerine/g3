package qe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyProjectionConstructorAllocationFrontiers(t *testing.T) {
	for _, name := range []string{"array", "arithmetic"} {
		t.Run(name, func(t *testing.T) {
			success, failures := false, 0
			for stop := int64(0); stop < 1024 && !success; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					f := newFMFixture(t)
					m := f.r.manager
					fmMust(t, m.RegisterDeclarationPlugins())
					m.Budget().FailAfter(stop)
					var closeOwner func() error
					var err error
					if name == "array" {
						var owner *legacyArrayProjection
						owner, err = newLegacyArrayProjection(m, f.r.config)
						if owner != nil {
							closeOwner = owner.Close
						}
					} else {
						var owner *legacyArithmeticProjection
						owner, err = newLegacyArithmeticProjection(m, f.r.config)
						if owner != nil {
							closeOwner = owner.Close
						}
					}
					m.Budget().FailAfter(-1)
					if closeOwner != nil {
						fmMust(t, closeOwner())
					}
					if err == nil {
						success = true
						return
					}
					var construction memory.ConstructionFailure
					if errors.As(err, &construction) {
						fmMust(t, construction.Cleanup())
					}
					var memout memory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
						t.Fatal(err)
					}
					failures++
				})
				if t.Failed() {
					t.Fatal("constructor allocation frontier", stop)
				}
			}
			if !success || failures == 0 {
				t.Fatal("did not reach both failure and success", failures, success)
			}
		})
	}
}

func TestLegacyArrayDirectEqualityAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 1024 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			p, err := newLegacyArrayProjection(m, f.r.config)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, p.Close()) })
			sort := f.own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{f.integer}), f.integer))
			a, b := f.constant("failure-array-a", sort), f.constant("failure-array-b", sort)
			formula := f.own(m.MkEq(a, b))
			base := f.own(m.MkConstArray(sort, f.number("0")))
			md, err := model.NewModel(m)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, md.Close()) })
			for _, variable := range []ast.ASTHandle{a, b} {
				decl, cause := m.GetDecl(variable)
				fmMust(t, cause)
				fmMust(t, md.Core().RegisterConstant(decl, base))
			}
			m.Budget().FailAfter(stop)
			result, eliminated, err := p.projectDirectEquality(md.Core(), a, formula)
			m.Budget().FailAfter(-1)
			if result != nil {
				fmMust(t, result.Close())
			}
			if err == nil {
				if !eliminated {
					t.Fatal("direct array equality was not eliminated")
				}
				success = true
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fmMust(t, construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal(err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("direct array equality allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("did not reach both failure and success", failures, success)
	}
}

func TestLegacyModelEvaluatorAllocationFrontiers(t *testing.T) {
	success, failures := false, 0
	for stop := int64(0); stop < 1024 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newFMFixture(t)
			m := f.r.manager
			fmMust(t, m.RegisterDeclarationPlugins())
			md, err := model.NewModel(m)
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, md.Close()) })
			config := model.ModelEvaluatorConfig{Context: f.r.config.Context, Globals: f.r.config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation}
			m.Budget().FailAfter(stop)
			evaluator, err := newLegacyModelEvaluator(md.Core(), config)
			m.Budget().FailAfter(-1)
			if evaluator != nil {
				fmMust(t, evaluator.Close())
			}
			if err == nil {
				success = true
				return
			}
			if evaluator != nil {
				t.Fatal("failed constructor returned an evaluator")
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				fmMust(t, construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal(err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("model evaluator allocation frontier", stop)
		}
	}
	if !success || failures == 0 {
		t.Fatal("did not reach both failure and success", failures, success)
	}
}
