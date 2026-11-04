package transforms_test

// Source: dl_mk_scale.cpp mk_pred and linearize.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestScaleNestedConstraintAndPredicateNumerals(t *testing.T) {
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
	zero := makeExpr(m.ArithmeticRealNumeralText("0"))
	one := makeExpr(m.ArithmeticRealNumeralText("1"))
	two := makeExpr(m.ArithmeticRealNumeralText("2"))
	three := makeExpr(m.ArithmeticRealNumeralText("3"))
	realSort := makeExpr(m.GetSort(zero))
	boolSort := makeExpr(m.MkBoolSort())
	x := makeExpr(m.MkVar(0, realSort))
	pname, err := m.Symbols.Intern("scale-nested-head")
	coneMust(t, err)
	qname, err := m.Symbols.Intern("scale-nested-tail")
	coneMust(t, err)
	domain := []ast.ASTHandle{realSort, realSort, realSort, realSort}
	p := makeExpr(m.MkFuncDecl(pname, domain, boolSort, ast.ASTFuncDeclInfo{}))
	q := makeExpr(m.MkFuncDecl(qname, domain, boolSort, ast.ASTFuncDeclInfo{}))
	coneMust(t, rm.RegisterPredicate(p))
	coneMust(t, rm.RegisterPredicate(q))
	head := makeExpr(m.MkApp(p, []ast.ASTHandle{zero, one, three, x}))
	tail := makeExpr(m.MkApp(q, []ast.ASTHandle{zero, one, two, x}))
	// linearize descends into or/not, comparisons and addition, but must
	// leave the multiplication (including its numeral) untouched.
	sum := makeExpr(m.MkArithmeticAdd(x, two))
	left := makeExpr(m.MkArithmeticLE(sum, one))
	product := makeExpr(m.MkArithmeticMul(x, three))
	right := makeExpr(m.MkArithmeticGT(product, zero))
	constraint := makeExpr(m.MkOr(left, makeExpr(m.MkNot(right))))
	rule, err := rm.NewRule(head, []ast.ASTHandle{tail, constraint}, nil, "scale-nested")
	coneMust(t, err)
	coneMust(t, source.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, source.SetOutputPredicate(p))
	plugin := transforms.NewScalePlugin(m, rm, 36030)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	result, err := plugin.Transform(source)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, result.Close()) })
	transformed, err := result.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	sigma := makeExpr(m.MkVar(1, realSort))
	tailVar := makeExpr(m.MkVar(2, realSort))
	headVar := makeExpr(m.MkVar(3, realSort))
	scaled := plugin.FreshPredicates()
	wantHead := makeExpr(m.MkApp(scaled[p], []ast.ASTHandle{zero, sigma, headVar, x, sigma}))
	wantTail := makeExpr(m.MkApp(scaled[q], []ast.ASTHandle{zero, sigma, tailVar, x, sigma}))
	gotHead, err := transformed.Head()
	coneMust(t, err)
	if gotHead != wantHead {
		t.Fatalf("head = %v, want %v", gotHead, wantHead)
	}
	gotTail, negated, err := transformed.Tail(0)
	coneMust(t, err)
	if gotTail != wantTail || negated {
		t.Fatalf("tail = %v, negated %v; want %v", gotTail, negated, wantTail)
	}
	scaledTwo := makeExpr(m.MkArithmeticMul(sigma, two))
	scaledOne := makeExpr(m.MkArithmeticMul(sigma, one))
	scaledZero := makeExpr(m.MkArithmeticMul(sigma, zero))
	wantSum := makeExpr(m.MkArithmeticAdd(x, scaledTwo))
	wantLeft := makeExpr(m.MkArithmeticLE(wantSum, scaledOne))
	wantRight := makeExpr(m.MkArithmeticGT(product, scaledZero))
	wantConstraint := makeExpr(m.MkOr(wantLeft, makeExpr(m.MkNot(wantRight))))
	tailEq := makeExpr(m.MkEq(tailVar, makeExpr(m.MkArithmeticMul(two, sigma))))
	headEq := makeExpr(m.MkEq(headVar, makeExpr(m.MkArithmeticMul(three, sigma))))
	positive := makeExpr(m.MkArithmeticGT(sigma, zero))
	expected := map[ast.ASTHandle]bool{wantConstraint: false, tailEq: false, headEq: false, positive: false}
	n, err := transformed.TailSize()
	coneMust(t, err)
	if n != 5 {
		t.Fatalf("tail count = %d, want 5", n)
	}
	for i := 1; i < n; i++ {
		h, neg, err := transformed.Tail(i)
		coneMust(t, err)
		if _, ok := expected[h]; !ok || expected[h] || neg {
			t.Fatalf("unexpected interpreted tail %d: %v (negated %v)", i, h, neg)
		}
		expected[h] = true
	}
	for h, found := range expected {
		if !found {
			t.Errorf("missing interpreted tail %v", h)
		}
	}
	output, err := result.IsOutputPredicate(scaled[p])
	coneMust(t, err)
	if !output {
		t.Fatal("scaled head was not marked as output")
	}
}
