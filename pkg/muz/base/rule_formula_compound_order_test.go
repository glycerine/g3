package base_test

// Source: dl_rule.cpp hoist_compound_predicates visits the head first, then
// each original body literal; mk_negations recognizes the rewritten literal.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleFormulaHoistsHeadBeforeNegativeBody(t *testing.T) {
	ctx, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, ctx.Close()) })
	m, err := ctx.Manager()
	ruleMust(t, err)
	rm, err := base.NewRuleManager(m)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rm.Close()) })
	boolSort, err := m.MkBoolSort()
	ruleMust(t, err)
	family, err := m.ArithmeticFamilyID()
	ruleMust(t, err)
	intSort, err := m.MkFamilySort(family, 1, nil)
	ruleMust(t, err)
	decls := make([]ast.ASTHandle, 2)
	for i, name := range []string{"compound-order-head", "compound-order-body"} {
		sym, e := m.Symbols.Intern(name)
		ruleMust(t, e)
		decls[i], e = m.MkFuncDecl(sym, []ast.ASTHandle{intSort}, boolSort, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, rm.RegisterPredicate(decls[i]))
	}
	x, err := m.MkVar(0, intSort)
	ruleMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	ruleMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	ruleMust(t, err)
	headArg, err := m.MkArithmeticAdd(x, one)
	ruleMust(t, err)
	bodyArg, err := m.MkArithmeticAdd(x, two)
	ruleMust(t, err)
	head, err := m.MkApp(decls[0], []ast.ASTHandle{headArg})
	ruleMust(t, err)
	body, err := m.MkApp(decls[1], []ast.ASTHandle{bodyArg})
	ruleMust(t, err)
	negative, err := m.MkNot(body)
	ruleMust(t, err)
	formula, err := m.MkImplies(negative, head)
	ruleMust(t, err)
	root, err := m.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&root)) }()
	rule, err := rm.NewRuleFromFormula(formula, "compound-order")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	gotHead, err := rule.Head()
	ruleMust(t, err)
	gotHeadArg, err := m.GetArg(gotHead, 0)
	ruleMust(t, err)
	if index, e := m.GetVarIndex(gotHeadArg); e != nil || index != 1 {
		t.Fatal("head variable", index, e)
	}
	if size, e := rule.TailSize(); e != nil || size != 3 {
		t.Fatal("tail size", size, e)
	}
	gotBody, negated, err := rule.Tail(0)
	ruleMust(t, err)
	gotBodyArg, err := m.GetArg(gotBody, 0)
	ruleMust(t, err)
	if index, e := m.GetVarIndex(gotBodyArg); e != nil || index != 2 || !negated {
		t.Fatal("negative body variable", index, negated, e)
	}
	found := map[ast.ASTHandle]ast.ASTHandle{}
	for i := 1; i < 3; i++ {
		eq, neg, e := rule.Tail(i)
		ruleMust(t, e)
		op, e := m.GetDeclKind(eq)
		ruleMust(t, e)
		if op != ast.ASTOpEq || neg {
			t.Fatal("expected positive equality", op, neg)
		}
		lhs, e := m.GetArg(eq, 0)
		ruleMust(t, e)
		rhs, e := m.GetArg(eq, 1)
		ruleMust(t, e)
		found[lhs] = rhs
	}
	if found[gotHeadArg] != headArg || found[gotBodyArg] != bodyArg {
		t.Fatal("wrong hoisted equalities", found)
	}
	// The proof-carrying source path rewrites the original proof to the
	// reconstructed rule after both compound arguments have been hoisted.
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	provedRule, err := rm.NewRuleFromFormulaWithProof(formula, proof, "compound-order-proof")
	ruleMust(t, err)
	defer func() { ruleMust(t, provedRule.Close()) }()
	translated, present, err := provedRule.Proof()
	ruleMust(t, err)
	if !present {
		t.Fatal("hoisted rule lost its proof")
	}
	fact, err := m.GetProofFact(translated)
	ruleMust(t, err)
	result, err := provedRule.ToFormula()
	ruleMust(t, err)
	defer func() { ruleMust(t, m.DelRef(&result)) }()
	resultHandle, err := m.RefGet(result)
	ruleMust(t, err)
	if fact != resultHandle {
		t.Fatal("proof does not establish reconstructed rule", fact, resultHandle)
	}
}
