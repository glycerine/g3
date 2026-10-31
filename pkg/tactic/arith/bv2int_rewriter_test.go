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
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type bv2IntFixture struct {
	r  *BV2IntRewriter
	m  *ast.ASTManager
	ev *model.ModelEvaluator
}

func bv2IntRewriteFixture(t *testing.T, max uint32) *bv2IntFixture {
	t.Helper()
	base, m := fixDLFixture(t, false)
	p, err := base.context.NewValue()
	equationMust(t, err)
	key := "max_bv_size"
	equationMust(t, base.context.SetUIntName(&p, &key, max))
	ctx, err := NewBV2IntContext(m, base.context, &p)
	equationMust(t, err)
	equationMust(t, base.context.Del(&p))
	t.Cleanup(func() { equationMust(t, ctx.Close()) })
	r, err := NewBV2IntRewriter(ctx)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	ev, err := model.NewModelEvaluator(&md.CoreField, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, ev.Close()) })
	return &bv2IntFixture{r, m, ev}
}
func (f *bv2IntFixture) bv(t *testing.T, n uint32, width uint32) ast.ASTHandle {
	t.Helper()
	h, err := f.m.BitVectorNumeralUnsigned(n, width)
	equationMust(t, err)
	fixDLPin(t, f.m, h)
	return h
}
func (f *bv2IntFixture) integer(t *testing.T, n uint32, width uint32) ast.ASTHandle {
	t.Helper()
	h, err := f.m.MkBitVectorToInt(f.bv(t, n, width))
	equationMust(t, err)
	fixDLPin(t, f.m, h)
	return h
}
func (f *bv2IntFixture) rewrite(t *testing.T, h ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, f.m.DelRef(&out)) })
	equationMust(t, f.r.Rewrite(h, out))
	v, err := f.m.RefGet(out)
	equationMust(t, err)
	equationMust(t, f.r.Reset())
	return v
}
func (f *bv2IntFixture) value(t *testing.T, h ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	ref, err := f.ev.EvaluateValue(h)
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&ref)) }()
	v, err := f.m.RefGet(ref)
	equationMust(t, err)
	fixDLPin(t, f.m, v)
	return v
}
func (f *bv2IntFixture) wantInt(t *testing.T, h ast.ASTHandle, n int64) {
	t.Helper()
	v := f.value(t, h)
	text, integer, ok, err := f.m.ArithmeticNumeralText(v)
	equationMust(t, err)
	q, valid := new(big.Rat).SetString(text)
	if !ok || !integer || !valid || q.Cmp(big.NewRat(n, 1)) != 0 {
		t.Fatalf("integer evaluation %q, want %d", text, n)
	}
}
func (f *bv2IntFixture) wantBool(t *testing.T, h ast.ASTHandle, b bool) {
	t.Helper()
	v := f.value(t, h)
	if f.m.IsTrue(v) != b || f.m.IsFalse(v) == b {
		t.Fatalf("Boolean evaluation differs, want %v", b)
	}
}
func TestBV2IntArithmeticIndependentIntegerOracle(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	for mode := 0; mode < 3; mode++ {
		for a := uint32(0); a < 8; a++ {
			for b := uint32(0); b < 8; b++ {
				lhs, rhs := f.integer(t, a, 3), f.integer(t, b, 4)
				x, y := int64(a), int64(b)
				if mode == 1 {
					lhs = fixDLApp(t, f.m, 7, lhs, fixDLNumber(t, f.m, true, "9"))
					x -= 9
				}
				if mode == 2 {
					rhs = fixDLNumber(t, f.m, true, fmt.Sprint(-int64(b)-1))
					y = -int64(b) - 1
				}
				for _, op := range []int32{2, 3, 4, 5, 6, 7, 9} {
					input := fixDLApp(t, f.m, op, lhs, rhs)
					output := f.rewrite(t, input)
					switch op {
					case 2:
						f.wantBool(t, output, x <= y)
					case 3:
						f.wantBool(t, output, x >= y)
					case 4:
						f.wantBool(t, output, x < y)
					case 5:
						f.wantBool(t, output, x > y)
					case 6:
						f.wantInt(t, output, x+y)
					case 7:
						f.wantInt(t, output, x-y)
					case 9:
						f.wantInt(t, output, x*y)
					}
				}
				eq, err := f.m.MkEq(lhs, rhs)
				equationMust(t, err)
				fixDLPin(t, f.m, eq)
				f.wantBool(t, f.rewrite(t, eq), x == y)
				neg := fixDLApp(t, f.m, 8, lhs)
				f.wantInt(t, f.rewrite(t, neg), -x)
			}
		}
	}
}
func TestBV2IntUnsignedModAndNaryArithmetic(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	for a := uint32(0); a < 16; a++ {
		for b := uint32(1); b < 8; b++ {
			s, tv := f.integer(t, a, 4), f.integer(t, b, 3)
			f.wantInt(t, f.rewrite(t, fixDLApp(t, f.m, 13, s, tv)), int64(a%b))
		}
	}
	for _, op := range []int32{6, 7, 9} {
		terms := []ast.ASTHandle{f.integer(t, 7, 3), f.integer(t, 3, 2), f.integer(t, 2, 2)}
		want := int64(12)
		if op == 7 {
			want = 2
		}
		if op == 9 {
			want = 42
		}
		f.wantInt(t, f.rewrite(t, fixDLApp(t, f.m, op, terms...)), want)
	}
}
func TestBV2IntMultiplicationWidthLimitAndSideConditions(t *testing.T) {
	for _, max := range []uint32{0, 2, 3, 4, 5, 6, 7} {
		t.Run(fmt.Sprint(max), func(t *testing.T) {
			f := bv2IntRewriteFixture(t, max)
			// Non-numeral bv2int leaves exercise real widening rather than constant identity paths.
			s, tv := f.integer(t, 7, 3), f.integer(t, 6, 3)
			output := f.rewrite(t, fixDLApp(t, f.m, 9, s, tv))
			bits, err := f.m.GetArg(output, 0)
			equationMust(t, err)
			width, err := f.m.BitVectorSize(bits)
			equationMust(t, err)
			want := uint32(6)
			if max < 6 {
				want = max
			}
			if max <= 3 {
				want = 3
			}
			if width != want {
				t.Fatal("product width", width, want)
			}
			side, err := f.r.context.SideConditions()
			equationMust(t, err)
			count := 0
			if max < 6 {
				count = 1
			}
			if len(side) != count {
				t.Fatal("overflow side condition count", len(side), count)
			}
			if count == 0 {
				f.wantInt(t, output, 42)
			} else {
				f.wantBool(t, side[0], false)
			}
		})
	}
}
func TestBV2IntDirectRefusalFallbackAndPartialFold(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	x := fixDLConst(t, f.m, true, "unbounded")
	for _, op := range []int32{10, 11, 12} {
		h := fixDLApp(t, f.m, op, f.integer(t, 7, 3), f.integer(t, 2, 2))
		d, err := f.m.GetDecl(h)
		equationMust(t, err)
		args := ast.SliceArguments([]ast.ASTHandle{f.integer(t, 7, 3), f.integer(t, 2, 2)})
		status, err := f.r.MkAppCore(d, args, out)
		equationMust(t, err)
		if status != rewriter.RewriteFailed {
			t.Fatal("source TBD operation rewritten", op, status)
		}
		equationMust(t, f.r.MkApp(d, args, out))
		got, err := f.m.RefGet(out)
		equationMust(t, err)
		if got != h {
			t.Fatal("fallback changed unreduced application")
		}
	}
	a, b := f.integer(t, 2, 2), f.integer(t, 3, 2)
	h := fixDLApp(t, f.m, 6, a, b, x)
	d, err := f.m.GetDecl(h)
	equationMust(t, err)
	status, err := f.r.MkAppCore(d, ast.SliceArguments([]ast.ASTHandle{a, b, x}), out)
	equationMust(t, err)
	if status != rewriter.RewriteFailed {
		t.Fatal("unsupported fold tail accepted")
	}
	prefix, err := f.m.RefGet(out)
	equationMust(t, err)
	f.wantInt(t, prefix, 5)
}

func TestBV2IntUnsupportedArithmeticSurfaceFallsBack(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	integerX, integerY := fixDLConst(t, f.m, true, "unsupported-int-x"), fixDLConst(t, f.m, true, "unsupported-int-y")
	realX, realY := fixDLConst(t, f.m, false, "unsupported-real-x"), fixDLConst(t, f.m, false, "unsupported-real-y")
	forms := []struct {
		name string
		expr ast.ASTHandle
	}{
		{name: "numeral", expr: fixDLNumber(t, f.m, true, "7")},
		{name: "div", expr: fixDLApp(t, f.m, 10, realX, realY)},
		{name: "idiv", expr: fixDLApp(t, f.m, 11, integerX, integerY)},
		{name: "rem", expr: fixDLApp(t, f.m, 12, integerX, integerY)},
		{name: "to-real", expr: fixDLApp(t, f.m, 14, integerX)},
		{name: "to-int", expr: fixDLApp(t, f.m, 15, realX)},
		{name: "is-int", expr: fixDLApp(t, f.m, 16, realX)},
	}
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	for _, test := range forms {
		t.Run(test.name, func(t *testing.T) {
			decl, getErr := f.m.GetDecl(test.expr)
			equationMust(t, getErr)
			count, getErr := f.m.GetNumArgs(test.expr)
			equationMust(t, getErr)
			args := make([]ast.ASTHandle, count)
			for i := range args {
				args[i], getErr = f.m.GetArg(test.expr, uint32(i))
				equationMust(t, getErr)
			}
			equationMust(t, f.m.SetRef(out, ast.ASTHandle{}))
			status, coreErr := f.r.MkAppCore(decl, ast.SliceArguments(args), out)
			equationMust(t, coreErr)
			if status != rewriter.RewriteFailed {
				t.Fatalf("source-declined operator status %v", status)
			}
			unchanged, getErr := f.m.RefGet(out)
			equationMust(t, getErr)
			if !unchanged.IsNull() {
				t.Fatal("failed rewrite published a result")
			}
			equationMust(t, f.r.MkApp(decl, ast.SliceArguments(args), out))
			got, getErr := f.m.RefGet(out)
			equationMust(t, getErr)
			if got != test.expr {
				t.Fatal("fallback changed a source-declined expression")
			}
		})
	}
}

func TestBV2IntCallCleanupSkipsAlreadyConsumedReferences(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	root := fixDLConst(t, f.m, true, "cleanup-consumed")
	ref, err := f.m.NewRef(root)
	equationMust(t, err)
	call := &bv2IntCall{r: f.r, refs: []ast.ASTRef{{}, ref}}
	if err := call.close(); err != nil {
		t.Fatalf("initial cleanup failed: %v", err)
	}
	if err := call.close(); err != nil {
		t.Fatalf("retry after a consumed reference returned an error: %v", err)
	}
}

func TestBV2IntClosedRewritesPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	f := bv2IntRewriteFixture(t, ^uint32(0))
	var script bytes.Buffer
	count := 0
	for _, a := range []uint32{0, 1, 3, 7} {
		for _, b := range []uint32{1, 2, 5} {
			for _, op := range []int32{2, 3, 4, 5, 6, 7, 9, 13} {
				input := fixDLApp(t, f.m, op, f.integer(t, a, 3), f.integer(t, b, 4))
				output := f.rewrite(t, input)
				// C++ checks equivalence of the actual generated expression to the original.
				script.WriteString("(push)(assert (not (= ")
				equationMust(t, f.r.context.writeExpr(&script, input))
				script.WriteByte(' ')
				equationMust(t, f.r.context.writeExpr(&script, output))
				script.WriteString(")))(check-sat)(pop)\n")
				count++
			}
		}
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader(script.String())
	raw, err := cmd.CombinedOutput()
	equationMust(t, err)
	fields := strings.Fields(string(raw))
	if len(fields) != count {
		t.Fatalf("C++ output count %d want %d: %s", len(fields), count, raw)
	}
	for i, status := range fields {
		if status != "unsat" {
			t.Fatalf("C++ equivalence %d: %s", i, raw)
		}
	}
}

func (f *bv2IntFixture) signedInput(t *testing.T, bits uint32, width uint32) ast.ASTHandle {
	t.Helper()
	return f.signedTerm(t, f.bv(t, bits, width), width)
}
func (f *bv2IntFixture) signedTerm(t *testing.T, b ast.ASTHandle, width uint32) ast.ASTHandle {
	t.Helper()
	m := f.m
	sign, err := m.MkBitVectorExtract(width-1, width-1, b)
	equationMust(t, err)
	fixDLPin(t, m, sign)
	cond, err := m.MkEq(f.bv(t, 1, 1), sign)
	equationMust(t, err)
	fixDLPin(t, m, cond)
	low, err := m.MkBitVectorExtract(width-2, 0, b)
	equationMust(t, err)
	fixDLPin(t, m, low)
	e, err := m.MkBitVectorToInt(low)
	equationMust(t, err)
	fixDLPin(t, m, e)
	subtract := fixDLApp(t, m, 7, e, fixDLNumber(t, m, true, fmt.Sprint(uint32(1)<<(width-1))))
	h, err := m.MkIte(cond, subtract, e)
	equationMust(t, err)
	fixDLPin(t, m, h)
	return h
}
func TestBV2IntSignedEncodingAndArithmetic(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	for a := uint32(0); a < 8; a++ {
		for b := uint32(0); b < 16; b++ {
			s, tv := f.signedInput(t, a, 3), f.signedInput(t, b, 4)
			x, y := int64(a), int64(b)
			if b >= 8 {
				y -= 16
			}
			if a >= 4 {
				x -= 8
			}
			for _, op := range []int32{2, 3, 4, 5, 6, 7, 9} {
				output := f.rewrite(t, fixDLApp(t, f.m, op, s, tv))
				switch op {
				case 2:
					f.wantBool(t, output, x <= y)
				case 3:
					f.wantBool(t, output, x >= y)
				case 4:
					f.wantBool(t, output, x < y)
				case 5:
					f.wantBool(t, output, x > y)
				case 6:
					f.wantInt(t, output, x+y)
				case 7:
					f.wantInt(t, output, x-y)
				case 9:
					f.wantInt(t, output, x*y)
				}
			}
			eq, err := f.m.MkEq(s, tv)
			equationMust(t, err)
			fixDLPin(t, f.m, eq)
			f.wantBool(t, f.rewrite(t, eq), x == y)
			for _, choose := range []bool{false, true} {
				cond, err := f.m.MkTrue()
				if !choose {
					cond, err = f.m.MkFalse()
				}
				equationMust(t, err)
				h, err := f.m.MkIte(cond, s, tv)
				equationMust(t, err)
				fixDLPin(t, f.m, h)
				want := y
				if choose {
					want = x
				}
				f.wantInt(t, f.rewrite(t, h), want)
			}
		}
	}
}
func TestBV2IntShiftProductAndDistinct(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	for n := uint32(0); n < 3; n++ {
		for v := uint32(0); v < 8; v++ {
			shl, err := f.m.MkBitVectorSHL(f.bv(t, 1, 3), f.bv(t, n, 3))
			equationMust(t, err)
			fixDLPin(t, f.m, shl)
			power, err := f.m.MkBitVectorToInt(shl)
			equationMust(t, err)
			fixDLPin(t, f.m, power)
			term := f.integer(t, v, 4)
			for _, swapped := range []bool{false, true} {
				a, b := power, term
				if swapped {
					a, b = b, a
				}
				f.wantInt(t, f.rewrite(t, fixDLApp(t, f.m, 9, a, b)), int64(v<<n))
			}
		}
	}
	for _, values := range [][]uint32{{1, 2}, {1, 1}, {0, 1, 2}, {0, 1, 0}} {
		var args []ast.ASTHandle
		seen := map[uint32]bool{}
		different := true
		for _, v := range values {
			args = append(args, f.integer(t, v, 3))
			if seen[v] {
				different = false
			}
			seen[v] = true
		}
		h, err := f.m.MkFamilyApp(f.m.BasicFamilyID(), ast.ASTOpDistinct, nil, args)
		equationMust(t, err)
		fixDLPin(t, f.m, h)
		f.wantBool(t, f.rewrite(t, h), different)
	}
}
func TestBV2IntSourceModDifferenceAndSignedMinimumQuirks(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	// Source cpp:246 computes s1%t1, not s2%t1. Preserve the actual source tree.
	input := fixDLApp(t, f.m, 13, fixDLApp(t, f.m, 7, f.integer(t, 3, 3), f.integer(t, 2, 3)), fixDLNumber(t, f.m, true, "5"))
	f.wantInt(t, input, 1)
	f.wantInt(t, f.rewrite(t, input), 0)
	// Source signed unary minus negates at the original width, without widening.
	input = fixDLApp(t, f.m, 8, f.signedInput(t, 4, 3))
	f.wantInt(t, input, 4)
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	decl, err := f.m.GetDecl(input)
	equationMust(t, err)
	arg, err := f.m.GetArg(input, 0)
	equationMust(t, err)
	status, err := f.r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{arg}), out)
	equationMust(t, err)
	if status != rewriter.RewriteDone {
		t.Fatal("signed unary minus source rule refused")
	}
	raw, err := f.m.RefGet(out)
	equationMust(t, err)
	f.wantInt(t, raw, -4)
	// Bottom-up traversal rewrites the ITE's branches first and avoids this raw-rule overflow.
	f.wantInt(t, f.rewrite(t, input), 4)
	// Unsigned BV remainder-by-zero follows the source BV operator behavior.
	input = fixDLApp(t, f.m, 13, f.integer(t, 3, 3), fixDLNumber(t, f.m, true, "0"))
	f.wantInt(t, f.rewrite(t, input), 3)
}
func TestBV2IntTraversalProofDisabledAndContextBorrowed(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	equationMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	h := fixDLApp(t, f.m, 2, f.integer(t, 3, 2), f.integer(t, 2, 2))
	proof, err := f.m.MkProofAsserted(h)
	equationMust(t, err)
	pr, err := f.m.NewRef(proof)
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&pr)) }()
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	equationMust(t, f.r.RewriteWithProof(h, out, pr))
	after, err := f.m.RefGet(pr)
	equationMust(t, err)
	if after != proof {
		t.Fatal("false proof mode overwrote caller proof")
	}
	equationMust(t, f.r.Close())
	if _, err = f.r.context.MaxNumBits(); err != nil {
		t.Fatal("rewriter closed borrowed context", err)
	}
}

func TestBV2IntSignedOverflowConstraintsAndRefusalFrontier(t *testing.T) {
	for fail := int64(-1); fail < 32; fail++ {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			f := bv2IntRewriteFixture(t, 3)
			a, b := f.signedInput(t, 5, 3), f.signedInput(t, 2, 3)
			input := fixDLApp(t, f.m, 9, a, b)
			d, err := f.m.GetDecl(input)
			equationMust(t, err)
			out, err := f.m.NewRef(ast.ASTHandle{})
			equationMust(t, err)
			defer func() { equationMust(t, f.m.DelRef(&out)) }()
			f.m.Budget().FailAfter(fail)
			status, err := f.r.MkAppCore(d, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
			f.m.Budget().FailAfter(-1)
			if err == nil {
				if status != rewriter.RewriteDone {
					t.Fatal("signed product refused")
				}
				side, err := f.r.context.SideConditions()
				equationMust(t, err)
				if len(side) != 2 {
					t.Fatal("signed side condition count", len(side))
				}
				for i, h := range side {
					op, err := f.m.GetDeclKind(h)
					equationMust(t, err)
					if op != int32(53+i) {
						t.Fatal("signed constraint order", i, op)
					}
				}
				// In three bits 5 encodes -3, so -3 * 2 underflows but
				// does not exceed the signed maximum.
				f.wantBool(t, side[0], true)
				f.wantBool(t, side[1], false)
			}
			equationMust(t, f.r.Close())
		})
	}
}
func TestBV2IntModTraceResultPublishedBeforeBody(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	a, b := f.integer(t, 3, 3), f.integer(t, 2, 3)
	input := fixDLApp(t, f.m, 13, a, b)
	d, err := f.m.GetDecl(input)
	equationMust(t, err)
	writer := &bv2IntTraceWriter{}
	calls := 0
	writer.check = func() {
		calls++
		h, err := f.m.RefGet(out)
		equationMust(t, err)
		if h.IsNull() {
			t.Fatal("mod trace before result assignment")
		}
	}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("bv2int_rewriter")
	equationMust(t, f.r.context.BindTrace(tags))
	status, err := f.r.MkAppCore(d, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	equationMust(t, err)
	if status != rewriter.RewriteDone {
		t.Fatal("mod refused")
	}
	if tags.ShouldTrace("bv2int_rewriter") {
		if calls == 0 || !strings.Contains(writer.String(), "mk_mod src/tactic/arith/bv2int_rewriter.cpp:236") {
			t.Fatal("missing source mod trace", writer.String())
		}
	} else if calls != 0 {
		t.Fatal("erased mod trace executed")
	}
}
func TestBV2IntSymbolicRewritesPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	f := bv2IntRewriteFixture(t, ^uint32(0))
	m := f.m
	mkVar := func(name string, width uint32) ast.ASTHandle {
		sort, err := m.BitVectorSort(width)
		equationMust(t, err)
		symbol, err := m.Symbols.Intern(name)
		equationMust(t, err)
		h, err := m.MkConst(symbol, sort)
		equationMust(t, err)
		fixDLPin(t, m, h)
		v, err := m.MkBitVectorToInt(h)
		equationMust(t, err)
		fixDLPin(t, m, v)
		return v
	}
	a, b := mkVar("bv2int_a", 3), mkVar("bv2int_b", 4)
	var script bytes.Buffer
	script.WriteString("(declare-const bv2int_a (_ BitVec 3))(declare-const bv2int_b (_ BitVec 4))\n")
	count := 0
	for mode := 0; mode < 3; mode++ {
		lhs, rhs := a, b
		if mode == 1 {
			lhs = fixDLApp(t, m, 7, a, fixDLNumber(t, m, true, "9"))
		}
		if mode == 2 {
			aa, err := m.GetArg(a, 0)
			equationMust(t, err)
			bb, err := m.GetArg(b, 0)
			equationMust(t, err)
			lhs = f.signedTerm(t, aa, 3)
			rhs = f.signedTerm(t, bb, 4)
		}
		for _, op := range []int32{2, 3, 4, 5, 6, 7, 9} {
			input := fixDLApp(t, m, op, lhs, rhs)
			output := f.rewrite(t, input)
			script.WriteString("(push)(assert (not (= ")
			equationMust(t, f.r.context.writeExpr(&script, input))
			script.WriteByte(' ')
			equationMust(t, f.r.context.writeExpr(&script, output))
			script.WriteString(")))(check-sat)(pop)\n")
			count++
		}
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = strings.NewReader(script.String())
	raw, err := cmd.CombinedOutput()
	equationMust(t, err)
	fields := strings.Fields(string(raw))
	if len(fields) != count {
		t.Fatal("C++ failed or returned surplus output", string(raw))
	}
	for i, s := range fields {
		if s != "unsat" {
			t.Fatal("symbolic equivalence", i, string(raw))
		}
	}
}

func TestBV2IntSourceAssertionGateAndClosedDiagnostic(t *testing.T) {
	f := bv2IntRewriteFixture(t, ^uint32(0))
	a, b := f.integer(t, 3, 3), f.integer(t, 2, 3)
	input := fixDLApp(t, f.m, 11, a, b)
	d, err := f.m.GetDecl(input)
	equationMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	equationMust(t, err)
	defer func() { equationMust(t, f.m.DelRef(&out)) }()
	debug := equationSourceDebugFixture(t)
	equationMust(t, debug.Close())
	status, err := f.r.MkAppCore(d, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	if diagnostic.DebugBuild {
		if err == nil {
			t.Fatal("source assertion bypassed closed diagnostic owner")
		}
		got, e := f.m.RefGet(out)
		equationMust(t, e)
		if !got.IsNull() {
			t.Fatal("result published before source arity assertion")
		}
	} else {
		equationMust(t, err)
		if status != rewriter.RewriteFailed {
			t.Fatal("erased assertion changed the source division refusal")
		}
	}
}
