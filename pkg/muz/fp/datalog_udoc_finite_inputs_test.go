package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestDatalogUDocFiniteForwardingStorageQueriesAndClones(t *testing.T) {
	for _, representation := range []string{"tr_hashtable", "tr_sparse", "tr_lazy_sparse"} {
		t.Run(representation, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
			must(e.SelectEngine("datalog"))
			boolean, err := m.MkBoolSort()
			must(err)
			bv, err := m.BitVectorSort(1)
			must(err)
			declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
				n, err := m.Symbols.Intern(name)
				must(err)
				d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			source := declare("doc-forward-table", []ast.ASTHandle{bv, bv})
			target := declare("doc-forward-target", []ast.ASTHandle{bv, bv})
			projected := declare("doc-forward-projected", []ast.ASTHandle{bv})
			must(e.SetPredicateRepresentation(source, []string{representation}))
			original := e.predicates[source].table
			switch representation {
			case "tr_hashtable":
				if _, ok := original.(*rel.HashTable); !ok {
					t.Fatal("expected hash input")
				}
			case "tr_sparse":
				if _, ok := original.(*rel.SparseTable); !ok {
					t.Fatal("expected sparse input")
				}
			case "tr_lazy_sparse":
				if _, ok := original.(*rel.LazySparseTable); !ok {
					t.Fatal("expected lazy input")
				}
			}
			must(e.AddTableFact(source, []uint32{1, 0}))
			must(e.AddTableFact(target, []uint32{0, 1}))
			if e.udocDatalog != nil {
				t.Fatal("mixed facts adopted before a forwarding consumer existed")
			}
			x, err := m.MkVar(0, bv)
			must(err)
			y, err := m.MkVar(1, bv)
			must(err)
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
				h, err := m.MkApp(d, args)
				must(err)
				return h
			}
			ta, da, pa := app(source, x, y), app(target, x, y), app(projected, x)
			rule, err := m.MkImplies(ta, da)
			must(err)
			must(e.AddRule(rule, "table-to-doc"))
			rule, err = m.MkImplies(da, pa)
			must(err)
			must(e.AddRule(rule, "doc-projection-after-table-union"))
			for i := 0; i < 2; i++ {
				checkDatalogUDocBV1Tuples(t, m, e, target, 2, [][]uint64{{0, 1}, {1, 0}})
				checkDatalogUDocBV1Tuples(t, m, e, projected, 1, [][]uint64{{0}, {1}})
				checkDatalogUDocBV1Tuples(t, m, e, source, 2, [][]uint64{{1, 0}})
			}
			if e.udocDatalog == nil || e.predicates[target].table != nil || e.predicates[source].table != original || e.udocDatalog.finiteInputs[source].table != original {
				t.Fatal("forwarding replaced the finite source or used finite target storage")
			}
			must(e.AddTableFact(source, []uint32{0, 0}))
			checkDatalogUDocBV1Tuples(t, m, e, target, 2, [][]uint64{{0, 0}, {0, 1}, {1, 0}})
			query := app(target, x, y)
			root, err := m.NewRef(query)
			must(err)
			defer func() { must(m.DelRef(&root)) }()
			status, err := e.QueryExpression(query)
			must(err)
			if status != lbool.True || e.queryClone == nil || e.queryClone.udocDatalog == nil {
				t.Fatal("forwarding expression query missed DOC", status)
			}
			clone := e.queryClone
			copy := clone.predicates[source].table
			if copy == nil || copy == original || clone.udocDatalog.finiteInputs[source].table != copy {
				t.Fatal("query clone did not own independent finite source")
			}
			must(copy.Reset())
			checkDatalogUDocBV1Tuples(t, m, clone, target, 2, [][]uint64{{0, 1}})
			if length, err := original.Len(); err != nil || length != 2 {
				t.Fatal("clone changed source table", length, err)
			}
			checkDatalogUDocBV1Tuples(t, m, e, target, 2, [][]uint64{{0, 0}, {0, 1}, {1, 0}})
			finiteQuery := app(source, x, y)
			finiteRoot, err := m.NewRef(finiteQuery)
			must(err)
			status, err = e.QueryExpression(finiteQuery)
			must(err)
			must(m.DelRef(&finiteRoot))
			if status != lbool.True || e.queryClone == nil || e.queryClone.udocDatalog == nil {
				t.Fatal("finite expression query missed the universal DOC forwarding consumer", status)
			}
			foundAuxiliary := false
			for d, input := range e.queryClone.udocDatalog.finiteInputs {
				if !input.auxiliary {
					continue
				}
				foundAuxiliary = true
				output := e.queryClone.udocDatalog.finiteResult[d]
				if found, err := output.Contains(rel.Fact{0, 1}); err != nil || !found {
					t.Fatal("finite query head lost descending variable order", found, err)
				}
				if found, err := output.Contains(rel.Fact{1, 0}); err != nil || found {
					t.Fatal("finite query head kept source column order", found, err)
				}
			}
			if !foundAuxiliary {
				t.Fatal("finite query did not preserve its requested table representation")
			}
		})
	}
}

func TestDatalogUDocFiniteForwardingKeepsUnsupportedInitialRoute(t *testing.T) {
	for _, checked := range []bool{false, true} {
		t.Run(map[bool]string{false: "raw-permutation", true: "checked-forward"}[checked], func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
			if checked {
				must(g.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
			}
			must(e.SelectEngine("datalog"))
			boolean, err := m.MkBoolSort()
			must(err)
			bv, err := m.BitVectorSort(1)
			must(err)
			declarations := make([]ast.ASTHandle, 2)
			for i, name := range []string{"forward-boundary-table", "forward-boundary-doc"} {
				n, err := m.Symbols.Intern(name)
				must(err)
				declarations[i], err = m.MkFuncDecl(n, []ast.ASTHandle{bv, bv}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(declarations[i]))
			}
			source, target := declarations[0], declarations[1]
			must(e.SetPredicateRepresentation(source, []string{"tr_hashtable"}))
			x, err := m.MkVar(0, bv)
			must(err)
			y, err := m.MkVar(1, bv)
			must(err)
			body, err := m.MkApp(source, []ast.ASTHandle{x, y})
			must(err)
			args, expected := []ast.ASTHandle{y, x}, [][]uint64{{0, 1}}
			if checked {
				args, expected = []ast.ASTHandle{x, y}, [][]uint64{{1, 0}}
			}
			head, err := m.MkApp(target, args)
			must(err)
			rule, err := m.MkImplies(body, head)
			must(err)
			must(e.AddRule(rule, "outside-raw-forwarding"))
			must(e.AddTableFact(source, []uint32{1, 0}))
			checkDatalogUDocBV1Tuples(t, m, e, target, 2, expected)
			if e.udocDatalog != nil {
				t.Fatal("unsupported mixed route adopted DOC")
			}
		})
	}
}

func TestDatalogUDocFiniteForwardingRefusesNewMixedJoinAfterAdoption(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	bv, err := m.BitVectorSort(1)
	must(err)
	decls := make([]ast.ASTHandle, 2)
	for i, name := range []string{"forward-adopted-table", "forward-adopted-doc"} {
		n, err := m.Symbols.Intern(name)
		must(err)
		decls[i], err = m.MkFuncDecl(n, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(decls[i]))
	}
	source, target := decls[0], decls[1]
	must(e.SetPredicateRepresentation(source, []string{"tr_hashtable"}))
	x, err := m.MkVar(0, bv)
	must(err)
	ta, err := m.MkApp(source, []ast.ASTHandle{x})
	must(err)
	da, err := m.MkApp(target, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(ta, da)
	must(err)
	must(e.AddRule(rule, "adopt-forwarding"))
	must(e.AddTableFact(source, []uint32{1}))
	checkDatalogUDocBV1Tuples(t, m, e, target, 1, [][]uint64{{1}})
	body, err := m.MkAndArgs([]ast.ASTHandle{ta, da})
	must(err)
	rule, err = m.MkImplies(body, da)
	must(err)
	must(e.AddRule(rule, "mixed-product-still-outside-slice"))
	status, err := e.QueryPredicate(target)
	if status != lbool.Undef || !errors.Is(err, ErrUDocDatalogFragment) {
		t.Fatal("mixed join silently changed adopted execution route", status, err)
	}
	if found, err := e.predicates[source].table.Contains(rel.Fact{1}); err != nil || !found {
		t.Fatal("unsupported query changed finite source", found, err)
	}
}

func TestDatalogUDocFiniteInputAnswerAllocationFailureAndCanonicalShape(t *testing.T) {
	failures, completed := 0, 0
	// Debug AST storage uses direct blocks instead of amortized pooled pages.
	// Keep every original checkpoint through 512, then extend the injected
	// allowance until a fresh fixture completes, with a finite upper bound.
	for allowance := int64(0); allowance <= 1<<16; {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if budget != nil && budget.Used() != 0 {
					t.Fatal("finite answer fixture leaked bytes", budget.Used())
				}
			})
			m, g := newPDRLifecycleServices(t)
			budget = m.Budget()
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			boolean, err := m.MkBoolSort()
			must(err)
			bv, err := m.BitVectorSort(10)
			must(err)
			name, err := m.Symbols.Intern("finite-answer-owner")
			must(err)
			decl, err := m.MkFuncDecl(name, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(decl))
			table := e.predicates[decl].table
			c := &datalogUDocContext{manager: m, relations: e.relations, finiteInputs: map[ast.ASTHandle]*udocFiniteInput{decl: {table: table, sorts: []ast.ASTHandle{bv}}}}
			empty, err := c.finiteInputAnswer(decl, table)
			must(err)
			h, err := m.RefGet(empty)
			must(err)
			if !m.IsFalse(h) {
				t.Fatal("empty finite input answer was not canonical false")
			}
			must(m.DelRef(&empty))
			_, err = table.Add(rel.Fact{5})
			must(err)
			singleton, err := c.finiteInputAnswer(decl, table)
			must(err)
			h, err = m.RefGet(singleton)
			must(err)
			declKind, err := m.GetDeclKind(h)
			must(err)
			if declKind != ast.ASTOpEq {
				t.Fatal("singleton finite input answer gained Boolean wrappers", declKind)
			}
			must(m.DelRef(&singleton))
			// Enough distinct rows cross actual AST storage page boundaries;
			// each failure ordinal therefore reaches real allocations.
			for value := rel.Element(0); value < 512; value++ {
				_, err = table.Add(rel.Fact{value})
				must(err)
			}
			before := m.Stats()
			budget.FailAfter(allowance)
			answer, cause := c.finiteInputAnswer(decl, table)
			budget.FailAfter(-1)
			var receipt memory.ConstructionFailure
			if errors.As(cause, &receipt) {
				must(receipt.Cleanup())
				must(receipt.Cleanup())
			}
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) || answer != (ast.ASTRef{}) {
					t.Fatal("failed finite answer publication", answer, cause)
				}
			} else {
				completed++
				must(m.DelRef(&answer))
			}
			after := m.Stats()
			if before.LiveReferences != after.LiveReferences || before.LiveNodes != after.LiveNodes {
				t.Fatal("finite answer leaked refs or unowned nodes", before, after, cause)
			}
			retry, err := c.finiteInputAnswer(decl, table)
			must(err)
			must(m.DelRef(&retry))
			if length, err := table.Len(); err != nil || length != 512 {
				t.Fatal("answer failure changed borrowed input", length, err)
			}
		})
		if allowance >= 512 && completed != 0 {
			break
		}
		if allowance == 0 {
			allowance = 1
		} else {
			allowance *= 2
		}
	}
	if failures == 0 || completed == 0 {
		t.Fatal("finite answer campaign missed failure or completion", failures, completed)
	}
}
