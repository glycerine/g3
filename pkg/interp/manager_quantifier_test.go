package interp

import (
	"testing"
)

func TestInterpolationManagerControllingEqualityQuantifiers(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	boolSort, err := w.BoolType()
	interpMust(t, err)
	x, err := w.MakeVariable("quant-x", intSort)
	interpMust(t, err)
	x = interpPin(t, m, x)
	three, err := w.MakeIntText("3")
	interpMust(t, err)
	equality, err := w.Make(OperatorEqual, x, three)
	interpMust(t, err)
	p, err := w.MakeVariable("quant-p", boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	formula, err := w.Make(OperatorImplies, equality, p)
	interpMust(t, err)
	optimized, err := w.ApplyQuantifier(OperatorForall, x, formula)
	interpMust(t, err)
	optimized = interpPin(t, m, optimized)
	if found, e := w.Occurs(x, optimized); e != nil || found {
		t.Fatal("controlling equality substitution", found, e)
	}
	if op, e := w.Operator(optimized); e != nil || op != OperatorImplies {
		t.Fatal("optimized implication", op, e)
	}

	positive, err := w.Make(OperatorGT, x, three)
	interpMust(t, err)
	quantified, err := w.ApplyQuantifier(OperatorExists, x, positive)
	interpMust(t, err)
	quantified = interpPin(t, m, quantified)
	if op, e := w.Operator(quantified); e != nil || op != OperatorExists {
		t.Fatal("fallback quantifier", op, e)
	}
	if count, e := w.QuantifierNumBound(quantified); e != nil || count != 1 {
		t.Fatal("fallback bound", count, e)
	}

	withoutX, err := w.ApplyQuantifier(OperatorForall, x, p)
	interpMust(t, err)
	if withoutX != p {
		t.Fatal("unused variable changed expression")
	}
}

func TestInterpolationManagerQuantifierDistributionAndArithmeticSolve(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	x, err := w.MakeVariable("quant-distribute-x", intSort)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := w.MakeVariable("quant-distribute-y", intSort)
	interpMust(t, err)
	y = interpPin(t, m, y)
	two, err := w.MakeIntText("2")
	interpMust(t, err)
	left, err := w.Make(OperatorPlus, x, two)
	interpMust(t, err)
	equality, err := w.Make(OperatorEqual, left, y)
	interpMust(t, err)
	optimized, err := w.ApplyQuantifier(OperatorExists, x, equality)
	interpMust(t, err)
	optimized = interpPin(t, m, optimized)
	if found, e := w.Occurs(x, optimized); e != nil || found {
		t.Fatal("arithmetic controlling equality", found, e)
	}

	zero, err := w.MakeIntText("0")
	interpMust(t, err)
	greater, err := w.Make(OperatorGT, x, zero)
	interpMust(t, err)
	less, err := w.Make(OperatorLT, x, two)
	interpMust(t, err)
	conjunction, err := w.Make(OperatorAnd, greater, less)
	interpMust(t, err)
	distributed, err := w.ApplyQuantifier(OperatorForall, x, conjunction)
	interpMust(t, err)
	distributed = interpPin(t, m, distributed)
	if op, e := w.Operator(distributed); e != nil || op != OperatorAnd {
		t.Fatal("distributed root", op, e)
	}
	for i := uint32(0); i < 2; i++ {
		child, e := w.Arg(distributed, i)
		interpMust(t, e)
		if op, e := w.Operator(child); e != nil || op != OperatorForall {
			t.Fatal("distributed child", i, op, e)
		}
	}
}

func TestInterpolationManagerCloseUniversally(t *testing.T) {
	m, w := interpRewritingManagerFixture(t)
	intSort, err := w.IntType()
	interpMust(t, err)
	free, err := m.MkVar(0, intSort)
	interpMust(t, err)
	free = interpPin(t, m, free)
	zero, err := w.MakeIntText("0")
	interpMust(t, err)
	body, err := w.Make(OperatorGE, free, zero)
	interpMust(t, err)
	closed, err := w.CloseUniversally(body)
	interpMust(t, err)
	closed = interpPin(t, m, closed)
	if op, e := w.Operator(closed); e != nil || op != OperatorForall {
		t.Fatal("universal closure", op, e)
	}
	if count, e := w.NumFreeVariables(closed); e != nil || count != 0 {
		t.Fatal("closed free variables", count, e)
	}
	if name, e := w.QuantifierBoundName(closed, 0); e != nil || name != "%%0" {
		t.Fatal("generated bound name", name, e)
	}
	// AUDIT.md interp_02-F3: iz3mgr::make_quant sets weight 0 and qid "itp".
	qid, _, err := m.GetQuantifierIDs(closed)
	interpMust(t, err)
	if text, e := m.Symbols.String(qid); e != nil || text != "itp" {
		t.Fatal("quantifier qid", text, e)
	}
	if weight, e := m.GetQuantifierWeight(closed); e != nil || weight != 0 {
		t.Fatal("quantifier weight", weight, e)
	}
	if _, err = w.ApplyQuantifier(OperatorPlus, free, body); err == nil {
		t.Fatal("accepted non-quantifier operator")
	}
}
