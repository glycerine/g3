package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrBinaryRulePublic(t *testing.T) {
	skipUnreachableInfiniteSortRules(t)
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
	p, q, r := decl("karr-binary-p", 2), decl("karr-binary-q", 2), decl("karr-binary-r", 3)
	must(e.AddTableFact(p, []uint32{1, 2}))
	must(e.AddTableFact(q, []uint32{2, 3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	four, err := m.ArithmeticIntegerNumeralText("4")
	must(err)
	left, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	right, err := m.MkApp(q, []ast.ASTHandle{y, z})
	must(err)
	body, err := m.MkAnd(left, right)
	must(err)
	head, err := m.MkApp(r, []ast.ASTHandle{z, four, x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, ""))
	status, err := e.QueryPredicate(r)
	must(err)
	if status != lbool.Undef {
		t.Fatal("join status", status)
	}
	answer, err := e.Answer(r)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	expr, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	check := func(values []string, want bool) {
		args := make([]ast.ASTHandle, len(values))
		for i, v := range values {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		grounded, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&grounded)) }()
		must(subst.Substitute(expr, args, grounded))
		h, err := m.RefGet(grounded)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		defer func() { must(m.DelRef(&reduced)) }()
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if (want && !m.IsTrue(h)) || (!want && !m.IsFalse(h)) {
			t.Fatal("join answer", values, want, h)
		}
	}
	check([]string{"3", "4", "1"}, true)
	check([]string{"3", "4", "2"}, false)
	check([]string{"3", "5", "1"}, false)
}
