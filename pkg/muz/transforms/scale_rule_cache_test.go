package transforms_test

// Source: dl_mk_scale.cpp resets m_cache per rule and supplies false tail flags.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"testing"
)

func TestScaleResetsCacheBetweenRulesAndClearsNegation(t *testing.T) {
	ctx, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, ctx.Close()) })
	m, err := ctx.Manager()
	coneMust(t, err)
	rm, err := base.NewRuleManager(m)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rm.Close()) })
	source, err := base.NewRuleSet(rm)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, source.Close()) })
	makeExpr := func(h ast.ASTHandle, err error) ast.ASTHandle { coneMust(t, err); return h }
	two := makeExpr(m.ArithmeticRealNumeralText("2"))
	realSort := makeExpr(m.GetSort(two))
	boolSort := makeExpr(m.MkBoolSort())
	x := makeExpr(m.MkVar(0, realSort))
	y := makeExpr(m.MkVar(1, realSort))
	pname, err := m.Symbols.Intern("scale-cache-head")
	coneMust(t, err)
	qname, err := m.Symbols.Intern("scale-cache-tail")
	coneMust(t, err)
	p := makeExpr(m.MkFuncDecl(pname, []ast.ASTHandle{realSort, realSort}, boolSort, ast.ASTFuncDeclInfo{}))
	q := makeExpr(m.MkFuncDecl(qname, []ast.ASTHandle{realSort}, boolSort, ast.ASTFuncDeclInfo{}))
	coneMust(t, rm.RegisterPredicate(p))
	coneMust(t, rm.RegisterPredicate(q))
	tail := makeExpr(m.MkApp(q, []ast.ASTHandle{x}))
	constraint := makeExpr(m.MkArithmeticLE(x, two))
	names := []string{"first", "second"}
	for i, second := range []ast.ASTHandle{x, y} {
		head := makeExpr(m.MkApp(p, []ast.ASTHandle{x, second}))
		rule, makeErr := rm.NewRule(head, []ast.ASTHandle{tail, constraint}, []bool{i == 0, false}, names[i])
		coneMust(t, makeErr)
		coneMust(t, source.Add(rule))
		coneMust(t, rule.Close())
	}
	plugin := transforms.NewScalePlugin(m, rm, 36030)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	result, err := plugin.Transform(source)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, result.Close()) })
	scaled := plugin.FreshPredicates()
	for i, second := range []ast.ASTHandle{x, y} {
		rule, getErr := result.Rule(i)
		coneMust(t, getErr)
		sigma := makeExpr(m.MkVar(uint32(i+1), realSort))
		wantHead := makeExpr(m.MkApp(scaled[p], []ast.ASTHandle{x, second, sigma}))
		wantTail := makeExpr(m.MkApp(scaled[q], []ast.ASTHandle{x, sigma}))
		wantConstraint := makeExpr(m.MkArithmeticLE(x, makeExpr(m.MkArithmeticMul(sigma, two))))
		head, getErr := rule.Head()
		coneMust(t, getErr)
		if head != wantHead {
			t.Errorf("rule %d head = %v, want %v", i, head, wantHead)
		}
		gotTail, negative, getErr := rule.Tail(0)
		coneMust(t, getErr)
		if gotTail != wantTail || negative {
			t.Errorf("rule %d tail = %v, negated %v; want %v, false", i, gotTail, negative, wantTail)
		}
		count, getErr := rule.TailSize()
		coneMust(t, getErr)
		found := false
		for j := 1; j < count; j++ {
			gotConstraint, negated, tailErr := rule.Tail(j)
			coneMust(t, tailErr)
			if gotConstraint == wantConstraint && !negated {
				found = true
			}
		}
		if !found {
			t.Errorf("rule %d missing rewritten constraint %v", i, wantConstraint)
		}
		name, getErr := rule.Name()
		coneMust(t, getErr)
		if name != names[i] {
			t.Errorf("rule %d name = %q", i, name)
		}
		coneMust(t, rule.Close())
	}
}
