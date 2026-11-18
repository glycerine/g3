package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func arithOrderID(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) uint32 {
	t.Helper()
	id, err := m.GetASTID(h)
	astTestMust(t, err)
	return id
}

// The reference z3 is a GCC build, which evaluates call arguments right to
// left: mk_sqrt creates 1/2 before k, mk_div creates the divisor first,
// mk_sub(sqrt(6), sqrt(2)) creates sqrt(2) first and mk_mul(k, pi) creates pi
// first. sin(acos x) never creates (^ 1.0 1/2).
func TestArithRewriterTranscendentalConstructionOrder(t *testing.T) {
	m, c, g, local := boolRewriterConfigFixture(t)
	r, err := NewArithRewriter(m, c, g, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })

	one := arithRewriterNumeral(t, m, "1", false)
	status, result := arithRewriterCore(t, r, arithOpASin, one)
	if status == RewriteFailed {
		t.Fatal("asin(1) status", status)
	}
	_, args, matched, err := r.operation(result, arithOpMul)
	astTestMust(t, err)
	if !matched || len(args) != 2 {
		t.Fatal("asin(1) shape", matched, len(args))
	}
	if arithOrderID(t, m, args[1]) >= arithOrderID(t, m, args[0]) {
		t.Fatal("pi must be created before the coefficient")
	}

	pi, err := r.pi()
	astTestMust(t, err)
	twelfth := arithRewriterNumeral(t, m, "1/12", false)
	twelfthPi, err := r.app(arithOpMul, twelfth, pi)
	astTestMust(t, err)
	astTestPin(t, m, twelfthPi)
	status, result = arithRewriterCore(t, r, arithOpSin, twelfthPi)
	if status != RewriteFull {
		t.Fatal("twelfth-angle sine status", status)
	}
	_, division, matched, err := r.operation(result, arithOpDiv)
	astTestMust(t, err)
	if !matched {
		t.Fatal("twelfth-angle sine division")
	}
	_, difference, matched, err := r.operation(division[0], arithOpSub)
	astTestMust(t, err)
	if !matched {
		t.Fatal("twelfth-angle sine difference")
	}
	four := arithOrderID(t, m, division[1])
	sixRoot := arithOrderID(t, m, difference[0])
	twoRoot := arithOrderID(t, m, difference[1])
	if !(four < twoRoot && twoRoot < sixRoot) {
		t.Fatal("sine value order", four, twoRoot, sixRoot)
	}
	_, power, matched, err := r.operation(difference[0], arithOpPower)
	astTestMust(t, err)
	if !matched || arithOrderID(t, m, power[1]) >= arithOrderID(t, m, power[0]) {
		t.Fatal("sqrt must create 1/2 before its base")
	}

	realSort, err := m.MkFamilySort(r.family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "arith-order-x"), realSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	acosX, err := r.app(arithOpACos, x)
	astTestMust(t, err)
	astTestPin(t, m, acosX)
	// Keep sin(acos x) referenced so no id is recycled before the check.
	sine, err := r.app(arithOpSin, acosX)
	astTestMust(t, err)
	astTestPin(t, m, sine)
	decl, err := m.GetDecl(sine)
	astTestMust(t, err)
	out := astTestPin(t, m, sine)
	status, err = r.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{acosX}), out)
	astTestMust(t, err)
	if status != RewriteFull {
		t.Fatal("sin(acos x) status", status)
	}
	result, err = m.RefGet(out)
	astTestMust(t, err)
	resultID := arithOrderID(t, m, result)
	half := arithRewriterNumeral(t, m, "1/2", false)
	extra, err := r.app(arithOpPower, one, half)
	astTestMust(t, err)
	if arithOrderID(t, m, extra) < resultID {
		t.Fatal("sin(acos x) created (^ 1.0 1/2)")
	}
}
