package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type intervalSieveTestFixture struct {
	ast                       *ast.ASTManager
	boolean, truth, falsehood ast.ASTHandle
}

func intervalSieveFixture(t *testing.T) (*intervalSieveTestFixture, ast.ASTHandle, func(string) ast.ASTHandle) {
	t.Helper()
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	astTestPin(t, m, boolean)
	astTestPin(t, m, truth)
	astTestPin(t, m, falsehood)
	f := &intervalSieveTestFixture{ast: m, boolean: boolean, truth: truth, falsehood: falsehood}
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	numeral := func(text string) ast.ASTHandle {
		h, e := f.ast.ArithmeticIntegerNumeralText(text)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	return f, integer, numeral
}

func TestIntervalSieveSelectsArithmeticAndIgnoresFiniteColumns(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sorts := []ast.ASTHandle{f.boolean, integer, f.boolean}
	r, err := NewIntervalSieveEmpty(f.ast, sorts)
	astTestMust(t, err)
	if got := r.InnerColumns(); len(got) != 3 || got[0] || !got[1] || got[2] {
		t.Fatal("source singleton capability mask", got)
	}
	two := numeral("2")
	astTestMust(t, r.AddFact([]ast.ASTHandle{f.truth, two, f.falsehood}))
	for _, fact := range [][]ast.ASTHandle{{f.truth, two, f.falsehood}, {f.falsehood, two, f.truth}} {
		found, e := r.ContainsFact(fact)
		if e != nil || !found {
			t.Fatal("ignored finite columns constrained membership", found, e)
		}
	}
	// Equality on an ignored column is deliberately an identity operation.
	astTestMust(t, r.FilterEqual(0, f.falsehood))
	found, err := r.ContainsFact([]ast.ASTHandle{f.truth, two, f.truth})
	if err != nil || !found {
		t.Fatal("ignored equality changed interval sieve", found, err)
	}
}

func TestIntervalSieveFormulaUsesExactOuterColumn(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	r, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{f.boolean, integer, f.boolean})
	astTestMust(t, err)
	astTestMust(t, r.FilterEqual(1, numeral("4")))
	formula, err := r.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	handle, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	used, err := ast.NewASTUsedVars(f.ast)
	astTestMust(t, err)
	astTestMust(t, used.Collect(handle))
	if used.Contains(0) != (ast.ASTHandle{}) || used.Contains(1) != integer || used.Contains(2) != (ast.ASTHandle{}) {
		t.Fatal("formula did not preserve outer variable numbering")
	}
}

func TestIntervalSieveJoinSkipsIgnoredEqualitiesAndProjectsInnerColumns(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	left, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{f.boolean, integer})
	astTestMust(t, err)
	right, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{integer, f.boolean})
	astTestMust(t, err)
	astTestMust(t, left.FilterEqual(1, numeral("2")))
	astTestMust(t, right.FilterEqual(0, numeral("3")))
	joined, err := JoinIntervalSieves(left, right, []uint32{1}, []uint32{0})
	astTestMust(t, err)
	if !joined.Empty() {
		t.Fatal("selected-column equality was not sent to inner join")
	}
	ignored, err := JoinIntervalSieves(left, right, []uint32{0}, []uint32{1})
	astTestMust(t, err)
	if ignored.Empty() {
		t.Fatal("ignored-column equality unexpectedly constrained join")
	}
	projected, err := ignored.Project([]uint32{0, 2})
	astTestMust(t, err)
	if got := projected.InnerColumns(); len(got) != 2 || !got[0] || got[1] {
		t.Fatal("projected sieve mask", got)
	}
	formula, err := projected.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
}

func TestIntervalSieveInterpretedIgnoredVariableIsIdentity(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	r, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{f.boolean, integer})
	astTestMust(t, err)
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, x)
	condition, err := f.ast.MkNot(x)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	astTestMust(t, r.FilterInterpreted(condition))
	found, err := r.ContainsFact([]ast.ASTHandle{f.truth, numeral("99")})
	if err != nil || !found {
		t.Fatal("ignored interpreted condition changed full sieve", found, err)
	}
}

func TestIntervalSieveJoinsOrdinaryIntervalComponentBothOrders(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	sieve, err := NewIntervalSieveFull(f.ast, []ast.ASTHandle{f.boolean, integer})
	astTestMust(t, err)
	astTestMust(t, sieve.FilterEqual(1, numeral("2")))
	ordinary := NewIntervalFull(1)
	astTestMust(t, ordinary.FilterEqualExpression(f.ast, 0, numeral("3")))
	joined, err := JoinIntervalSieveWithInterval(sieve, ordinary, []ast.ASTHandle{integer}, []uint32{1}, []uint32{0})
	astTestMust(t, err)
	if !joined.Empty() {
		t.Fatal("sieve/ordinary selected equality was ignored")
	}
	reverse, err := JoinIntervalWithIntervalSieve(ordinary, []ast.ASTHandle{integer}, sieve, []uint32{0}, []uint32{1})
	astTestMust(t, err)
	if !reverse.Empty() {
		t.Fatal("ordinary/sieve selected equality was ignored")
	}
	ignored, err := JoinIntervalSieveWithInterval(sieve, ordinary, []ast.ASTHandle{integer}, []uint32{0}, []uint32{0})
	astTestMust(t, err)
	if ignored.Empty() {
		t.Fatal("ignored sieve endpoint constrained ordinary join")
	}
}
