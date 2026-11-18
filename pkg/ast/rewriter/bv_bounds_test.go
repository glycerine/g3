package rewriter

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestBVBoundsSourceDebugGuards(t *testing.T) {
	if !ast.ASTSourceDebug {
		t.Skip("source assertions erased")
	}
	m, x := bvBoundsFixture(t)
	bounds, err := NewBVBounds(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, bounds.Close()) })
	for _, interval := range [][2]*unumeral.Rational{
		{pbSmall(-1), pbSmall(7)},
		{pbSmall(8), pbSmall(7)},
		{pbSmall(0), pbSmall(256)},
	} {
		if _, err = bounds.AddBoundUnsigned(x, interval[0], interval[1], false); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("unsigned interval source assertion", interval, err)
		}
	}
	if _, err = bounds.BoundLower(x, pbSmall(-1)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("lower-bound source assertion", err)
	}
	if _, err = bounds.BoundUpper(x, pbSmall(256)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("upper-bound source assertion", err)
	}
	if _, err = bounds.AddBoundSigned(x, pbSmall(2), pbSmall(1), false); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("signed interval source assertion", err)
	}
}

func TestBVBoundsSourceTraceRecords(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bv_bounds")
	if !tags.ShouldTrace("bv_bounds") {
		t.Skip("source trace macro erased")
	}
	symbols, numerals, budget := astTestOwners(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget, TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	bindBVTraceGlobals(t, m)
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	bounds, err := NewBVBounds(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, bounds.Close()) })
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "bv-bounds-trace-x"), sort)
	astTestMust(t, err)
	if _, err = bounds.AddBoundUnsigned(x, pbSmall(2), pbSmall(7), false); err != nil {
		t.Fatal(err)
	}
	if sat, e := bounds.IsSatisfiable(); e != nil || !sat {
		t.Fatal("trace satisfiability", sat, e)
	}
	astTestMust(t, bounds.Reset())
	seven := bvAddNum(t, m, 7, 8)
	constraint, err := m.MkBitVectorULE(x, seven)
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	conjunction, err := m.MkAndArgs([]ast.ASTHandle{constraint, truth})
	astTestMust(t, err)
	if status, _ := bvBoundsRewrite(t, bounds, conjunction); status != RewriteFailed {
		t.Fatal("trace rewrite status", status)
	}
	text := output.String()
	for _, fragment := range []string{
		"[bv_bounds] add_bound_unsigned src/ast/rewriter/bv_bounds.cpp:453",
		"bound_unsigned bv-bounds-trace-x: [2;7]",
		"[bv_bounds] bound_lo src/ast/rewriter/bv_bounds.cpp:524",
		"[bv_bounds] bound_up src/ast/rewriter/bv_bounds.cpp:535",
		"[bv_bounds] is_sat src/ast/rewriter/bv_bounds.cpp:606",
		"[bv_bounds] is_sat_core src/ast/rewriter/bv_bounds.cpp:623",
		"res: 1",
		"[bv_bounds] rewrite src/ast/rewriter/bv_bounds.cpp:284",
		"[bv_bounds] rewrite src/ast/rewriter/bv_bounds.cpp:286",
		"[bv_bounds] rewrite src/ast/rewriter/bv_bounds.cpp:302",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("bounds trace missing %q:\n%s", fragment, text)
		}
	}
}

func bvBoundsFixture(t *testing.T) (*ast.ASTManager, ast.ASTHandle) {
	t.Helper()
	m, _ := bvAddFixture(t)
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "bv-bounds-x"), sort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	return m, x
}

func bvBoundsRewrite(t *testing.T, b *BVBounds, expression ast.ASTHandle) (RewriteStatus, ast.ASTHandle) {
	t.Helper()
	m := b.manager
	decl, err := m.GetDecl(expression)
	astTestMust(t, err)
	n, err := m.GetNumArgs(expression)
	astTestMust(t, err)
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = m.GetArg(expression, uint32(i))
		astTestMust(t, err)
	}
	out := astTestPin(t, m, ast.ASTHandle{})
	status, err := b.Rewrite(32, decl, ast.SliceArguments(args), out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	return status, result
}

func TestBVBoundsContradictionAndSingletonRewrite(t *testing.T) {
	for _, contradiction := range []bool{false, true} {
		m, x := bvBoundsFixture(t)
		b, err := NewBVBounds(m)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, b.Close()) })
		three := bvAddNum(t, m, 3, 8)
		lowerValue := uint32(3)
		if contradiction {
			lowerValue = 4
		}
		lower := bvAddNum(t, m, lowerValue, 8)
		upperConstraint, err := m.MkBitVectorULE(x, three)
		astTestMust(t, err)
		lowerConstraint, err := m.MkBitVectorULE(lower, x)
		astTestMust(t, err)
		both, err := m.MkAndArgs([]ast.ASTHandle{upperConstraint, lowerConstraint})
		astTestMust(t, err)
		status, result := bvBoundsRewrite(t, b, both)
		if status != RewriteDone {
			t.Fatal("expected completed bounds rewrite", status)
		}
		if contradiction {
			if !m.IsFalse(result) {
				t.Fatal("contradictory bounds did not reduce to false", result)
			}
			continue
		}
		if kind, e := m.GetDeclKind(result); e != nil || kind != ast.ASTOpEq {
			t.Fatal("singleton did not reduce to equality", kind, e)
		}
		left, err := m.GetArg(result, 0)
		astTestMust(t, err)
		right, err := m.GetArg(result, 1)
		astTestMust(t, err)
		if left != x {
			t.Fatal("singleton equality variable", left, x)
		}
		text, matched, err := m.BitVectorNumeralText(right)
		astTestMust(t, err)
		if !matched || text != "3" {
			t.Fatal("singleton equality value", text, matched)
		}
	}
}

func TestBVBoundsNegativeIntervalsAndSignedConstraint(t *testing.T) {
	m, x := bvBoundsFixture(t)
	b, err := NewBVBounds(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	if okay, err := b.AddBoundUnsigned(x, pbSmall(0), pbSmall(254), true); err != nil || !okay {
		t.Fatal("negative interval", okay, err)
	}
	if sat, err := b.IsSatisfiable(); err != nil || !sat {
		t.Fatal("negative interval satisfiability", sat, err)
	}
	singletons, err := b.Singletons()
	astTestMust(t, err)
	if value := singletons[x]; value == nil || !bvBoundsIs(value, 255) {
		t.Fatal("negative interval singleton", value)
	}

	astTestMust(t, b.Reset())
	minusOne := bvAddNum(t, m, 255, 8)
	signedUpper, err := m.MkBitVectorSLE(x, minusOne)
	astTestMust(t, err)
	if okay, err := b.AddConstraint(signedUpper); err != nil || !okay {
		t.Fatal("signed bound", okay, err)
	}
	if okay, err := b.AddBoundUnsigned(x, pbSmall(0), pbSmall(127), false); err != nil || !okay {
		t.Fatal("unsigned bound addition", okay, err)
	}
	if sat, err := b.IsSatisfiable(); err != nil || sat {
		t.Fatal("signed/unsigned contradiction", sat, err)
	}
}

func bvBoundsIs(r *unumeral.Rational, want int32) bool {
	v, ok := r.SmallInt32()
	return ok && v == want
}
