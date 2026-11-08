package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestIntervalDatalogEvaluatorFactRulesAndOwnership(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	e, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("interval-fact-p")
	fragmentMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{real}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, e.RegisterPredicate(p))
	one, err := m.ArithmeticRealNumeralText("1")
	fragmentMust(t, err)
	three, err := m.ArithmeticRealNumeralText("3")
	fragmentMust(t, err)
	for _, value := range []ast.ASTHandle{one, three} {
		fact, cause := m.MkApp(p, []ast.ASTHandle{value})
		fragmentMust(t, cause)
		fragmentMust(t, e.addRuleEager(fact, "interval fact"))
	}
	before, err := m.GetRefCount(p)
	fragmentMust(t, err)
	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{p}, nil)
	fragmentMust(t, err)
	after, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if after != before+1 {
		t.Fatal("evaluator did not pin declaration", before, after)
	}
	fragmentMust(t, evaluator.SaturateFacts())
	result, err := evaluator.Relation(p)
	fragmentMust(t, err)
	for _, value := range []ast.ASTHandle{one, three} {
		found, cause := result.ContainsFactExpressions(m, []ast.ASTHandle{value})
		fragmentMust(t, cause)
		if !found {
			t.Fatal("fact missing from interval result")
		}
	}
	fragmentMust(t, result.FilterEqualExpression(m, 0, one))
	unchanged, err := evaluator.Relation(p)
	fragmentMust(t, err)
	found, err := unchanged.ContainsFactExpressions(m, []ast.ASTHandle{three})
	fragmentMust(t, err)
	if !found {
		t.Fatal("caller mutation changed evaluator state")
	}
	fragmentMust(t, evaluator.Close())
	closedCount, err := m.GetRefCount(p)
	fragmentMust(t, err)
	if closedCount != before {
		t.Fatal("evaluator declaration pin survived close", before, closedCount)
	}
	if _, err = evaluator.Relation(p); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("closed evaluator remained usable", err)
	}
}

func TestIntervalDatalogEvaluatorRejectsUnsupportedRulesAndSeeds(t *testing.T) {
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
	decl := make([]ast.ASTHandle, 2)
	for i, text := range []string{"interval-tail-p", "interval-tail-q"} {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		decl[i], cause = m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, e.RegisterPredicate(decl[i]))
	}
	x, err := m.MkVar(0, integer)
	fragmentMust(t, err)
	p, err := m.MkApp(decl[0], []ast.ASTHandle{x})
	fragmentMust(t, err)
	q, err := m.MkApp(decl[1], []ast.ASTHandle{x})
	fragmentMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	pOne, err := m.MkApp(decl[0], []ast.ASTHandle{one})
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(pOne, "fact before unsupported tail"))
	rule, err := m.MkImplies(p, q)
	fragmentMust(t, err)
	fragmentMust(t, e.addRuleEager(rule, "unsupported tail"))
	evaluator, err := NewIntervalDatalogEvaluator(m, e.set, decl, nil)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, evaluator.Close()) }()
	if err = evaluator.SaturateFacts(); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("non-fact rule accepted", err)
	}
	unchanged, err := evaluator.Relation(decl[0])
	fragmentMust(t, err)
	if !unchanged.Empty() {
		t.Fatal("failed saturation published preceding fact")
	}
	if _, err = NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{decl[0]}, map[ast.ASTHandle]*rel.IntervalRelation{decl[1]: rel.NewIntervalEmpty(1)}); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("undeclared seed accepted", err)
	}
	if _, err = NewIntervalDatalogEvaluator(m, e.set, []ast.ASTHandle{decl[0]}, map[ast.ASTHandle]*rel.IntervalRelation{decl[0]: rel.NewIntervalEmpty(2)}); !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("wrong-arity seed accepted", err)
	}
}
