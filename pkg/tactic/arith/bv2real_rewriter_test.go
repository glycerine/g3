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
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func bv2RealRewriteFixture(t *testing.T) (*BV2RealRewriter, *BV2RealUtil, *bv2IntFixture) {
	t.Helper()
	u, f := bv2RealFixture(t, 128)
	r, err := NewBV2RealRewriter(u)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r, u, f
}

func bv2RealEncoding(t *testing.T, u *BV2RealUtil, a, b, d, root int64) ast.ASTHandle {
	t.Helper()
	return bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
		return c.encoding(c.signedNumeral(big.NewRat(a, 1)), c.signedNumeral(big.NewRat(b, 1)), big.NewRat(d, 1), big.NewRat(root, 1))
	})
}

func bv2RealRewrite(t *testing.T, r *BV2RealRewriter, h ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	out := bv2RealOut(t, r.utility.manager)
	equationMust(t, r.Rewrite(h, out))
	v, err := r.utility.manager.RefGet(out)
	equationMust(t, err)
	equationMust(t, r.Reset())
	return v
}

func bv2RealWantCoefficients(t *testing.T, u *BV2RealUtil, f *bv2IntFixture, h ast.ASTHandle, a, b, d, root int64) {
	t.Helper()
	x, y := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
	var divisor, radical big.Rat
	ok, err := u.Match(h, x, y, &divisor, &radical)
	equationMust(t, err)
	if !ok || divisor.Cmp(big.NewRat(d, 1)) != 0 || radical.Cmp(big.NewRat(root, 1)) != 0 {
		t.Fatal("missing encoding or wrong divisor/root", ok, &divisor, &radical)
	}
	for i, ref := range []ast.ASTRef{x, y} {
		v, err := f.m.RefGet(ref)
		equationMust(t, err)
		want := a
		if i == 1 {
			want = b
		}
		if got := bv2RealSignedValue(t, f, v); got.Cmp(big.NewInt(want)) != 0 {
			t.Fatal("coefficient", i, got, want)
		}
	}
}

func TestBV2RealRewriterArithmeticIndependentCoefficientOracle(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	for _, a := range []int64{-3, 0, 4} {
		for _, b := range []int64{-2, 0, 3} {
			for _, root := range []int64{2, 3} {
				s := bv2RealEncoding(t, u, a, b, 2, root)
				tv := bv2RealEncoding(t, u, -2, 1, 1, root)
				for _, op := range []int32{6, 7, 9} {
					x, y := a-4, b+2
					if op == 7 {
						x, y = a+4, b-2
					}
					if op == 9 {
						x, y = -2*a+root*b, a-2*b
					}
					h := bv2RealRewrite(t, r, fixDLApp(t, f.m, op, s, tv))
					bv2RealWantCoefficients(t, u, f, h, x, y, 2, root)
				}
				bv2RealWantCoefficients(t, u, f, bv2RealRewrite(t, r, fixDLApp(t, f.m, 8, s)), -a, -b, 2, root)
			}
		}
	}
}

func TestBV2RealRewriterRefusalsAndFailedFoldPrefix(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	s, tv := bv2RealEncoding(t, u, 2, 3, 2, 2), bv2RealEncoding(t, u, 4, -1, 2, 2)
	x := fixDLConst(t, f.m, false, "unencoded")
	out := bv2RealOut(t, f.m)
	for _, op := range []int32{6, 7, 9} {
		h := fixDLApp(t, f.m, op, s, tv, x)
		decl, err := f.m.GetDecl(h)
		equationMust(t, err)
		status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv, x}), out)
		equationMust(t, err)
		if status != rewriter.RewriteFailed {
			t.Fatal("unsupported tail accepted", op, status)
		}
		prefix, err := f.m.RefGet(out)
		equationMust(t, err)
		a, b, d := int64(6), int64(2), int64(2)
		if op == 7 {
			a, b = -2, 4
		}
		if op == 9 {
			a, b, d = 2, 10, 4
		}
		bv2RealWantCoefficients(t, u, f, prefix, a, b, d, 2)
	}
	for _, other := range []ast.ASTHandle{x, bv2RealEncoding(t, u, 1, 1, 2, 3)} {
		input := fixDLApp(t, f.m, 6, s, other)
		if got := bv2RealRewrite(t, r, input); got != input {
			t.Fatal("refusal changed application")
		}
	}
	input := fixDLApp(t, f.m, 10, s, tv)
	if bv2RealRewrite(t, r, input) != input {
		t.Fatal("source division refusal lost")
	}
}

func TestBV2RealRewriterEliminationAndArithmeticPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	r, u, f := bv2RealRewriteFixture(t)
	elim, err := NewBV2RealElimRewriter(u)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, elim.Close()) })
	var input bytes.Buffer
	count := 0
	for _, root := range []int64{2, 3} {
		for _, op := range []int32{6, 7, 9} {
			for _, a := range []int64{-3, 0, 4} {
				s, tv := bv2RealEncoding(t, u, a, -2, 2, root), bv2RealEncoding(t, u, 2, 3, 1, root)
				result := bv2RealRewrite(t, r, fixDLApp(t, f.m, op, s, tv))
				reduced := bv2RealRewrite(t, elim, result)
				contains, err := u.ContainsBV2Real(reduced)
				equationMust(t, err)
				if contains {
					t.Fatal("elimination left encoding")
				}
				operator := map[int32]string{6: "+", 7: "-", 9: "*"}[op]
				input.WriteString("(push)(assert (not (= ")
				equationMust(t, f.r.context.writeExpr(&input, reduced))
				fmt.Fprintf(&input, " (%s (/ (+ %d.0 (* (- 2.0) (^ %d.0 (/ 1.0 2.0)))) 2.0) (+ 2.0 (* 3.0 (^ %d.0 (/ 1.0 2.0))))))))(check-sat)(pop)\n", operator, a, root, root)
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
	statuses := strings.Fields(string(raw))
	if len(statuses) != count {
		t.Fatal("C++ error or surplus output", string(raw))
	}
	for i, status := range statuses {
		if status != "unsat" {
			t.Fatal("arithmetic/elimination mismatch", i, string(raw))
		}
	}
}

func TestBV2RealRewriterEqualityAndConditional(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	s := bv2RealEncoding(t, u, -3, 2, 2, 3)
	for _, tc := range []struct {
		a, b, d int64
		equal   bool
	}{
		{-3, 2, 2, true}, {-6, 4, 4, true}, {-3, 3, 2, false}, {3, 2, 2, false},
	} {
		tv := bv2RealEncoding(t, u, tc.a, tc.b, tc.d, 3)
		eq := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.basic(ast.ASTOpEq, s, tv) })
		f.wantBool(t, bv2RealRewrite(t, r, eq), tc.equal)
	}
	for _, value := range []bool{false, true} {
		cond, err := f.m.MkTrue()
		if !value {
			cond, err = f.m.MkFalse()
		}
		equationMust(t, err)
		tv := bv2RealEncoding(t, u, 5, -1, 1, 3)
		input := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle { return c.basic(ast.ASTOpIte, cond, s, tv) })
		a, b := int64(10), int64(-2)
		if value {
			a, b = -3, 2
		}
		decl, err := f.m.GetDecl(input)
		equationMust(t, err)
		out := bv2RealOut(t, f.m)
		status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{cond, s, tv}), out)
		equationMust(t, err)
		if status != rewriter.RewriteDone {
			t.Fatal("conditional refused", status)
		}
		result, err := f.m.RefGet(out)
		equationMust(t, err)
		bv2RealWantCoefficients(t, u, f, result, a, b, 2, 3)
	}
}

// Source: src/tactic/arith/bv2real_rewriter.h normal and elimination configs;
// src/ast/rewriter/rewriter_def.h:491-514.
func TestBV2RealRewriterPatternTraversalMatchesSourceModes(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	m := f.m
	encoded := bv2RealEncoding(t, u, 1, 1, 2, 2)
	combined := fixDLApp(t, m, 6, encoded, encoded)
	truth, err := m.MkTrue()
	equationMust(t, err)
	fixDLPin(t, m, truth)
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	name := symbol.Numerical(0)
	for _, place := range []string{"no-pattern", "pattern"} {
		spec := ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{name}, Body: truth}
		if place == "pattern" {
			pattern, err := m.MkPattern([]ast.ASTHandle{combined})
			equationMust(t, err)
			fixDLPin(t, m, pattern)
			spec.Patterns = []ast.ASTHandle{pattern}
		} else {
			spec.NoPatterns = []ast.ASTHandle{combined}
		}
		q, err := m.MkQuantifier(spec)
		equationMust(t, err)
		fixDLPin(t, m, q)
		for _, eliminate := range []bool{false, true} {
			var rewrite *BV2RealRewriter
			if eliminate {
				rewrite, err = NewBV2RealElimRewriter(u)
			} else {
				rewrite, err = NewBV2RealRewriter(u)
			}
			equationMust(t, err)
			if got, want := rewrite.cfg.RewritePatterns(), !eliminate; got != want {
				t.Fatal("rewriter pattern policy", eliminate, got, want)
			}
			t.Cleanup(func() { equationMust(t, rewrite.Close()) })
			beforeDecls, err := u.AuxDecls()
			equationMust(t, err)
			out := bv2RealOut(t, m)
			equationMust(t, rewrite.Rewrite(q, out))
			afterDecls, err := u.AuxDecls()
			equationMust(t, err)
			if place == "no-pattern" {
				if eliminate && len(afterDecls) != len(beforeDecls) {
					t.Fatal("elimination config traversed no-pattern declarations", len(beforeDecls), len(afterDecls))
				}
				if !eliminate && len(afterDecls) <= len(beforeDecls) {
					t.Fatal("normal config did not visit and construct the no-pattern expression", len(beforeDecls), len(afterDecls))
				}
			}
			got, err := m.RefGet(out)
			equationMust(t, err)
			annotation := ast.ASTHandle{}
			if place == "pattern" {
				annotation, err = m.GetPattern(got, 0)
			} else {
				annotation, err = m.GetNoPattern(got, 0)
			}
			equationMust(t, err)
			annotationExpr := annotation
			if place == "pattern" {
				annotationExpr, err = m.GetArg(annotation, 0)
				equationMust(t, err)
			}
			actualDecl, err := m.GetDecl(annotationExpr)
			equationMust(t, err)
			if eliminate || place == "no-pattern" {
				wantDecl, err := m.GetDecl(combined)
				equationMust(t, err)
				if actualDecl != wantDecl {
					t.Fatalf("%s elimination preserved decl %v, want %v", place, actualDecl, wantDecl)
				}
			} else {
				isEncoding, err := u.IsEncodingDecl(actualDecl)
				equationMust(t, err)
				if !isEncoding {
					t.Fatalf("%s normal rewrite did not process the encoded annotation: actual=%v", place, actualDecl)
				}
			}
			equationMust(t, rewrite.Close())
		}
	}
}

func TestBV2RealRewriterPositivePredicatesAndStrictStatus(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	lt, err := u.PositiveLTDecl()
	equationMust(t, err)
	le, err := u.PositiveLEDecl()
	equationMust(t, err)
	out := bv2RealOut(t, f.m)
	for _, root := range []int64{2, 3} {
		for _, decl := range []ast.ASTHandle{lt, le} {
			equationMust(t, u.Reset())
			s, tv := bv2RealEncoding(t, u, -2, 3, 2, root), bv2RealEncoding(t, u, 1, 1, 2, root)
			status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), out)
			equationMust(t, err)
			side, err := u.SideConditions()
			equationMust(t, err)
			if root == 3 {
				if status != rewriter.RewriteFailed || len(side) != 0 {
					t.Fatal("positive approximation accepted other root")
				}
				continue
			}
			if status != rewriter.RewriteDone || len(side) != 2 {
				t.Fatal("one-polarity constraints", status, len(side))
			}
			h, err := f.m.RefGet(out)
			equationMust(t, err)
			kind, err := f.m.GetDeclKind(h)
			equationMust(t, err)
			if decl == lt && kind != ast.ASTOpNot {
				t.Fatal("positive strict comparison did not negate swapped proxy")
			}
		}
	}
	for _, op := range []int32{4, 5} {
		s, tv := fixDLConst(t, f.m, false, "s"), fixDLConst(t, f.m, false, "t")
		input := fixDLApp(t, f.m, op, s, tv)
		decl, err := f.m.GetDecl(input)
		equationMust(t, err)
		status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), out)
		equationMust(t, err)
		if status != rewriter.Rewrite2 {
			t.Fatal("strict comparison revisit status", status)
		}
		result, err := f.m.RefGet(out)
		equationMust(t, err)
		kind, err := f.m.GetDeclKind(result)
		equationMust(t, err)
		if kind != ast.ASTOpNot {
			t.Fatal("strict comparison polarity")
		}
	}
}

func TestBV2RealRewriterComparisonsPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	r, u, f := bv2RealRewriteFixture(t)
	var script bytes.Buffer
	var expected []string
	for _, root := range []int64{2, 3} {
		for _, op := range []int32{2, 3, 4, 5} {
			for _, a := range []int64{-5, 0, 5} {
				for _, b := range []int64{-3, 0, 3} {
					equationMust(t, u.Reset())
					before, err := u.AuxDecls()
					equationMust(t, err)
					s, tv := bv2RealEncoding(t, u, a, b, 2, root), bv2RealEncoding(t, u, -2, 1, 1, root)
					result := bv2RealRewrite(t, r, fixDLApp(t, f.m, op, s, tv))
					after, err := u.AuxDecls()
					equationMust(t, err)
					// Root two uses a fresh proxy with four ordered approximation
					// constraints. Root three takes the exact squared comparison.
					side, err := u.SideConditions()
					equationMust(t, err)
					want := 0
					if root == 2 {
						want = 4
					}
					if len(side) != want {
						t.Fatal("comparison side conditions", root, len(side), want)
					}
					script.WriteString("(push)\n")
					if root == 2 {
						if len(after) < len(before)+1 {
							t.Fatal("missing auxiliary proxy")
						}
						proxy, err := f.m.MkApp(after[len(after)-1], nil)
						equationMust(t, err)
						fixDLPin(t, f.m, proxy)
						script.WriteString("(declare-fun ")
						equationMust(t, f.r.context.writeExpr(&script, proxy))
						script.WriteString(" () Bool)\n")
					}
					for _, condition := range side {
						script.WriteString("(assert ")
						equationMust(t, f.r.context.writeExpr(&script, condition))
						script.WriteString(")\n")
					}
					expect := "sat"
					if root == 2 {
						av, bv := a+4, b-2
						if op == 3 || op == 4 {
							av, bv = -av, -bv
						}
						canTrue, canFalse := 2*av+3*bv <= 0, 4*av+5*bv > 0
						if bv <= 0 {
							canTrue, canFalse = 4*av+5*bv <= 0, 2*av+3*bv > 0
						}
						if !canTrue && !canFalse {
							expect = "unsat"
						}
					}
					script.WriteString("(check-sat)\n")
					expected = append(expected, expect, "unsat")
					script.WriteString("(assert (not (= ")
					equationMust(t, f.r.context.writeExpr(&script, result))
					operator := map[int32]string{2: "<=", 3: ">=", 4: "<", 5: ">"}[op]
					fmt.Fprintf(&script, " (%s (/ (+ %d.0 (* %d.0 (^ %d.0 (/ 1.0 2.0)))) 2.0) (+ (- 2.0) (^ %d.0 (/ 1.0 2.0)))))))(check-sat)(pop)\n", operator, a, b, root, root)
				}
			}
		}
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader(script.String())
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(raw))
	}
	statuses := strings.Fields(string(raw))
	if len(statuses) != len(expected) {
		t.Fatal("C++ error or surplus output", string(raw))
	}
	for i, status := range statuses {
		if status != expected[i] {
			t.Fatal("comparison feasibility or polarity", i, expected[i], string(raw))
		}
	}
}
