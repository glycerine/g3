package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrJoinGuardsPublicAndRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q, h := decl("karr-join-guard-p"), decl("karr-join-guard-q"), decl("karr-join-guard-h")
	must(e.AddTableFact(p, []uint32{1, 2}))
	must(e.AddTableFact(q, []uint32{2, 3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	num := func(s string) ast.ASTHandle { v, err := m.ArithmeticIntegerNumeralText(s); must(err); return v }
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		v, err := m.MkApp(d, args)
		must(err)
		return v
	}
	left, right := app(p, x, y), app(q, y, z)
	head := app(h, z, x)
	sum, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, z})
	must(err)
	c1, err := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{sum, num("4")})
	must(err)
	c2, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{z, num("3")})
	must(err)
	body, err := m.MkAnd(left, right)
	must(err)
	body, err = m.MkAnd(body, c1)
	must(err)
	body, err = m.MkAnd(body, c2)
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(rule, ""))
	status, err := e.QueryPredicate(h)
	must(err)
	if status != lbool.Undef {
		t.Fatal("query", status)
	}
	r, err := e.set.Rule(0)
	must(err)
	_, plan, err := e.karrDatalog.compileBinaryRule(r)
	must(err)
	must(r.Close())
	if len(plan.guards) != 2 || len(plan.join.expressions) != 2 {
		t.Fatal("join guard plan", plan)
	}
	// The last interpreted tail is flattened first; inspect its z <= 3 row.
	if plan.guards[0].constant.Cmp(big.NewRat(3, 1)) != 0 || plan.guards[1].constant.Cmp(big.NewRat(-4, 1)) != 0 {
		t.Fatal("guard order", plan.guards)
	}
	answer, err := e.Answer(h)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	expr, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	for _, tc := range []struct {
		vals []string
		want bool
	}{{[]string{"3", "1"}, true}, {[]string{"1", "3"}, false}} {
		args := []ast.ASTHandle{num(tc.vals[0]), num(tc.vals[1])}
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(expr, args, ref))
		v, err := m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(v, nil, reduced))
		v, err = m.RefGet(reduced)
		must(err)
		if (tc.want && !m.IsTrue(v)) || (!tc.want && !m.IsFalse(v)) {
			t.Fatal("answer", tc, v)
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&reduced))
	}
	armed, fired := true, false
	checkpoint := func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	}
	sp := rel.NewKarrEmpty(2, checkpoint)
	must(sp.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}))
	sq := rel.NewKarrEmpty(2, checkpoint)
	must(sq.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = sp
	e.karrDatalog.seeds[q] = sq
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || e.karrDatalog.results != nil {
		t.Fatal("cancel", fired, status, err)
	}
	armed = false
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	over := rel.NewKarrEmpty(2, nil)
	must(over.AddFact([]*big.Rat{large, big.NewRat(2, 1)}))
	e.karrDatalog.seeds[p] = over
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
		t.Fatal("overflow", status, err)
	}
	e.karrDatalog.seeds[p] = sp
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	must(err)
	if status != lbool.Undef {
		t.Fatal("overflow retry", status)
	}
	// y is needed by the guard but absent from the head, requiring a different
	// source join-project plan; this bounded route must reject it.
	guardOnly, err := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{y, num("0")})
	must(err)
	badBody, err := m.MkAnd(left, right)
	must(err)
	badBody, err = m.MkAnd(badBody, guardOnly)
	must(err)
	bad, err := m.MkImplies(badBody, head)
	must(err)
	prior, err := e.set.Len()
	must(err)
	must(e.AddRule(bad, ""))
	must(e.flushPendingRules())
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) || e.karrDatalog.results != nil {
		t.Fatal("guard-only variable", status, err)
	}
	badRule, err := e.set.Rule(prior)
	must(err)
	must(e.set.Remove(badRule))
	must(badRule.Close())
	status, err = e.karrDatalog.query([]ast.ASTHandle{h})
	must(err)
	if status != lbool.Undef {
		t.Fatal("guard-only retry", status)
	}
}
