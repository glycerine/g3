package arith

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func bv2RealFixture(t *testing.T, max uint32) (*BV2RealUtil, *bv2IntFixture) {
	t.Helper()
	f := bv2IntRewriteFixture(t, ^uint32(0))
	u, err := NewBV2RealUtil(f.m, 2, 2, max)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, u.Close()) })
	return u, f
}
func bv2RealOut(t *testing.T, m *ast.ASTManager) ast.ASTRef {
	t.Helper()
	out, err := m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, m.DelRef(&out)) })
	return out
}
func bv2RealCallResult(t *testing.T, u *BV2RealUtil, build func(*bv2RealCall) ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	c := &bv2RealCall{u: u}
	h := build(c)
	if c.err != nil {
		t.Fatal(c.err)
	}
	fixDLPin(t, u.manager, h)
	equationMust(t, c.close())
	return h
}
func bv2RealSignedValue(t *testing.T, f *bv2IntFixture, h ast.ASTHandle) *big.Int {
	t.Helper()
	h = f.value(t, h)
	q, err := f.m.NumeralService().NewValue()
	equationMust(t, err)
	defer func() { equationMust(t, f.m.NumeralService().Del(&q)) }()
	var width uint32
	ok, err := f.m.BitVectorNumeralValue(h, &q, &width)
	equationMust(t, err)
	if !ok {
		t.Fatal("expected evaluated BV numeral")
	}
	text, err := f.m.NumeralService().String(&q)
	equationMust(t, err)
	r, ok := new(big.Rat).SetString(text)
	if !ok || !r.IsInt() {
		t.Fatal(text)
	}
	n := new(big.Int).Set(r.Num())
	if n.Bit(int(width-1)) != 0 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(width)))
	}
	return n
}
func TestBV2RealUtilDefaultsDeclarationsCacheAndReset(t *testing.T) {
	u, f := bv2RealFixture(t, 32)
	m := f.m
	root, d, max, err := defaultsRat(u)
	equationMust(t, err)
	if root.Cmp(big.NewRat(2, 1)) != 0 || d.Cmp(big.NewRat(2, 1)) != 0 || max.Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("source defaults")
	}
	decls, err := u.AuxDecls()
	equationMust(t, err)
	if len(decls) != 2 || decls[0] == decls[1] {
		t.Fatal("positive comparison declarations")
	}
	for _, decl := range decls {
		refs, err := m.GetRefCount(decl)
		equationMust(t, err)
		if refs != 2 {
			t.Fatal("source member plus declaration-vector ownership", refs)
		}
	}
	a, b := f.bv(t, 1, 3), f.bv(t, 2, 5)
	out := bv2RealOut(t, m)
	equationMust(t, mkBV2RealCRat(u, a, b, big.NewRat(2, 1), big.NewRat(2, 1), out))
	first, err := m.RefGet(out)
	equationMust(t, err)
	firstDecl, err := m.GetDecl(first)
	equationMust(t, err)
	equationMust(t, mkBV2RealCRat(u, f.bv(t, 3, 3), b, big.NewRat(4, 2), big.NewRat(4, 2), out))
	same, err := m.RefGet(out)
	equationMust(t, err)
	sameDecl, err := m.GetDecl(same)
	equationMust(t, err)
	if sameDecl != firstDecl {
		t.Fatal("cache compared argument values instead of source widths/divisor/root")
	}
	for _, change := range []string{"width", "divisor", "root"} {
		x, divisor, r := a, big.NewRat(2, 1), big.NewRat(2, 1)
		switch change {
		case "width":
			x = f.bv(t, 1, 4)
		case "divisor":
			divisor = big.NewRat(3, 1)
		case "root":
			r = big.NewRat(3, 1)
		}
		equationMust(t, mkBV2RealCRat(u, x, b, divisor, r, out))
		h, err := m.RefGet(out)
		equationMust(t, err)
		decl, err := m.GetDecl(h)
		equationMust(t, err)
		if decl == firstDecl {
			t.Fatal("cache signature omitted", change)
		}
	}
	decls, err = u.AuxDecls()
	equationMust(t, err)
	if len(decls) != 6 {
		t.Fatal("cache declaration count", len(decls))
	}
	truth, err := m.MkTrue()
	equationMust(t, err)
	equationMust(t, u.AddSideCondition(truth))
	equationMust(t, u.Reset())
	side, err := u.SideConditions()
	equationMust(t, err)
	if len(side) != 0 {
		t.Fatal("reset kept conditions")
	}
	equationMust(t, mkBV2RealCRat(u, a, b, big.NewRat(2, 1), big.NewRat(2, 1), out))
	h, err := m.RefGet(out)
	equationMust(t, err)
	decl, err := m.GetDecl(h)
	equationMust(t, err)
	if decl != firstDecl {
		t.Fatal("reset incorrectly cleared signature cache")
	}
}
func TestBV2RealUtilSignedArithmeticIndependentOracle(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	for _, a := range []int64{-5, -1, 0, 1, 5} {
		for _, b := range []int64{-7, -1, 0, 2, 9} {
			for _, op := range []string{"add", "sub", "mul"} {
				h := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
					s, tv := c.signedNumeral(big.NewRat(a, 1)), c.signedNumeral(big.NewRat(b, 1))
					switch op {
					case "add":
						return c.add(s, tv)
					case "sub":
						return c.sub(s, tv)
					default:
						return c.mul(s, tv)
					}
				})
				want := a + b
				if op == "sub" {
					want = a - b
				}
				if op == "mul" {
					want = a * b
				}
				if actual := bv2RealSignedValue(t, f, h); actual.Cmp(big.NewInt(want)) != 0 {
					t.Fatal("signed arithmetic", a, b, op, actual, want)
				}
			}
		}
	}
	for _, value := range []uint32{3, 4} {
		h := f.bv(t, value, 3)
		extended := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.extend(2, h) })
		op, err := f.m.GetDeclKind(extended)
		equationMust(t, err)
		want := int32(0)
		if value == 4 {
			want = 38
		}
		if op != want {
			t.Fatal("source signed extension branch", value, op, want)
		}
	}
}
func TestBV2RealUtilRealNumeralMatchAndIntegerRefusal(t *testing.T) {
	u, f := bv2RealFixture(t, 32)
	m := f.m
	a, b := bv2RealOut(t, m), bv2RealOut(t, m)
	var d, r big.Rat
	h := fixDLNumber(t, m, false, "-7/3")
	ok, err := matchRat(u, h, a, b, &d, &r)
	equationMust(t, err)
	if !ok || d.Cmp(big.NewRat(3, 1)) != 0 || r.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("real numeral match")
	}
	av, err := m.RefGet(a)
	equationMust(t, err)
	bv, err := m.RefGet(b)
	equationMust(t, err)
	if bv2RealSignedValue(t, f, av).Cmp(big.NewInt(-7)) != 0 || bv2RealSignedValue(t, f, bv).Sign() != 0 {
		t.Fatal("real coefficient numerators")
	}
	width, err := m.BitVectorSize(bv)
	equationMust(t, err)
	if width != 2 {
		t.Fatal("source signed zero width", width)
	}
	beforeA, beforeB := av, bv
	ok, err = matchRat(u, fixDLNumber(t, m, true, "4"), a, b, &d, &r)
	equationMust(t, err)
	if ok {
		t.Fatal("integer numeral accepted as real encoding")
	}
	av, err = m.RefGet(a)
	equationMust(t, err)
	bv, err = m.RefGet(b)
	equationMust(t, err)
	if av != beforeA || bv != beforeB {
		t.Fatal("refused match changed outputs")
	}
}
func TestBV2RealUtilDivisorAlignmentAndRefusalPrefixes(t *testing.T) {
	for _, divisor := range []int64{6, 8, 20, 24, 32} {
		t.Run(fmt.Sprint(divisor), func(t *testing.T) {
			u, f := bv2RealFixture(t, 32)
			m := f.m
			out := bv2RealOut(t, m)
			a, b := f.bv(t, 4, 3), f.bv(t, 2, 3)
			d := big.NewRat(divisor, 1)
			ok, err := mkBV2RealRat(u, a, b, d, big.NewRat(2, 1), out)
			equationMust(t, err)
			want := divisor == 8 || divisor == 32
			if ok != want {
				t.Fatal("divisor acceptance", ok, want)
			}
			side, err := u.SideConditions()
			equationMust(t, err)
			count := 0
			if divisor == 8 || divisor == 32 {
				count = 2
			}
			if divisor == 24 {
				count = 1
			}
			if len(side) != count {
				t.Fatal("published side condition prefix", len(side), count)
			}
			if ok {
				if d.Cmp(big.NewRat(4, 1)) != 0 {
					t.Fatal("divisor was not capped")
				}
				h, err := m.RefGet(out)
				equationMust(t, err)
				first, err := m.GetArg(h, 0)
				equationMust(t, err)
				width, err := m.BitVectorSize(first)
				equationMust(t, err)
				expected := uint32(2)
				if divisor == 32 {
					expected = 1
				}
				if width != expected {
					t.Fatal("coefficient shrink", width, expected)
				}
			} else {
				if d.Cmp(big.NewRat(divisor, 1)) != 0 {
					t.Fatal("refusal changed divisor")
				}
				h, err := m.RefGet(out)
				equationMust(t, err)
				if !h.IsNull() {
					t.Fatal("refusal published encoding")
				}
			}
		})
	}
}
func TestBV2RealUtilMultiplicationWidthLimit(t *testing.T) {
	for _, max := range []uint32{0, 2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(max), func(t *testing.T) {
			u, f := bv2RealFixture(t, max)
			h := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
				return c.mul(c.signedNumeral(big.NewRat(2, 1)), c.signedNumeral(big.NewRat(3, 1)))
			})
			width, err := f.m.BitVectorSize(h)
			equationMust(t, err)
			want := max
			if max <= 3 {
				want = 3
			}
			if max >= 6 {
				want = 6
			}
			if width != want {
				t.Fatal("product width", width, want)
			}
			side, err := u.SideConditions()
			equationMust(t, err)
			count := 2
			if max >= 6 {
				count = 0
			}
			if len(side) != count {
				t.Fatal("signed overflow constraints", len(side), count)
			}
			for i, condition := range side {
				op, err := f.m.GetDeclKind(condition)
				equationMust(t, err)
				if op != int32(53+i) {
					t.Fatal("overflow/underflow publication order", op)
				}
			}
		})
	}
}
func TestBV2RealUtilReducedPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	u, f := bv2RealFixture(t, 64)
	out := bv2RealOut(t, f.m)
	var input bytes.Buffer
	count := 0
	for _, a := range []int64{-7, 0, 5} {
		for _, b := range []int64{-3, 0, 4} {
			for _, root := range []int64{2, 3} {
				s := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.signedNumeral(big.NewRat(a, 1)) })
				tv := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.signedNumeral(big.NewRat(b, 1)) })
				equationMust(t, mkReducedRat(u, s, tv, big.NewRat(3, 1), big.NewRat(root, 1), out))
				h, err := f.m.RefGet(out)
				equationMust(t, err)
				input.WriteString("(push)(assert (not (= ")
				equationMust(t, f.r.context.writeExpr(&input, h))
				fmt.Fprintf(&input, " (+ (/ %d.0 3.0) (* (/ %d.0 3.0) (^ %d.0 (/ 1.0 2.0)))))))(check-sat)(pop)\n", a, b, root)
				count++
			}
		}
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader(input.String())
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(raw))
	}
	fields := strings.Fields(string(raw))
	if len(fields) != count {
		t.Fatal("C++ error or surplus result", string(raw))
	}
	for i, status := range fields {
		if status != "unsat" {
			t.Fatal("signed real interpretation", i, string(raw))
		}
	}
}

func TestBV2RealUtilContainsQuantifierMetadataAndBorrowedViews(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	m := f.m
	encoded := bv2RealEncoding(t, u, 1, -2, 2, 2)
	truth, err := m.MkTrue()
	equationMust(t, err)
	fixDLPin(t, m, truth)
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	pattern, err := m.MkPattern([]ast.ASTHandle{encoded})
	equationMust(t, err)
	fixDLPin(t, m, pattern)
	for _, place := range []string{"absent", "body", "pattern", "no-pattern"} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: truth}
		switch place {
		case "body":
			spec.Body = bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.basic(ast.ASTOpEq, encoded, encoded) })
		case "pattern":
			spec.Patterns = []ast.ASTHandle{pattern}
		case "no-pattern":
			spec.NoPatterns = []ast.ASTHandle{encoded}
		}
		q, err := m.MkQuantifier(spec)
		equationMust(t, err)
		fixDLPin(t, m, q)
		contains, err := u.ContainsBV2Real(q)
		equationMust(t, err)
		if contains != (place != "absent") {
			t.Fatal("quantifier traversal", place, contains)
		}
	}
	decls, err := u.AuxDecls()
	equationMust(t, err)
	decls[0] = ast.ASTHandle{}
	again, err := u.AuxDecls()
	equationMust(t, err)
	if again[0].IsNull() {
		t.Fatal("auxiliary view mutated owner")
	}
	equationMust(t, u.AddSideCondition(truth))
	side, err := u.SideConditions()
	equationMust(t, err)
	side[0] = ast.ASTHandle{}
	side, err = u.SideConditions()
	equationMust(t, err)
	if side[0] != truth {
		t.Fatal("side-condition view mutated owner")
	}
}

func TestBV2RealUtilWideDefaultsAndOwnerGuards(t *testing.T) {
	_, f := bv2RealFixture(t, 64)
	u, err := NewBV2RealUtil(f.m, ^uint32(0), ^uint32(0), 64)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, u.Close()) })
	root, d, max, err := defaultsRat(u)
	equationMust(t, err)
	if root.Cmp(big.NewRat(4294967295, 1)) != 0 || d.Cmp(&root) != 0 || max.Cmp(big.NewRat(8589934590, 1)) != 0 {
		t.Fatal("default parameter overflow")
	}
	copy := *u
	if _, _, _, err = defaultsRat(&copy); err == nil {
		t.Fatal("copied utility accepted")
	}
	r, err := NewBV2RealRewriter(u)
	equationMust(t, err)
	rcopy := *r
	if err := rcopy.Reset(); err == nil {
		t.Fatal("copied traversal accepted")
	}
	equationMust(t, r.Close())
	equationMust(t, r.Close())
	if err := r.Reset(); err == nil {
		t.Fatal("closed traversal accepted")
	}
	equationMust(t, u.Close())
	equationMust(t, u.Close())
	if _, _, _, err = defaultsRat(u); err == nil {
		t.Fatal("closed utility accepted")
	}
	var nilUtil *BV2RealUtil
	if _, err := NewBV2RealRewriter(nilUtil); err == nil {
		t.Fatal("nil utility accepted")
	}
}
