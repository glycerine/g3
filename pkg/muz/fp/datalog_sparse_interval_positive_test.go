package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalDatalogDependencyFirstRecursiveFixpoint(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p, q, r := declare("sparse-interval-p"), declare("sparse-interval-q"), declare("sparse-interval-r")
	app := func(declaration ast.ASTHandle, values ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, values)
		fragmentMust(t, cause)
		return result
	}
	b, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	x, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	// Reverse insertion order ensures the evaluator must honor dependency SCCs.
	step, err := m.MkImplies(app(q, b, x), app(r, b, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(step, "r from q"))
	step, err = m.MkImplies(app(p, b, x), app(q, b, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(step, "q from p"))
	step, err = m.MkImplies(app(q, b, x), app(q, b, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(step, "recursive q identity"))

	store, err := NewSparseIntervalDatalogStore(m)
	fragmentMust(t, err)
	declarations := []ast.ASTHandle{p, q, r}
	for _, declaration := range declarations {
		fragmentMust(t, store.Register(declaration))
	}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{truth, zero}))
	evaluator, err := store.Evaluator(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, evaluator.SaturatePositive(true))
	for _, declaration := range []ast.ASTHandle{q, r} {
		relation, cause := evaluator.Relation(declaration)
		fragmentMust(t, cause)
		found, cause := relation.ContainsFact([]ast.ASTHandle{truth, zero})
		fragmentMust(t, cause)
		if !found {
			t.Fatal("dependency-first derived product fact missing", declaration)
		}
		fragmentMust(t, relation.Close())
	}
	if status, cause := evaluator.Query([]ast.ASTHandle{r}); cause != nil || status != lbool.Undef {
		t.Fatal("nonempty product query status", status, cause)
	}
	formula, err := evaluator.RelationFormula(r)
	fragmentMust(t, err)
	fragmentMust(t, m.DelRef(&formula))
	fragmentMust(t, evaluator.Close())
	fragmentMust(t, store.Close())
}

func TestSparseIntervalDatalogActivationCloneOwnsIndependentSeeds(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("sparse-interval-activation-clone")
	fragmentMust(t, err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(predicate))
	extraName, err := m.Symbols.Intern("sparse-interval-activation-sync")
	fragmentMust(t, err)
	extra, err := m.MkFuncDecl(extraName, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(extra))
	activation, selected, err := newSparseIntervalDatalogActivation(m, engine.set, engine.guard, []ast.ASTHandle{predicate})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("mixed sparse/interval activation not selected")
	}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, activation.addFact(predicate, []ast.ASTHandle{truth, one}))
	copyActivation, err := activation.clone(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, activation.close())
	fragmentMust(t, copyActivation.syncPredicates([]ast.ASTHandle{predicate, extra}))
	fragmentMust(t, copyActivation.addFact(extra, []ast.ASTHandle{truth, one}))
	status, err := copyActivation.query([]ast.ASTHandle{predicate}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("clone lost nonempty product seed", status)
	}
	relation, err := copyActivation.relation(predicate)
	fragmentMust(t, err)
	found, err := relation.ContainsFact([]ast.ASTHandle{truth, one})
	fragmentMust(t, err)
	if !found {
		t.Fatal("clone seed unavailable after source context close")
	}
	fragmentMust(t, relation.Close())
	status, err = copyActivation.query([]ast.ASTHandle{extra}, true)
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("synced predicate lost seed", status)
	}
	fragmentMust(t, copyActivation.reset())
	status, err = copyActivation.query([]ast.ASTHandle{predicate}, true)
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("reset clone relation not empty", status)
	}
	fragmentMust(t, copyActivation.close())
}

func TestSparseIntervalDatalogMixedGuardsUseSingleProductFilter(t *testing.T) {
	f := newDirectFactFixture(t)
	m, engine := f.manager, f.engine
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	declare := func(name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("sparse-interval-guard-p"), declare("sparse-interval-guard-q")
	b, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	x, err := m.MkVar(1, integer)
	fragmentMust(t, err)
	app := func(declaration ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, args)
		fragmentMust(t, cause)
		return result
	}
	truth, err := m.MkTrue()
	fragmentMust(t, err)
	finiteGuard, err := m.MkEq(b, truth)
	fragmentMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	fragmentMust(t, err)
	intervalGuard, err := m.MkArithmeticGT(x, three)
	fragmentMust(t, err)
	body, err := m.MkAndArgs([]ast.ASTHandle{app(p, b, x), finiteGuard, intervalGuard})
	fragmentMust(t, err)
	rule, err := m.MkImplies(body, app(q, b, x))
	fragmentMust(t, err)
	fragmentMust(t, engine.AddRule(rule, "mixed component guards"))
	store, err := NewSparseIntervalDatalogStore(m)
	fragmentMust(t, err)
	fragmentMust(t, store.Register(p))
	fragmentMust(t, store.Register(q))
	falsehood, err := m.MkFalse()
	fragmentMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	fragmentMust(t, err)
	ten, err := m.ArithmeticIntegerNumeralText("10")
	fragmentMust(t, err)
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{truth, zero}))
	fragmentMust(t, store.AddFact(p, []ast.ASTHandle{falsehood, ten}))
	evaluator, err := store.Evaluator(engine.set, engine.guard)
	fragmentMust(t, err)
	fragmentMust(t, evaluator.SaturatePositive(false))
	result, err := evaluator.Relation(q)
	fragmentMust(t, err)
	// Source passes the conjunction to each independent sieve. Because each
	// component sees a variable owned by the other, both conservatively keep
	// their inputs, including this abstract cross tuple.
	found, err := result.ContainsFact([]ast.ASTHandle{falsehood, zero})
	fragmentMust(t, err)
	if !found {
		t.Fatal("mixed guards were applied separately and over-refined the product")
	}
	fragmentMust(t, result.Close())
	fragmentMust(t, evaluator.Close())
	fragmentMust(t, store.Close())
}
