package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type publicUDocPairFixture struct {
	*publicUDocForwardFixture
	first, second, secondDoc, secondTable, pair ast.ASTHandle
	c                                           ast.ASTHandle
}

func newPublicUDocPairFixture(t *testing.T, kind string, reverse bool) *publicUDocPairFixture {
	t.Helper()
	f := &publicUDocPairFixture{publicUDocForwardFixture: newPublicUDocForwardFixture(t, kind)}
	f.c = f.pin(f.m.MkVar(3, f.boolean))
	f.first = f.declare("pair-first", f.bits, f.boolean)
	f.secondDoc = f.declare("pair-second-doc", f.bits)
	f.secondTable = f.declare("pair-second-table", f.boolean, f.bits)
	f.second = f.declare("pair-second", f.bits, f.boolean)
	f.pair = f.declare("pair-output", f.bits, f.boolean, f.boolean)
	left, right := f.app(f.first, f.x, f.b), f.app(f.second, f.x, f.c)
	if reverse {
		left, right = right, left
	}
	// The consumer is submitted before either producer. The first producer
	// itself forwards an existing three-column product through projection.
	f.rule("pair-output-rule", f.pin(f.m.MkAnd(left, right)), f.app(f.pair, f.x, f.b, f.c))
	f.rule("pair-first-rule", f.app(f.head, f.x, f.b, f.z), f.app(f.first, f.x, f.b))
	body := f.pin(f.m.MkAnd(f.app(f.secondDoc, f.x), f.app(f.secondTable, f.c, f.x)))
	f.rule("pair-second-rule", body, f.app(f.second, f.x, f.c))
	f.anchor()
	return f
}

func (f *publicUDocPairFixture) seed(firstDoc, firstKey, secondDoc, secondKey uint32) {
	f.t.Helper()
	fragmentMust(f.t, f.e.AddTableFact(f.doc, []uint32{firstDoc}))
	fragmentMust(f.t, f.e.AddTableFact(f.finite, []uint32{1, firstKey, 0}))
	fragmentMust(f.t, f.e.AddTableFact(f.secondDoc, []uint32{secondDoc}))
	fragmentMust(f.t, f.e.AddTableFact(f.secondTable, []uint32{0, secondKey}))
}

func TestDatalogUDocProductsPairPublicComponentsAndDependencyOrder(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			for _, tc := range []struct {
				name                           string
				firstKey, secondDoc, secondKey uint32
				status                         lbool.LBool
				rows                           [][]uint32
			}{
				{"matching", 1, 1, 1, lbool.True, [][]uint32{{1, 1, 0}}},
				{"coarse-false-formula", 2, 1, 2, lbool.True, nil},
				{"doc-empty", 2, 3, 2, lbool.False, nil},
				{"table-empty", 2, 1, 3, lbool.False, nil},
			} {
				t.Run(fmt.Sprintf("%s/reverse=%v/%s", kind, reverse, tc.name), func(t *testing.T) {
					f := newPublicUDocPairFixture(t, kind, reverse)
					f.seed(1, tc.firstKey, tc.secondDoc, tc.secondKey)
					firstTable, secondTable := f.e.predicates[f.finite].table, f.e.predicates[f.secondTable].table
					for run := 0; run < 2; run++ {
						f.check(f.pair, tc.status, tc.rows...)
						f.checkProduct(kind, f.head, f.first, f.second)
						if tc.status == lbool.True {
							f.checkProduct(kind, f.pair)
						}
					}
					if f.e.predicates[f.finite].table != firstTable || f.e.predicates[f.secondTable].table != secondTable {
						t.Fatal("paired product query replaced actual finite source storage")
					}
					for _, table := range []ast.ASTHandle{f.finite, f.secondTable} {
						if count, err := f.e.predicates[table].table.Len(); err != nil || count != 1 {
							t.Fatal("paired product query mutated extensional source facts", count, err)
						}
					}
				})
			}
		}
	}
}

func TestDatalogUDocProductsPairPublicBoolCartesianAndSelfJoins(t *testing.T) {
	for _, kind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newPublicUDocPairFixture(t, kind, false)
			boolean := f.declare("pair-bool-key", f.bits, f.bits, f.boolean)
			cartesian := f.declare("pair-cartesian", f.bits, f.boolean, f.bits, f.boolean)
			self := f.declare("pair-self", f.bits, f.boolean, f.boolean)
			f.rule("pair-bool-rule", f.pin(f.m.MkAnd(f.app(f.first, f.x, f.b), f.app(f.second, f.z, f.b))), f.app(boolean, f.x, f.z, f.b))
			f.rule("pair-cartesian-rule", f.pin(f.m.MkAnd(f.app(f.first, f.x, f.b), f.app(f.second, f.z, f.c))), f.app(cartesian, f.x, f.b, f.z, f.c))
			f.rule("pair-self-rule", f.pin(f.m.MkAnd(f.app(f.first, f.x, f.b), f.app(f.first, f.x, f.c))), f.app(self, f.x, f.b, f.c))
			f.seed(1, 1, 2, 2)
			// Bool keys are ignored only by the DOC component. The actual
			// finite join must detect true/false disagreement and be empty.
			f.check(boolean, lbool.False)
			f.check(cartesian, lbool.True, []uint32{1, 1, 2, 0})
			f.check(self, lbool.True, []uint32{1, 1, 1})
			f.checkProduct(kind, cartesian, self)
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 1, 0}))
			f.check(self, lbool.True, []uint32{1, 0, 0}, []uint32{1, 0, 1}, []uint32{1, 1, 0}, []uint32{1, 1, 1})
			f.check(boolean, lbool.True, []uint32{1, 2, 0})
		})
	}
}

func TestDatalogUDocProductsPairPublicHeadAssemblyAndQueryClone(t *testing.T) {
	for _, kind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newPublicUDocPairFixture(t, kind, true)
			permuted := f.declare("pair-permuted", f.boolean, f.bits, f.boolean)
			projected := f.declare("pair-projected", f.bits, f.boolean)
			body := f.pin(f.m.MkAnd(f.app(f.first, f.x, f.b), f.app(f.second, f.x, f.c)))
			formula := f.rule("pair-permuted-rule", body, f.app(permuted, f.c, f.x, f.b))
			f.rule("pair-projected-rule", body, f.app(projected, f.x, f.b))
			f.seed(1, 1, 1, 1)
			f.check(permuted, lbool.True, []uint32{0, 1, 1})
			f.check(projected, lbool.True, []uint32{1, 1})
			owner := f.e.udocDatalog
			for _, tc := range []struct {
				query    ast.ASTHandle
				bindings []ast.ASTHandle
				want     bool
			}{
				{f.app(f.pair, f.x, f.b, f.c), []ast.ASTHandle{f.no, f.yes, f.values[1]}, true},
				{f.app(f.pair, f.values[1], f.yes, f.no), nil, true},
				{f.app(f.pair, f.values[2], f.yes, f.no), nil, false},
			} {
				status, err := f.e.QueryExpression(tc.query)
				fragmentMust(t, err)
				if status == lbool.Undef || (status == lbool.True) != tc.want || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
					t.Fatal("paired product expression status or owner", status, tc.want)
				}
				clone := f.e.queryClone.udocDatalog
				if clone.products == nil || clone.products.result[f.first] == owner.products.result[f.first] || clone.products.result[f.second] == owner.products.result[f.second] {
					t.Fatal("paired product query shared mutable producer snapshots")
				}
				answer, err := f.e.ExpressionAnswer()
				fragmentMust(t, err)
				f.checkFormula(answer, tc.bindings, tc.want)
				fragmentMust(t, f.e.clearQueryResources())
				f.checkFormula(answer, tc.bindings, tc.want)
				fragmentMust(t, f.m.DelRef(&answer))
			}
			fragmentMust(t, f.e.UpdateRule(formula, "pair-permuted-rule"))
			f.check(permuted, lbool.True, []uint32{0, 1, 1})
			fragmentMust(t, f.e.AddTableFact(f.secondTable, []uint32{1, 1}))
			f.check(f.pair, lbool.True, []uint32{1, 1, 0}, []uint32{1, 1, 1})
			f.check(permuted, lbool.True, []uint32{0, 1, 1}, []uint32{1, 1, 1})
		})
	}
}

func TestDatalogUDocProductsPairPublicOrdinarySeedIsNotSilentlyEmpty(t *testing.T) {
	f := newPublicUDocPairFixture(t, "hashtable", false)
	f.check(f.pair, lbool.False)
	if f.e.udocDatalog == nil || f.e.udocDatalog.products == nil {
		t.Fatal("empty paired program did not select actual product execution")
	}
	fragmentMust(t, f.e.AddTableFact(f.first, []uint32{1, 1}))
	// A missing second register proves this join empty without requiring
	// a table/product operation for the ordinary first input yet.
	f.check(f.pair, lbool.False)
	fragmentMust(t, f.e.AddTableFact(f.secondDoc, []uint32{1}))
	fragmentMust(t, f.e.AddTableFact(f.secondTable, []uint32{0, 1}))
	_, err := f.e.QueryPredicate(f.pair)
	if !errors.Is(err, ErrUDocDatalogFragment) {
		t.Fatal("ordinary nonempty input was silently treated as absent product", err)
	}
	answer, answerErr := f.e.Answer(f.pair)
	if answer != (ast.ASTRef{}) || answerErr == nil {
		if answer != (ast.ASTRef{}) {
			_ = f.m.DelRef(&answer)
		}
		t.Fatal("unsupported actual input kind exposed stale paired result", answerErr)
	}
	if count, err := f.e.predicates[f.first].table.Len(); err != nil || count != 1 {
		t.Fatal("unsupported pair consumed the ordinary input seed", count, err)
	}
}

func TestDatalogUDocProductsPairPublicInitialSeedsKeepFiniteFallback(t *testing.T) {
	for _, seedProducer := range []bool{false, true} {
		t.Run(fmt.Sprintf("seed-producer=%v", seedProducer), func(t *testing.T) {
			f := newPublicUDocPairFixture(t, "hashtable", false)
			var rows [][]uint32
			if seedProducer {
				fragmentMust(t, f.e.AddTableFact(f.first, []uint32{1, 1}))
				fragmentMust(t, f.e.AddTableFact(f.secondDoc, []uint32{1}))
				fragmentMust(t, f.e.AddTableFact(f.secondTable, []uint32{0, 1}))
				rows = [][]uint32{{1, 1, 0}}
			} else {
				fragmentMust(t, f.e.AddTableFact(f.pair, []uint32{3, 0, 1}))
				f.seed(1, 1, 1, 1)
				rows = [][]uint32{{1, 1, 0}, {3, 0, 1}}
			}
			f.check(f.pair, lbool.True, rows...)
			if f.e.udocDatalog != nil {
				t.Fatal("initially seeded paired program abandoned finite compatibility route")
			}
		})
	}
}
