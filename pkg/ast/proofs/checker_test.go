package proofs_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) *ast.ASTManager {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	s, err := usymbol.NewManager(usymbol.Config{Budget: b})
	must(t, err)
	n, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	must(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: n, Budget: b, ProofMode: ast.ASTProofEnabled})
	must(t, err)
	t.Cleanup(func() {
		must(t, m.Close())
		must(t, n.Close())
		must(t, s.Close())
		if b.Used() != 0 {
			t.Fatalf("owner graph retained %d bytes", b.Used())
		}
	})
	return m
}

func boolean(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := m.MkBoolSort()
	must(t, err)
	sym, err := m.Symbols.Intern(name)
	must(t, err)
	h, err := m.MkConst(sym, sort)
	must(t, err)
	return h
}

func check(t *testing.T, c *proofs.Checker, p ast.ASTHandle, want bool) []ast.ASTHandle {
	t.Helper()
	got, side, err := c.Check(p)
	must(t, err)
	if got != want {
		t.Fatalf("check = %v, want %v", got, want)
	}
	t.Cleanup(func() { must(t, c.ReleaseSideConditions(side)) })
	return side
}

func TestCheckerStructuralRulesAndInvalidConclusion(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	a, b, d := boolean(t, m, "a"), boolean(t, m, "b"), boolean(t, m, "d")
	ab, err := m.MkIff(a, b)
	must(t, err)
	pa, err := m.MkProofHypothesis(a)
	must(t, err)
	pab, err := m.MkProofHypothesis(ab)
	must(t, err)
	mp, err := m.MkProofModusPonens(pa, pab)
	must(t, err)
	check(t, c, mp, true)

	bad, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTPrModusPonens, nil, []ast.ASTHandle{pa, pab, d})
	must(t, err)
	check(t, c, bad, false)

	symm, err := m.MkProofSymmetry(pab)
	must(t, err)
	check(t, c, symm, true)
	refl, err := m.MkProofReflexivity(a)
	must(t, err)
	check(t, c, refl, true)
}

func TestCheckerEliminationResolutionAndRewriteSideConditions(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	a, b := boolean(t, m, "a"), boolean(t, m, "b")
	and, err := m.MkAnd(a, b)
	must(t, err)
	pand, err := m.MkProofAsserted(and)
	must(t, err)
	elim, err := m.MkProofAndElim(pand, 1)
	must(t, err)
	check(t, c, elim, true)

	nb, err := m.MkNot(b)
	must(t, err)
	or, err := m.MkOr(a, b)
	must(t, err)
	por, err := m.MkProofAsserted(or)
	must(t, err)
	pnb, err := m.MkProofAsserted(nb)
	must(t, err)
	unit, err := m.MkProofUnitResolution([]ast.ASTHandle{por, pnb})
	must(t, err)
	check(t, c, unit, true)

	rewrite, err := m.MkProofRewrite(a, b)
	must(t, err)
	side := check(t, c, rewrite, true)
	if len(side) != 1 {
		t.Fatalf("rewrite side conditions = %d", len(side))
	}

	// rewrite* with no parents: m.mk_or(rewrite_eq.size(), ...) keeps a
	// one-term or (ast_sub_02-F11).
	star, err := m.MkProofRewriteStar(a, b, nil)
	must(t, err)
	side = check(t, c, star, true)
	if len(side) != 1 {
		t.Fatalf("rewrite* side conditions = %d", len(side))
	}
	if kind, e := m.GetDeclKind(side[0]); e != nil || kind != ast.ASTOpOr {
		t.Fatalf("rewrite* side condition is not an or: %d, %v", kind, e)
	}
	if n, e := m.GetNumArgs(side[0]); e != nil || n != 1 {
		t.Fatalf("rewrite* side condition arity = %d, %v", n, e)
	}
}

func TestCheckerLemmaClosesHypotheses(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	a := boolean(t, m, "a")
	na, err := m.MkNot(a)
	must(t, err)
	pa, err := m.MkProofHypothesis(a)
	must(t, err)
	pna, err := m.MkProofHypothesis(na)
	must(t, err)
	falseProof, err := m.MkProofUnitResolution([]ast.ASTHandle{pa, pna})
	must(t, err)
	tautology, err := m.MkOr(na, a)
	must(t, err)
	lemma, err := m.MkProofLemma(falseProof, tautology)
	must(t, err)
	check(t, c, lemma, true)

	// A single quoted hypothesis closes directly against its negation.
	f, err := m.MkFalse()
	must(t, err)
	pf, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTPrUnitResolution, nil, []ast.ASTHandle{pa, pna, f})
	must(t, err)
	closed, err := m.MkProofLemma(pf, na)
	must(t, err)
	check(t, c, closed, false)
}

func TestCheckerHyperResolutionReconstructsSideCondition(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	a, b := boolean(t, m, "a"), boolean(t, m, "b")
	clause, err := m.MkOr(a, b)
	must(t, err)
	notB, err := m.MkNot(b)
	must(t, err)
	p0, err := m.MkProofAsserted(clause)
	must(t, err)
	p1, err := m.MkProofAsserted(notB)
	must(t, err)
	p, err := m.MkProofHyperResolve(
		[]ast.ASTHandle{p0, p1}, a,
		[]ast.ASTProofPosition{{First: 1, Second: 0}},
		[][]ast.ASTHandle{nil, nil},
	)
	must(t, err)
	side := check(t, c, p, true)
	if len(side) != 1 {
		t.Fatalf("hyper-resolution side conditions = %d", len(side))
	}
	if kind, e := m.GetDeclKind(side[0]); e != nil || kind != ast.ASTOpIff {
		t.Fatalf("side condition is not an equivalence: %d, %v", kind, e)
	}
}

func TestCheckerArithmeticFarkasLemma(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	must(t, m.RegisterDeclarationPlugins())
	arithName, err := m.Symbols.Intern("arith")
	must(t, err)
	arith, err := m.FamilyIDForSymbol(arithName)
	must(t, err)
	intSort, err := m.MkFamilySort(arith, 1, nil)
	must(t, err)
	xName, err := m.Symbols.Intern("x")
	must(t, err)
	x, err := m.MkConst(xName, intSort)
	must(t, err)
	service := m.NumeralService()
	zeroValue, err := service.NewValue()
	must(t, err)
	oneValue, err := service.NewValue()
	must(t, err)
	must(t, service.SetString(&zeroValue, "0"))
	must(t, service.SetString(&oneValue, "1"))
	zero, err := m.ArithmeticNumeral(&zeroValue, true)
	must(t, err)
	one, err := m.ArithmeticNumeral(&oneValue, true)
	must(t, err)
	must(t, service.Del(&zeroValue))
	must(t, service.Del(&oneValue))
	le, err := m.MkFamilyApp(arith, 2, nil, []ast.ASTHandle{x, zero})
	must(t, err)
	ge, err := m.MkFamilyApp(arith, 3, nil, []ast.ASTHandle{x, one})
	must(t, err)
	p0, err := m.MkProofAsserted(le)
	must(t, err)
	p1, err := m.MkProofAsserted(ge)
	must(t, err)
	falseExpr, err := m.MkFalse()
	must(t, err)
	farkasName, err := m.Symbols.Intern("farkas")
	must(t, err)
	farkas, err := m.NewParameterSymbol(farkasName)
	must(t, err)
	coefficientValue, err := service.NewValue()
	must(t, err)
	must(t, service.SetString(&coefficientValue, "1"))
	coefficient0, err := m.NewParameterRational(&coefficientValue)
	must(t, err)
	coefficient1, err := m.NewParameterRational(&coefficientValue)
	must(t, err)
	must(t, service.Del(&coefficientValue))
	lemma, err := m.MkProofTheoryLemma(arith, falseExpr, []ast.ASTHandle{p0, p1}, []ast.ASTParameter{farkas, coefficient0, coefficient1})
	must(t, err)
	must(t, m.DelParameter(&coefficient1))
	must(t, m.DelParameter(&coefficient0))
	must(t, m.DelParameter(&farkas))
	check(t, c, lemma, true)
}

// check1_basic has no PR_TRUE case and check_arith_proof refuses a farkas
// parameter that is not rational: both reach UNREACHABLE(); return false.
func TestCheckerRejectsTrueProofAndNonRationalFarkas(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	must(t, m.RegisterDeclarationPlugins())
	truth, err := m.MkProofTrue()
	must(t, err)
	check(t, c, truth, false)

	arithName, err := m.Symbols.Intern("arith")
	must(t, err)
	arith, err := m.FamilyIDForSymbol(arithName)
	must(t, err)
	falseExpr, err := m.MkFalse()
	must(t, err)
	farkasName, err := m.Symbols.Intern("farkas")
	must(t, err)
	farkas, err := m.NewParameterSymbol(farkasName)
	must(t, err)
	oddName, err := m.Symbols.Intern("not-a-coefficient")
	must(t, err)
	odd, err := m.NewParameterSymbol(oddName)
	must(t, err)
	lemma, err := m.MkProofTheoryLemma(arith, falseExpr, nil, []ast.ASTParameter{farkas, odd})
	must(t, err)
	must(t, m.DelParameter(&odd))
	must(t, m.DelParameter(&farkas))
	check(t, c, lemma, false)
}

// SAME_OP compares basic-family equivalences: arith <= shares OP_EQ's kind
// number but is no equivalence premise for monotonicity (ast_sub_03-F2).
func TestCheckerMonotonicityRejectsNonEquivalencePremise(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	must(t, m.RegisterDeclarationPlugins())
	arithName, err := m.Symbols.Intern("arith")
	must(t, err)
	arith, err := m.FamilyIDForSymbol(arithName)
	must(t, err)
	intSort, err := m.MkFamilySort(arith, 1, nil)
	must(t, err)
	constant := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		must(t, e)
		h, e := m.MkConst(sym, intSort)
		must(t, e)
		return h
	}
	x, y := constant("mono-x"), constant("mono-y")
	fName, err := m.Symbols.Intern("mono-f")
	must(t, err)
	f, err := m.MkFuncDecl(fName, []ast.ASTHandle{intSort}, intSort, ast.ASTFuncDeclInfo{})
	must(t, err)
	fx, err := m.MkApp(f, []ast.ASTHandle{x})
	must(t, err)
	fy, err := m.MkApp(f, []ast.ASTHandle{y})
	must(t, err)
	// arith OP_LE has the same kind number as basic OP_EQ.
	le, err := m.MkFamilyApp(arith, 2, nil, []ast.ASTHandle{x, y})
	must(t, err)
	premise, err := m.MkProofAsserted(le)
	must(t, err)
	eq, err := m.MkEq(fx, fy)
	must(t, err)
	eqDecl, err := m.GetDecl(eq)
	must(t, err)
	mono, err := m.MkProofMonotonicity(eqDecl, fx, fy, []ast.ASTHandle{premise})
	must(t, err)
	check(t, c, mono, false)
	xy, err := m.MkEq(x, y)
	must(t, err)
	good, err := m.MkProofAsserted(xy)
	must(t, err)
	mono, err = m.MkProofMonotonicity(eqDecl, fx, fy, []ast.ASTHandle{good})
	must(t, err)
	check(t, c, mono, true)
}

// PR_QUANT_INTRO needs iff or ~ on both sides, with m.is_oeq agreeing
// (ast_sub_03-F3).
func TestCheckerQuantIntroOperatorMustAgree(t *testing.T) {
	m := fixture(t)
	c, err := proofs.NewChecker(m)
	must(t, err)
	b, err := m.MkBoolSort()
	must(t, err)
	v0, err := m.MkVar(0, b)
	must(t, err)
	pName, err := m.Symbols.Intern("qi-p")
	must(t, err)
	pDecl, err := m.MkFuncDecl(pName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(t, err)
	qName, err := m.Symbols.Intern("qi-q")
	must(t, err)
	qDecl, err := m.MkFuncDecl(qName, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(t, err)
	pv, err := m.MkApp(pDecl, []ast.ASTHandle{v0})
	must(t, err)
	qv, err := m.MkApp(qDecl, []ast.ASTHandle{v0})
	must(t, err)
	forall := func(body ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: body})
		must(t, e)
		return h
	}
	fp, fq := forall(pv), forall(qv)
	iff, err := m.MkIff(pv, qv)
	must(t, err)
	iffProof, err := m.MkProofAsserted(iff)
	must(t, err)
	oeq, err := m.MkOEq(pv, qv)
	must(t, err)
	oeqProof, err := m.MkProofAsserted(oeq)
	must(t, err)
	// The source-debug factory asserts the premise shape; the release
	// factory builds the mixed step, which the checker must refuse.
	if !ast.ASTSourceDebug {
		mixed, err := m.MkProofOEqQuantIntro(fp, fq, iffProof)
		must(t, err)
		check(t, c, mixed, false)
	}
	control, err := m.MkProofOEqQuantIntro(fp, fq, oeqProof)
	must(t, err)
	check(t, c, control, true)
	plain, err := m.MkProofQuantIntro(fp, fq, iffProof)
	must(t, err)
	check(t, c, plain, true)
}
