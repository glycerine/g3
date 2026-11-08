package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalDatalogPositiveUnaryRepeatedBoundsAndFixpoint(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	decl := make(map[string]ast.ASTHandle)
	for name, arity := range map[string]int{"interval-positive-p": 2, "interval-positive-q": 1, "interval-positive-r": 1} {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		decl[name], cause = m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(decl[name]))
	}
	number := func(text string) ast.ASTHandle {
		h, cause := m.ArithmeticIntegerNumeralText(text)
		fragmentMust(t, cause)
		return h
	}
	app := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, cause := m.MkApp(declaration, args)
		fragmentMust(t, cause)
		return h
	}
	for _, tuple := range [][2]string{{"0", "5"}, {"10", "5"}} {
		fragmentMust(t, e.addRuleEager(app(decl["interval-positive-p"], number(tuple[0]), number(tuple[1])), "p fact"))
	}
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	qx, rx := app(decl["interval-positive-q"], x), app(decl["interval-positive-r"], x)
	// Put R <- Q first so reaching R requires another scheduling round.
	implication, err := m.MkImplies(qx, rx)
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(implication, "r from q"))
	pxx := app(decl["interval-positive-p"], x, x)
	lower, err := m.MkArithmeticGE(x, number("2"))
	fragmentMust(t, err)
	upper, err := m.MkArithmeticLE(x, number("8"))
	fragmentMust(t, err)
	body, err := m.MkAndArgs([]ast.ASTHandle{pxx, lower, upper})
	fragmentMust(t, err)
	implication, err = m.MkImplies(body, qx)
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(implication, "bounded repeated tail"))

	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{decl["interval-positive-p"], decl["interval-positive-q"], decl["interval-positive-r"]}, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, evaluator.SaturatePositive(false))
	for _, name := range []string{"interval-positive-q", "interval-positive-r"} {
		relation, cause := evaluator.Relation(decl[name])
		fragmentMust(t, cause)
		found, cause := relation.ContainsFactExpressions(m, []ast.ASTHandle{number("5")})
		fragmentMust(t, cause)
		if !found {
			t.Fatal("derived fixedpoint value missing", name)
		}
		outside, cause := relation.ContainsFactExpressions(m, []ast.ASTHandle{number("4")})
		fragmentMust(t, cause)
		if outside {
			t.Fatal("repeated-variable intersection or bounds were ignored", name)
		}
	}
}

func TestIntervalDatalogPositiveUnaryConstantAndHeadPermutation(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	mkDecl := func(name string, arity int) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		domain := make([]ast.ASTHandle, arity)
		for i := range domain {
			domain[i] = integer
		}
		d, cause := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	p, q := mkDecl("interval-permute-p", 3), mkDecl("interval-permute-q", 2)
	n := func(text string) ast.ASTHandle {
		h, cause := m.ArithmeticIntegerNumeralText(text)
		fragmentMust(t, cause)
		return h
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, cause := m.MkApp(d, args)
		fragmentMust(t, cause)
		return h
	}
	fragmentMust(t, e.addRuleEager(app(p, n("1"), n("7"), n("3")), "fact"))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	tail := app(p, x, n("7"), y)
	head := app(q, y, x)
	rule, err := m.MkImplies(tail, head)
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(rule, "constant and permutation"))
	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{p, q}, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, evaluator.SaturatePositive(false))
	result, err := evaluator.Relation(q)
	fragmentMust(t, err)
	found, err := result.ContainsFactExpressions(m, []ast.ASTHandle{n("3"), n("1")})
	fragmentMust(t, err)
	if !found {
		t.Fatal("constant-filtered permuted head fact missing")
	}
}

func TestIntervalDatalogPositiveSharedTailJoin(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	mkDecl := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		d, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	p, s, h := mkDecl("interval-join-p"), mkDecl("interval-join-s"), mkDecl("interval-join-h")
	n := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		fragmentMust(t, cause)
		return value
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		value, cause := m.MkApp(d, args)
		fragmentMust(t, cause)
		return value
	}
	fragmentMust(t, e.addRuleEager(app(p, n("1"), n("8")), "p"))
	fragmentMust(t, e.addRuleEager(app(s, n("7"), n("3")), "s"))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	y, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	z, err := m.MkVar(2, integer)
	fragmentMust(t, err)
	body, err := m.MkAnd(app(p, x, y), app(s, y, z))
	fragmentMust(t, err)
	rule, err := m.MkImplies(body, app(h, z, x))
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(rule, "disjoint shared join"))
	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{p, s, h}, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, evaluator.SaturatePositive(false))
	result, err := evaluator.Relation(h)
	fragmentMust(t, err)
	// The direct join is empty, then the C++ head projection loses m_empty
	// while constructing a full interval destination.
	if result.Empty() {
		t.Fatal("disjoint join projection propagated the C++-dropped empty flag")
	}
	found, err := result.ContainsFactExpressions(m, []ast.ASTHandle{n("3"), n("1")})
	fragmentMust(t, err)
	if !found {
		t.Fatal("projection did not preserve the copied endpoint payload")
	}
}

func TestIntervalDatalogWideningOnlyAppliesToRecursivePredicates(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	mkDecl := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		d, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	p, q, h := mkDecl("interval-widen-p"), mkDecl("interval-widen-q"), mkDecl("interval-widen-h")
	n := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		fragmentMust(t, cause)
		return value
	}
	app := func(d ast.ASTHandle, value ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(d, []ast.ASTHandle{value})
		fragmentMust(t, cause)
		return result
	}
	fragmentMust(t, e.addRuleEager(app(p, n("0")), "p"))
	fragmentMust(t, e.addRuleEager(app(q, n("10")), "q"))
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	for _, source := range []ast.ASTHandle{p, q} {
		rule, cause := m.MkImplies(app(source, x), app(h, x))
		fragmentMust(t, cause)
		fragmentMust(t, e.addRuleEager(rule, "nonrecursive union"))
	}
	self, err := m.MkImplies(app(h, x), app(h, x))
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(self, "recursive identity after external seeds"))
	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{p, q, h}, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	fragmentMust(t, evaluator.SaturatePositive(true))
	result, err := evaluator.Relation(h)
	fragmentMust(t, err)
	outside, err := result.ContainsFactExpressions(m, []ast.ASTHandle{n("100")})
	fragmentMust(t, err)
	if outside {
		t.Fatal("external SCC seeds were widened before ordinary initialization")
	}
}
