package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func collectorEq(t *testing.T, m *ast.ASTManager, left, right ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkEq(left, right)
	equationMust(t, e)
	fixDLPin(t, m, h)
	return h
}

func TestDegreeShiftCandidateGCDAndOrdinaryOccurrences(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	y := fixDLConst(t, m, false, "y")
	z := fixDLConst(t, m, false, "z")
	integer := fixDLConst(t, m, true, "integer")
	power := func(v ast.ASTHandle, exponent string) ast.ASTHandle {
		return fixDLApp(t, m, 18, v, fixDLNumber(t, m, false, exponent))
	}
	x6, x10 := power(x, "6"), power(x, "10")
	y4 := power(y, "4")
	roots := []ast.ASTHandle{x6, x10, x6, y4, power(z, "21474836480"), power(integer, "4")}
	got, e := degreeShiftCandidates(m, roots)
	equationMust(t, e)
	if len(got) != 3 || got[x].String() != "2" || got[y].String() != "4" || got[z].String() != "21474836480" {
		t.Fatalf("candidate degrees: %v", got)
	}
	// Seeing x alone as a root does not call save_degree(x,1); seeing it as
	// an ordinary application's argument does, even after a shared power.
	got, e = degreeShiftCandidates(m, append(roots, x))
	equationMust(t, e)
	if got[x] == nil {
		t.Fatal("bare root unexpectedly disqualified x")
	}
	ordinary := fixDLApp(t, m, 6, x, fixDLNumber(t, m, false, "1"))
	got, e = degreeShiftCandidates(m, append(roots, ordinary))
	equationMust(t, e)
	if _, exists := got[x]; exists || len(got) != 2 {
		t.Fatalf("ordinary occurrence: %v", got)
	}
	for _, exponent := range []string{"0", "-2", "1/2", "1"} {
		got, e = degreeShiftCandidates(m, []ast.ASTHandle{y4, power(y, exponent)})
		equationMust(t, e)
		if len(got) != 0 {
			t.Fatalf("exponent %s left candidates: %v", exponent, got)
		}
	}
	compound := fixDLApp(t, m, 6, x, y)
	got, e = degreeShiftCandidates(m, []ast.ASTHandle{x6, y4, power(compound, "4")})
	equationMust(t, e)
	if len(got) != 0 {
		t.Fatalf("compound power failed to expose ordinary children: %v", got)
	}
}

func TestEq2BVCandidateBoundsMaximaAndDisqualification(t *testing.T) {
	_, m := fixDLFixture(t, false)
	x := fixDLConst(t, m, true, "x")
	y := fixDLConst(t, m, true, "y")
	r := fixDLConst(t, m, false, "r")
	num := func(s string) ast.ASTHandle { return fixDLNumber(t, m, true, s) }
	eq := func(v ast.ASTHandle, s string) ast.ASTHandle { return collectorEq(t, m, v, num(s)) }
	x0, x7, y3 := eq(x, "0"), eq(x, "7"), eq(y, "3")
	reversed := collectorEq(t, m, num("4294967295"), y)
	roots := []ast.ASTHandle{
		fixDLApp(t, m, 2, num("0"), x), x0, x7,
		fixDLApp(t, m, 3, num("10"), x), y3, reversed,
		collectorEq(t, m, r, fixDLNumber(t, m, false, "5")),
	}
	got, e := eq2bvCandidates(m, roots)
	equationMust(t, e)
	if len(got) != 2 || got[x] != 7 || got[y] != 4294967295 {
		t.Fatalf("maxima: %v", got)
	}
	// Shared equality nodes are visited once and do not expose their variable.
	got, e = eq2bvCandidates(m, []ast.ASTHandle{x0, x0})
	equationMust(t, e)
	if v, ok := got[x]; !ok || v != 0 {
		t.Fatalf("zero domain: %v", got)
	}
	for _, other := range []ast.ASTHandle{
		eq(x, "-1"), eq(x, "4294967296"),
		fixDLApp(t, m, 2, x, num("-1")),
		fixDLApp(t, m, 2, fixDLApp(t, m, 6, x, num("1")), num("10")),
	} {
		for _, forms := range [][]ast.ASTHandle{{x7, other}, {other, x7}} {
			got, e = eq2bvCandidates(m, forms)
			equationMust(t, e)
			if len(got) != 0 {
				t.Fatalf("non-finite occurrence left candidate: %v", got)
			}
		}
	}
	// Bounds are skipped only at goal roots. Nested bounds expose the variable.
	nested, e := m.MkAnd(x7, fixDLApp(t, m, 2, x, num("10")))
	equationMust(t, e)
	fixDLPin(t, m, nested)
	got, e = eq2bvCandidates(m, []ast.ASTHandle{nested})
	equationMust(t, e)
	if len(got) != 0 {
		t.Fatalf("nested bound: %v", got)
	}
}

func TestRecover01ClauseShapeSizeAndFirstArity(t *testing.T) {
	_, m := fixDLFixture(t, false)
	sort, e := m.MkBoolSort()
	equationMust(t, e)
	boolean := func(name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		equationMust(t, e)
		h, e := m.MkConst(symbol, sort)
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	p, q := boolean("p"), boolean("q")
	notP, e := m.MkNot(p)
	equationMust(t, e)
	fixDLPin(t, m, notP)
	x := fixDLConst(t, m, true, "x")
	r := fixDLConst(t, m, false, "r")
	x0 := collectorEq(t, m, x, fixDLNumber(t, m, true, "0"))
	x1 := collectorEq(t, m, fixDLNumber(t, m, true, "1"), x)
	rHalf := collectorEq(t, m, r, fixDLNumber(t, m, false, "-1/2"))
	clause := func(args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkOrArgs(args)
		equationMust(t, e)
		fixDLPin(t, m, h)
		return h
	}
	a, b, wide := clause(p, x0), clause(notP, x1), clause(p, q, x1)
	c := clause(q, rHalf)
	bad := clause(x0, x1)
	plain := clause(p, q)
	got, e := recover01Clauses(m, []ast.ASTHandle{a, wide, b, c, bad, plain, x0}, 10)
	equationMust(t, e)
	xDecl, e := m.GetDecl(x)
	equationMust(t, e)
	rDecl, e := m.GetDecl(r)
	equationMust(t, e)
	if len(got) != 2 || len(got[xDecl]) != 2 || got[xDecl][0] != a || got[xDecl][1] != b || len(got[rDecl]) != 1 || got[rDecl][0] != c {
		t.Fatalf("clauses: %v", got)
	}
	got, e = recover01Clauses(m, []ast.ASTHandle{wide, a, b}, 10)
	equationMust(t, e)
	if len(got[xDecl]) != 1 || got[xDecl][0] != wide {
		t.Fatalf("first arity did not win: %v", got)
	}
	got, e = recover01Clauses(m, []ast.ASTHandle{a, b, wide}, 2)
	equationMust(t, e)
	if len(got) != 0 {
		t.Fatalf("strict size limit: %v", got)
	}
	got, e = recover01Clauses(m, []ast.ASTHandle{a, b, wide}, 3)
	equationMust(t, e)
	if len(got[xDecl]) != 2 {
		t.Fatalf("size boundary: %v", got)
	}
}

func TestArithmeticCandidateQuantifierAnnotations(t *testing.T) {
	_, m := fixDLFixture(t, false)
	for _, integer := range []bool{false, true} {
		x := fixDLConst(t, m, integer, "quantifier-candidate")
		sort, e := m.GetSort(x)
		equationMust(t, e)
		v, e := m.MkVar(0, sort)
		equationMust(t, e)
		fixDLPin(t, m, v)
		annotation := fixDLApp(t, m, 6, x, v)
		pattern, e := m.MkPattern([]ast.ASTHandle{annotation})
		equationMust(t, e)
		fixDLPin(t, m, pattern)
		term := x
		if !integer {
			term = fixDLApp(t, m, 18, x, fixDLNumber(t, m, false, "4"))
		}
		body := collectorEq(t, m, term, fixDLNumber(t, m, integer, "3"))
		for _, patterns := range []bool{false, true} {
			spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body}
			if patterns {
				spec.Patterns = []ast.ASTHandle{pattern}
			} else {
				spec.NoPatterns = []ast.ASTHandle{annotation}
			}
			q, e := m.MkQuantifier(spec)
			equationMust(t, e)
			fixDLPin(t, m, q)
			if integer {
				got, e := eq2bvCandidates(m, []ast.ASTHandle{q})
				equationMust(t, e)
				if len(got) != 1 || got[x] != 3 {
					t.Fatalf("eq2bv must ignore annotations: %v", got)
				}
			} else {
				got, e := degreeShiftCandidates(m, []ast.ASTHandle{q})
				equationMust(t, e)
				if len(got) != 0 {
					t.Fatalf("degree shift must visit annotations: %v", got)
				}
			}
		}
	}
}
