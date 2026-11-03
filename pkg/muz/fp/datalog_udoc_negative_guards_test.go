package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocNegativeFullColumnShapes(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	domain, err := m.BitVectorSort(1)
	must(err)
	declare := func(label string, arity int) ast.ASTHandle {
		t.Helper()
		name, cause := m.Symbols.Intern(label)
		must(cause)
		sorts := make([]ast.ASTHandle, arity)
		for index := range sorts {
			sorts[index] = domain
		}
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	seed := declare("doc-negative-full-seed", 1)
	absent := declare("doc-negative-full-absent", 1)
	blocker := declare("doc-negative-full-blocker", 2)
	constantAllowed := declare("doc-negative-full-constant-allowed", 1)
	constantBlocked := declare("doc-negative-full-constant-blocked", 1)
	constantUnbound := declare("doc-negative-full-constant-unbound", 1)
	unbound := declare("doc-negative-full-unbound", 1)
	negativeOnly := declare("doc-negative-full-only", 1)
	repeated := declare("doc-negative-full-repeated", 1)
	repeatedProjected := declare("doc-negative-full-repeated-projected", 0)
	shared := declare("doc-negative-full-shared", 1)
	interpreted := declare("doc-negative-full-interpreted", 1)
	missing := declare("doc-negative-full-missing", 0)
	stop := declare("doc-negative-full-stop", 0)
	nullaryAllowed := declare("doc-negative-full-nullary-allowed", 0)
	nullaryBlocked := declare("doc-negative-full-nullary-blocked", 0)
	must(e.AddTableFact(seed, []uint32{1}))
	must(e.AddTableFact(blocker, []uint32{1, 1}))
	must(e.AddTableFact(stop, nil))
	pin := func(term ast.ASTHandle, cause error) ast.ASTHandle {
		t.Helper()
		must(cause)
		root, cause := m.NewRef(term)
		must(cause)
		t.Cleanup(func() { must(m.DelRef(&root)) })
		return term
	}
	x := pin(m.MkVar(0, domain))
	y := pin(m.MkVar(1, domain))
	zero := pin(m.BitVectorNumeralUnsigned(0, 1))
	one := pin(m.BitVectorNumeralUnsigned(1, 1))
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		term, cause := m.MkApp(declaration, arguments)
		must(cause)
		return term
	}
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, flags []bool, name string) {
		t.Helper()
		rule, cause := e.rules.NewRule(head, tails, flags, name)
		must(cause)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	add(app(constantAllowed, x), []ast.ASTHandle{app(seed, x), app(blocker, zero, x)}, []bool{false, true}, "negative-constant-allowed")
	add(app(constantBlocked, x), []ast.ASTHandle{app(seed, x), app(blocker, one, x)}, []bool{false, true}, "negative-constant-blocked")
	add(app(constantUnbound, y), []ast.ASTHandle{app(blocker, one, y)}, []bool{true}, "negative-constant-unbound")
	add(app(unbound, y), []ast.ASTHandle{app(seed, x), app(blocker, x, y)}, []bool{false, true}, "negative-unbound")
	add(app(negativeOnly, x), []ast.ASTHandle{app(absent, x)}, []bool{true}, "negative-only")
	add(app(repeated, y), []ast.ASTHandle{app(blocker, y, y)}, []bool{true}, "negative-repeated")
	add(app(repeatedProjected), []ast.ASTHandle{app(blocker, y, y)}, []bool{true}, "negative-repeated-projected")
	add(app(shared, y), []ast.ASTHandle{app(absent, y), app(blocker, y, y)}, []bool{true, true}, "negative-shared-unbound")
	yIsOne := pin(m.MkEq(y, one))
	add(app(interpreted, y), []ast.ASTHandle{app(absent, y), yIsOne}, []bool{true, false}, "negative-interpreted")
	add(app(nullaryAllowed), []ast.ASTHandle{app(missing)}, []bool{true}, "negative-nullary-allowed")
	add(app(nullaryBlocked), []ast.ASTHandle{app(stop)}, []bool{true}, "negative-nullary-blocked")
	checkDatalogUDocBV1Tuples(t, m, e, constantAllowed, 1, [][]uint64{{1}})
	checkDatalogUDocBV1Tuples(t, m, e, constantBlocked, 1, nil)
	checkDatalogUDocBV1Tuples(t, m, e, constantUnbound, 1, [][]uint64{{0}})
	checkDatalogUDocBV1Tuples(t, m, e, unbound, 1, [][]uint64{{0}})
	checkDatalogUDocBV1Tuples(t, m, e, negativeOnly, 1, [][]uint64{{0}, {1}})
	checkDatalogUDocBV1Tuples(t, m, e, repeated, 1, [][]uint64{{0}})
	checkDatalogUDocBV1Tuples(t, m, e, repeatedProjected, 0, [][]uint64{{}})
	checkDatalogUDocBV1Tuples(t, m, e, shared, 1, [][]uint64{{0}})
	checkDatalogUDocBV1Tuples(t, m, e, interpreted, 1, [][]uint64{{1}})
	checkDatalogUDocBV1Tuples(t, m, e, nullaryAllowed, 0, [][]uint64{{}})
	checkDatalogUDocBV1Tuples(t, m, e, nullaryBlocked, 0, nil)
}

func TestDatalogUDocFullColumnPreflightDoesNotPublishGroundPrefix(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	domain, err := m.BitVectorSort(1)
	must(err)
	declare := func(label string, sorts []ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(label)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	prefix := declare("doc-full-preflight-prefix", []ast.ASTHandle{domain})
	output := declare("doc-full-preflight-output", nil)
	one, err := m.BitVectorNumeralUnsigned(1, 1)
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	y, err := m.MkVar(7, integer)
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	guard, err := m.MkArithmeticLT(y, zero)
	must(err)
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		term, cause := m.MkApp(declaration, arguments)
		must(cause)
		return term
	}
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, cause := e.rules.NewRule(head, tails, nil, name)
		must(cause)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	add(app(prefix, one), nil, "full-preflight-ground-prefix")
	add(app(output), []ast.ASTHandle{guard}, "full-preflight-infinite-column")
	for iteration := 0; iteration < 2; iteration++ {
		status, cause := e.QueryPredicate(output)
		if status != lbool.Undef || !errors.Is(cause, ErrUDocDatalogFragment) {
			t.Fatal("unsupported full-column sort query", status, cause)
		}
		if e.udocDatalog == nil || e.udocDatalog.result[prefix] == nil {
			t.Fatal("query did not reach concrete UDoc preflight")
		}
		if empty, getErr := e.udocDatalog.result[prefix].Empty(); getErr != nil || !empty {
			t.Fatal("failed full-column preflight published earlier ground prefix", empty, getErr)
		}
		if empty, getErr := e.udocDatalog.seeds[prefix].Empty(); getErr != nil || !empty {
			t.Fatal("failed full-column preflight changed seeds", empty, getErr)
		}
		if finalized, getErr := e.set.IsFinalized(); getErr != nil || finalized {
			t.Fatal("failed query did not restore open rule set", finalized, getErr)
		}
		if _, getErr := e.Answer(prefix); getErr == nil {
			t.Fatal("failed full-column preflight exposed a prefix answer")
		}
	}
}
