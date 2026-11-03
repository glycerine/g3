package fp

// Source: compiler constant-column assembly infers a DOC/table product for
// mixed ground heads, then product joins match that actual DOC component.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *publicUDocProductFixture) mixedGroundValues(value, boolean ast.ASTHandle) []ast.ASTHandle {
	if f.finiteBitsFirst {
		return []ast.ASTHandle{value, boolean}
	}
	return []ast.ASTHandle{boolean, value}
}

func (f *publicUDocProductFixture) addMixedGroundRule(name string, value, boolean ast.ASTHandle) {
	f.t.Helper()
	f.addGroundRule(f.finite, name, f.mixedGroundValues(value, boolean)...)
}

func TestDatalogUDocProductsMixedGroundExplicitDOCWithFiniteFavouriteKeepsFallback(t *testing.T) {
	f := newPublicUDocProductFixture(t, "hashtable", true, false)
	prepareUDocProductGroundRules(f)
	f.addRule(false)
	f.addGroundRule(f.doc, "doc-one", f.values[1])
	f.addMixedGroundRule("true-one", f.values[1], f.yes)
	f.addMixedGroundRule("false-two", f.values[2], f.no)
	// With an ordinary finite favourite, source BV singleton kinds depend on
	// which ground predicate first populates the shared (sort,value) cache.
	// This bounded route requires uniform DOC selection and leaves the prior
	// finite behavior intact when an explicit DOC predicate is insufficient.
	f.checkRows(lbool.True, [2]uint32{1, 1})
	if f.e.udocDatalog != nil {
		t.Fatal("mixed ground inputs ignored conflicting singleton selection")
	}
}

func (f *publicUDocProductFixture) checkMixedGroundOwner(kind string, keys ...uint32) {
	f.t.Helper()
	c := f.e.udocDatalog
	if c == nil || c.products == nil || c.products.result[f.finite] == nil || c.result[f.doc] == nil {
		f.t.Fatal("mixed ground rule escaped actual DOC/product execution")
	}
	product := c.products.result[f.finite]
	if c.finiteInputs[f.finite] != nil || c.finiteResult[f.finite] != nil || udocProductFiniteKind(product.Table()) != kind {
		f.t.Fatal("mixed ground head kept finite fallback or changed concrete product table")
	}
	// The initially empty ordinary owner may live until the next sync sees
	// the inferred product template; it must never supply query tuples.
	if table := f.e.predicates[f.finite].table; table != nil {
		if count, err := table.Len(); err != nil || count != 0 {
			f.t.Fatal("mixed ground query populated ordinary finite seed storage", count, err)
		}
	}
	answer, err := product.ToFormula()
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, f.m.DelRef(&answer)) }()
	for k, value := range f.values {
		want := false
		for _, key := range keys {
			want = want || uint32(k) == key
		}
		for _, boolean := range []ast.ASTHandle{f.no, f.yes} {
			f.checkFormula(answer, f.mixedGroundValues(value, boolean), want)
		}
	}
}

func TestDatalogUDocProductsMixedGroundRulesUseActualProductInputs(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, bitsFirst := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/bits-first=%v/reverse=%v", kind, bitsFirst, reverse), func(t *testing.T) {
					f := newPublicUDocProductFixtureColumnOrder(t, kind, false, false, bitsFirst)
					prepareUDocProductGroundRules(f)
					// Input ground rules occur after the consuming rule and include
					// a duplicate, so dependency order and replay both matter.
					f.addRule(reverse)
					f.addGroundRule(f.doc, "doc-one", f.values[1])
					f.addMixedGroundRule("true-one", f.values[1], f.yes)
					f.addMixedGroundRule("false-two", f.values[2], f.no)
					f.addMixedGroundRule("duplicate", f.values[1], f.yes)
					for run := 0; run < 2; run++ {
						// The source unions DOC keys and finite Bool columns
						// independently; both Bool values hold at each key.
						f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1})
						f.checkMixedGroundOwner(kind, 1, 2)
					}
					owner := f.e.udocDatalog
					if owner.products.result[f.head] == nil || owner.products.seeds[f.finite] == nil {
						t.Fatal("mixed ground input or head lost inferred product kind")
					}
					if empty, err := owner.products.seeds[f.finite].Empty(); err != nil || !empty {
						t.Fatal("mixed ground rule evaluation mutated persistent API product seeds", empty, err)
					}
					status, err := f.e.QueryPredicate(f.finite)
					fragmentMust(t, err)
					if status != lbool.True {
						t.Fatal("direct mixed ground input query", status)
					}
					f.checkMixedGroundOwner(kind, 1, 2)
					for _, tc := range []struct {
						query ast.ASTHandle
						want  bool
					}{
						{f.app(f.head, f.values[1], f.no), true},
						{f.app(f.head, f.values[3], f.yes), false},
						{f.app(f.finite, f.mixedGroundValues(f.values[2], f.yes)...), true},
						{f.app(f.finite, f.mixedGroundValues(f.values[3], f.no)...), false},
					} {
						status, err := f.e.QueryExpression(tc.query)
						fragmentMust(t, err)
						if (status == lbool.True) != tc.want || status == lbool.Undef || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
							t.Fatal("mixed ground expression status or owner", status, tc.want)
						}
						clone := f.e.queryClone.udocDatalog
						if clone.products == nil || clone.products.result[f.finite] == nil || clone.products.result[f.finite] == owner.products.result[f.finite] || clone.products.seeds[f.finite] == owner.products.seeds[f.finite] {
							t.Fatal("expression query did not independently clone mixed ground products")
						}
						answer, err := f.e.ExpressionAnswer()
						fragmentMust(t, err)
						f.checkFormula(answer, nil, tc.want)
						fragmentMust(t, f.e.clearQueryResources())
						f.checkFormula(answer, nil, tc.want)
						fragmentMust(t, f.m.DelRef(&answer))
					}
					f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1})
				})
			}
		}
	}
}

func TestDatalogUDocProductsMixedGroundDisjointKeysAreDOCEmpty(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, bitsFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bits-first=%v", kind, bitsFirst), func(t *testing.T) {
				f := newPublicUDocProductFixtureColumnOrder(t, kind, false, false, bitsFirst)
				prepareUDocProductGroundRules(f)
				f.addRule(bitsFirst)
				f.addGroundRule(f.doc, "doc-one", f.values[1])
				f.addMixedGroundRule("true-two", f.values[2], f.yes)
				f.checkRows(lbool.False)
				c := f.e.udocDatalog
				if c == nil || c.products == nil || c.products.result[f.finite] == nil || c.finiteResult[f.finite] != nil {
					t.Fatal("disjoint ground keys bypassed actual matched DOC components")
				}
				input := c.products.result[f.finite]
				if empty, err := input.Empty(); err != nil || empty {
					t.Fatal("ground input unexpectedly empty", empty, err)
				}
				if count, err := input.Table().Len(); err != nil || count != 4 {
					t.Fatal("ground singleton product did not keep full finite BV column", count, err)
				}
				status, err := f.e.QueryExpression(f.app(f.head, f.values[1], f.yes))
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("disjoint matched DOC survived expression query", status)
				}
				f.checkRows(lbool.False)
			})
		}
	}
}

func TestDatalogUDocProductsMixedGroundReplayRuleUpdatesAndAPISeeds(t *testing.T) {
	for _, kind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newPublicUDocProductFixtureColumnOrder(t, kind, false, false, true)
			prepareUDocProductGroundRules(f)
			f.addRule(true)
			f.addGroundRule(f.doc, "doc-one", f.values[1])
			f.addMixedGroundRule("true-one", f.values[1], f.yes)
			f.checkRows(lbool.True, [2]uint32{1, 1})
			fragmentMust(t, f.e.UpdateRule(f.app(f.finite, f.mixedGroundValues(f.values[1], f.yes)...), "true-one"))
			f.checkRows(lbool.True, [2]uint32{1, 1})
			f.addMixedGroundRule("false-two", f.values[2], f.no)
			f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1})
			fragmentMust(t, f.e.UpdateRule(f.app(f.doc, f.values[2]), "new-doc-two"))
			f.checkRows(lbool.True, [2]uint32{1, 0}, [2]uint32{1, 1}, [2]uint32{2, 0}, [2]uint32{2, 1})
			fragmentMust(t, f.e.AddFact(f.finite, f.mixedGroundValues(f.values[3], f.no)))
			fragmentMust(t, f.e.AddFact(f.doc, []ast.ASTHandle{f.values[3]}))
			rows := [][2]uint32{{1, 0}, {1, 1}, {2, 0}, {2, 1}, {3, 0}, {3, 1}}
			f.checkRows(lbool.True, rows...)
			f.checkMixedGroundOwner(kind, 1, 2, 3)
			owner := f.e.udocDatalog
			seed := owner.products.seeds[f.finite]
			for _, boolean := range []ast.ASTHandle{f.no, f.yes} {
				found, err := seed.ContainsFact(f.mixedGroundValues(f.values[3], boolean))
				fragmentMust(t, err)
				if found != (boolean == f.no) {
					t.Fatal("query replay wrote derived tuples into persistent API product seed", found)
				}
			}
			status, err := f.e.QueryExpression(f.app(f.head, f.values[3], f.yes))
			fragmentMust(t, err)
			if status != lbool.True || f.e.queryClone.udocDatalog.products.seeds[f.finite] == seed {
				t.Fatal("expression query lost or aliased adopted mixed input seed", status)
			}
			f.checkRows(lbool.True, rows...)
		})
	}
}

func TestDatalogUDocProductsMixedGroundPreserveInitialFiniteFallbacks(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, fallback := range []string{"seeded-input", "seeded-head", "explicit-input-kind"} {
			t.Run(kind+"/"+fallback, func(t *testing.T) {
				f := newPublicUDocProductFixture(t, kind, false, false)
				prepareUDocProductGroundRules(f)
				f.addRule(false)
				f.addGroundRule(f.doc, "doc-one", f.values[1])
				f.addMixedGroundRule("true-one", f.values[1], f.yes)
				f.addMixedGroundRule("false-two", f.values[2], f.no)
				rows := [][2]uint32{{1, 1}}
				switch fallback {
				case "seeded-input":
					fragmentMust(t, f.e.AddFact(f.finite, f.mixedGroundValues(f.values[3], f.no)))
				case "seeded-head":
					fragmentMust(t, f.e.AddTableFact(f.head, []uint32{3, 0}))
					rows = append(rows, [2]uint32{3, 0})
				case "explicit-input-kind":
					fragmentMust(t, f.e.SetPredicateRepresentation(f.finite, []string{"tr_" + kind}))
				}
				f.checkRows(lbool.True, rows...)
				if f.e.udocDatalog != nil {
					t.Fatal("initial seeded or explicitly finite input lost its compatibility route")
				}
			})
		}
	}
}
