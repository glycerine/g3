package fp

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type publicUDocStandaloneFixture struct {
	*publicUDocForwardFixture
	other, c ast.ASTHandle
}

// This fixture registers only mixed predicates: no unary DOC predicate or
// raw-DOC anchor rule can activate the product path on its behalf.
func newPublicUDocStandaloneFixture(t *testing.T, kind string, bitsFirst, paired bool) *publicUDocStandaloneFixture {
	t.Helper()
	m, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	f := &publicUDocStandaloneFixture{publicUDocForwardFixture: &publicUDocForwardFixture{
		publicUDocProductFixture: &publicUDocProductFixture{t: t, m: m, e: engine, finiteBitsFirst: bitsFirst},
	}}
	t.Cleanup(func() { fragmentMust(t, engine.Close()) })
	fragmentMust(t, globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	fragmentMust(t, globals.SourceSetName("fixedpoint.datalog.default_table", kind))
	fragmentMust(t, engine.SelectEngine("datalog"))
	prepareUDocProductGroundRules(f.publicUDocProductFixture)
	f.boolean = f.pin(m.MkBoolSort())
	f.bits = f.pin(m.BitVectorSort(2))
	f.yes, f.no = f.pin(m.MkTrue()), f.pin(m.MkFalse())
	f.x = f.pin(m.MkVar(0, f.bits))
	f.b = f.pin(m.MkVar(1, f.boolean))
	f.z = f.pin(m.MkVar(2, f.bits))
	f.c = f.pin(m.MkVar(3, f.boolean))
	for value := uint64(0); value < 4; value++ {
		f.values = append(f.values, f.pin(m.BitVectorNumeralUint64(value, 2)))
	}
	sorts := []ast.ASTHandle{f.boolean, f.bits}
	if bitsFirst {
		sorts = []ast.ASTHandle{f.bits, f.boolean}
	}
	f.finite = f.declare("standalone-T", sorts...)
	f.head = f.finite
	if paired {
		f.other = f.declare("standalone-U", sorts...)
		f.head = f.declare("standalone-K", f.bits, f.boolean)
	}
	return f
}

func (f *publicUDocStandaloneFixture) ground(declaration ast.ASTHandle, name string, value, boolean ast.ASTHandle) {
	f.t.Helper()
	f.addGroundRule(declaration, name, f.mixedGroundValues(value, boolean)...)
}

func (f *publicUDocStandaloneFixture) pair(reverse bool) ast.ASTHandle {
	f.t.Helper()
	left := f.app(f.finite, f.mixedGroundValues(f.x, f.b)...)
	right := f.app(f.other, f.mixedGroundValues(f.x, f.c)...)
	if reverse {
		left, right = right, left
	}
	return f.rule("standalone-pair", f.pin(f.m.MkAnd(left, right)), f.app(f.head, f.x, f.b))
}

func (f *publicUDocStandaloneFixture) checkStandalone(kind string, declarations ...ast.ASTHandle) {
	f.t.Helper()
	f.checkProduct(kind, declarations...)
	c := f.e.udocDatalog
	if len(c.seeds) != 0 || len(c.result) != 0 || len(c.order) != 0 || len(c.products.program.docs) != 0 {
		f.t.Fatal("standalone products unexpectedly depended on a raw DOC predicate")
	}
	for _, d := range declarations {
		if c.finiteInputs[d] != nil || c.finiteResult[d] != nil {
			f.t.Fatal("standalone product answer came from finite fallback")
		}
		if table := f.e.predicates[d].table; table != nil {
			if count, err := table.Len(); err != nil || count != 0 {
				f.t.Fatal("standalone product populated ordinary seed storage", count, err)
			}
		}
	}
}

func TestDatalogUDocStandaloneGroundDirectCorrelationLoss(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, bitsFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bits-first=%v", kind, bitsFirst), func(t *testing.T) {
				f := newPublicUDocStandaloneFixture(t, kind, bitsFirst, false)
				f.ground(f.finite, "true-one", f.values[1], f.yes)
				f.ground(f.finite, "false-two", f.values[2], f.no)
				f.ground(f.finite, "duplicate", f.values[1], f.yes)
				rows := [][]uint32{{0, 1}, {1, 1}, {0, 2}, {1, 2}}
				if bitsFirst {
					rows = [][]uint32{{1, 0}, {1, 1}, {2, 0}, {2, 1}}
				}
				for run := 0; run < 2; run++ {
					f.check(f.finite, lbool.True, rows...)
					f.checkStandalone(kind, f.finite)
				}
				if len(f.e.udocDatalog.products.program.order) != 0 || len(f.e.udocDatalog.products.program.groundFacts) != 3 {
					t.Fatal("direct ground-only query unexpectedly needed a nonground anchor")
				}
				if len(f.e.predicateOrder) != 1 {
					t.Fatal("ground-only fixture registered an unrelated predicate")
				}
			})
		}
	}
}

func TestDatalogUDocStandaloneGroundPairAndForwarding(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, bitsFirst := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				for _, disjoint := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/bits-first=%v/reverse=%v/disjoint=%v", kind, bitsFirst, reverse, disjoint), func(t *testing.T) {
						f := newPublicUDocStandaloneFixture(t, kind, bitsFirst, true)
						permuted := f.declare("standalone-permuted", f.boolean, f.bits)
						f.rule("standalone-forward", f.app(f.head, f.x, f.b), f.app(permuted, f.b, f.x))
						f.pair(reverse)
						// Ground producers follow both consumers in source order.
						f.ground(f.finite, "true-one", f.values[1], f.yes)
						f.ground(f.finite, "false-two", f.values[2], f.no)
						otherKey := uint32(1)
						if disjoint {
							otherKey = 3
						}
						f.ground(f.other, "other", f.values[otherKey], f.no)
						if disjoint {
							f.check(f.head, lbool.False)
							f.check(permuted, lbool.False)
						} else {
							f.check(f.head, lbool.True, []uint32{1, 0}, []uint32{1, 1})
							f.check(permuted, lbool.True, []uint32{0, 1}, []uint32{1, 1})
							f.checkStandalone(kind, f.head, permuted)
						}
						f.checkStandalone(kind, f.finite, f.other)
						// Ground input products have a full finite BV column, so
						// disjoint keys must be rejected by their actual DOC join.
						for _, d := range []ast.ASTHandle{f.finite, f.other} {
							seed := f.e.udocDatalog.products.seeds[d]
							if empty, err := seed.Empty(); err != nil || !empty {
								t.Fatal("ground facts contaminated persistent API seeds", empty, err)
							}
						}
						if count, err := f.e.udocDatalog.products.result[f.other].Table().Len(); err != nil || count != 4 {
							t.Fatal("standalone singleton assembly lost its full finite BV column", count, err)
						}
					})
				}
			}
		}
	}
}

func TestDatalogUDocStandaloneGroundQueryClonesAndMutations(t *testing.T) {
	for _, kind := range []string{"hashtable", "lazy_sparse"} {
		for _, bitsFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/bits-first=%v", kind, bitsFirst), func(t *testing.T) {
				f := newPublicUDocStandaloneFixture(t, kind, bitsFirst, true)
				formula := f.pair(true)
				f.ground(f.finite, "true-one", f.values[1], f.yes)
				f.ground(f.finite, "false-two", f.values[2], f.no)
				f.ground(f.other, "other-one", f.values[1], f.no)
				f.check(f.head, lbool.True, []uint32{1, 0}, []uint32{1, 1})
				owner := f.e.udocDatalog.products
				for _, tc := range []struct {
					query    ast.ASTHandle
					bindings []ast.ASTHandle
					want     bool
				}{
					{f.app(f.finite, f.mixedGroundValues(f.x, f.b)...), []ast.ASTHandle{f.no, f.values[1]}, true},
					{f.app(f.finite, f.mixedGroundValues(f.values[1], f.no)...), nil, true},
					{f.app(f.head, f.x, f.b), []ast.ASTHandle{f.no, f.values[1]}, true},
					{f.app(f.head, f.values[2], f.no), nil, false},
				} {
					status, err := f.e.QueryExpression(tc.query)
					fragmentMust(t, err)
					if status == lbool.Undef || (status == lbool.True) != tc.want || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
						t.Fatal("standalone expression result or execution owner", status, tc.want)
					}
					clone := f.e.queryClone.udocDatalog.products
					for _, d := range []ast.ASTHandle{f.finite, f.other} {
						if clone == nil || clone.result[d] == nil || clone.result[d] == owner.result[d] || clone.seeds[d] == owner.seeds[d] {
							t.Fatal("standalone expression shared source product ownership")
						}
					}
					answer, err := f.e.ExpressionAnswer()
					fragmentMust(t, err)
					fragmentMust(t, f.e.clearQueryResources())
					f.checkFormula(answer, tc.bindings, tc.want)
					fragmentMust(t, f.m.DelRef(&answer))
				}
				fragmentMust(t, f.e.UpdateRule(formula, "standalone-pair"))
				fragmentMust(t, f.e.UpdateRule(f.app(f.finite, f.mixedGroundValues(f.values[1], f.yes)...), "true-one"))
				f.check(f.head, lbool.True, []uint32{1, 0}, []uint32{1, 1})
				fragmentMust(t, f.e.AddFact(f.finite, f.mixedGroundValues(f.values[3], f.yes)))
				f.ground(f.other, "other-three", f.values[3], f.yes)
				f.check(f.head, lbool.True, []uint32{1, 0}, []uint32{1, 1}, []uint32{3, 0}, []uint32{3, 1})
				f.checkStandalone(kind, f.finite, f.other, f.head)
				seed := f.e.udocDatalog.products.seeds[f.finite]
				for _, value := range []struct {
					boolean ast.ASTHandle
					want    bool
				}{{f.no, false}, {f.yes, true}} {
					found, err := seed.ContainsFact(f.mixedGroundValues(f.values[3], value.boolean))
					fragmentMust(t, err)
					if found != value.want {
						t.Fatal("ground replay changed the standalone persistent API seed", found, value.want)
					}
				}
			})
		}
	}
}

func TestDatalogUDocStandaloneGroundFiniteFallbackGuards(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reason := range []string{"seed-input", "seed-head", "explicit-input", "conflicting-kind"} {
			t.Run(kind+"/"+reason, func(t *testing.T) {
				f := newPublicUDocStandaloneFixture(t, kind, true, true)
				f.pair(false)
				f.ground(f.finite, "true-one", f.values[1], f.yes)
				f.ground(f.finite, "false-two", f.values[2], f.no)
				f.ground(f.other, "other-one", f.values[1], f.no)
				rows := [][]uint32{{1, 1}}
				switch reason {
				case "seed-input":
					fragmentMust(t, f.e.AddFact(f.finite, f.mixedGroundValues(f.values[3], f.no)))
				case "seed-head":
					fragmentMust(t, f.e.AddTableFact(f.head, []uint32{3, 0}))
					rows = append(rows, []uint32{3, 0})
				case "explicit-input":
					fragmentMust(t, f.e.SetPredicateRepresentation(f.finite, []string{"tr_" + kind}))
				case "conflicting-kind":
					// A separate explicit finite ground head can populate the
					// compiler's shared (sort,value) singleton cache first.
					conflict := f.declare("standalone-finite-singleton", f.bits)
					fragmentMust(t, f.e.SetPredicateRepresentation(conflict, []string{"tr_" + kind}))
					f.addGroundRule(conflict, "finite-singleton", f.values[1])
				}
				f.check(f.head, lbool.True, rows...)
				if f.e.udocDatalog != nil {
					t.Fatal("standalone mixed inputs abandoned their initial finite compatibility route")
				}
			})
		}
	}
}
