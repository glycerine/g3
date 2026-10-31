package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationProductFactsAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := f.ast.Symbols.Intern("finite-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(symbol, ruleSort)
	astTestMust(t, err)
	ref, err := f.ast.NewRef(explanation)
	astTestMust(t, err)
	product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, explanation))
	astTestMust(t, f.ast.DelRef(&ref))
	got, found, err := product.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || got != explanation {
		t.Fatal("finite explanation", got, found)
	}
	_, found, err = product.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if found {
		t.Fatal("missing finite tuple found")
	}
	if err = product.AddFact([]ast.ASTHandle{f.truth}, explanation); err == nil {
		t.Fatal("duplicate inner explanation accepted")
	}
	if err = product.AddFact(nil, explanation); err == nil {
		t.Fatal("wrong tuple arity accepted")
	}
	count, err := product.Len()
	astTestMust(t, err)
	if count != 1 {
		t.Fatal("product row count", count)
	}
	astTestMust(t, product.Close())
	if _, err = f.ast.GetKind(explanation); err == nil {
		t.Fatal("closed product kept explanation alive")
	}
}

func TestFiniteExplanationProductZeroFiniteColumns(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := f.ast.Symbols.Intern("zero-key-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(symbol, ruleSort)
	astTestMust(t, err)
	product, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	astTestMust(t, product.AddFact(nil, explanation))
	got, found, err := product.Explanation(nil)
	astTestMust(t, err)
	if !found || got != explanation {
		t.Fatal("zero-column explanation", got, found)
	}
}

func TestFiniteExplanationProductUnionAndDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	makeExplanation := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, ruleSort)
		astTestMust(t, e)
		return h
	}
	old := makeExplanation("finite-union-old")
	fresh := makeExplanation("finite-union-new")
	prior := makeExplanation("finite-delta-prior")
	target, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, target.Close()) }()
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	delta, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, delta.Close()) }()
	astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth}, old))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, fresh))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood}, fresh))
	astTestMust(t, delta.AddFact([]ast.ASTHandle{f.truth}, prior))
	astTestMust(t, target.Union(source, delta))
	for _, tc := range []struct {
		p         *FiniteExplanationProduct
		key, want ast.ASTHandle
	}{{target, f.truth, old}, {target, f.falsehood, fresh}, {delta, f.truth, prior}, {delta, f.falsehood, fresh}} {
		got, found, e := tc.p.Explanation([]ast.ASTHandle{tc.key})
		astTestMust(t, e)
		if !found || got != tc.want {
			t.Fatal("union explanation", got, found, tc.want)
		}
	}
	astTestMust(t, target.Union(source, nil))
	count, err := target.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("repeated union count", count)
	}
	if err = target.Union(source, target); err == nil {
		t.Fatal("accepted aliased delta")
	}
}

func TestFiniteExplanationProductProjectCollisions(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	makeExplanation := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, sort)
		astTestMust(t, e)
		return h
	}
	first := makeExplanation("finite-project-first")
	second := makeExplanation("finite-project-second")
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, f.falsehood}, first))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, f.truth}, second))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood, f.truth}, second))
	projected, err := source.Project([]int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	got, found, err := projected.Explanation([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || got != first {
		t.Fatal("colliding explanation", got, found, first)
	}
	got, found, err = projected.Explanation([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	if !found || got != second {
		t.Fatal("distinct explanation", got, found, second)
	}
	count, err := projected.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("projected count", count)
	}
	if _, err = source.Project([]int{2}); err == nil {
		t.Fatal("removed explanation column accepted")
	}
}

func TestFiniteExplanationProductJoinRows(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	makeExplanation := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, sort)
		astTestMust(t, e)
		return h
	}
	leftFact := makeExplanation("finite-join-left")
	rightFact := makeExplanation("finite-join-right")
	left, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	right, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	astTestMust(t, left.AddFact([]ast.ASTHandle{f.truth}, leftFact))
	astTestMust(t, left.AddFact([]ast.ASTHandle{f.falsehood}, leftFact))
	astTestMust(t, right.AddFact([]ast.ASTHandle{f.truth}, rightFact))
	astTestMust(t, right.AddFact([]ast.ASTHandle{f.falsehood}, rightFact))
	joined, err := left.Join(right, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	count, err := joined.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("joined count", count)
	}
	row, found, err := joined.ExplanationRow([]ast.ASTHandle{f.truth, f.truth})
	astTestMust(t, err)
	if !found || len(row) != 2 || row[0] != leftFact || row[1] != rightFact {
		t.Fatal("joined explanation row", row, found)
	}
	_, found, err = joined.ExplanationRow([]ast.ASTHandle{f.truth, f.falsehood})
	astTestMust(t, err)
	if found {
		t.Fatal("unequal join keys matched")
	}
	if _, _, err = joined.Explanation([]ast.ASTHandle{f.truth, f.truth}); err == nil {
		t.Fatal("accepted multiple explanations as one")
	}
	projected, err := joined.Project([]int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	row, found, err = projected.ExplanationRow([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	if !found || len(row) != 2 || row[0] != leftFact || row[1] != rightFact {
		t.Fatal("projected joined row", row, found)
	}
	empty, err := newFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean}, 2)
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	astTestMust(t, empty.Union(joined, nil))
	row, found, err = empty.ExplanationRow([]ast.ASTHandle{f.truth, f.truth})
	astTestMust(t, err)
	if !found || len(row) != 2 || row[0] != leftFact || row[1] != rightFact {
		t.Fatal("union joined row", row, found)
	}
	blank, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, blank.Close()) }()
	astTestMust(t, blank.AddFact(nil, ast.ASTHandle{}))
	withBlank, err := joined.Join(blank, nil, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, withBlank.Close()) }()
	name, err := f.ast.Symbols.Intern("finite-rule-combine")
	astTestMust(t, err)
	combine, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	v0, err := f.ast.MkVar(0, sort)
	astTestMust(t, err)
	v1, err := f.ast.MkVar(1, sort)
	astTestMust(t, err)
	v2, err := f.ast.MkVar(2, sort)
	astTestMust(t, err)
	body, err := f.ast.MkApp(combine, []ast.ASTHandle{v0, v1})
	astTestMust(t, err)
	condition, err := f.ast.MkEq(v2, body)
	astTestMust(t, err)
	conditionRef, err := f.ast.NewRef(condition)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&conditionRef)) }()
	astTestMust(t, withBlank.AssignExplanation(condition))
	ruleResult, err := withBlank.ProjectExplanation([]int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, ruleResult.Close()) }()
	want, err := f.ast.MkApp(combine, []ast.ASTHandle{leftFact, rightFact})
	astTestMust(t, err)
	got, found, err := ruleResult.Explanation([]ast.ASTHandle{f.truth, f.truth})
	astTestMust(t, err)
	if !found || got != want {
		a0, e0 := f.ast.GetArg(got, 0)
		a1, e1 := f.ast.GetArg(got, 1)
		t.Fatal("constructed rule explanation", got, found, want, "args", a0, a1, "expected", leftFact, rightFact, "vars", v0, v1, v2, "errors", e0, e1)
	}
	if _, err = left.Join(right, []int{1}, []int{0}); err == nil {
		t.Fatal("accepted invalid join column")
	}
}

func TestFiniteExplanationProductRenameFinite(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("finite-rename-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(name, sort)
	astTestMust(t, err)
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	two := f.bv(2)
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth, two}, explanation))
	renamed, err := source.RenameFinite([]int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	got, found, err := renamed.Explanation([]ast.ASTHandle{two, f.truth})
	astTestMust(t, err)
	if !found || got != explanation {
		t.Fatal("renamed tuple", got, found)
	}
	_, found, err = source.Explanation([]ast.ASTHandle{f.truth, two})
	astTestMust(t, err)
	if !found {
		t.Fatal("source was mutated")
	}
	if _, err = source.RenameFinite([]int{0, 0}); err == nil {
		t.Fatal("accepted repeated rename column")
	}
}
