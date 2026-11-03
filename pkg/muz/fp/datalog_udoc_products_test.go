package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type publicUDocProductFixture struct {
	t                 *testing.T
	m                 *ast.ASTManager
	e                 *PropositionalEngine
	boolean, bits     ast.ASTHandle
	doc, finite, head ast.ASTHandle
	x, b, yes, no     ast.ASTHandle
	values            []ast.ASTHandle
	cartesian         bool
	finiteBitsFirst   bool
}

func newPublicUDocProductFixture(t *testing.T, tableKind string, explicitDoc, cartesian bool, finiteBoolColumns ...int) *publicUDocProductFixture {
	t.Helper()
	return newPublicUDocProductFixtureColumnOrder(t, tableKind, explicitDoc, cartesian, false, finiteBoolColumns...)
}

func newPublicUDocProductFixtureColumnOrder(t *testing.T, tableKind string, explicitDoc, cartesian, finiteBitsFirst bool, finiteBoolColumns ...int) *publicUDocProductFixture {
	t.Helper()
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	f := &publicUDocProductFixture{t: t, m: m, e: e, cartesian: cartesian, finiteBitsFirst: finiteBitsFirst}
	t.Cleanup(func() { fragmentMust(t, e.Close()) })
	fragmentMust(t, g.SourceSetName("fixedpoint.datalog.default_table", tableKind))
	if !explicitDoc {
		fragmentMust(t, g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	}
	fragmentMust(t, e.SelectEngine("datalog"))
	f.boolean = f.pin(m.MkBoolSort())
	f.bits = f.pin(m.BitVectorSort(2))
	f.yes = f.pin(m.MkTrue())
	f.no = f.pin(m.MkFalse())
	f.x = f.pin(m.MkVar(0, f.bits))
	f.b = f.pin(m.MkVar(1, f.boolean))
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		fragmentMust(t, err)
		d := f.pin(m.MkFuncDecl(n, sorts, f.boolean, ast.ASTFuncDeclInfo{}))
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	f.doc = declare("public-doc-product-D", []ast.ASTHandle{f.bits})
	finiteSorts := []ast.ASTHandle{f.boolean, f.bits}
	if finiteBitsFirst {
		finiteSorts = []ast.ASTHandle{f.bits, f.boolean}
	}
	if cartesian {
		finiteSorts = []ast.ASTHandle{f.boolean}
		if len(finiteBoolColumns) != 0 {
			for i := 1; i < finiteBoolColumns[0]; i++ {
				finiteSorts = append(finiteSorts, f.boolean)
			}
		}
	}
	f.finite = declare("public-doc-product-T", finiteSorts)
	f.head = declare("public-doc-product-H", []ast.ASTHandle{f.bits, f.boolean})
	if explicitDoc {
		fragmentMust(t, e.SetPredicateRepresentation(f.doc, []string{"doc"}))
	}
	for i := uint64(0); i < 4; i++ {
		f.values = append(f.values, f.pin(m.BitVectorNumeralUint64(i, 2)))
	}
	return f
}

func (f *publicUDocProductFixture) pin(value ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	fragmentMust(f.t, err)
	root, err := f.m.NewRef(value)
	fragmentMust(f.t, err)
	f.t.Cleanup(func() { fragmentMust(f.t, f.m.DelRef(&root)) })
	return value
}

func (f *publicUDocProductFixture) app(declaration ast.ASTHandle, values ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	return f.pin(f.m.MkApp(declaration, values))
}

func (f *publicUDocProductFixture) addRule(reverse bool) {
	f.t.Helper()
	doc := f.app(f.doc, f.x)
	args := []ast.ASTHandle{f.b, f.x}
	if f.finiteBitsFirst {
		args = []ast.ASTHandle{f.x, f.b}
	}
	if f.cartesian {
		args = []ast.ASTHandle{f.b}
	}
	finite := f.app(f.finite, args...)
	left, right := doc, finite
	if reverse {
		left, right = finite, doc
	}
	body := f.pin(f.m.MkAnd(left, right))
	head := f.app(f.head, f.x, f.b)
	rule := f.pin(f.m.MkImplies(body, head))
	fragmentMust(f.t, f.e.AddRule(rule, "public-doc-table-product"))
}

func (f *publicUDocProductFixture) checkFormula(answer ast.ASTRef, values []ast.ASTHandle, want bool) {
	f.t.Helper()
	formula, err := f.m.RefGet(answer)
	fragmentMust(f.t, err)
	substitution, err := astrewriter.NewVarSubstitution(f.m, false)
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, substitution.Close()) }()
	ground, err := f.m.NewRef(ast.ASTHandle{})
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, f.m.DelRef(&ground)) }()
	fragmentMust(f.t, substitution.Substitute(formula, values, ground))
	term, err := f.m.RefGet(ground)
	fragmentMust(f.t, err)
	reduced, err := f.m.NewRef(ast.ASTHandle{})
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, f.m.DelRef(&reduced)) }()
	fragmentMust(f.t, f.e.guard.RewriteWithBindings(term, nil, reduced))
	truth, err := f.m.RefGet(reduced)
	fragmentMust(f.t, err)
	if f.m.IsTrue(truth) != want || !f.m.IsTrue(truth) && !f.m.IsFalse(truth) {
		f.t.Fatal("public DOC/table answer disagrees with grounded valuation", values, want)
	}
}

func (f *publicUDocProductFixture) checkRows(wantStatus lbool.LBool, rows ...[2]uint32) {
	f.t.Helper()
	status, err := f.e.QueryPredicate(f.head)
	fragmentMust(f.t, err)
	if status != wantStatus {
		f.t.Fatal("mixed DOC/table predicate status", status, wantStatus)
	}
	want := make(map[[2]uint32]bool)
	for _, row := range rows {
		want[row] = true
	}
	answer, err := f.e.Answer(f.head)
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, f.m.DelRef(&answer)) }()
	for x, value := range f.values {
		for b, boolean := range []ast.ASTHandle{f.no, f.yes} {
			f.checkFormula(answer, []ast.ASTHandle{value, boolean}, want[[2]uint32{uint32(x), uint32(b)}])
		}
	}
}

func TestDatalogUDocProductsPublicKeyedJoinsDefaultsAndFactUpdates(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, reverse := range []bool{false, true} {
			name := tableKind + "/doc-first"
			if reverse {
				name = tableKind + "/table-first-explicit-doc"
			}
			t.Run(name, func(t *testing.T) {
				f := newPublicUDocProductFixture(t, tableKind, reverse, false)
				fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
				fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
				fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 2}))
				original := f.e.predicates[f.finite].table
				f.addRule(reverse)
				for run := 0; run < 2; run++ {
					f.checkRows(lbool.True, [2]uint32{1, 1})
					if f.e.udocDatalog == nil || f.e.udocDatalog.products.result[f.head] == nil {
						t.Fatal("public mixed join missed concrete DOC/table product")
					}
					if f.e.predicates[f.doc].table != nil || f.e.predicates[f.finite].table != original {
						t.Fatal("mixed product copied DOC into finite storage or replaced finite input")
					}
					product := f.e.udocDatalog.products.result[f.head]
					switch tableKind {
					case "hashtable":
						if _, ok := product.Table().(*rel.HashTable); !ok {
							t.Fatalf("public mixed join changed hash kind to %T", product.Table())
						}
					case "sparse":
						if _, ok := product.Table().(*rel.SparseTable); !ok {
							t.Fatalf("public mixed join changed sparse kind to %T", product.Table())
						}
					case "lazy_sparse":
						if _, ok := product.Table().(*rel.LazySparseTable); !ok {
							t.Fatalf("public mixed join changed lazy kind to %T", product.Table())
						}
					}
				}
				fragmentMust(t, f.e.AddFact(f.doc, []ast.ASTHandle{f.values[2]}))
				f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{2, 0})
				fragmentMust(t, f.e.AddFact(f.finite, []ast.ASTHandle{f.yes, f.values[2]}))
				f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{2, 0}, [2]uint32{2, 1})
			})
		}
	}
}

func TestDatalogUDocProductsExpressionQueriesOwnPermutationProjectionAndAnswers(t *testing.T) {
	f := newPublicUDocProductFixture(t, "hashtable", false, true)
	fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{2}))
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1}))
	f.addRule(false)
	f.checkRows(lbool.True, [2]uint32{2, 1})
	b0 := f.pin(f.m.MkVar(0, f.boolean))
	x1 := f.pin(f.m.MkVar(1, f.bits))
	for _, tc := range []struct {
		name     string
		query    ast.ASTHandle
		status   lbool.LBool
		bindings []ast.ASTHandle
		want     bool
	}{
		{"full-descending", f.app(f.head, f.x, f.b), lbool.True, []ast.ASTHandle{f.yes, f.values[2]}, true},
		{"full-permuted", f.app(f.head, x1, b0), lbool.True, []ast.ASTHandle{f.values[2], f.yes}, true},
		{"ground-present", f.app(f.head, f.values[2], f.yes), lbool.True, nil, true},
		{"ground-absent", f.app(f.head, f.values[1], f.yes), lbool.False, nil, false},
		{"project-bool", f.app(f.head, f.values[2], b0), lbool.True, []ast.ASTHandle{f.yes}, true},
		{"project-bv", f.app(f.head, f.x, f.yes), lbool.True, []ast.ASTHandle{f.values[2]}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := *f
			local.t = t
			status, err := f.e.QueryExpression(tc.query)
			fragmentMust(t, err)
			if status != tc.status || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
				t.Fatal("mixed product expression query lost its execution owner", status, tc.status)
			}
			clone := f.e.queryClone
			if clone.udocDatalog == f.e.udocDatalog || clone.predicates[f.finite].table == f.e.predicates[f.finite].table {
				t.Fatal("mixed expression query shared mutable source ownership")
			}
			answer, err := f.e.ExpressionAnswer()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
			local.checkFormula(answer, tc.bindings, tc.want)
			fragmentMust(t, f.e.clearQueryResources())
			local.checkFormula(answer, tc.bindings, tc.want)
		})
	}
	f.checkRows(lbool.True, [2]uint32{2, 1})
}

func TestDatalogUDocProductsPreserveCoarseEmptinessAndHoistedQueryColumns(t *testing.T) {
	f := newPublicUDocProductFixture(t, "hashtable", false, false)
	fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 2}))
	f.addRule(false)
	// Both source components are nonempty even though their conjunction has
	// disjoint keys. The public status follows product_relation::empty.
	f.checkRows(lbool.True)
	if f.e.udocDatalog == nil || f.e.udocDatalog.products.result[f.head] == nil {
		t.Fatal("disjoint mixed join used exact finite intersection")
	}
	body := f.app(f.head, f.x, f.b)
	query := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Sorts: []ast.ASTHandle{f.bits}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body,
	}))
	status, err := f.e.QueryExpression(query)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("hoisted product query replaced source coarse emptiness", status)
	}
	// Source rule_manager::mk_query hoists existential variables into the
	// synthetic head before collecting its columns (dl_rule.cpp:274-280).
	// Both columns remain, so this answer still has the false conjunction.
	arity, err := f.m.GetArity(f.e.queryClonePredicate)
	fragmentMust(t, err)
	if arity != 2 {
		t.Fatal("existential query projected a source query-head column", arity)
	}
	firstSort, err := f.m.GetDomain(f.e.queryClonePredicate, 0)
	fragmentMust(t, err)
	secondSort, err := f.m.GetDomain(f.e.queryClonePredicate, 1)
	fragmentMust(t, err)
	if !(firstSort == f.boolean && secondSort == f.bits || firstSort == f.bits && secondSort == f.boolean) {
		t.Fatal("hoisted query changed original column sorts", firstSort, secondSort)
	}
	answer, err := f.e.ExpressionAnswer()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
	for _, value := range f.values {
		for _, boolean := range []ast.ASTHandle{f.no, f.yes} {
			bindings := []ast.ASTHandle{value, boolean}
			if firstSort == f.boolean {
				bindings = []ast.ASTHandle{boolean, value}
			}
			f.checkFormula(answer, bindings, false)
		}
	}
	f.checkRows(lbool.True)
}

func TestDatalogUDocProductsKeepInitiallySeededMixedHeadOnFiniteRoute(t *testing.T) {
	f := newPublicUDocProductFixture(t, "hashtable", false, false)
	fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
	fragmentMust(t, f.e.AddTableFact(f.head, []uint32{3, 0}))
	f.addRule(false)
	f.checkRows(lbool.True, [2]uint32{1, 1}, [2]uint32{3, 0})
	if f.e.udocDatalog != nil {
		t.Fatal("nonempty initial finite head silently changed into product storage")
	}
}

func TestDatalogUDocProductsAdoptedHeadFactsSurviveSourceInvalidationAndQueryClone(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "lazy_sparse"} {
		for _, mutateDoc := range []bool{false, true} {
			name := tableKind + "/finite-update"
			if mutateDoc {
				name = tableKind + "/doc-update"
			}
			t.Run(name, func(t *testing.T) {
				f := newPublicUDocProductFixture(t, tableKind, false, false)
				fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
				fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
				f.addRule(false)
				f.checkRows(lbool.True, [2]uint32{1, 1})
				owner := f.e.udocDatalog
				if owner == nil || owner.products.result[f.head] == nil {
					t.Fatal("fixture did not infer product head storage")
				}
				if mutateDoc {
					fragmentMust(t, f.e.AddFact(f.doc, []ast.ASTHandle{f.values[2]}))
				} else {
					fragmentMust(t, f.e.AddFact(f.finite, []ast.ASTHandle{f.no, f.values[1]}))
				}
				if owner.products.result[f.head] != nil {
					t.Fatal("source mutation left prior product result published")
				}
				// No intervening query may be needed to recover the head kind.
				fragmentMust(t, f.e.AddFact(f.head, []ast.ASTHandle{f.values[3], f.no}))
				if f.e.udocDatalog != owner || f.e.predicates[f.head].table != nil || owner.products.seeds[f.head] == nil {
					t.Fatal("adopted head fact abandoned its source product kind")
				}
				rows := [][2]uint32{{1, 1}, {3, 0}}
				if !mutateDoc {
					rows = append(rows, [2]uint32{1, 0})
				}
				f.checkRows(lbool.True, rows...)
				query := f.app(f.head, f.x, f.b)
				status, err := f.e.QueryExpression(query)
				fragmentMust(t, err)
				if status != lbool.True || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
					t.Fatal("adopted head expression query", status)
				}
				clone := f.e.queryClone.udocDatalog
				if clone.products.seeds[f.head] == owner.products.seeds[f.head] {
					t.Fatal("query clone shared the mutated product seed owner")
				}
				if tableKind == "lazy_sparse" {
					if _, ok := clone.products.seeds[f.head].Table().(*rel.LazySparseTable); !ok {
						t.Fatal("adopted lazy product seed changed storage in query clone")
					}
				}
				answer, err := f.e.ExpressionAnswer()
				fragmentMust(t, err)
				defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
				f.checkFormula(answer, []ast.ASTHandle{f.no, f.values[3]}, true)
				f.checkFormula(answer, []ast.ASTHandle{f.yes, f.values[3]}, false)
				fragmentMust(t, f.e.clearQueryResources())
				f.checkFormula(answer, []ast.ASTHandle{f.no, f.values[3]}, true)
				f.checkRows(lbool.True, rows...)
			})
		}
	}
}

func TestDatalogUDocProductsSourceExpressionQueriesAfterAdoption(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(tableKind, func(t *testing.T) {
			f := newPublicUDocProductFixture(t, tableKind, false, false)
			fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
			f.addRule(false)
			f.checkRows(lbool.True, [2]uint32{1, 1})
			b0 := f.pin(f.m.MkVar(0, f.boolean))
			x1 := f.pin(f.m.MkVar(1, f.bits))
			for _, tc := range []struct {
				name     string
				query    ast.ASTHandle
				status   lbool.LBool
				bindings []ast.ASTHandle
				want     bool
			}{
				{"doc-full", f.app(f.doc, f.x), lbool.True, []ast.ASTHandle{f.values[1]}, true},
				{"doc-ground", f.app(f.doc, f.values[1]), lbool.True, nil, true},
				{"doc-ground-absent", f.app(f.doc, f.values[2]), lbool.False, nil, false},
				{"table-full", f.app(f.finite, f.b, f.x), lbool.True, []ast.ASTHandle{f.yes, f.values[1]}, true},
				{"table-permuted", f.app(f.finite, b0, x1), lbool.True, []ast.ASTHandle{f.values[1], f.yes}, true},
				{"table-ground", f.app(f.finite, f.yes, f.values[1]), lbool.True, nil, true},
				{"table-ground-absent", f.app(f.finite, f.no, f.values[1]), lbool.False, nil, false},
				{"table-project-bool", f.app(f.finite, b0, f.values[1]), lbool.True, []ast.ASTHandle{f.yes}, true},
				{"table-project-bv", f.app(f.finite, f.yes, f.x), lbool.True, []ast.ASTHandle{f.values[1]}, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					local := *f
					local.t = t
					status, err := f.e.QueryExpression(tc.query)
					fragmentMust(t, err)
					if status != tc.status || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
						t.Fatal("source expression query lost adopted DOC execution", status, tc.status)
					}
					answer, err := f.e.ExpressionAnswer()
					fragmentMust(t, err)
					defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
					local.checkFormula(answer, tc.bindings, tc.want)
					fragmentMust(t, f.e.clearQueryResources())
					local.checkFormula(answer, tc.bindings, tc.want)
				})
			}
			f.checkRows(lbool.True, [2]uint32{1, 1})
		})
	}
}

func TestDatalogUDocProductsEmptyFirstQueryKeepsOrdinaryHeadKind(t *testing.T) {
	for _, tableKind := range []string{"hashtable", "lazy_sparse"} {
		t.Run(tableKind, func(t *testing.T) {
			f := newPublicUDocProductFixture(t, tableKind, false, false)
			f.addRule(false)
			f.checkRows(lbool.False)
			owner := f.e.udocDatalog
			ordinary := f.e.predicates[f.head].table
			if owner == nil || owner.products == nil || ordinary == nil || owner.products.seeds[f.head] != nil || owner.products.result[f.head] != nil {
				t.Fatal("empty first query inferred a product head without incoming rows")
			}
			fragmentMust(t, f.e.AddFact(f.head, []ast.ASTHandle{f.values[3], f.no}))
			if f.e.predicates[f.head].table != ordinary || owner.products.seeds[f.head] != nil {
				t.Fatal("head fact after empty query changed ordinary storage")
			}
			f.checkRows(lbool.True, [2]uint32{3, 0})
			if owner.products.seeds[f.head] != nil || owner.finiteResult[f.head] == nil {
				t.Fatal("ordinary head seed was answered through fabricated product storage")
			}
			if found, err := owner.finiteResult[f.head].Contains(rel.Fact{3, 0}); err != nil || !found {
				t.Fatal("ordinary head snapshot lost its seed", found, err)
			}
			query := f.app(f.head, f.x, f.b)
			status, err := f.e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil {
				t.Fatal("ordinary head expression query after empty product run", status)
			}
			clone := f.e.queryClone
			if clone.predicates[f.head].table == nil || clone.predicates[f.head].table == ordinary || clone.udocDatalog.products.seeds[f.head] != nil || clone.udocDatalog.finiteResult[f.e.queryClonePredicate] == nil {
				t.Fatal("ordinary head expression query lost its concrete finite owner")
			}
			answer, err := f.e.ExpressionAnswer()
			fragmentMust(t, err)
			defer func() { fragmentMust(t, f.m.DelRef(&answer)) }()
			f.checkFormula(answer, []ast.ASTHandle{f.no, f.values[3]}, true)
			f.checkFormula(answer, []ast.ASTHandle{f.yes, f.values[3]}, false)
			fragmentMust(t, f.e.clearQueryResources())
			f.checkFormula(answer, []ast.ASTHandle{f.no, f.values[3]}, true)
			fragmentMust(t, f.e.AddFact(f.doc, []ast.ASTHandle{f.values[1]}))
			fragmentMust(t, f.e.AddFact(f.finite, []ast.ASTHandle{f.yes, f.values[1]}))
			status, err = f.e.QueryPredicate(f.head)
			if status != lbool.Undef || !errors.Is(err, ErrUDocDatalogFragment) {
				t.Fatal("incoming product silently upgraded seeded ordinary head", status, err)
			}
			if f.e.predicates[f.head].table != ordinary || owner.products.seeds[f.head] != nil {
				t.Fatal("unsupported union changed the ordinary head kind")
			}
			if found, err := ordinary.Contains(rel.Fact{3, 0}); err != nil || !found {
				t.Fatal("unsupported union consumed the ordinary head seed", found, err)
			}
			if _, err := f.e.Answer(f.head); err == nil {
				t.Fatal("failed product union exposed an earlier answer")
			}
		})
	}
}
