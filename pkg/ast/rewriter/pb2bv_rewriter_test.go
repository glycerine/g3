package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func pb2BVFixture(t *testing.T) (*ast.ASTManager, *PB2BVRewriter) {
	t.Helper()
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	c, p := enum2BVParameters(t, m)
	r, err := NewPB2BVRewriter(m, c, p)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	return m, r
}

func TestPB2BVCardinalityTraversalAndOrShortcut(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "pb2bv-a")
	b := goalTestAtom(t, m, "pb2bv-b")
	c := goalTestAtom(t, m, "pb2bv-c")

	atLeastOne, err := m.MkPseudoBooleanAtLeast([]ast.ASTHandle{a, b, c}, 1)
	astTestMust(t, err)
	out := astTestPin(t, m, atLeastOne)
	astTestMust(t, r.Rewrite(atLeastOne, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if family, e := m.GetFamilyID(got); e != nil || family != m.BasicFamilyID() {
		t.Fatal("at-least-one OR shortcut", family, e)
	}
	if kind, e := m.GetDeclKind(got); e != nil || kind != ast.ASTOpOr {
		t.Fatal("at-least-one result", kind, e)
	}

	atMostOne, err := m.MkPseudoBooleanAtMost([]ast.ASTHandle{a, b, c}, 1)
	astTestMust(t, err)
	astTestMust(t, r.Rewrite(atMostOne, out, ast.ASTRef{}))
	got, err = m.RefGet(out)
	astTestMust(t, err)
	if got == atMostOne {
		t.Fatal("sorting-network cardinality was not translated")
	}
	if family, e := m.GetFamilyID(got); e != nil || family == r.pseudo.family {
		t.Fatal("PB result remained in PB family", family, e)
	}
	if translated, e := r.NumTranslated(); e != nil || translated != 2 {
		t.Fatal("translated count", translated, e)
	}
	if steps, e := r.NumSteps(); e != nil || steps == 0 {
		t.Fatal("traversal steps", steps, e)
	}
	clauses, err := r.FlushSideConstraints()
	astTestMust(t, err)
	for _, clause := range clauses {
		astTestMust(t, m.DecRef(clause))
	}
}

func TestPB2BVWeightedConversionAndScopeGuards(t *testing.T) {
	m, r := pb2BVFixture(t)
	name, value := "pb.solver", astTestSymbol(t, m, "bv")
	params, err := r.paramsContext.NewValue()
	astTestMust(t, err)
	astTestMust(t, r.paramsContext.SetSymbolName(&params, &name, value))
	astTestMust(t, r.UpdateParams(&params))
	astTestMust(t, r.paramsContext.Del(&params))
	a := goalTestAtom(t, m, "pb2bv-weight-a")
	b := goalTestAtom(t, m, "pb2bv-weight-b")
	values := pbTestRationals(t, m, "2", "3", "4")
	weighted, err := m.MkPseudoBooleanGE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, err)
	out := astTestPin(t, m, a)
	astTestMust(t, r.Rewrite(weighted, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got == weighted {
		t.Fatal("weighted constraint was not translated")
	}
	if kind, e := m.GetDeclKind(got); e != nil || kind != ast.ASTOpAnd {
		t.Fatal("dualized weighted result", kind, e)
	}
	if left, e := m.GetArg(got, 0); e != nil || left != a {
		t.Fatal("dualized first literal", left, a, e)
	}
	if right, e := m.GetArg(got, 1); e != nil || right != b {
		t.Fatal("dualized second literal", right, b, e)
	}
	if translated, e := r.NumTranslated(); e != nil || translated != 1 {
		t.Fatal("weighted translation count", translated, e)
	}

	astTestMust(t, r.Push())
	astTestMust(t, r.context.Clause(0, ast.ASTLiteralVectorSpan{}))
	if err = r.Pop(1); err != ast.ErrASTDomain {
		t.Fatal("pop accepted unflushed clauses", err)
	}
	clauses, err := r.FlushSideConstraints()
	astTestMust(t, err)
	for _, clause := range clauses {
		astTestMust(t, m.DecRef(clause))
	}
	astTestMust(t, r.Pop(1))
	if err = r.Pop(1); err != ast.ErrASTDomain {
		t.Fatal("pop accepted missing scope", err)
	}
}

func TestPB2BVArithmeticBooleanITERecognition(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "pb2bv-arithmetic-a")
	zero := arithRewriterNumeral(t, m, "0", true)
	two := arithRewriterNumeral(t, m, "2", true)
	conditional, err := m.MkIte(a, two, zero)
	astTestMust(t, err)
	comparison, err := m.MkFamilyApp(r.arithFamily, arithOpGE, nil, []ast.ASTHandle{conditional, two})
	astTestMust(t, err)
	out := astTestPin(t, m, comparison)
	astTestMust(t, r.Rewrite(comparison, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got != a {
		t.Fatal("arithmetic ITE comparison", got, a)
	}
	if translated, e := r.NumTranslated(); e != nil || translated != 1 {
		t.Fatal("arithmetic translation count", translated, e)
	}
}

func TestPB2BVCloseGuards(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "pb2bv-close")
	out := astTestPin(t, m, a)
	astTestMust(t, r.Close())
	if err := r.Rewrite(a, out, ast.ASTRef{}); err != ast.ErrASTHandle {
		t.Fatal("closed rewriter accepted traversal", err)
	}
}

func TestPB2BVCleanupReconstructsImplementation(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "pb2bv-cleanup-a")
	b := goalTestAtom(t, m, "pb2bv-cleanup-b")
	constraint, err := m.MkPseudoBooleanAtMost([]ast.ASTHandle{a, b}, 1)
	astTestMust(t, err)
	astTestPin(t, m, constraint)
	out := astTestPin(t, m, constraint)
	astTestMust(t, r.Rewrite(constraint, out, ast.ASTRef{}))
	if translated, e := r.NumTranslated(); e != nil || translated != 1 {
		t.Fatal("pre-cleanup translation", translated, e)
	}
	manager, err := r.Manager()
	astTestMust(t, err)
	if manager != m {
		t.Fatal("manager borrow changed")
	}
	astTestMust(t, r.Cleanup())
	if translated, e := r.NumTranslated(); e != nil || translated != 0 {
		t.Fatal("cleanup translation reset", translated, e)
	}
	stats, err := r.CollectStatistics()
	astTestMust(t, err)
	if stats != (PB2BVStatistics{}) {
		t.Fatal("cleanup statistics reset", stats)
	}
	astTestMust(t, r.Rewrite(constraint, out, ast.ASTRef{}))
	if translated, e := r.NumTranslated(); e != nil || translated != 1 {
		t.Fatal("post-cleanup translation", translated, e)
	}
}
