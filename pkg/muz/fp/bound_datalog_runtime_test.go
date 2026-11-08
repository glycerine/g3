package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestBoundDatalogRuntimeFactsStatusAndOwnership(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("bound-runtime-p")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(p))
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fact, err := m.MkApp(p, []ast.ASTHandle{one})
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(fact, "bound fact"))
	before, err := m.GetRefCount(p)
	fragmentMust(t, err)
	runtime, err := newBoundDatalogRuntime(m, engine.set, []ast.ASTHandle{p})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()
	after, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if after != before+1 {
		t.Fatal("bound runtime did not pin declaration", before, after)
	}
	status, err := runtime.query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("nonempty imprecise bound relation was not unknown", status)
	}
	result, err := runtime.relation(p)
	fragmentMust(t, err)
	if result.Empty() || !result.ContainsFact([]ast.ASTHandle{one}) {
		t.Fatal("bound fact was not published")
	}
	fragmentMust(t, result.AddLess(0, 0))
	if !result.Empty() {
		t.Fatal("caller clone mutation did not make a strict self-cycle")
	}
	again, err := runtime.relation(p)
	fragmentMust(t, err)
	if again.Empty() {
		t.Fatal("caller mutation changed runtime result")
	}
	formula, err := runtime.relationFormula(p)
	fragmentMust(t, err)
	fragmentMust(t, m.DelRef(&formula))
	fragmentMust(t, runtime.reset())
	// Rules are source facts and therefore repopulate after reset.
	status, err = runtime.query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("rule fact did not repopulate reset runtime", status)
	}
	fragmentMust(t, runtime.close())
	closed, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if closed != before {
		t.Fatal("bound runtime pin survived close", before, closed)
	}
}

func TestBoundDatalogRuntimeCancellationClearsPublishedResult(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("bound-runtime-cancel")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	runtime, err := newBoundDatalogRuntime(m, engine.set, []ast.ASTHandle{p})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()
	status, err := runtime.query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("empty pre-cancel result", status)
	}
	m.Limit.Cancel()
	status, err = runtime.query([]ast.ASTHandle{p})
	m.Limit.ResetCancel()
	if status != lbool.Undef || err == nil || err.Error() != messages.Canceled {
		t.Fatal("preexisting cancellation was not propagated", status, err)
	}
	if _, cause := runtime.relation(p); !errors.Is(cause, ErrBoundDatalogFragment) {
		t.Fatal("canceled saturation retained a published result", cause)
	}
}

func TestBoundDatalogRuntimeDirectFactEmptyAndPositiveRule(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	decl := make([]ast.ASTHandle, 2)
	for i, text := range []string{"bound-runtime-source", "bound-runtime-target"} {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		decl[i], cause = m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(decl[i]))
	}
	runtime, err := newBoundDatalogRuntime(m, engine.set, decl)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()
	status, err := runtime.query(decl)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("empty bound relations were not false", status)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, runtime.addFact(decl[0], []ast.ASTHandle{one}))
	status, err = runtime.query([]ast.ASTHandle{decl[0]})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("direct bound fact was not unknown", status)
	}
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	p, err := m.MkApp(decl[0], []ast.ASTHandle{x})
	fragmentMust(t, err)
	q, err := m.MkApp(decl[1], []ast.ASTHandle{x})
	fragmentMust(t, err)
	rule, err := m.MkImplies(p, q)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "positive bound copy"))
	if status, cause := runtime.query([]ast.ASTHandle{decl[1]}); cause != nil || status != lbool.Undef {
		t.Fatal("positive bound rule did not derive an imprecise result", status, cause)
	}
	derived, cause := runtime.relation(decl[1])
	fragmentMust(t, cause)
	if derived.Empty() {
		t.Fatal("positive bound rule result remained empty")
	}
}

func TestBoundDatalogPositiveOrdersJoinsRepeatedNumeralsAndPermutation(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(text string, arity int) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p, s := declare("bound-positive-p", 2), declare("bound-positive-s", 2)
	q, coarse, h, repeated := declare("bound-positive-q", 2), declare("bound-positive-coarse", 2), declare("bound-positive-h", 2), declare("bound-positive-r", 1)
	declarations := []ast.ASTHandle{p, s, q, coarse, h, repeated}
	app := func(declaration ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, arguments)
		fragmentMust(t, cause)
		return result
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(app(p, zero, zero), "p fact"))
	fragmentMust(t, engine.addRuleEager(app(s, zero, zero), "s fact"))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	z, err := m.MkVar(2, integer)
	fragmentMust(t, err)
	less, err := m.MkArithmeticLT(x, y)
	fragmentMust(t, err)
	orderedBody, err := m.MkAnd(app(p, x, y), less)
	fragmentMust(t, err)
	rule, err := m.MkImplies(orderedBody, app(q, y, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "order and head permutation"))
	reverseLess, err := m.MkArithmeticLT(y, x)
	fragmentMust(t, err)
	coarseBody, err := m.MkAndArgs([]ast.ASTHandle{app(p, x, y), less, reverseLess})
	fragmentMust(t, err)
	rule, err = m.MkImplies(coarseBody, app(coarse, x, y))
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "source conjunction is not applicable to bound filter"))
	joinBody, err := m.MkAnd(app(p, x, y), app(s, y, z))
	fragmentMust(t, err)
	rule, err = m.MkImplies(joinBody, app(h, x, z))
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "shared positive join"))
	rule, err = m.MkImplies(app(p, x, x), app(repeated, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "repeated tail variable"))
	rule, err = m.MkImplies(app(p, x, seven), app(repeated, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(rule, "coarse numeral tail"))
	runtime, err := newBoundDatalogRuntime(m, engine.set, declarations)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()
	status, err := runtime.query([]ast.ASTHandle{q, coarse, h, repeated})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("positive bound results were not imprecise unknown", status)
	}
	ordered, err := runtime.relation(q)
	fragmentMust(t, err)
	strict, _, err := ordered.Bounds(1)
	fragmentMust(t, err)
	if len(strict) != 1 || strict[0] != 0 {
		t.Fatal("interpreted order did not survive head permutation", strict)
	}
	coarseResult, err := runtime.relation(coarse)
	fragmentMust(t, err)
	strict, weak, err := coarseResult.Bounds(0)
	fragmentMust(t, err)
	if coarseResult.Empty() || len(strict) != 0 || len(weak) != 0 {
		t.Fatal("multiple interpreted tails were applied separately instead of source conjunction no-op", strict, weak)
	}
	for _, declaration := range []ast.ASTHandle{h, repeated} {
		result, cause := runtime.relation(declaration)
		fragmentMust(t, cause)
		if result.Empty() {
			t.Fatal("positive rule did not derive relation", declaration)
		}
	}
}
