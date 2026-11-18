package interp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestInterpolationManagerLinearCombinations(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	x, err := w.MakeVariable("linear-x", intSort)
	interpMust(t, err)
	x = interpPin(t, m, x)
	zero, err := w.MakeIntText("0")
	interpMust(t, err)
	one, err := w.MakeIntText("1")
	interpMust(t, err)
	two, err := w.MakeIntText("2")
	interpMust(t, err)
	three, err := w.MakeIntText("3")
	interpMust(t, err)
	p, err := w.Make(OperatorLE, zero, zero)
	interpMust(t, err)
	q, err := w.Make(OperatorLE, x, three)
	interpMust(t, err)

	combined, err := w.LinearCombination(p, two, q, false)
	interpMust(t, err)
	combined = interpPin(t, m, combined)
	if op, e := w.Operator(combined); e != nil || op != OperatorLE {
		t.Fatal("non-strict combination", op, e)
	}
	if found, e := w.Occurs(x, combined); e != nil || !found {
		t.Fatal("combined variable", found, e)
	}
	strictQ, err := w.Make(OperatorLT, x, three)
	interpMust(t, err)
	strict, err := w.LinearCombination(p, one, strictQ, false)
	interpMust(t, err)
	strict = interpPin(t, m, strict)
	if op, e := w.Operator(strict); e != nil || op != OperatorLT {
		t.Fatal("strict combination", op, e)
	}

	sum, err := w.SumInequalities([]ast.ASTHandle{two, one}, []ast.ASTHandle{q, strictQ}, true)
	interpMust(t, err)
	sum = interpPin(t, m, sum)
	if op, e := w.Operator(sum); e != nil || op != OperatorLT {
		t.Fatal("summed inequalities", op, e)
	}
	if _, err = w.SumInequalities([]ast.ASTHandle{one}, nil, false); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("coefficient count", err)
	}
	truth, err := w.Make(OperatorTrue)
	interpMust(t, err)
	if _, err = w.LinearCombination(p, one, truth, false); err == nil || err.Error() != "not an inequality" {
		t.Fatal("non-inequality", err)
	}
}

func TestInterpolationManagerIntegerDivisionDecomposition(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	x, err := w.MakeVariable("idiv-x", intSort)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := w.MakeVariable("idiv-y", intSort)
	interpMust(t, err)
	y = interpPin(t, m, y)
	six, err := w.MakeIntText("6")
	interpMust(t, err)
	sixX, err := w.Make(OperatorTimes, six, x)
	interpMust(t, err)
	term, err := w.Make(OperatorPlus, sixX, y)
	interpMust(t, err)
	term = interpPin(t, m, term)

	resultRef, err := w.MakeIDivRational(term, interpSmall(3, 1))
	interpMust(t, err)
	defer func() { interpMust(t, m.DelRef(&resultRef)) }()
	result, err := m.RefGet(resultRef)
	interpMust(t, err)
	if op, e := w.Operator(result); e != nil || op != OperatorPlus {
		t.Fatal("division decomposition", op, e)
	}
	if found, e := w.Occurs(x, result); e != nil || !found {
		t.Fatal("whole part", found, e)
	}
	if found, e := w.Occurs(y, result); e != nil || !found {
		t.Fatal("fractional part", found, e)
	}

	oneRef, err := w.MakeIDivRational(term, interpSmall(1, 1))
	interpMust(t, err)
	defer func() { interpMust(t, m.DelRef(&oneRef)) }()
	oneResult, err := m.RefGet(oneRef)
	interpMust(t, err)
	if op, e := w.Operator(oneResult); e != nil || op != OperatorPlus {
		t.Fatal("unit divisor simplification", op, e)
	}
	if _, err = w.MakeIDivRational(term, interpSmall(1, 2)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("fractional divisor", err)
	}
}
