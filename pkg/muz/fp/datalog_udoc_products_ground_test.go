package fp

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func prepareUDocProductGroundRules(f *publicUDocProductFixture) {
	f.t.Helper()
	for _, option := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		fragmentMust(f.t, f.e.globals.SourceSetName(option, "false"))
	}
}

func (f *publicUDocProductFixture) addGroundRule(declaration ast.ASTHandle, name string, values ...ast.ASTHandle) {
	f.t.Helper()
	fragmentMust(f.t, f.e.AddRule(f.app(declaration, values...), name))
}

func TestDatalogUDocProductsGroundRulesUseActualSnapshots(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%v", tableKind, reverse), func(t *testing.T) {
				f := newPublicUDocProductFixture(t, tableKind, false, true)
				prepareUDocProductGroundRules(f)
				// The source dependency strata run these ground inputs first,
				// even though the consuming rule appeared before them.
				f.addRule(reverse)
				f.addGroundRule(f.doc, "doc-one", f.values[1])
				f.addGroundRule(f.finite, "finite-yes", f.yes)
				f.addGroundRule(f.doc, "doc-two", f.values[2])
				f.addGroundRule(f.doc, "doc-duplicate", f.values[1])
				f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{2, 1})
				owner := f.e.udocDatalog
				if owner == nil || owner.products == nil || owner.products.result[f.head] == nil || owner.result[f.doc] == nil {
					t.Fatal("ground rules used finite fallback instead of actual DOC/product execution")
				}
				if got := udocProductFiniteKind(owner.finiteResult[f.finite]); got != tableKind {
					t.Fatal("ground table singleton unions changed concrete backing", got)
				}
				empty, err := owner.seeds[f.doc].Empty()
				fragmentMust(t, err)
				rows, err := f.e.predicates[f.finite].table.Len()
				fragmentMust(t, err)
				if !empty || rows != 0 {
					t.Fatal("ground rule evaluation mutated persistent API seeds", empty, rows)
				}
				for _, tc := range []struct {
					query ast.ASTHandle
					want  lbool.LBool
				}{
					{f.app(f.head, f.values[1], f.yes), lbool.True},
					{f.app(f.head, f.values[1], f.no), lbool.False},
					{f.app(f.doc, f.values[2]), lbool.True},
					{f.app(f.finite, f.no), lbool.False},
				} {
					status, err := f.e.QueryExpression(tc.query)
					fragmentMust(t, err)
					if status != tc.want || f.e.queryClone == nil || f.e.queryClone.udocDatalog.products == nil {
						t.Fatal("ground-rule expression query did not clone actual execution", status, tc.want)
					}
					answer, err := f.e.ExpressionAnswer()
					fragmentMust(t, err)
					f.checkFormula(answer, nil, tc.want == lbool.True)
					fragmentMust(t, f.m.DelRef(&answer))
				}
				f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{2, 1})
			})
		}
	}
}

func TestDatalogUDocProductsGroundDOCFeedsExistingKeyedFiniteInput(t *testing.T) {
	f := newPublicUDocProductFixture(t, "hashtable", true, false)
	prepareUDocProductGroundRules(f)
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 2}))
	f.addRule(false)
	f.addGroundRule(f.doc, "doc-one", f.values[1])
	// Source product Empty is component-coarse, even when these actual
	// components' formulas have disjoint keys. Exact table fallback is false.
	f.checkRows(lbool.True)
	if f.e.udocDatalog == nil || f.e.udocDatalog.products.result[f.head] == nil {
		t.Fatal("ground DOC input lost actual coarse product behavior")
	}
}

func TestDatalogUDocProductsGroundRulesReplayAndUpdates(t *testing.T) {
	f := newPublicUDocProductFixture(t, "lazy_sparse", false, true)
	prepareUDocProductGroundRules(f)
	f.addRule(false)
	f.addGroundRule(f.doc, "doc-one", f.values[1])
	f.addGroundRule(f.finite, "finite-yes", f.yes)
	f.checkRows(lbool.True, [2]uint32{1, 1})
	fragmentMust(t, f.e.UpdateRule(f.app(f.doc, f.values[1]), "doc-one"))
	f.checkRows(lbool.True, [2]uint32{1, 1})
	fragmentMust(t, f.e.UpdateRule(f.app(f.finite, f.no), "new-finite-fact"))
	f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1})
	fragmentMust(t, f.e.AddFact(f.doc, []ast.ASTHandle{f.values[3]}))
	f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1}, [2]uint32{3, 0}, [2]uint32{3, 1})
}

func TestDatalogUDocProductsExplicitGroundMixedTuplesAndSeededHeadsKeepFiniteRoute(t *testing.T) {
	for _, mixedTuple := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed-ground-tuple=%v", mixedTuple), func(t *testing.T) {
			f := newPublicUDocProductFixture(t, "hashtable", false, !mixedTuple)
			prepareUDocProductGroundRules(f)
			if mixedTuple {
				fragmentMust(t, f.e.SetPredicateRepresentation(f.finite, []string{"tr_hashtable"}))
			}
			f.addRule(false)
			f.addGroundRule(f.doc, "doc-one", f.values[1])
			if mixedTuple {
				f.addGroundRule(f.finite, "mixed-tuple", f.yes, f.values[1])
				f.checkRows(lbool.True, [2]uint32{1, 1})
			} else {
				f.addGroundRule(f.finite, "finite-yes", f.yes)
				fragmentMust(t, f.e.AddTableFact(f.head, []uint32{3, 0}))
				f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{3, 0})
			}
			if f.e.udocDatalog != nil {
				t.Fatal("unsupported ground kind assembly or seeded head changed legacy route")
			}
		})
	}
}

func TestDatalogUDocProductsGroundRulesAllocationFailureRetryAndClose(t *testing.T) {
	checkPublicUDocProductAllocationFailureRetryAndClose(t, func(t *testing.T) *publicUDocProductFixture {
		return newPublicUDocProductFailureFixtureMode(t, true)
	})
}

func TestDatalogUDocProductsGroundAllBoolTupleAssembly(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(tableKind, func(t *testing.T) {
			f := newPublicUDocProductFixture(t, tableKind, false, true, 2)
			prepareUDocProductGroundRules(f)
			second := f.pin(f.m.MkVar(2, f.boolean))
			body := f.pin(f.m.MkAnd(f.app(f.doc, f.x), f.app(f.finite, f.b, second)))
			formula := f.pin(f.m.MkImplies(body, f.app(f.head, f.x, f.b)))
			fragmentMust(t, f.e.AddRule(formula, "all-bool-product"))
			f.addGroundRule(f.doc, "doc-one", f.values[1])
			f.addGroundRule(f.finite, "true-false", f.yes, f.no)
			f.addGroundRule(f.finite, "false-true", f.no, f.yes)
			f.addGroundRule(f.finite, "duplicate", f.yes, f.no)
			f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1})
			owner := f.e.udocDatalog
			if owner == nil || owner.products == nil || owner.products.result[f.head] == nil {
				t.Fatal("all-Bool ground tuple escaped actual product execution")
			}
			table := owner.finiteResult[f.finite]
			rows, err := table.Len()
			fragmentMust(t, err)
			if rows != 2 || udocProductFiniteKind(table) != tableKind {
				t.Fatal("all-Bool singleton assembly changed finite rows or kind", rows)
			}
			for _, tc := range []struct {
				fact rel.Fact
				want bool
			}{{rel.Fact{1, 0}, true}, {rel.Fact{0, 1}, true}, {rel.Fact{1, 1}, false}, {rel.Fact{0, 0}, false}} {
				found, err := table.Contains(tc.fact)
				fragmentMust(t, err)
				if found != tc.want {
					t.Fatal("ground tuple singleton joins lost column correlation", tc.fact, found)
				}
			}
		})
	}
}
