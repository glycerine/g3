package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// A native full relation has an empty basis when no inequality was added.
// Propagation through projection and Cartesian join must not fabricate rays
// that would erase an existing head fact at union.
func TestDatalogKarrFullPropagationThroughUnaryAndBinary(t *testing.T) {
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
	decl := func(name string, n int) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(sym, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q, one, joined := decl("full-route-p", 2), decl("full-route-q", 1), decl("full-route-one", 1), decl("full-route-joined", 3)
	must(e.AddTableFact(q, []uint32{7}))
	must(e.AddTableFact(joined, []uint32{7, 8, 9}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		v, err := m.MkApp(d, args)
		must(err)
		return v
	}
	must(e.addRuleEager(app(p, x, y), ""))
	must(e.addRuleEager(app(one, z), ""))
	unary, err := m.MkImplies(app(p, x, y), app(q, y))
	must(err)
	must(e.addRuleEager(unary, ""))
	body, err := m.MkAnd(app(p, x, y), app(one, z))
	must(err)
	binary, err := m.MkImplies(body, app(joined, x, y, z))
	must(err)
	must(e.addRuleEager(binary, ""))
	for _, d := range []ast.ASTHandle{q, joined} {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("status", status)
		}
	}
	for _, d := range []ast.ASTHandle{q, joined} {
		rows, err := e.karrDatalog.results[d].Basis()
		must(err)
		if rows.Len() != 1 {
			t.Fatal("full source changed target fact", d, rows)
		}
	}
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	check := func(d ast.ASTHandle, values []string, want bool) {
		answer, err := e.Answer(d)
		must(err)
		defer func() { must(m.DelRef(&answer)) }()
		h, err := m.RefGet(answer)
		must(err)
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&ref)) }()
		must(subst.Substitute(h, args, ref))
		h, err = m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if (want && !m.IsTrue(h)) || (!want && !m.IsFalse(h)) {
			t.Fatal("answer", values, want, h)
		}
	}
	check(q, []string{"7"}, true)
	check(q, []string{"8"}, false)
	check(joined, []string{"7", "8", "9"}, true)
	check(joined, []string{"8", "8", "9"}, false)

}
