package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocPublicFactsProjectionQueryAndOwnership(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	small, err := m.BitVectorSort(2)
	must(err)
	wide, err := m.BitVectorSort(4)
	must(err)
	declare := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("doc-public-P", []ast.ASTHandle{small, wide})
	q := declare("doc-public-Q", []ast.ASTHandle{wide})
	empty := declare("doc-public-empty", []ast.ASTHandle{wide})
	must(e.AddTableFact(p, []uint32{2, 9}))
	if e.udocDatalog == nil || e.predicates[p].table != nil {
		t.Fatal("doc favourite did not select actual UDoc storage")
	}
	x, err := m.MkVar(0, small)
	must(err)
	y, err := m.MkVar(1, wide)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, args)
		must(err)
		return h
	}
	pa, qa, ea := app(p, x, y), app(q, y), app(empty, y)
	rule, err := m.MkImplies(pa, qa)
	must(err)
	must(e.AddRule(rule, "doc-project"))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("UDoc projection", status)
	}
	answer, err := e.Answer(p)
	must(err)
	formula, err := m.RefGet(answer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	for _, tc := range []struct {
		a, b uint64
		want bool
	}{{2, 9, true}, {1, 9, false}, {2, 8, false}} {
		a, err := m.BitVectorNumeralUint64(tc.a, 2)
		must(err)
		ar, err := m.NewRef(a)
		must(err)
		b, err := m.BitVectorNumeralUint64(tc.b, 4)
		must(err)
		br, err := m.NewRef(b)
		must(err)
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(formula, []ast.ASTHandle{a, b}, ground))
		h, err := m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		truth, err := m.RefGet(reduced)
		must(err)
		if m.IsTrue(truth) != tc.want || !m.IsTrue(truth) && !m.IsFalse(truth) {
			t.Fatal("UDoc answer columns/widths", tc)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
		must(m.DelRef(&br))
		must(m.DelRef(&ar))
	}
	must(substitution.Close())
	must(m.DelRef(&answer))
	queryRef, err := m.NewRef(pa)
	must(err)
	status, err = e.QueryExpression(pa)
	must(err)
	must(m.DelRef(&queryRef))
	if status != lbool.True || e.queryClone == nil || e.queryClone.udocDatalog == nil || e.queryClone.udocDatalog == e.udocDatalog {
		t.Fatal("UDoc expression clone", status)
	}
	held, err := e.ExpressionAnswer()
	must(err)
	status, err = e.QueryExpression(pa)
	must(err)
	if status != lbool.True {
		t.Fatal("repeated UDoc expression query", status)
	}

	must(e.clearQueryResources())
	if _, err = m.RefGet(held); err != nil {
		t.Fatal("UDoc held formula lost clone", err)
	}
	must(m.DelRef(&held))
	guarded, err := m.MkAnd(pa, ea)
	must(err)
	strong, err := m.MkImplies(guarded, qa)
	must(err)
	must(e.UpdateRule(strong, "doc-project"))
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.False {
		t.Fatal("UDoc update preserved stale result", status)
	}
	must(e.AddTableFact(empty, []uint32{9}))
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("UDoc fact seed lost after update", status)
	}
	m.Limit.Cancel()
	_, err = e.QueryPredicate(q)
	m.Limit.ResetCancel()
	if err == nil {
		t.Fatal("UDoc query ignored cancellation")
	}
	if _, err = e.Answer(q); err == nil {
		t.Fatal("cancelled UDoc query exposed stale answer")
	}
}

func TestDatalogUDocSelectionAndNullary(t *testing.T) {
	for _, mode := range []string{"favourite", "requested", "finite-override", "bool-fallback", "arithmetic-fallback", "nullary", "ground-rule"} {
		t.Run(mode, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			if mode != "requested" {
				must(g.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
			}
			must(e.SelectEngine("datalog"))
			boolean, err := m.MkBoolSort()
			must(err)
			sort, err := m.BitVectorSort(3)
			must(err)
			if mode == "bool-fallback" {
				sort = boolean
			}
			if mode == "arithmetic-fallback" {
				family, err := m.RegisterArithmeticPlugin()
				must(err)
				sort, err = m.MkFamilySort(family, 1, nil)
				must(err)
			}
			sorts := []ast.ASTHandle{sort}
			values := []uint32{1}
			if mode == "nullary" {
				sorts = nil
				values = nil
			}
			name, err := m.Symbols.Intern("doc-selection")
			must(err)
			p, err := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			if mode == "requested" {
				must(e.SetPredicateRepresentation(p, []string{"doc"}))
			}
			if mode == "finite-override" {
				must(e.SetPredicateRepresentation(p, []string{"tr_hashtable"}))
			}
			if mode == "ground-rule" {
				value, err := m.MkDatalogNumeral(1, sort)
				must(err)
				fact, err := m.MkApp(p, []ast.ASTHandle{value})
				must(err)
				must(e.AddRule(fact, "doc-ground-fact"))
			} else {
				must(e.AddTableFact(p, values))
			}

			result, err := e.QueryPredicate(p)
			must(err)
			want := lbool.True
			if mode == "arithmetic-fallback" {
				want = lbool.Undef
			}
			if result != want {
				t.Fatal("doc selection result", result, want)
			}
			selected := mode == "favourite" || mode == "requested" || mode == "nullary" || mode == "ground-rule"
			if (e.udocDatalog != nil) != selected {
				t.Fatal("wrong source doc capability/precedence", mode, e.udocDatalog != nil)
			}
			if mode == "arithmetic-fallback" && e.boundDatalog == nil {
				t.Fatal("doc capability failure skipped source bound fallback")
			}
		})
	}
}
