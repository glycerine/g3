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

func TestSpacerImplicantGeneratedTermsColdFailureCleanupAndRetry(t *testing.T) {
	for _, name := range []string{"false-distinct", "arithmetic-disequality", "reverse-arithmetic-disequality", "expanded-distinct"} {
		t.Run(name, func(t *testing.T) {
			succeeded, failures := false, 0
			for stop := int64(0); stop < 2048 && !succeeded; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					var budget *arena.Budget
					t.Cleanup(func() {
						if budget != nil && budget.Used() != 0 {
							t.Errorf("implicant fixture leaked %d budget bytes", budget.Used())
						}
					})
					m, globals := newSpacerTestServices(t)
					budget = m.Budget()
					owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
					depthMust(t, err)
					defer func() { depthMust(t, owner.Close()) }()
					var pins []ast.ASTRef
					defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						depthMust(t, err)
						ref, err := m.NewRef(h)
						depthMust(t, err)
						pins = append(pins, ref)
						return h
					}
					zero := own(m.ArithmeticIntegerNumeralText("0"))
					integer, err := m.GetSort(zero)
					depthMust(t, err)
					md, err := model.NewModel(m)
					depthMust(t, err)
					defer func() { depthMust(t, md.Close()) }()
					terms := make([]ast.ASTHandle, 8)
					for i := range terms {
						terms[i] = own(m.MkFreshConst("implicant-failure", integer))
						value := i
						if name == "reverse-arithmetic-disequality" && i < 2 {
							value = 1 - i
						}
						if name == "false-distinct" && i == 1 {
							value = 0
						}
						numeral := own(m.ArithmeticIntegerNumeralText(fmt.Sprint(value)))
						decl, err := m.GetDecl(terms[i])
						depthMust(t, err)
						depthMust(t, md.Core().RegisterConstant(decl, numeral))
					}
					var input ast.ASTHandle
					expectedCount := 1
					switch name {
					case "false-distinct":
						input = own(m.MkNot(own(m.MkDistinct(terms))))
					case "arithmetic-disequality", "reverse-arithmetic-disequality":
						input = own(m.MkNot(own(m.MkEq(terms[0], terms[1]))))
					case "expanded-distinct":
						input = own(m.MkDistinct(terms))
						expectedCount = 28
					}
					counts := make([]uint32, len(terms))
					for i, term := range terms {
						counts[i], err = m.GetRefCount(term)
						depthMust(t, err)
					}
					before := m.Stats().LiveReferences
					config := owner.contexts[0].config.ModelEvaluator
					m.Budget().FailAfter(stop)
					picked, err := computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{input})
					m.Budget().FailAfter(-1)
					depthMust(t, releaseNormalizeRefs(m, picked))
					if err != nil {
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							depthMust(t, cleanup.Cleanup())
						}
						var failure memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
							t.Fatal("unexpected implicant failure", err)
						}
						failures++
					} else {
						succeeded = true
					}
					check := func() {
						t.Helper()
						if got := m.Stats().LiveReferences; got != before {
							t.Fatal("implicant leaked reference slots", got, before)
						}
						for i, term := range terms {
							count, err := m.GetRefCount(term)
							depthMust(t, err)
							if count != counts[i] {
								t.Fatal("generated term leaked source ownership", i, count, counts[i])
							}
						}
					}
					check()
					retry, err := computeImplicantLiterals(m, md.Core(), config, []ast.ASTHandle{input})
					depthMust(t, err)
					defer func() { depthMust(t, releaseNormalizeRefs(m, retry)) }()
					if len(retry) != expectedCount {
						t.Fatal("retry lost implicant literals", len(retry), expectedCount)
					}
					if name != "expanded-distinct" {
						actual, err := m.RefGet(retry[0])
						depthMust(t, err)
						var expected ast.ASTHandle
						if name == "false-distinct" {
							expected, err = m.MkEq(terms[1], terms[0])
						} else if name == "reverse-arithmetic-disequality" {
							expected, err = m.MkArithmeticLT(terms[1], terms[0])
						} else {
							expected, err = m.MkArithmeticLT(terms[0], terms[1])
						}
						depthMust(t, err)
						if actual != expected {
							t.Fatal("retry produced wrong implicant")
						}
					}
					depthMust(t, releaseNormalizeRefs(m, retry))
					retry = nil
					check()
				})
				if t.Failed() {
					break
				}
			}
			if !succeeded || failures == 0 {
				t.Fatal("missing implicant failure/success frontier", failures, succeeded)
			}
		})
	}
}
