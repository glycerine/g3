package base_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestRuleAndRuleSetFiniteDomainClassification(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	defer func() { ruleMust(t, context.Close()) }()
	rm, err := context.RuleManager()
	ruleMust(t, err)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	bitvector, err := f.manager.BitVectorSort(8)
	ruleMust(t, err)
	integerZero, err := f.manager.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	integer, err := f.manager.GetSort(integerZero)
	ruleMust(t, err)
	makeRule := func(name string, sort ast.ASTHandle) *base.Rule {
		symbol, e := f.manager.Symbols.Intern(name)
		ruleMust(t, e)
		declaration, e := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, context.RegisterPredicate(declaration, true))
		variable, e := f.manager.MkVar(0, sort)
		ruleMust(t, e)
		head, e := f.manager.MkApp(declaration, []ast.ASTHandle{variable})
		ruleMust(t, e)
		rule, e := rm.NewRule(head, nil, nil, name)
		ruleMust(t, e)
		return rule
	}
	boolRule := makeRule("fd-bool", boolean)
	defer func() { ruleMust(t, boolRule.Close()) }()
	bvRule := makeRule("fd-bv", bitvector)
	defer func() { ruleMust(t, bvRule.Close()) }()
	intRule := makeRule("fd-int", integer)
	defer func() { ruleMust(t, intRule.Close()) }()
	for _, rule := range []*base.Rule{boolRule, bvRule} {
		finite, e := rule.IsFiniteDomain()
		ruleMust(t, e)
		if !finite {
			t.Fatal("finite rule rejected")
		}
	}
	if finite, e := intRule.IsFiniteDomain(); e != nil || finite {
		t.Fatalf("integer rule finite = %v,%v", finite, e)
	}
	set, err := base.NewRuleSet(rm)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	ruleMust(t, set.Add(boolRule))
	ruleMust(t, set.Add(bvRule))
	if finite, e := set.IsFiniteDomain(); e != nil || !finite {
		t.Fatalf("finite set = %v,%v", finite, e)
	}
	ruleMust(t, set.Add(intRule))
	if finite, e := set.IsFiniteDomain(); e != nil || finite {
		t.Fatalf("mixed set finite = %v,%v", finite, e)
	}
}
