package fp

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrBinaryRulePublic(t *testing.T) {
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
	must(e.AddRule(rule, ""))
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

func TestDatalogKarrBinaryCancellationRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
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
	p, q, r := decl("karr-join-cancel-p", 2), decl("karr-join-cancel-q", 1), decl("karr-join-cancel-r", 1)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	left, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	right, err := m.MkApp(q, []ast.ASTHandle{y})
	must(err)
	body, err := m.MkAnd(left, right)
	must(err)
	head, err := m.MkApp(r, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(rule, ""))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q, r})
	must(err)
	defer func() { must(a.close()) }()
	armed, fired := true, false
	checkpoint := func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	}
	seed := rel.NewKarrEmpty(2, checkpoint)
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}))
	a.seeds[p] = seed
	other := rel.NewKarrEmpty(1, checkpoint)
	must(other.AddFact([]*big.Rat{big.NewRat(2, 1)}))
	a.seeds[q] = other
	status, err := a.query([]ast.ASTHandle{r})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || a.results != nil {
		t.Fatal("join cancellation", fired, status, err)
	}
	armed = false
	status, err = a.query([]ast.ASTHandle{r})
	must(err)
	if status != lbool.Undef || a.results[r].Empty() {
		t.Fatal("join retry", status)
	}
}

func TestDatalogKarrBinaryMultipleLocalDuplicateGroupsRejectedAndRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
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
	p, q, r := decl("karr-join-groups-p", 2), decl("karr-join-groups-q", 2), decl("karr-join-groups-r", 4)
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	left, err := m.MkApp(p, []ast.ASTHandle{x, x})
	must(err)
	right, err := m.MkApp(q, []ast.ASTHandle{y, y})
	must(err)
	body, err := m.MkAnd(left, right)
	must(err)
	head, err := m.MkApp(r, []ast.ASTHandle{x, x, y, y})
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(formula, ""))
	must(e.flushPendingRules())
	a, err := newKarrDatalogActivation(m, e.set, []ast.ASTHandle{p, q, r})
	must(err)
	defer func() { must(a.close()) }()
	a.seeds[p] = rel.NewKarrEmpty(2, nil)
	must(a.seeds[p].AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}))
	a.seeds[q] = rel.NewKarrEmpty(2, nil)
	must(a.seeds[q].AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1)}))
	for i := 0; i < 2; i++ {
		status, err := a.query([]ast.ASTHandle{r})
		if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) || a.results != nil {
			t.Fatal("multiple local groups", status, err)
		}
	}
	// Removing the unsupported rule makes the same activation usable again.
	rule, err := e.set.Rule(0)
	must(err)
	must(e.set.Remove(rule))
	must(rule.Close())
	status, err := a.query([]ast.ASTHandle{p})
	must(err)
	if status != lbool.Undef || a.results[p].Empty() || !a.results[r].Empty() {
		t.Fatal("retry after removing rule", status)
	}
}
