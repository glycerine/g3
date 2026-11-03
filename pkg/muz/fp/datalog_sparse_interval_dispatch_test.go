package fp

import (
	"bytes"
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogSparseIntervalRequestedPairLifecycle(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, kinds []string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		must(e.SetPredicateRepresentation(d, kinds))
		return d
	}
	names := []string{"interval_relation", "tr_sparse"}
	p := declare("public-product-P", names)
	names[0] = "tr_hashtable"
	if e.predicates[p].requestedRepresentations[0] != "interval_relation" || e.predicates[p].explicitFiniteRepresentation {
		t.Fatal("product names aliased caller or used finite-only marker")
	}
	must(e.AddTableFact(p, []uint32{1, 7}))
	q := declare("public-product-late-Q", []string{"tr_sparse", "interval_relation"})
	must(e.SetPredicateRepresentation(q, []string{"interval_relation", "tr_sparse"}))
	if e.predicates[q].requestedRepresentations[0] != "interval_relation" || e.predicates[q].explicitFiniteRepresentation {
		t.Fatal("latest successful product request did not replace metadata")
	}
	if err := e.SetPredicateRepresentation(q, []string{"tr_hashtable"}); err == nil {
		t.Fatal("finite plugin accepted mixed signature")
	}
	if e.predicates[q].requestedRepresentations[0] != "interval_relation" || e.predicates[q].explicitFiniteRepresentation {
		t.Fatal("failed representation request changed valid product selection")
	}

	r := declare("public-product-empty-R", []string{"interval_relation", "tr_sparse"})
	must(e.RegisterPredicate(q))
	if len(e.predicates[q].requestedRepresentations) != 2 {
		t.Fatal("reregistration discarded requested kind")
	}
	b, err := m.MkVar(0, boolean)
	must(err)
	x, err := m.MkVar(1, integer)
	must(err)
	app := func(d, b, x ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, []ast.ASTHandle{b, x})
		must(err)
		return h
	}
	px, qx, rx := app(p, b, x), app(q, b, x), app(r, b, x)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.AddRule(rule, "copy-product"))
	prior := e.set
	var text bytes.Buffer
	must(e.WriteContextSMT2(&text, nil, nil))
	if e.set == prior || e.sparseIntervalDatalog.rules != e.set {
		t.Fatal("open-rule display did not rebind product source program")
	}
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("nonempty sieve product did not report source precision", status)
	}
	if e.sparseIntervalDatalog.store.entries[q].auxiliary {
		t.Fatal("late original predicate misclassified as auxiliary")
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	sevenRef, err := m.NewRef(seven)
	must(err)
	defer func() { must(m.DelRef(&sevenRef)) }()
	eight, err := m.ArithmeticIntegerNumeralText("8")
	must(err)
	eightRef, err := m.NewRef(eight)
	must(err)
	defer func() { must(m.DelRef(&eightRef)) }()
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(substitution.Close()) }()
	formula, err := m.RefGet(answer)
	must(err)
	for _, tc := range []struct {
		values []ast.ASTHandle
		want   bool
	}{{[]ast.ASTHandle{yes, seven}, true}, {[]ast.ASTHandle{no, seven}, false}, {[]ast.ASTHandle{yes, eight}, false}} {
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(formula, tc.values, ground))
		h, err := m.RefGet(ground)
		must(err)
		out, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, out))
		value, err := m.RefGet(out)
		must(err)
		if m.IsTrue(value) != tc.want || !m.IsTrue(value) && !m.IsFalse(value) {
			t.Fatal("public product answer disagrees with valuation", tc.want)
		}
		must(m.DelRef(&out))
		must(m.DelRef(&ground))
	}
	z, err := m.MkVar(0, integer)
	must(err)
	for _, tc := range []struct {
		name  string
		query ast.ASTHandle
		want  lbool.LBool
	}{{"nullary", app(q, yes, seven), lbool.True}, {"finite-empty", app(q, no, seven), lbool.False}, {"arithmetic-empty", app(q, yes, eight), lbool.False}, {"pure-finite", app(q, b, seven), lbool.True}, {"pure-arithmetic", app(q, yes, z), lbool.True}} {
		t.Run(tc.name, func(t *testing.T) {
			queryRef, err := m.NewRef(tc.query)
			must(err)
			defer func() { must(m.DelRef(&queryRef)) }()
			status, err := e.QueryExpression(tc.query)
			must(err)
			if status != tc.want {
				t.Fatal("product query", status, tc.want)
			}
			clone := e.queryClone
			if clone.sparseIntervalDatalog == nil || clone.sparseIntervalDatalog == e.sparseIntervalDatalog {
				t.Fatal("product query clone shares seed owner")
			}
			if !clone.sparseIntervalDatalog.store.entries[e.queryClonePredicate].auxiliary {
				t.Fatal("generated query head did not inherit product kind")
			}
			if clone.predicates[p].requestedRepresentations[0] != "interval_relation" {
				t.Fatal("clone lost requested name order")
			}
			held, err := e.ExpressionAnswer()
			must(err)
			defer func() { must(m.DelRef(&held)) }()
			must(e.clearQueryResources())
			if _, err = m.RefGet(held); err != nil {
				t.Fatal("answer lost with clone", err)
			}
		})
	}
	body, err := m.MkAnd(px, rx)
	must(err)
	replacement, err := m.MkImplies(body, qx)
	must(err)
	must(e.UpdateRule(replacement, "copy-product"))
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.False {
		t.Fatal("named product update ignored empty new tail", status)
	}
	must(e.AddTableFact(r, []uint32{1, 7}))
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("product update lost prior seeds", status)
	}
	m.Limit.Cancel()
	_, err = e.QueryPredicate(q)
	m.Limit.ResetCancel()
	if err == nil {
		t.Fatal("product ignored cancellation")
	}
	if _, err = e.Answer(q); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("canceled product exposed stale answer", err)
	}
}

func TestDatalogSparseIntervalRequiresUniformOriginalRequests(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	var first ast.ASTHandle
	for _, name := range []string{"requested-product", "unrequested-product"} {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		if first.IsNull() {
			first = d
			must(e.SetPredicateRepresentation(d, []string{"tr_sparse", "interval_relation"}))
		}
	}
	status, err := e.QueryPredicate(first)
	if !errors.Is(err, ErrSparseIntervalDatalogFragment) || status != lbool.Undef {
		t.Fatal("unrequested original inherited product backend", status, err)
	}
	if e.sparseIntervalDatalog != nil {
		t.Fatal("unsupported nonuniform program installed product owner")
	}
	if _, err = e.Answer(first); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("unsupported selection published an answer", err)
	}
}
