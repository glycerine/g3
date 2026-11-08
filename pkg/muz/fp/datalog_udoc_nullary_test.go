package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocFiniteNullaryGuardsPreserveStorageAndClones(t *testing.T) {
	for _, favourite := range []string{"hashtable", "sparse", "lazy_sparse"} {
		for _, guardFirst := range []bool{true, false} {
			name := favourite + "/last"
			if guardFirst {
				name = favourite + "/first"
			}
			t.Run(name, func(t *testing.T) {
				m, g := newPDRLifecycleServices(t)
				e, err := NewPropositionalEngineWithServices(m, g.Context, g)
				fragmentMust(t, err)
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				defer func() { must(e.Close()) }()
				must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
				must(g.SourceSetName("fixedpoint.datalog.default_table", favourite))
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
				p := declare("doc-mixed-P", []ast.ASTHandle{bv})
				q := declare("doc-mixed-Q", []ast.ASTHandle{bv})
				// Adopt UDoc facts before the finite guard is introduced.
				must(e.AddTableFact(p, []uint32{1}))
				gate := declare("doc-mixed-gate", nil)
				// Sparse and LazySparse source plugins reject nullary signatures;
				// explicitly select the supporting Hash plugin for the guard.
				must(e.SetPredicateRepresentation(gate, []string{"tr_hashtable"}))
				original := e.predicates[gate].table
				if _, ok := original.(*rel.HashTable); !ok {
					t.Fatal("explicit finite guard did not use Hash storage")
				}
				x, err := m.MkVar(0, bv)
				must(err)
				pa, err := m.MkApp(p, []ast.ASTHandle{x})
				must(err)
				qa, err := m.MkApp(q, []ast.ASTHandle{x})
				must(err)
				ga, err := m.MkApp(gate, nil)
				must(err)
				terms := []ast.ASTHandle{pa, ga}
				if guardFirst {
					terms[0], terms[1] = terms[1], terms[0]
				}
				body, err := m.MkAndArgs(terms)
				must(err)
				rule, err := m.MkImplies(body, qa)
				must(err)
				must(e.addRuleEager(rule, "doc-mixed-guard"))
				checkDatalogUDocBV1Tuples(t, m, e, q, 1, nil)
				if e.udocDatalog == nil || e.predicates[p].table != nil || e.predicates[gate].table != original || e.udocDatalog.finiteNullary[gate] != original {
					t.Fatal("mixed query replaced a concrete relation owner")
				}
				must(e.AddTableFact(gate, nil))
				checkDatalogUDocBV1Tuples(t, m, e, q, 1, [][]uint64{{1}})
				status, err := e.QueryPredicate(gate)
				must(err)
				if status != lbool.True {
					t.Fatal("finite nullary query lost its fact", status)
				}
				answer, err := e.Answer(gate)
				must(err)
				handle, err := m.RefGet(answer)
				must(err)
				if !m.IsTrue(handle) {
					t.Fatal("finite nullary answer is not true")
				}
				must(m.DelRef(&answer))
				qa, err = m.MkApp(q, []ast.ASTHandle{x})
				must(err)
				queryRoot, err := m.NewRef(qa)
				must(err)
				defer func() { must(m.DelRef(&queryRoot)) }()
				status, err = e.QueryExpression(qa)
				must(err)
				if status != lbool.True || e.queryClone == nil || e.queryClone.udocDatalog == nil {
					t.Fatal("mixed expression query lost UDoc execution", status)
				}
				clone := e.queryClone
				copy := clone.predicates[gate].table
				if copy == nil || copy == original || clone.udocDatalog.finiteNullary[gate] != copy {
					t.Fatal("mixed expression guard did not use independent finite storage")
				}
				must(copy.Reset())
				status, err = clone.QueryPredicate(q)
				must(err)
				if status != lbool.False {
					t.Fatal("clone ignored its empty guard", status)
				}
				if present, err := original.Contains(nil); err != nil || !present {
					t.Fatal("clone mutation changed source guard", present, err)
				}
				checkDatalogUDocBV1Tuples(t, m, e, q, 1, [][]uint64{{1}})
			})
		}
	}
}

func TestDatalogUDocNullaryRepresentationMigrationAndRetry(t *testing.T) {
	for _, present := range []bool{false, true} {
		failures, complete := 0, false
		for stop := int64(0); stop < 256 && !complete; stop++ {
			t.Run(fmt.Sprintf("present=%v/stop=%d", present, stop), func(t *testing.T) {
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
				p := declare("doc-migrate-P", []ast.ASTHandle{bv})
				// DOC storage has its own allocator configuration. Use its real
				// budget here so injected failures reach migration allocations,
				// rather than only unrelated AST allocations.
				factory, err := rel.NewUDocRelationFactory(m, arena.VectorConfig{PageSize: 4, Descriptors: arena.Config{PageSize: 4, Budget: m.Budget()}})
				must(err)
				e.udocDatalog = &datalogUDocContext{manager: m, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}
				must(e.AddTableFact(p, []uint32{1}))
				gate := declare("doc-migrate-gate", nil)
				must(e.SetPredicateRepresentation(gate, []string{"tr_hashtable"}))
				if present {
					must(e.AddTableFact(gate, nil))
				} else {
					must(e.AddTableFact(p, []uint32{1}))
				}
				original := e.predicates[gate].table
				if e.udocDatalog.finiteNullary[gate] != original {
					t.Fatal("migration fixture did not borrow its finite guard")
				}
				must(e.SetPredicateRepresentation(gate, []string{"doc"}))
				budget := m.Budget()
				budget.FailAfter(stop)
				selected, cause := e.ensureUDocDatalog()
				budget.FailAfter(-1)
				if cause != nil {
					failures++
					if !errors.Is(cause, arena.ErrInjectedFailure) || e.predicates[gate].table != original || e.udocDatalog.finiteNullary[gate] != original {
						t.Fatal("failed migration published a replacement", selected, cause)
					}
					selected, err = e.ensureUDocDatalog()
					must(err)
				} else {
					complete = true
				}
				if !selected || e.predicates[gate].table != nil || e.udocDatalog.finiteNullary[gate] != nil || len(e.udocDatalog.finiteOrder) != 0 {
					t.Fatal("migration left a stale finite guard owner")
				}
				if got, err := original.Contains(nil); err != nil || got != present {
					t.Fatal("migration mutated its borrowed finite source", got, err)
				}
				if got, err := e.udocDatalog.seeds[gate].ContainsFact(nil); err != nil || got != present {
					t.Fatal("migration lost nullary truth", got, err)
				}
				status, err := e.QueryPredicate(gate)
				must(err)
				want := lbool.False
				if present {
					want = lbool.True
				}
				if status != want {
					t.Fatal("migrated nullary query", status, want)
				}
			})
			if t.Failed() {
				return
			}
		}
		if !complete || failures == 0 {
			t.Fatal("migration campaign missed refusal or completion", present, failures, complete)
		}
	}
}

func TestDatalogUDocMixedFiniteHeadRefusesBeforeRulePublication(t *testing.T) {
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
	p := declare("doc-mixed-head-P", []ast.ASTHandle{bv})
	must(e.AddTableFact(p, []uint32{1}))
	gate := declare("doc-mixed-head-gate", nil)
	must(e.SetPredicateRepresentation(gate, []string{"tr_hashtable"}))
	ga, err := m.MkApp(gate, nil)
	must(err)
	must(e.addRuleEager(ga, "finite-head-outside-mixed-guard-slice"))
	status, err := e.QueryPredicate(p)
	if status != lbool.Undef || !errors.Is(err, ErrUDocDatalogFragment) {
		t.Fatal("mixed finite head was published without its union dispatch", status, err)
	}
	if present, err := e.predicates[gate].table.Contains(nil); err != nil || present {
		t.Fatal("unsupported finite head changed storage", present, err)
	}
}
