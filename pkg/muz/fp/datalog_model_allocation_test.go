package fp

// Finite-table model export is a Go extension of the source Datalog engine's
// empty model. Preserve that existing behavior while testing its concrete
// model, function interpretation, and decoded-fact ownership scopes.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestDatalogModelAllocationFailureAndRetry(t *testing.T) {
	for _, expression := range []bool{false, true} {
		for _, withBits := range []bool{false, true} {
			t.Run(fmt.Sprintf("expression=%t/bits=%t", expression, withBits), func(t *testing.T) {
				failures, completed := 0, false
				for allowance := int64(0); allowance < 256 && !completed; allowance++ {
					if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
						f := newQueryVariablesOwnershipFixture(t)
						fragmentMust(t, f.e.SelectEngine("datalog"))
						boolean := f.own(f.m.MkBoolSort())
						truth, falsity := f.own(f.m.MkTrue()), f.own(f.m.MkFalse())
						x := f.own(f.m.MkVar(0, boolean))
						p, query := f.predicate("model-allocation-P", x)
						fragmentMust(t, f.e.AddFact(p, []ast.ASTHandle{truth}))
						graphs := map[ast.ASTHandle][][]ast.ASTHandle{p: {{truth}}}
						if withBits {
							bits := f.own(f.m.BitVectorSort(4))
							y := f.own(f.m.MkVar(1, bits))
							r, application := f.predicate("model-allocation-R", x, y)
							one := f.own(f.m.BitVectorNumeralUint64(1, 4))
							three := f.own(f.m.BitVectorNumeralUint64(3, 4))
							graphs[r] = [][]ast.ASTHandle{{truth, one}, {falsity, three}}
							for _, row := range graphs[r] {
								fragmentMust(t, f.e.AddFact(r, row))
							}
							query = application
						}
						originalOmitted, _ := f.predicate("model-allocation-omitted")
						var status lbool.LBool
						var err error
						if expression {
							status, err = f.e.QueryExpression(query)
						} else {
							status, err = f.e.QueryPredicate(originalOmitted)
						}
						fragmentMust(t, err)
						if expression && status != lbool.True || !expression && status != lbool.False {
							t.Fatal("unexpected finite query status", status)
						}
						omitted := originalOmitted
						if expression {
							omitted = f.e.queryClonePredicate
						}
						export := func() (*model.Model, error) {
							if expression {
								return f.e.ExpressionModel()
							}
							return f.e.Model(originalOmitted)
						}
						checkModel := func(md *model.Model) {
							t.Helper()
							present, err := md.Core().HasInterpretation(omitted)
							fragmentMust(t, err)
							if present {
								t.Fatal("model included the exact omitted query declaration")
							}
							wantCount := len(graphs)
							if expression {
								wantCount++
								value, err := md.Core().ConstantInterpretation(originalOmitted)
								fragmentMust(t, err)
								if value != falsity {
									t.Fatal("synthetic query filtering also removed the original empty predicate", value)
								}
							}
							count, err := md.Core().NumDeclarations()
							fragmentMust(t, err)
							if count != uint32(wantCount) {
								t.Fatal("model changed its declaration set", count, wantCount)
							}
							for declaration, rows := range graphs {
								interpretation, err := md.Core().FunctionInterpretation(declaration)
								fragmentMust(t, err)
								if interpretation == nil || interpretation.NumEntries() != uint32(len(rows)) || interpretation.Arity() != uint32(len(rows[0])) {
									t.Fatal("model changed its finite graph", declaration, interpretation)
								}
								fallback, err := interpretation.Else()
								fragmentMust(t, err)
								if fallback != falsity {
									t.Fatal("finite graph lost its false Else", fallback)
								}
								seen := make([]bool, len(rows))
								for i := uint32(0); i < interpretation.NumEntries(); i++ {
									entry, err := interpretation.Entry(i)
									fragmentMust(t, err)
									value, err := entry.Result()
									fragmentMust(t, err)
									if value != truth {
										t.Fatal("finite graph entry is not true", value)
									}
									matched := -1
									for j, row := range rows {
										same := true
										for column, want := range row {
											actual, err := entry.Argument(uint32(column))
											fragmentMust(t, err)
											same = same && actual == want
										}
										if same {
											matched = j
										}
									}
									if matched < 0 || seen[matched] {
										t.Fatal("model introduced an unexpected or duplicate tuple", i)
									}
									seen[matched] = true
								}
							}
						}
						// Settle reusable AST reference pages. The model and every
						// function/entry header are still allocated anew on each export.
						warm, err := export()
						fragmentMust(t, err)
						checkModel(warm)
						fragmentMust(t, warm.Close())
						before, used := contextDisplayLiveCounts(f.m.Stats()), f.m.Budget().Used()
						clone, head, cloneRevision := f.e.queryClone, f.e.queryClonePredicate, f.e.queryCloneRevision
						revision, queryRevision, rules := f.e.revision, f.e.queryRevision, f.e.set
						checkUnchanged := func() {
							t.Helper()
							if after := contextDisplayLiveCounts(f.m.Stats()); after != before || f.m.Budget().Used() != used {
								t.Fatal("model export lost ownership", allowance, before, after, used, f.m.Budget().Used())
							}
							if f.e.queryClone != clone || f.e.queryClonePredicate != head || f.e.queryCloneRevision != cloneRevision || f.e.revision != revision || f.e.queryRevision != queryRevision || f.e.set != rules || !f.e.queryReady || f.e.lastResult != status {
								t.Fatal("model export changed the published query")
							}
						}
						f.m.Budget().FailAfter(allowance)
						md, cause := export()
						f.m.Budget().FailAfter(-1)
						if cause != nil {
							failures++
							if md != nil {
								fragmentMust(t, md.Close())
								t.Fatal("failed export published a partial model")
							}
							if !errors.Is(cause, arena.ErrInjectedFailure) {
								t.Fatal("model export lost allocation failure", cause)
							}
							var cleanup memory.ConstructionFailure
							if errors.As(cause, &cleanup) {
								fragmentMust(t, cleanup.Cleanup())
							}
						} else {
							completed = true
							checkModel(md)
							fragmentMust(t, md.Close())
						}
						checkUnchanged()
						md, err = export()
						fragmentMust(t, err)
						checkModel(md)
						fragmentMust(t, md.Close())
						fragmentMust(t, md.Close())
						checkUnchanged()
					}) {
						break
					}
				}
				if failures == 0 || !completed {
					t.Fatal("model campaign missed failure or completion", failures, completed)
				}
			})
		}
	}
}
