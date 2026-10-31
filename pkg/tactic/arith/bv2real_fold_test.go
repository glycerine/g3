package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:601-610,623-632,655-664.

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBV2RealVariadicFailurePublishesCompletedPrefixes(t *testing.T) {
	for _, op := range []int32{6, 7, 9} {
		prefixSeen, succeeded := false, false
		for stop := int64(0); stop < 512; stop++ {
			t.Run(fmt.Sprintf("op=%d/%d", op, stop), func(t *testing.T) {
				r, u, f := bv2RealRewriteFixture(t)
				// The source trace precedes result assignment; isolate the fold itself.
				equationMust(t, u.BindTrace(diagnostic.NewTraceTagsWithWriter(io.Discard)))
				s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
				input := fixDLApp(t, f.m, op, s, tv, s)
				decl, err := f.m.GetDecl(input)
				equationMust(t, err)
				prefixOut, out := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
				status, err := r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), prefixOut)
				equationMust(t, err)
				if status != rewriter.RewriteDone {
					t.Fatal("prefix refused", status)
				}
				prefix, err := f.m.RefGet(prefixOut)
				equationMust(t, err)
				old := fixDLConst(t, f.m, false, "previous-output")
				equationMust(t, f.m.SetRef(out, old))
				f.m.Budget().FailAfter(stop)
				status, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv, s}), out)
				f.m.Budget().FailAfter(-1)
				got, readErr := f.m.RefGet(out)
				equationMust(t, readErr)
				if err == nil {
					succeeded = true
					if status != rewriter.RewriteDone || got.IsNull() || got == old || got == s || got == prefix {
						t.Fatal("full fold lost result", status, got)
					}
				} else {
					var failure umemory.Failure
					if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(err)
					}
					switch got {
					case s:
						// A cold first binary operation can fail before any prefix.
					case prefix:
						prefixSeen = true
					default:
						t.Fatal("failure lost the last completed source assignment", got, "old", old, "first", s, "prefix", prefix, err)
					}
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded || !prefixSeen {
			t.Fatal("source fold publication stages not all reached", op, prefixSeen, succeeded)
		}
	}
}

func TestBV2RealVariadicRefusalAndUnaryPublication(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
	unsupported := fixDLConst(t, f.m, false, "unencoded")
	for _, op := range []int32{6, 7, 9} {
		input := fixDLApp(t, f.m, op, s, tv, unsupported)
		decl, err := f.m.GetDecl(input)
		equationMust(t, err)
		for _, args := range [][]ast.ASTHandle{{s}, {s, unsupported}, {s, tv, unsupported}} {
			out := bv2RealOut(t, f.m)
			equationMust(t, f.m.SetRef(out, unsupported))
			status, err := r.MkAppCore(decl, ast.SliceArguments(args), out)
			equationMust(t, err)
			got, err := f.m.RefGet(out)
			equationMust(t, err)
			if len(args) == 1 {
				if status != rewriter.RewriteDone || got != s {
					t.Fatal("unary fold did not assign first operand", status, got)
				}
			} else if status != rewriter.RewriteFailed {
				t.Fatal("unsupported fold tail accepted", status)
			} else if len(args) == 2 && got != s {
				t.Fatal("first binary refusal lost initial assignment", got)
			} else if len(args) == 3 {
				prefixOut := bv2RealOut(t, f.m)
				prefixStatus, err := r.MkAppCore(decl, ast.SliceArguments(args[:2]), prefixOut)
				equationMust(t, err)
				prefix, err := f.m.RefGet(prefixOut)
				equationMust(t, err)
				if prefixStatus != rewriter.RewriteDone || got != prefix {
					t.Fatal("later refusal lost completed binary prefix", got, prefix)
				}
			}
		}
	}
}

func TestBV2RealVariadicInitialAssignmentBeforeAllocation(t *testing.T) {
	for _, op := range []int32{6, 7, 9} {
		t.Run(fmt.Sprint(op), func(t *testing.T) {
			r, u, f := bv2RealRewriteFixture(t)
			equationMust(t, u.BindTrace(diagnostic.NewTraceTagsWithWriter(io.Discard)))
			s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
			input := fixDLApp(t, f.m, op, s, tv)
			decl, err := f.m.GetDecl(input)
			equationMust(t, err)
			out := bv2RealOut(t, f.m)
			old := fixDLConst(t, f.m, false, "old-output")
			equationMust(t, f.m.SetRef(out, old))
			f.m.Budget().FailAfter(0)
			_, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), out)
			f.m.Budget().FailAfter(-1)
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("cold binary construction did not reach injected failure", err)
			}
			got, err := f.m.RefGet(out)
			equationMust(t, err)
			if got != s {
				t.Fatal("first operand was not assigned before binary allocation", got, s)
			}
		})
	}
}

func TestBV2RealVariadicTraceFailurePrecedesInitialAssignment(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	tags := diagnostic.NewTraceTagsWithWriter(io.Discard)
	tags.Enable("bv2real_rewriter")
	if !tags.ShouldTrace("bv2real_rewriter") {
		t.Skip("source TRACE macro erased")
	}
	equationMust(t, u.BindTrace(tags))
	s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
	input := fixDLApp(t, f.m, 6, s, tv)
	decl, err := f.m.GetDecl(input)
	equationMust(t, err)
	out := bv2RealOut(t, f.m)
	old := fixDLConst(t, f.m, false, "old-output")
	equationMust(t, f.m.SetRef(out, old))
	f.m.Budget().FailAfter(0)
	_, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), out)
	f.m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("initial trace operand failure not reached", err)
	}
	got, err := f.m.RefGet(out)
	equationMust(t, err)
	if got != old {
		t.Fatal("trace failure published the first operand too early", got, old)
	}
}
