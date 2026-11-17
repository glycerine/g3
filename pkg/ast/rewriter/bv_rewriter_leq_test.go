package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:436-584.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVRewriterLEOriginalBranchesAndStatuses(t *testing.T) {
	m, _ := bvAddFixture(t)
	p := bvNotTestOwner(t, m, false, false)
	xs := bvMergeVars(t, m, 3)
	x, y, z := xs[0], xs[1], xs[2]
	n := func(v uint32, w uint32) ast.ASTHandle { return bvAddNum(t, m, v, w) }
	eq := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkEq(a, b)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	cmp := func(s bool, a, b ast.ASTHandle) ast.ASTHandle {
		var h ast.ASTHandle
		var e error
		if s {
			h, e = m.MkBitVectorSLE(a, b)
		} else {
			h, e = m.MkBitVectorULE(a, b)
		}
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	cat := func(hs ...ast.ASTHandle) ast.ASTHandle { return bvPropagationConcat(t, m, hs...) }
	ex := func(hi, lo uint32, a ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkBitVectorExtract(hi, lo, a)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	and := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkAndArgs([]ast.ASTHandle{a, b})
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	yes, e := m.MkTrue()
	astTestMust(t, e)
	astTestPin(t, m, yes)
	no, e := m.MkFalse()
	astTestMust(t, e)
	astTestPin(t, m, no)
	rem := bvPropagationOp(t, m, p, 19, x, n(4, 8))
	minus := bvPropagationOp(t, m, p, 6, n(255, 8), rem)
	sum := bvPropagationOp(t, m, p, 4, x, minus)
	for _, tc := range []struct {
		name       string
		signed     bool
		a, b, want ast.ASTHandle
		st         RewriteStatus
	}{
		{"identity", false, x, x, yes, RewriteDone},
		{"unsigned-numeric", false, n(255, 8), n(0, 8), no, RewriteDone},
		{"signed-numeric", true, n(255, 8), n(0, 8), yes, RewriteDone},
		{"right-lower", false, x, n(0, 8), eq(x, n(0, 8)), Rewrite1},
		{"right-upper", false, x, n(255, 8), yes, RewriteDone},
		{"left-lower", false, n(0, 8), x, yes, RewriteDone},
		{"left-upper", false, n(255, 8), x, eq(n(255, 8), x), Rewrite1},
		{"signed-right-lower", true, x, n(128, 8), eq(x, n(128, 8)), Rewrite1},
		{"signed-right-upper", true, x, n(127, 8), yes, RewriteDone},
		{"signed-left-lower", true, n(128, 8), x, yes, RewriteDone},
		{"signed-left-upper", true, n(127, 8), x, eq(n(127, 8), x), Rewrite1},
		{"signed-remainder", true, sum, n(8, 8), cmp(true, x, n(11, 8)), Rewrite2},
		{"unknown-unsigned", false, x, y, ast.ASTHandle{}, RewriteFailed},
		{"unknown-signed", true, x, y, ast.ASTHandle{}, RewriteFailed},
		{"all-zero-concat", false, x, cat(n(0, 4), n(0, 4)), eq(x, n(0, 8)), Rewrite1},
		{"leading-zero-numeric", false, x, n(15, 8), and(eq(ex(7, 4, x), n(0, 4)), cmp(false, ex(3, 0, x), ex(3, 0, n(15, 8)))), Rewrite3},
		{"leading-zero-concat", false, x, cat(n(0, 4), bvPropagationVar(t, m, 9, 4)), and(eq(ex(7, 4, x), n(0, 4)), cmp(false, ex(3, 0, x), ex(3, 0, cat(n(0, 4), bvPropagationVar(t, m, 9, 4))))), Rewrite3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, e := m.NewRef(z)
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			st, e := p.RewriteLE(tc.signed, tc.a, tc.b, out)
			astTestMust(t, e)
			got, e := m.RefGet(out)
			astTestMust(t, e)
			want := tc.want
			if tc.st == RewriteFailed {
				want = z
			}
			if st != tc.st || got != want || p.leq != nil {
				t.Fatal("source LE", st, tc.st, got, want)
			}
		})
	}
}

func TestBVRewriterLESignedRemainderGuardsAndExtraHelpers(t *testing.T) {
	m, _ := bvAddFixture(t)
	p := bvNotTestOwner(t, m, false, false)
	xs := bvMergeVars(t, m, 2)
	x, y := xs[0], xs[1]
	for _, tc := range []struct {
		name                       string
		divisor, right, multiplier uint32
		kind                       int32
		matched                    bool
	}{
		{"nondivisible", 4, 9, 255, 19, false},
		{"nonpositive-divisor", 252, 8, 255, 19, false},
		{"nonpositive-right", 4, 254, 255, 19, false},
		{"range-failure", 64, 64, 255, 19, true},
		{"range-at-power", 43, 86, 255, 19, false},
		{"not-minus-one", 4, 8, 254, 19, false},
		{"external-remainder", 4, 8, 255, 9, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rem := bvPropagationOp(t, m, p, tc.kind, y, bvAddNum(t, m, tc.divisor, 8))
			mul := bvPropagationOp(t, m, p, 6, bvAddNum(t, m, tc.multiplier, 8), rem)
			a := bvPropagationOp(t, m, p, 4, x, mul)
			b := bvAddNum(t, m, tc.right, 8)
			out, e := m.NewRef(y)
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			st, e := p.RewriteLE(true, a, b, out)
			astTestMust(t, e)
			got, e := m.RefGet(out)
			astTestMust(t, e)
			if tc.matched {
				// The source does not compare the remainder dividend against x.
				child, e := m.GetArg(got, 0)
				astTestMust(t, e)
				if st != Rewrite2 || child != x {
					t.Fatal("source guard/result", st)
				}
			} else if st != RewriteFailed || got != y {
				t.Fatal("source failed predicate", st)
			}
		})
	}
	p.polynomial.config.bitVector.LEExtra = true
	a := bvPropagationConcat(t, m, x, x)
	b := bvPropagationConcat(t, m, x, y)
	out, e := m.NewRef(y)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteLE(true, a, b, out)
	astTestMust(t, e)
	want, e := m.MkBitVectorULE(x, y)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != Rewrite2 || got != want {
		t.Fatal("concat helper ordering", st)
	}
	a = bvPropagationOp(t, m, p, 4, bvAddNum(t, m, 2, 8), x)
	b = bvPropagationOp(t, m, p, 4, bvAddNum(t, m, 5, 8), x)
	st, e = p.RewriteLE(false, a, b, out)
	astTestMust(t, e)
	want, e = m.MkBitVectorULE(bvAddNum(t, m, 3, 8), b)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if st != Rewrite2 || got != want || p.leConcat != nil || p.leOverflow != nil || p.leq != nil {
		t.Fatal("overflow helper ordering", st)
	}
}

func TestBVRewriterLEEveryColdReservationAndPublication(t *testing.T) {
	for _, mode := range []string{"identity", "numeric", "bounds", "remainder", "leading", "zero", "concat-helper", "overflow-helper", "failed"} {
		complete, published := false, false
		for ordinal := int64(0); ordinal < 4096 && !complete; ordinal++ {
			t.Run(fmt.Sprintf("%s/%d", mode, ordinal), func(t *testing.T) {
				m, _ := bvAddFixture(t)
				p := bvNotTestOwner(t, m, false, false)
				xs := bvMergeVars(t, m, 2)
				x, y := xs[0], xs[1]
				a, b := x, y
				signed := false
				want := RewriteDone
				switch mode {
				case "identity":
					b = a
				case "numeric":
					a, b = bvAddNum(t, m, 255, 8), bvAddNum(t, m, 3, 8)
				case "bounds":
					b = bvAddNum(t, m, 128, 8)
					signed = true
					want = Rewrite1
				case "remainder":
					rem := bvPropagationOp(t, m, p, 19, x, bvAddNum(t, m, 4, 8))
					mul := bvPropagationOp(t, m, p, 6, bvAddNum(t, m, 255, 8), rem)
					a = bvPropagationOp(t, m, p, 4, x, mul)
					b = bvAddNum(t, m, 8, 8)
					signed = true
					want = Rewrite2
				case "leading":
					b = bvAddNum(t, m, 15, 8)
					want = Rewrite3
				case "zero":
					b = bvPropagationConcat(t, m, bvAddNum(t, m, 0, 4), bvAddNum(t, m, 0, 4))
					want = Rewrite1
				case "concat-helper":
					p.polynomial.config.bitVector.LEExtra = true
					a, b = bvPropagationConcat(t, m, x, x), bvPropagationConcat(t, m, x, y)
					signed = true
					want = Rewrite2
				case "overflow-helper":
					p.polynomial.config.bitVector.LEExtra = true
					a = bvPropagationOp(t, m, p, 4, bvAddNum(t, m, 9, 8), x)
					b = bvPropagationOp(t, m, p, 4, bvAddNum(t, m, 5, 8), x)
					want = Rewrite2
				case "failed":
					want = RewriteFailed
				}
				sort, e := m.BitVectorSort(8)
				astTestMust(t, e)
				var oldArgs [32]ast.ASTHandle
				for i := range oldArgs {
					oldArgs[i], e = m.MkVar(uint32(i+100), sort)
					astTestMust(t, e)
				}
				old, e := m.MkBitVectorOr(oldArgs[:])
				astTestMust(t, e)
				out, e := m.NewRef(old)
				astTestMust(t, e)
				defer func() { astTestMust(t, m.DelRef(&out)) }()
				m.Budget().FailAfter(ordinal)
				st, body := p.RewriteLE(signed, a, b, out)
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
					if st != want || p.leq != nil || p.leConcat != nil || p.leOverflow != nil || want == RewriteFailed && got != old {
						t.Fatal("source return", st, want)
					}
				} else {
					if !numericHoistRefusal(body) {
						t.Fatal(body)
					}
					if got != old {
						published = true
						if st != want {
							t.Fatal("publication status", st, want)
						}
					}
					_, replay := p.RewriteLE(signed, a, b, out)
					if replay != body {
						t.Fatal("stopped replay", replay, body)
					}
				}
				cause := p.cause
				astTestMust(t, p.Close())
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("history changed")
				}
			})
		}
		if !complete || mode != "failed" && !published {
			t.Fatal("cold LE incomplete", mode, complete, published)
		}
	}
}

// Quantifier instances repeatedly compare symbolic bit-vector terms.
func TestBVRewriterSymbolicLEAllocations(t *testing.T) {
	m, _ := bvAddFixture(t)
	p := bvNotTestOwner(t, m, false, false)
	x := bvMergeVars(t, m, 2)
	out, err := m.NewRef(x[0])
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	for _, signed := range []bool{false, true} {
		allocs := testing.AllocsPerRun(100, func() {
			status, err := p.RewriteLE(signed, x[0], x[1], out)
			if err != nil || status != RewriteFailed {
				t.Fatalf("symbolic comparison returned %v, %v", status, err)
			}
		})
		if allocs != 0 {
			t.Errorf("signed=%v: inapplicable comparison allocated %.0f times", signed, allocs)
		}
	}
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got != x[0] {
		t.Fatal("failed rewrite changed the output")
	}
}
