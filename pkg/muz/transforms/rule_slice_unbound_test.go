package transforms_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

// AUDIT.md muz_transforms_02-F2: mk_slice::update_rule calls
// rm.fix_unbound_vars(new_rule, false), whose reduce_unbound_vars drops the
// constraint over the sliced-away column (b = a*a).
func TestRuleSliceReducesUnboundConstraints(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	number := func(n int64) ast.ASTHandle {
		value, makeErr := bigTestArithNumeral(f.manager, big.NewRat(n, 1), true)
		coneMust(t, makeErr)
		return value
	}
	integer, err := f.manager.GetSort(number(0))
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-unbound-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	q := declare("Q", []ast.ASTHandle{integer, integer})
	g := declare("G", nil)
	v := func(i uint32) ast.ASTHandle {
		variable, makeErr := f.manager.MkVar(i, integer)
		coneMust(t, makeErr)
		return variable
	}
	x, y, a, b := v(0), v(1), v(2), v(3)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		result, makeErr := f.manager.MkApp(d, args)
		coneMust(t, makeErr)
		return result
	}
	eq := func(l, r ast.ASTHandle) ast.ASTHandle {
		result, makeErr := f.manager.MkEq(l, r)
		coneMust(t, makeErr)
		return result
	}
	plus, err := f.manager.MkArithmeticAdd(x, number(1))
	coneMust(t, err)
	square, err := f.manager.MkArithmeticMul(a, a)
	coneMust(t, err)
	below, err := f.manager.MkArithmeticLT(x, number(10))
	coneMust(t, err)
	above, err := f.manager.MkArithmeticGT(x, number(20))
	coneMust(t, err)
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, makeErr := f.rules.NewRule(head, tails, nil, name)
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	add(app(q, number(0), number(0)), nil, "init")
	add(app(q, y, b), []ast.ASTHandle{app(q, x, a), eq(y, plus), eq(b, square), below}, "step")
	add(app(g), []ast.ASTHandle{app(q, x, a), above}, "goal")
	coneMust(t, f.set.SetOutputPredicate(g))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("slice fixture rejected")
	}
	cooperation, err := cooperate.NewCooperation(f.manager.Budget())
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, cooperation.Close()) })
	lite, err := qe.NewLite(f.manager, nil, false, qe.Config{Context: f.globals.Context, Globals: f.globals, Cooperation: cooperation})
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, lite.Close()) })
	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	plugin.SetUnboundFixer(func(rule *base.Rule) (*base.Rule, error) {
		fixed, _, fixErr := f.rules.FixUnboundVariables(rule, lite, false)
		return fixed, fixErr
	})
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("slice plugin did not rewrite the second column")
	}
	derived, found := plugin.DerivedPredicate(q)
	if !found {
		t.Fatal("Q was not sliced")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	checked := false
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		coneMust(t, getErr)
		uninterpreted, getErr := rule.UninterpretedTailSize()
		coneMust(t, getErr)
		total, getErr := rule.TailSize()
		coneMust(t, getErr)
		if head == derived && uninterpreted == 1 {
			checked = true
			if total-uninterpreted != 2 {
				t.Fatal("sliced recursive rule keeps", total-uninterpreted, "interpreted tails, want 2")
			}
			for i := uninterpreted; i < total; i++ {
				tail, _, getErr := rule.Tail(i)
				coneMust(t, getErr)
				found, getErr := containsMul(f.manager, tail)
				coneMust(t, getErr)
				if found {
					t.Fatal("sliced rule kept the constraint over the removed column")
				}
			}
		}
		coneMust(t, rule.Close())
	}
	if !checked {
		t.Fatal("sliced recursive rule not found")
	}
}

func containsMul(m *ast.ASTManager, e ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(e)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	decl, err := m.GetDecl(e)
	if err != nil {
		return false, err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return false, err
	}
	text, _, err := m.Symbols.BareString(name)
	if err != nil {
		return false, err
	}
	if text == "*" {
		return true, nil
	}
	n, err := m.GetNumArgs(e)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(e, i)
		if err != nil {
			return false, err
		}
		found, err := containsMul(m, arg)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}
