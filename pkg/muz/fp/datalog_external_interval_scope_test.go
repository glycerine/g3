package fp

// Source: rel_context::scoped_query keeps original relations as STORE
// destinations while temporary filter relations belong to the query scope.

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type externalIntervalScopeFixture struct {
	manager   *ast.ASTManager
	engine    *PropositionalEngine
	callbacks *externalIntervalCallbackState
	source    *externalIntervalDatalogActivation
	compiler  *externalIntervalDatalogCompiler
	helper    externalIntervalDatalogPredicate
	p, q, h   ast.ASTHandle
}

func newExternalIntervalScopeFixture(t *testing.T, kinds []string, matches bool) *externalIntervalScopeFixture {
	t.Helper()
	m, e, callbacks := newExternalIntervalDatalogFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	sorts := []ast.ASTHandle{integer, integer}
	f := &externalIntervalScopeFixture{manager: m, engine: e, callbacks: callbacks}
	f.p = declareExternalIntervalPredicate(t, m, e, "scope-P", sorts, kinds)
	f.q = declareExternalIntervalPredicate(t, m, e, "scope-Q", sorts, kinds)
	f.h = declareExternalIntervalPredicate(t, m, e, "scope-H", sorts, kinds)
	for _, fact := range []struct {
		declaration ast.ASTHandle
		values      []int
	}{{f.p, []int{2, 2}}, {f.q, []int{2, 10}}} {
		if fact.declaration == f.p && !matches {
			fact.values[1] = 3
		}
		values := make([]ast.ASTHandle, len(fact.values))
		roots := make([]ast.ASTRef, len(values))
		for i, value := range fact.values {
			values[i], err = m.ArithmeticIntegerNumeralText(strconv.Itoa(value))
			fragmentMust(t, err)
			roots[i], err = m.NewRef(values[i])
			fragmentMust(t, err)
		}
		fragmentMust(t, e.AddFact(fact.declaration, values))
		for i := range roots {
			fragmentMust(t, m.DelRef(&roots[i]))
		}
	}
	f.source = e.externalIntervalDatalog
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	pxx, err := m.MkApp(f.p, []ast.ASTHandle{x, x})
	fragmentMust(t, err)
	qxy, err := m.MkApp(f.q, []ast.ASTHandle{x, y})
	fragmentMust(t, err)
	hxy, err := m.MkApp(f.h, []ast.ASTHandle{x, y})
	fragmentMust(t, err)
	raw, err := e.rules.NewRule(hxy, []ast.ASTHandle{pxx, qxy}, nil, "scope-filter-join")
	fragmentMust(t, err)
	fragmentMust(t, e.set.Add(raw))
	fragmentMust(t, raw.Close())
	localRules, err := base.NewRuleManager(m)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, localRules.Close()) })
	for _, declaration := range e.predicateOrder {
		fragmentMust(t, localRules.RegisterPredicate(declaration))
	}
	localSet, err := base.NewRuleSet(localRules)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, localSet.Close()) })
	original, err := e.set.Rule(0)
	fragmentMust(t, err)
	fragmentMust(t, localSet.Add(original))
	fragmentMust(t, original.Close())
	plugin := transforms.NewFilterRulesPlugin(m, localRules, 2000)
	t.Cleanup(func() { fragmentMust(t, plugin.Close()) })
	filtered, err := plugin.Transform(localSet)
	fragmentMust(t, err)
	if filtered == nil || len(plugin.FreshPredicates()) != 1 {
		t.Fatal("scope fixture did not generate a real repeated-column filter")
	}
	t.Cleanup(func() { fragmentMust(t, filtered.Close()) })
	filter := plugin.FreshPredicates()[0]
	origin, found := plugin.FreshPredicateOrigin(filter)
	if !found || origin != f.p {
		t.Fatal("scope helper has the wrong source origin")
	}
	domain, err := m.GetDomain(filter, 0)
	fragmentMust(t, err)
	f.helper = externalIntervalDatalogPredicate{declaration: filter, sorts: []ast.ASTHandle{domain}, kinds: append([]string(nil), kinds...)}
	f.compiler = e.externalIntervalCompiler()
	f.compiler.set = filtered
	f.compiler.predicates[filter] = &f.helper
	return f
}

func TestExternalIntervalScopePublishesOriginalsAndRebuildsHelpers(t *testing.T) {
	for _, kinds := range [][]string{{"external_relation", "interval_relation"}, {"interval_relation", "external_relation"}} {
		for _, matches := range []bool{false, true} {
			t.Run(kinds[0]+"/matches="+strconv.FormatBool(matches), func(t *testing.T) {
				f := newExternalIntervalScopeFixture(t, kinds, matches)
				originals := make(map[ast.ASTHandle]*externalIntervalDatalogEntry)
				roots := make(map[ast.ASTHandle]ast.ASTRef)
				for declaration, entry := range f.source.entries {
					originals[declaration], roots[declaration] = entry, entry.root
				}
				originalOrder := append([]ast.ASTHandle(nil), f.source.order...)
				for run := 0; run < 2; run++ {
					if run == 1 {
						// Facts change after the old helper has been destroyed.
						// The new filter must derive its bounds from current P.
						for _, fact := range []struct {
							declaration ast.ASTHandle
							values      []int
						}{{f.p, []int{5, 5}}, {f.q, []int{5, 20}}} {
							values := make([]ast.ASTHandle, 2)
							roots := make([]ast.ASTRef, len(values))
							for i, value := range fact.values {
								var err error
								values[i], err = f.manager.ArithmeticIntegerNumeralText(strconv.Itoa(value))
								fragmentMust(t, err)
								roots[i], err = f.manager.NewRef(values[i])
								fragmentMust(t, err)
							}
							fragmentMust(t, f.source.addFact(fact.declaration, values))
							for i := range roots {
								fragmentMust(t, f.manager.DelRef(&roots[i]))
							}
						}
					}
					program, err := f.compiler.externalIntervalProgramPlan()
					fragmentMust(t, err)
					resetExternalIntervalUnaryCallbacks(f.callbacks)
					scope, err := newExternalIntervalDatalogScope(f.source, []externalIntervalDatalogPredicate{f.helper}, program)
					fragmentMust(t, err)
					t.Cleanup(func() { fragmentMust(t, scope.close()) })
					assertExternalIntervalRegisterOperations(t, f.callbacks.operations, []int32{ast.DatalogOpEmpty})
					for declaration, original := range originals {
						if scope.entries[declaration] != original || scope.borrowedEntries[declaration] != original {
							t.Fatal("scope cloned an original activation entry")
						}
					}
					helperEntry := scope.entries[f.helper.declaration]
					helperRoot := helperEntry.root
					empty, err := helperEntry.product.Empty()
					fragmentMust(t, err)
					if !empty {
						t.Fatal("new query scope imported a stale helper result")
					}
					resetExternalIntervalUnaryCallbacks(f.callbacks)
					fragmentMust(t, scope.executeExternalIntervalProgram())
					status, err := scope.query([]ast.ASTHandle{f.h})
					fragmentMust(t, err)
					want := lbool.Undef
					if run == 0 && !matches {
						want = lbool.False
					}
					if status != want {
						t.Fatal("scoped repeated-column filter result", run, status, want)
					}
					clones := 3 // Two original LOADs and the filter mutation clone.
					if run == 1 && matches {
						clones++ // The previously published H is now nonempty.
					}
					operations := make([]int32, clones)
					for i := range operations {
						operations[i] = ast.DatalogOpClone
					}
					operations = append(operations, ast.DatalogOpFilter)
					if want == lbool.Undef {
						operations = append(operations, ast.DatalogOpProject, ast.DatalogOpUnion, ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpUnion)
						if !reflect.DeepEqual(f.callbacks.joinParameters, [][]uint32{{0, 0}}) || !reflect.DeepEqual(f.callbacks.projectParameters, [][]uint32{{0}, {0}}) {
							t.Fatal("scope changed the source filter/join columns", f.callbacks.joinParameters, f.callbacks.projectParameters)
						}
					}
					var actual []int32
					for _, operation := range f.callbacks.operations {
						switch operation {
						case ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpJoin, ast.DatalogOpUnion, ast.DatalogOpStore:
							actual = append(actual, operation)
						}
					}
					assertExternalIntervalRegisterOperations(t, actual, operations)
					if !reflect.DeepEqual(f.callbacks.filterColumns, [][]uint32{{0, 1}}) {
						t.Fatal("scope omitted repeated-column filtering", f.callbacks.filterColumns)
					}
					if want == lbool.Undef {
						lower, upper, yUpper := "2", "2", "10"
						if run == 1 {
							upper, yUpper = "5", "20"
							if !matches {
								lower = "3"
							}
						}
						assertExternalIntervalGroundProgramInterval(t, f.source.entries[f.h].product, 0, lower, upper)
						assertExternalIntervalGroundProgramInterval(t, f.source.entries[f.h].product, 1, "10", yUpper)
					}
					helperProduct := helperEntry.product
					fragmentMust(t, scope.close())
					fragmentMust(t, scope.close())
					if helperEntry.product != nil || helperEntry.root != (ast.ASTRef{}) {
						t.Fatal("scope close left a helper owner")
					}
					if _, err = helperProduct.Sorts(); !errors.Is(err, rel.ErrTableRelation) {
						t.Fatal("scope close did not destroy its helper product", err)
					}
					if _, err = f.manager.RefGet(helperRoot); !errors.Is(err, ast.ErrASTHandle) {
						t.Fatal("scope close did not release its helper declaration reference", err)
					}
					for declaration, original := range originals {
						if f.source.entries[declaration] != original || original.root != roots[declaration] {
							t.Fatal("scope changed an original entry or declaration owner")
						}
						_, err = original.product.Sorts()
						fragmentMust(t, err)
						_, err = f.manager.RefGet(original.root)
						fragmentMust(t, err)
					}
					if f.source.closed || len(f.source.entries) != 3 || !reflect.DeepEqual(f.source.order, originalOrder) || f.source.program != nil || f.engine.predicates[f.helper.declaration] != nil {
						t.Fatal("temporary execution changed persistent scope metadata")
					}
					status, err = f.source.query([]ast.ASTHandle{f.h})
					fragmentMust(t, err)
					if status != want {
						t.Fatal("scope did not publish H in its source entry", status, want)
					}
				}
			})
		}
	}
}
