package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1362-1406.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVRewriterBV2IntOriginalBranches(t *testing.T) {
	m, qc, _, _ := boolRewriterConfigFixture(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	_, e = m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	c, g, _ := bvPolynomialParamsFixture(t)
	p, e := NewBVRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	xs := bvMergeVars(t, m, 3)
	out, e := m.NewRef(xs[0])
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	n := bvAddNum(t, m, 255, 8)
	st, e := p.RewriteBV2Int(n, out)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	v, e := qc.NumeralsField.NewValue()
	astTestMust(t, e)
	astTestMust(t, qc.NumeralsField.SetInt64(&v, 255))
	want, e := m.ArithmeticNumeral(&v, true)
	astTestMust(t, e)
	astTestMust(t, qc.NumeralsField.Del(&v))
	if st != RewriteDone || got != want {
		t.Fatal("numeral", st, got, want)
	}
	concat, e := m.MkFamilyApp(p.bvFamily, 37, nil, xs)
	astTestMust(t, e)
	astTestPin(t, m, concat)
	st, e = p.RewriteBV2Int(concat, out)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	checkBV2IntFamily(t, m, got, p.arithFamily, 6, 3)
	args, e := m.AppArgumentView(got)
	astTestMust(t, e)
	for i := uint32(0); i < 3; i++ {
		term, e := args.At(i)
		astTestMust(t, e)
		if i < 2 {
			checkBV2IntFamily(t, m, term, p.arithFamily, 9, 2)
			coefficient, e := m.GetArg(term, 0)
			astTestMust(t, e)
			value, e := qc.NumeralsField.NewValue()
			astTestMust(t, e)
			astTestMust(t, qc.NumeralsField.SetInt64(&value, int64(1)<<((2-i)*8)))
			expected, e := m.ArithmeticNumeral(&value, true)
			astTestMust(t, e)
			astTestMust(t, qc.NumeralsField.Del(&value))
			if coefficient != expected {
				t.Fatal("concat coefficient order", i)
			}
			term, e = m.GetArg(term, 1)
			astTestMust(t, e)
		}
		checkBV2IntFamily(t, m, term, p.bvFamily, 58, 1)
		base, e := m.GetArg(term, 0)
		astTestMust(t, e)
		if base != xs[i] {
			t.Fatal("operand order", i)
		}
	}
	if st != Rewrite2 {
		t.Fatal(st)
	}
	for _, tc := range []struct {
		kind   int32
		values []uint32
		status RewriteStatus
		result int32
	}{
		{6, []uint32{3, 7}, Rewrite2, 9}, {6, []uint32{15, 15}, RewriteFailed, 0},
		{4, []uint32{64, 64, 64, 64}, Rewrite2, 6}, {4, []uint32{128, 1}, RewriteFailed, 0},
	} {
		operands := make([]ast.ASTHandle, len(tc.values))
		for i, n := range tc.values {
			operands[i] = bvAddNum(t, m, n, 8)
		}
		arg, e := m.MkFamilyApp(p.bvFamily, tc.kind, nil, operands)
		astTestMust(t, e)
		astTestPin(t, m, arg)
		before, e := m.RefGet(out)
		astTestMust(t, e)
		st, e = p.RewriteBV2Int(arg, out)
		astTestMust(t, e)
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if st != tc.status {
			t.Fatal(tc, st)
		}
		if st == RewriteFailed {
			if got != before {
				t.Fatal("FAILED changed output")
			}
		} else {
			checkBV2IntFamily(t, m, got, p.arithFamily, tc.result, uint32(len(operands)))
		}
	}

	// Source guards also handle zero/one-operand concat metadata.
	for _, children := range [][]ast.ASTHandle{nil, {xs[0]}} {
		sort, e := m.BitVectorSort(8)
		astTestMust(t, e)
		info, e := m.NewFuncDeclInfo(p.bvFamily, 37, nil, ast.ASTFuncFlags{}, false)
		astTestMust(t, e)
		var domains []ast.ASTHandle
		for range children {
			domains = append(domains, sort)
		}
		d, e := m.MkFuncDecl(astTestSymbol(t, m, fmt.Sprint("raw-concat-", len(children))), domains, sort, info)
		astTestMust(t, e)
		astTestMust(t, m.DelFuncDeclInfo(&info))
		h, e := m.MkApp(d, children)
		astTestMust(t, e)
		astTestPin(t, m, h)
		st, e := p.RewriteBV2Int(h, out)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if len(children) == 0 {
			if st != RewriteDone {
				t.Fatal(st)
			}
			checkBV2IntFamily(t, m, got, p.arithFamily, 0, 0)
		} else {
			if st != Rewrite2 {
				t.Fatal(st)
			}
			checkBV2IntFamily(t, m, got, p.bvFamily, 58, 1)
		}
	}
	before, e := m.RefGet(out)
	astTestMust(t, e)
	st, e = p.RewriteBV2Int(xs[1], out)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteFailed || got != before {
		t.Fatal("symbolic FAILED", st)
	}
	st, e = p.RewriteBV2Int(xs[1], ast.ASTRef{})
	astTestMust(t, e)
	if st != RewriteFailed {
		t.Fatal("FAILED must leave output unread")
	}

}

func checkBV2IntFamily(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, family ast.FamilyID, kind int32, count uint32) {
	t.Helper()
	d, e := m.GetDecl(h)
	astTestMust(t, e)
	f, e := m.GetFamilyID(d)
	astTestMust(t, e)
	k, e := m.GetDeclKind(d)
	astTestMust(t, e)
	n, e := m.GetNumArgs(h)
	astTestMust(t, e)
	if f != family || k != kind || n != count {
		t.Fatal("raw expression", f, k, n, family, kind, count)
	}
}

func TestBVRewriterBV2IntEveryColdReservationAndPublication(t *testing.T) {
	for _, mode := range []string{"numeric", "concat", "concat-one", "concat-wide", "mul", "add", "unsafe", "failed"} {
		complete, published := false, false
		for ordinal := int64(0); ordinal < 512 && !complete; ordinal++ {
			t.Run(fmt.Sprintf("%s/%d", mode, ordinal), func(t *testing.T) {
				m, _, _, _ := boolRewriterConfigFixture(t)
				_, e := m.RegisterArithmeticPlugin()
				astTestMust(t, e)
				_, e = m.RegisterBitVectorPlugin()
				astTestMust(t, e)
				c, g, _ := bvPolynomialParamsFixture(t)
				p, e := NewBVRewriter(m, c, g, nil)
				astTestMust(t, e)
				xs := bvMergeVars(t, m, 3)
				arg := xs[0]
				switch mode {
				case "numeric":
					arg = bvAddNum(t, m, 255, 8)
				case "concat", "concat-one":
					if mode == "concat-one" {
						xs = xs[:1]
					}
					arg, e = m.MkFamilyApp(p.bvFamily, 37, nil, xs)
					astTestMust(t, e)
					astTestPin(t, m, arg)
				case "concat-wide":
					sort, e := m.BitVectorSort(65)
					astTestMust(t, e)
					operands := make([]ast.ASTHandle, 3)
					for i := range operands {
						operands[i], e = m.MkVar(uint32(i+500), sort)
						astTestMust(t, e)
					}
					arg, e = m.MkFamilyApp(p.bvFamily, 37, nil, operands)
					astTestMust(t, e)
					astTestPin(t, m, arg)
				case "mul", "unsafe", "add":
					values := []uint32{3, 7}
					kind := int32(6)
					if mode == "unsafe" {
						values = []uint32{15, 15}
					}
					if mode == "add" {
						kind = 4
						values = []uint32{64, 64, 64, 64}
					}
					operands := make([]ast.ASTHandle, len(values))
					for i, n := range values {
						operands[i] = bvAddNum(t, m, n, 8)
					}
					arg, e = m.MkFamilyApp(p.bvFamily, kind, nil, operands)
					astTestMust(t, e)
					astTestPin(t, m, arg)
				}
				sort, e := m.BitVectorSort(8)
				astTestMust(t, e)
				oldArgs := make([]ast.ASTHandle, 32)
				for i := range oldArgs {
					oldArgs[i], e = m.MkVar(uint32(i+100), sort)
					astTestMust(t, e)
				}
				old, e := m.MkBitVectorOr(oldArgs)
				astTestMust(t, e)
				out, e := m.NewRef(old)
				astTestMust(t, e)
				defer func() { astTestMust(t, m.DelRef(&out)) }()
				m.Budget().FailAfter(ordinal)
				st, body := p.RewriteBV2Int(arg, out)
				m.Budget().FailAfter(-1)
				got, e := m.RefGet(out)
				astTestMust(t, e)
				if body == nil {
					complete = true
					nativeResult, nativeErr := m.RefGet(out)
					astTestMust(t, nativeErr)
					if nativeResult != old {
						published = true
					}
					want := Rewrite2
					if mode == "numeric" {
						want = RewriteDone
					}
					if mode == "unsafe" || mode == "failed" {
						want = RewriteFailed
					}
					if st != want {
						t.Fatal("status", st, want)
					}
					if want == RewriteFailed && got != old {
						t.Fatal("FAILED publication")
					}
				} else {
					if !numericHoistRefusal(body) {
						t.Fatal(body)
					}
					if got != old {
						published = true
						if st == RewriteFailed {
							t.Fatal("published without status")
						}
					}
					if _, e = p.RewriteBV2Int(arg, out); e != body {
						t.Fatal("stopped replay", body, e)
					}
				}
				cause := p.cause
				astTestMust(t, p.Close())
				astTestMust(t, p.Close())
				if cause != p.cause {
					t.Fatal("history changed")
				}
			})
		}
		if !complete {
			t.Fatal("cold campaign incomplete", mode)
		}
		if mode != "unsafe" && mode != "failed" && !published {
			t.Fatal("native replacement was not published", mode)
		}
	}
}

func TestBV2IntNativeRootReplacementConsumedOldReference(t *testing.T) {
	m := astTestManager(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	_, e = m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	sort, e := m.BitVectorSort(8)
	astTestMust(t, e)
	children := make([]ast.ASTHandle, 32)
	for i := range children {
		children[i], e = m.MkVar(uint32(i+1000), sort)
		astTestMust(t, e)
	}
	old, e := m.MkBitVectorOr(children)
	astTestMust(t, e)
	replacement, e := m.MkVar(2000, sort)
	astTestMust(t, e)
	var roots []ast.ASTHandle
	astTestMust(t, appendRewriterRoot(m, &roots, old))
	m.Budget().FailAfter(0)
	body := replaceRewriterRoot(m, &roots, 0, replacement)
	m.Budget().FailAfter(-1)
	astTestMust(t, body)
	if len(roots) != 1 || roots[0] != replacement {
		t.Fatal("native replacement did not publish acquired reference")
	}
	if m.IsLiveHandle(old) {
		t.Fatal("native replacement retained old root")
	}
	astTestMust(t, shrinkRewriterRoots(m, &roots, 0))
	astTestMust(t, shrinkRewriterRoots(m, &roots, 0))
}
