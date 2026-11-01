package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerImplicantEvaluatorConstructionFailureAndRetry(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf("direct=%v", direct), func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 512 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					m, globals := newSpacerTestServices(t)
					owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := owner.Close(); err != nil {
							t.Error(err)
						}
					}()
					md, err := model.NewModel(m)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := md.Close(); err != nil {
							t.Error(err)
						}
					}()
					truth, err := m.MkTrue()
					if err != nil {
						t.Fatal(err)
					}
					input, err := m.NewRef(truth)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := m.DelRef(&input); err != nil {
							t.Error(err)
						}
					}()
					config := owner.contexts[0].config.ModelEvaluator
					construct := func() ([]ast.ASTRef, error) {
						if !direct {
							return computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{truth})
						}
						evaluator, err := newSpacerModelEvaluator(md.Core(), config)
						if err != nil {
							if evaluator != nil {
								t.Fatal("failed constructor returned an owner")
							}
							return nil, err
						}
						return nil, evaluator.Close()
					}
					before := m.Stats().LiveReferences
					m.Budget().FailAfter(stop)
					result, err := construct()
					m.Budget().FailAfter(-1)
					if cleanup := releaseNormalizeRefs(m, result); cleanup != nil {
						t.Fatal(cleanup)
					}
					if err != nil {
						var construction memory.ConstructionFailure
						if errors.As(err, &construction) {
							if cleanup := construction.Cleanup(); cleanup != nil {
								t.Fatal(cleanup)
							}
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected construction failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					if got := m.Stats().LiveReferences; got != before {
						t.Fatalf("implicant references = %d, want %d", got, before)
					}
					result, err = construct()
					if err != nil {
						t.Fatal("retry", err)
					}
					if len(result) != 0 {
						t.Fatal("true input produced implicant literals", len(result))
					}
					if err = releaseNormalizeRefs(m, result); err != nil {
						t.Fatal(err)
					}
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("did not reach construction failure and success", failures, succeeded)
			}
		})
	}
}
