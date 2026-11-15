package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogCompiledMixedSparseHashAndNullary(t *testing.T) {
	testDatalogCompiledMixedFiniteAndNullary(t, "tr_sparse")
}
func TestDatalogCompiledMixedLazySparseAndNullary(t *testing.T) {
	testDatalogCompiledMixedFiniteAndNullary(t, "tr_lazy_sparse")
}
func TestDatalogCompiledDefaultLazySparseMixedAndNullary(t *testing.T) {
	testDatalogCompiledMixedFiniteAndNullary(t, "default_lazy_sparse")
}
func testDatalogCompiledMixedFiniteAndNullary(t *testing.T, representation string) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	if representation == "default_lazy_sparse" {
		must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "lazy_sparse"))
	}
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, sparse bool) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		if sparse && representation != "default_lazy_sparse" {
			must(e.SetPredicateRepresentation(d, []string{representation}))
		}
		return d
	}
	p, r, q := declare("compiled-sparse-source", true), declare("compiled-hash-filter", false), declare("compiled-sparse-target", true)
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	must(e.AddFact(p, []ast.ASTHandle{no}))
	must(e.AddFact(p, []ast.ASTHandle{yes}))
	must(e.AddFact(r, []ast.ASTHandle{yes}))
	must(e.AddFact(f.declarations["q"], nil))
	x, err := m.MkVar(0, boolean)
	must(err)
	app := func(d, arg ast.ASTHandle) ast.ASTHandle {
		v, err := m.MkApp(d, []ast.ASTHandle{arg})
		must(err)
		return v
	}
	body, err := m.MkAnd(app(p, x), app(r, x))
	must(err)
	body, err = m.MkAnd(body, f.applications["q"])
	must(err)
	rule, err := m.MkImplies(body, app(q, x))
	must(err)
	must(e.addRuleEager(rule, "mixed-three-tail"))
	rule, err = m.MkImplies(app(q, yes), f.applications["u"])
	must(err)
	must(e.addRuleEager(rule, "project-all-true"))
	rule, err = m.MkImplies(app(q, no), f.applications["p"])
	must(err)
	must(e.addRuleEager(rule, "project-all-false"))
	for _, tc := range []struct {
		decl ast.ASTHandle
		want lbool.LBool
	}{{q, lbool.True}, {f.declarations["u"], lbool.True}, {f.declarations["p"], lbool.False}, {q, lbool.True}} {
		status, err := e.QueryPredicate(tc.decl)
		must(err)
		if status != tc.want || !e.compiled {
			t.Fatal("mixed sparse query did not execute compiled semantics", status, tc.want, e.compiled)
		}
		// q starts empty, so instr_io loads it as a null register and
		// instr_union creates the target with the rule result's plugin
		// (r_src.get_plugin().mk_empty(r_src)); store_relation then keeps
		// that relation, not one of q's requested kind (muz_rel_03-F1).
		selected := e.predicates[q].table
		rows, err := selected.Facts()
		must(err)
		if len(rows) != 1 || len(rows[0]) != 1 || rows[0][0] != 1 {
			t.Fatal("mixed sparse/hash join tuples", rows)
		}
		// When the lazy_sparse kind reaches u, the nullary union target comes
		// from the lazy source plugin: a lazy table over the Hash fallback
		// (the fork's lazy_table_plugin::mk_empty nullary fix). With
		// tr_lazy_sparse the join planner's split (compiled-hash-filter
		// joined with compiled-sparse-source, as z3 4.7.1 chooses) carries
		// the lazy kind to u.
		switch e.predicates[f.declarations["u"]].table.(type) {
		case *rel.HashTable:
		case *rel.LazySparseTable:
			if representation != "default_lazy_sparse" && representation != "tr_lazy_sparse" {
				t.Fatal("nullary output lost Hash fallback")
			}
		default:
			t.Fatal("nullary output lost Hash fallback")
		}
	}
}

func TestDatalogCompiledSparseRecursiveFixpoint(t *testing.T) {
	testDatalogCompiledFiniteRecursiveFixpoint(t, "tr_sparse")
}
func TestDatalogCompiledLazySparseRecursiveFixpoint(t *testing.T) {
	testDatalogCompiledFiniteRecursiveFixpoint(t, "tr_lazy_sparse")
}
func TestDatalogCompiledDefaultLazySparseRecursiveFixpoint(t *testing.T) {
	testDatalogCompiledFiniteRecursiveFixpoint(t, "default_lazy_sparse")
}
func testDatalogCompiledFiniteRecursiveFixpoint(t *testing.T, representation string) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	if representation == "default_lazy_sparse" {
		must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "lazy_sparse"))
	}
	domainName, err := m.Symbols.Intern("compiled-recursive-domain")
	must(err)
	domain, err := m.MkDatalogFiniteSort(domainName, 5)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		symbol, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(symbol, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		if representation != "default_lazy_sparse" {
			must(e.SetPredicateRepresentation(d, []string{representation}))
		}
		return d
	}
	edge := declare("compiled-sparse-edge", []ast.ASTHandle{domain, domain})
	reach := declare("compiled-sparse-reach", []ast.ASTHandle{domain})
	values := make([]ast.ASTHandle, 5)
	for i := range values {
		values[i], err = m.MkDatalogNumeral(uint64(i), domain)
		must(err)
	}
	must(e.AddFact(reach, values[:1]))
	for _, pair := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 1}} {
		must(e.AddFact(edge, []ast.ASTHandle{values[pair[0]], values[pair[1]]}))
	}
	x, err := m.MkVar(0, domain)
	must(err)
	y, err := m.MkVar(1, domain)
	must(err)
	rx, err := m.MkApp(reach, []ast.ASTHandle{x})
	must(err)
	exy, err := m.MkApp(edge, []ast.ASTHandle{x, y})
	must(err)
	ry, err := m.MkApp(reach, []ast.ASTHandle{y})
	must(err)
	body, err := m.MkAnd(rx, exy)
	must(err)
	rule, err := m.MkImplies(body, ry)
	must(err)
	must(e.addRuleEager(rule, "recursive-step"))
	for i := 0; i < 2; i++ {
		status, err := e.QueryPredicate(reach)
		must(err)
		if status != lbool.True || !e.compiled {
			t.Fatal("recursive sparse query did not run compiled fixpoint", status, e.compiled)
		}
		table := e.predicates[reach].table
		if !matchesCompiledRepresentation(table, representation) {
			t.Fatal("recursive store changed representation")
		}
		rows, err := table.Facts()
		must(err)
		if len(rows) != 4 {
			t.Fatal("recursive sparse closure", rows)
		}
		for value := rel.Element(0); value < 5; value++ {
			found, err := table.Contains(rel.Fact{value})
			must(err)
			if found != (value < 4) {
				t.Fatal("recursive sparse reachability", value, found)
			}
		}
	}
}

func matchesCompiledRepresentation(table rel.FiniteTable, representation string) bool {
	if representation == "tr_lazy_sparse" || representation == "default_lazy_sparse" {
		_, ok := table.(*rel.LazySparseTable)
		return ok
	}
	_, ok := table.(*rel.SparseTable)
	return ok
}
