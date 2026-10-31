package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp local expr_ref destructors in
// mk_bv_add/sub/mul, mk_sbv, mk_sbv2real and mk_bv2real_reduced.

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func bv2RealSymbolicBV(t *testing.T, m *ast.ASTManager, name string, width uint32) ast.ASTHandle {
	t.Helper()
	sort, err := m.BitVectorSort(width)
	equationMust(t, err)
	h, err := m.MkFreshConst(name, sort)
	equationMust(t, err)
	fixDLPin(t, m, h)
	return h
}

func bv2RealScopeArg(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, i uint32) ast.ASTHandle {
	t.Helper()
	a, err := m.GetArg(h, i)
	equationMust(t, err)
	return a
}

func bv2RealWantNodeRefs(t *testing.T, m *ast.ASTManager, h ast.ASTHandle, want uint32) {
	t.Helper()
	got, err := m.GetRefCount(h)
	equationMust(t, err)
	if got != want {
		t.Fatal("temporary expression owners survived helper return", got, want)
	}
}

func TestBV2RealExpressionHelperSourceReferenceCounts(t *testing.T) {
	for _, op := range []string{"add", "sub", "mul", "signed-numeral", "signed-real", "reduced"} {
		t.Run(op, func(t *testing.T) {
			u, f := bv2RealFixture(t, 128)
			a, b := bv2RealSymbolicBV(t, f.m, "scope-a", 4), bv2RealSymbolicBV(t, f.m, "scope-b", 6)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			var h ast.ASTHandle
			switch op {
			case "add":
				h = c.add(a, b)
			case "sub":
				h = c.sub(a, b)
			case "mul":
				h = c.mul(a, b)
			case "signed-numeral":
				h = c.signedNumeral(big.NewRat(-7, 1))
			case "signed-real":
				h = c.signedReal(a)
			case "reduced":
				h = c.reduced(a, b, big.NewRat(2, 1), big.NewRat(3, 1))
			}
			equationMust(t, c.err)
			bv2RealWantNodeRefs(t, f.m, h, 1)
			switch op {
			case "add", "sub", "mul":
				// Each extended operand has exactly the result's incoming edge.
				bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, h, 0), 1)
				bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, h, 1), 1)
			case "signed-numeral":
				bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, h, 0), 1)
			case "signed-real":
				// The to_real conversion is shared by the nonnegative branch and subtraction.
				v := bv2RealScopeArg(t, f.m, h, 1)
				bv2RealWantNodeRefs(t, f.m, v, 2)
				negative := bv2RealScopeArg(t, f.m, h, 2)
				if bv2RealScopeArg(t, f.m, negative, 0) != v {
					t.Fatal("signed conversion lost shared operand")
				}
			case "reduced":
				x, y := bv2RealScopeArg(t, f.m, h, 0), bv2RealScopeArg(t, f.m, h, 1)
				bv2RealWantNodeRefs(t, f.m, x, 1)
				bv2RealWantNodeRefs(t, f.m, y, 1)
				bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, x, 0), 1)
				ydiv := bv2RealScopeArg(t, f.m, y, 0)
				bv2RealWantNodeRefs(t, f.m, ydiv, 1)
				bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, ydiv, 0), 1)
			}
			// Owning caller output survives destruction of all helper/call roots.
			out := bv2RealOut(t, f.m)
			equationMust(t, f.m.SetRef(out, h))
			equationMust(t, c.close())
			equationMust(t, c.close())
			bv2RealWantNodeRefs(t, f.m, h, 1)
		})
	}
}

func TestBV2RealExpressionScopeFailureFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 512; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			u, f := bv2RealFixture(t, 128)
			a, b := bv2RealSymbolicBV(t, f.m, "failure-a", 4), bv2RealSymbolicBV(t, f.m, "failure-b", 6)
			outer := bv2RealSymbolicBV(t, f.m, "outer-owner", 3)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			c.hold(outer, nil)
			equationMust(t, c.err)
			before, err := f.m.GetRefCount(outer)
			equationMust(t, err)
			f.m.Budget().FailAfter(stop)
			result := c.reduced(a, b, big.NewRat(2, 1), big.NewRat(3, 1))
			f.m.Budget().FailAfter(-1)
			if c.err == nil {
				succeeded = true
				if result.IsNull() {
					t.Fatal("successful expression scope lost its output")
				}
				bv2RealWantNodeRefs(t, f.m, result, 1)
			} else {
				var failure umemory.Failure
				if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
					t.Fatal(c.err)
				}
				if !result.IsNull() {
					t.Fatal("failed expression helper returned an unowned handle")
				}
			}
			bv2RealWantNodeRefs(t, f.m, outer, before)
			equationMust(t, c.close())
			equationMust(t, c.close())
			bv2RealWantNodeRefs(t, f.m, outer, before-1)
		})
		if succeeded {
			break
		}
	}
	if !succeeded {
		t.Fatal("expression scope success frontier not reached")
	}
}

func bv2RealSymbolicEncoding(t *testing.T, u *BV2RealUtil, prefix string, width uint32, root int64) ast.ASTHandle {
	t.Helper()
	a := bv2RealSymbolicBV(t, u.manager, prefix+"-a", width)
	b := bv2RealSymbolicBV(t, u.manager, prefix+"-b", width)
	out := bv2RealOut(t, u.manager)
	equationMust(t, u.MkBV2RealC(a, b, big.NewRat(1, 1), big.NewRat(root, 1), out))
	h, err := u.manager.RefGet(out)
	equationMust(t, err)
	return h
}

func TestBV2RealReducerHelperSourceReferenceCounts(t *testing.T) {
	for _, op := range []string{"add", "sub", "mul", "eq", "ite", "negate", "le", "approx-le"} {
		t.Run(op, func(t *testing.T) {
			u, f := bv2RealFixture(t, 128)
			root := int64(3)
			if op == "approx-le" {
				root = 2
			}
			s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, root), bv2RealSymbolicEncoding(t, u, "right", 6, root)
			var cond ast.ASTHandle
			// mk_ite requires a Boolean condition, independent of the real coefficients.
			boolean, err := f.m.MkBoolSort()
			equationMust(t, err)
			cond, err = f.m.MkFreshConst("condition", boolean)
			equationMust(t, err)
			fixDLPin(t, f.m, cond)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			var h ast.ASTHandle
			var ok bool
			switch op {
			case "add":
				h, ok = c.binary(6, s, tv)
			case "sub":
				h, ok = c.binary(7, s, tv)
			case "mul":
				h, ok = c.binary(9, s, tv)
			case "eq":
				h, ok = c.equal(s, tv)
			case "ite":
				h, ok = c.ite(cond, s, tv)
			case "negate":
				h, ok = c.negate(s)
			case "le":
				h, ok = c.le(s, tv)
			case "approx-le":
				h, ok = c.approxLE(s, tv, true, true)
			}
			equationMust(t, c.err)
			if !ok {
				t.Fatal("symbolic source reducer refused supported operands")
			}
			if op == "approx-le" {
				// Result owner, two positive premises and the shared negated proxy.
				bv2RealWantNodeRefs(t, f.m, h, 4)
				side, err := u.SideConditions()
				equationMust(t, err)
				if len(side) != 4 {
					t.Fatal("unexpected approximation side conditions", len(side))
				}
				for _, clause := range side {
					bv2RealWantNodeRefs(t, f.m, clause, 1)
				}
			} else {
				bv2RealWantNodeRefs(t, f.m, h, 1)
				count, err := f.m.GetNumArgs(h)
				equationMust(t, err)
				for i := uint32(0); i < count; i++ {
					bv2RealWantNodeRefs(t, f.m, bv2RealScopeArg(t, f.m, h, i), 1)
				}
			}
		})
	}
}

func TestBV2RealVariadicFoldReleasesPreviousEncoding(t *testing.T) {
	for _, op := range []int32{6, 7, 9} {
		t.Run(fmt.Sprint(op), func(t *testing.T) {
			u, f := bv2RealFixture(t, 128)
			s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
			prefix := bv2RealCallResult(t, u, func(c *bv2RealCall) ast.ASTHandle {
				h, ok := c.binary(op, s, tv)
				if !ok {
					t.Fatal("prefix refused")
				}
				return h
			})
			input := fixDLApp(t, f.m, op, s, tv, s)
			decl, err := f.m.GetDecl(input)
			equationMust(t, err)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			h, status := c.reduce(decl, []ast.ASTHandle{s, tv, s})
			equationMust(t, c.err)
			if status != rewriter.RewriteDone || h.IsNull() {
				t.Fatal("fold failed", status)
			}
			// The prefix's coefficient terms occur in the final encoding, but the
			// obsolete encoding application itself has only our external test owner.
			bv2RealWantNodeRefs(t, f.m, prefix, 1)
			bv2RealWantNodeRefs(t, f.m, h, 1)
		})
	}
}
