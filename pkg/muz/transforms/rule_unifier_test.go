package transforms_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleUnifierModelProofSubstitutionsAndReset(t *testing.T) {
	f := newConeFixture(t)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	one, err := f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	declare := func(name string) ast.ASTHandle {
		sym, e := f.manager.Symbols.Intern(name)
		coneMust(t, e)
		d, e := f.manager.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, e)
		coneMust(t, f.rules.RegisterPredicate(d))
		return d
	}
	p, q := declare("unifier-p"), declare("unifier-q")
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	app := func(d, arg ast.ASTHandle) ast.ASTHandle {
		value, e := f.manager.MkApp(d, []ast.ASTHandle{arg})
		coneMust(t, e)
		return value
	}
	target, err := f.rules.NewRule(app(q, x), []ast.ASTHandle{app(p, x)}, nil, "target")
	coneMust(t, err)
	defer func() { coneMust(t, target.Close()) }()
	first, err := f.rules.NewRule(app(p, zero), nil, nil, "first")
	coneMust(t, err)
	defer func() { coneMust(t, first.Close()) }()
	second, err := f.rules.NewRule(app(p, one), nil, nil, "second")
	coneMust(t, err)
	defer func() { coneMust(t, second.Close()) }()
	u, err := transforms.NewRuleUnifier(f.manager)
	coneMust(t, err)
	defer func() { coneMust(t, u.Close()) }()
	for _, tc := range []struct {
		source *base.Rule
		want   ast.ASTHandle
	}{{first, zero}, {second, one}} {
		ok, err := u.UnifyRules(target, 0, tc.source)
		coneMust(t, err)
		if !ok {
			t.Fatal("unification failed")
		}
		refs, err := u.RuleSubstitution(target, true)
		coneMust(t, err)
		if len(refs) != 1 {
			t.Fatal("target substitution length", len(refs))
		}
		got, err := f.manager.RefGet(refs[0])
		coneMust(t, err)
		if got != tc.want {
			t.Fatal("target substitution", got, tc.want)
		}
		coneMust(t, f.manager.DelRef(&refs[0]))
		refs, err = u.RuleSubstitution(tc.source, false)
		coneMust(t, err)
		if len(refs) != 0 {
			t.Fatal("ground source has substitutions", len(refs))
		}
		resolved, err := u.ResolveRaw(f.rules, target, 0, tc.source)
		coneMust(t, err)
		head, err := resolved.Head()
		coneMust(t, err)
		argument, err := f.manager.GetArg(head, 0)
		coneMust(t, err)
		tails, err := resolved.TailSize()
		coneMust(t, err)
		if argument != tc.want || tails != 0 {
			t.Fatal("resolved rule", argument, tails, tc.want)
		}
		coneMust(t, resolved.Close())
	}
	variableSource, err := f.rules.NewRule(app(p, x), nil, nil, "variable")
	coneMust(t, err)
	defer func() { coneMust(t, variableSource.Close()) }()
	ok, err := u.UnifyRules(target, 0, variableSource)
	coneMust(t, err)
	if !ok {
		t.Fatal("variable unification failed")
	}
	left, err := u.RuleSubstitution(target, true)
	coneMust(t, err)
	right, err := u.RuleSubstitution(variableSource, false)
	coneMust(t, err)
	l, err := f.manager.RefGet(left[0])
	coneMust(t, err)
	r, err := f.manager.RefGet(right[0])
	coneMust(t, err)
	if l != r {
		t.Fatal("variable banks disagree", l, r)
	}
	index, err := f.manager.GetVarIndex(l)
	coneMust(t, err)
	resolved, renaming, err := u.ResolveRawWithRenaming(f.rules, target, 0, variableSource)
	coneMust(t, err)
	if int(index) >= len(renaming) || renaming[index] == (ast.ASTRef{}) {
		t.Fatal("missing variable renaming", index, renaming)
	}
	normalized, err := f.manager.RefGet(renaming[index])
	coneMust(t, err)
	if normalized != x {
		t.Fatal("unifier renaming does not match normalized result", normalized, x)
	}
	for i := range renaming {
		if renaming[i] != (ast.ASTRef{}) {
			coneMust(t, f.manager.DelRef(&renaming[i]))
		}
	}
	coneMust(t, resolved.Close())
	coneMust(t, f.manager.DelRef(&left[0]))
	coneMust(t, f.manager.DelRef(&right[0]))
	ok, err = u.UnifyRules(target, 0, target)
	coneMust(t, err)
	if ok {
		t.Fatal("different predicate heads unified")
	}
	if _, err = u.RuleSubstitution(target, true); err == nil {
		t.Fatal("failed unification left a usable substitution")
	}
}
