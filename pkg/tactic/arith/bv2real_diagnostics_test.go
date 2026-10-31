package arith

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestBV2RealSourceAssertionProfilesAndClosedDiagnostic(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	debug := equationSourceDebugFixture(t)
	a, b := bv2RealEncoding(t, u, 1, 2, 2, 2), bv2RealEncoding(t, u, 3, -1, 2, 2)
	input := fixDLApp(t, f.m, 10, a, b)
	decl, err := f.m.GetDecl(input)
	equationMust(t, err)
	out := bv2RealOut(t, f.m)
	// The source division operation refuses reduction, but its arity assertion
	// still has to query the live diagnostic owner in debug builds.
	equationMust(t, debug.Close())
	status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	if diagnostic.DebugBuild {
		if err == nil {
			t.Fatal("closed diagnostic owner bypassed")
		}
	} else {
		equationMust(t, err)
		if status != rewriter.RewriteFailed {
			t.Fatal("erased assertion changed division refusal")
		}
	}
	h, err := f.m.RefGet(out)
	equationMust(t, err)
	if !h.IsNull() {
		t.Fatal("division published a result")
	}
}

func TestBV2RealSourcePrimitiveAssertionsAndDisabledGate(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	debug := equationSourceDebugFixture(t)
	truth, err := f.m.MkTrue()
	equationMust(t, err)
	fixDLPin(t, f.m, truth)
	for _, op := range []string{"add", "mul", "fraction"} {
		c := &bv2RealCall{u: u}
		switch op {
		case "add":
			c.add(truth, f.bv(t, 0, 3))
		case "mul":
			c.mul(truth, f.bv(t, 0, 3))
		case "fraction":
			c.signedNumeral(big.NewRat(1, 2))
		}
		if diagnostic.DebugBuild && !errors.Is(c.err, diagnostic.ErrDebugAssertion) {
			t.Fatal("missing source assertion", op, c.err)
		}
		equationMust(t, c.close())
	}
	equationMust(t, debug.EnableAssertions(false))
	// The disabled source BV checks must permit the original zero shortcut;
	// reaching a sort query here would reject the deliberately Boolean input.
	c := &bv2RealCall{u: u}
	zero := f.bv(t, 0, 3)
	if got := c.mul(truth, zero); got != zero || c.err != nil {
		t.Fatal("disabled assertion evaluated BV predicate", got, c.err)
	}
	equationMust(t, c.close())
}

func TestBV2RealSourceCachedSignatureAssertions(t *testing.T) {
	for _, field := range []string{"divisor", "root", "first-width", "second-width"} {
		t.Run(field, func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			equationSourceDebugFixture(t)
			h := bv2RealEncoding(t, u, 1, 2, 2, 3)
			decl, err := f.m.GetDecl(h)
			equationMust(t, err)
			sig := u.reverse[decl]
			switch field {
			case "divisor":
				equationMust(t, f.m.NumeralService().SetInt64(sig.divisor, -1))
			case "root":
				equationMust(t, f.m.NumeralService().SetString(sig.root, "3/2"))
			case "first-width":
				sig.first++
			case "second-width":
				sig.second++
			}
			u.reverse[decl] = sig
			a, b := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
			var divisor, radical big.Rat
			_, err = u.Match(h, a, b, &divisor, &radical)
			if diagnostic.DebugBuild {
				if !errors.Is(err, diagnostic.ErrDebugAssertion) {
					t.Fatal("missing cache signature assertion", err)
				}
			} else {
				equationMust(t, err)
			}
		})
	}
}

func TestBV2RealTraceComparisonPublicationAndErasure(t *testing.T) {
	for _, root := range []int64{2, 3} {
		r, u, f := bv2RealRewriteFixture(t)
		a, b := bv2RealEncoding(t, u, -2, 1, 2, root), bv2RealEncoding(t, u, 3, -1, 2, root)
		input := fixDLApp(t, f.m, 2, a, b)
		decl, err := f.m.GetDecl(input)
		equationMust(t, err)
		out := bv2RealOut(t, f.m)
		writer := &bv2IntTraceWriter{}
		appSeen, comparisonSeen := false, false
		writer.check = func() {
			text := writer.String()
			if !appSeen && strings.Contains(text, "mk_app_core src/tactic/arith/bv2real_rewriter.cpp:359 ---------\n") {
				appSeen = true
				h, err := f.m.RefGet(out)
				equationMust(t, err)
				if !h.IsNull() || len(u.side) != 0 {
					t.Fatal("dispatch trace after reduction")
				}
			}
			line := "464"
			if root == 3 {
				line = "541"
			}
			if !comparisonSeen && strings.Contains(text, "mk_le src/tactic/arith/bv2real_rewriter.cpp:"+line+" ---------\n") {
				comparisonSeen = true
				h, err := f.m.RefGet(out)
				equationMust(t, err)
				if root == 2 {
					if !h.IsNull() || len(u.side) != 4 {
						t.Fatal("approximation trace result/conditions order")
					}
				} else if h.IsNull() {
					t.Fatal("exact comparison trace before assignment")
				}
			}
		}
		tags := diagnostic.NewTraceTagsWithWriter(writer)
		tags.Enable("bv2real_rewriter")
		equationMust(t, u.BindTrace(tags))
		_, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
		equationMust(t, err)
		if tags.ShouldTrace("bv2real_rewriter") {
			if !appSeen || !comparisonSeen {
				t.Fatal("missing source trace", writer.String())
			}
		} else if writer.Len() != 0 {
			t.Fatal("erased trace body evaluated")
		}
	}
}

func TestBV2RealTraceDivisibilityPrefixes(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	writer := &bytes.Buffer{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("bv2real_rewriter")
	equationMust(t, u.BindTrace(tags))
	c := &bv2RealCall{u: u}
	_, ok := c.divisible(f.bv(t, 12, 5), big.NewRat(6, 1))
	equationMust(t, c.err)
	equationMust(t, c.close())
	if ok || len(u.side) != 1 {
		t.Fatal("odd-factor refusal lost its side-condition prefix")
	}
	if tags.ShouldTrace("bv2real_rewriter") {
		text := writer.String()
		before, after := strings.Index(text, ".cpp:318"), strings.Index(text, ".cpp:339")
		if before < 0 || after <= before || !strings.Contains(text, "#b01100 6\n") || !strings.Contains(text, "((_ extract 4 1) #b01100) 3\n") {
			t.Fatal("divisibility trace", text)
		}
	} else if writer.Len() != 0 {
		t.Fatal("erased divisibility trace")
	}
}

type bv2RealRefusingTrace struct {
	before func()
	calls  int
}

func (w *bv2RealRefusingTrace) Write(p []byte) (int, error) {
	w.calls++
	if w.before != nil {
		w.before()
		w.before = nil
	}
	return 0, io.ErrClosedPipe
}

func TestBV2RealTraceStreamFailureStillEvaluatesOperands(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	a, b := bv2RealEncoding(t, u, 1, 2, 2, 2), bv2RealEncoding(t, u, 3, -1, 2, 2)
	input := fixDLApp(t, f.m, 6, a, b)
	decl, err := f.m.GetDecl(input)
	equationMust(t, err)
	out := bv2RealOut(t, f.m)
	writer := &bv2RealRefusingTrace{}
	tags := diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("bv2real_rewriter")
	equationMust(t, u.BindTrace(tags))
	_, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	equationMust(t, err)
	h, err := f.m.RefGet(out)
	equationMust(t, err)
	bv2RealWantCoefficients(t, u, f, h, 4, 1, 2, 2)
	if !tags.ShouldTrace("bv2real_rewriter") {
		if writer.calls != 0 {
			t.Fatal("erased trace wrote")
		}
		return
	}
	// Fail the first header output and remove the printer service. A stream
	// failure cannot suppress the later mk_pp operand's own service failure.
	writer = &bv2RealRefusingTrace{}
	previous := uparams.BorrowProcessGlobalParamsService()
	defer func() { _, err := uparams.BindProcessGlobalParamsService(previous); equationMust(t, err) }()
	writer.before = func() { _, err := uparams.BindProcessGlobalParamsService(nil); equationMust(t, err) }
	tags = diagnostic.NewTraceTagsWithWriter(writer)
	tags.Enable("bv2real_rewriter")
	equationMust(t, u.BindTrace(tags))
	_, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	if !errors.Is(err, uparams.ErrGlobalParamsOwner) {
		t.Fatal("stream failure swallowed printing operand error", err)
	}
}

func TestBV2RealTraceFormattingAllocationFrontiers(t *testing.T) {
	failures, completed := 0, false
	for stop := int64(0); stop < 192; stop++ {
		ok := false
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			h := bv2RealEncoding(t, u, 3, -2, 2, 3)
			decl, err := f.m.GetDecl(h)
			equationMust(t, err)
			var out bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&out)
			tags.Enable("bv2real_rewriter")
			equationMust(t, u.BindTrace(tags))
			if !tags.ShouldTrace("bv2real_rewriter") {
				ok = true
				return
			}
			f.m.Budget().FailAfter(stop)
			err = u.traceApp(decl, []ast.ASTHandle{h})
			f.m.Budget().FailAfter(-1)
			if err == nil {
				ok = true
			} else {
				failures++
			}
			if len(u.tracePending) != 0 {
				if err := u.Reset(); err == nil {
					t.Fatal("stopped formatter allowed utility reset")
				}
			}
			equationMust(t, u.Close())
			if len(u.tracePending) != 0 {
				t.Fatal("formatter cleanup retained")
			}
		})
		if ok {
			completed = true
			break
		}
	}
	if !completed {
		t.Fatal("no completed formatting allocation frontier")
	}
	t.Log("formatting refusal cases", failures)
}
