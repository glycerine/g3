package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalSieveRenameMovesSelectedAndIgnoredColumns(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	real, err := f.ast.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	five, err := f.ast.ArithmeticRealNumeralText("5")
	if err != nil {
		t.Fatal(err)
	}
	astTestPin(t, f.ast, five)
	two := numeral("2")
	sorts := []ast.ASTHandle{f.boolean, integer, f.boolean, real}
	source, err := NewIntervalSieveEmpty(f.ast, sorts)
	if err != nil {
		t.Fatal(err)
	}
	if err = source.AddFact([]ast.ASTHandle{f.truth, two, f.falsehood, five}); err != nil {
		t.Fatal(err)
	}
	renamed, err := source.Rename([]int{0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if got := renamed.InnerColumns(); len(got) != 4 || !got[0] || got[1] || !got[2] || got[3] {
		t.Fatal("renamed selected mask", got)
	}
	gotSorts := renamed.sorts
	if len(gotSorts) != 4 || gotSorts[0] != integer || gotSorts[1] != f.boolean || gotSorts[2] != real || gotSorts[3] != f.boolean {
		t.Fatal("renamed sorts", gotSorts)
	}
	found, err := renamed.ContainsFact([]ast.ASTHandle{two, f.truth, five, f.falsehood})
	if err != nil || !found {
		t.Fatal("renamed tuple", found, err)
	}
	// Ignored coordinates remain unconstrained after moving.
	found, err = renamed.ContainsFact([]ast.ASTHandle{two, f.falsehood, five, f.truth})
	if err != nil || !found {
		t.Fatal("renamed ignored columns constrained", found, err)
	}
	found, err = source.ContainsFact([]ast.ASTHandle{f.falsehood, two, f.truth, five})
	if err != nil || !found {
		t.Fatal("rename modified source", found, err)
	}
}

func TestIntervalSieveRenameEmptyInputFollowsInnerProjectQuirkAndRejectsInvalidCycle(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	source, err := NewIntervalSieveEmpty(f.ast, []ast.ASTHandle{integer, f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := source.Rename([]int{0, 1})
	if err != nil || renamed.Empty() {
		t.Fatal("C++ interval rename turns the empty inner relation full", renamed, err)
	}
	if found, err := renamed.ContainsFact([]ast.ASTHandle{f.truth, numeral("2")}); err != nil || !found {
		t.Fatal("renamed full inner relation rejected an unconstrained tuple", found, err)
	}
	if _, err = source.Rename([]int{0, 0}); err == nil {
		t.Fatal("duplicate rename column accepted")
	}
	if _, err = source.Rename([]int{2}); err == nil {
		t.Fatal("out-of-range rename column accepted")
	}
	if !source.Empty() {
		t.Fatal("rejected rename mutated source")
	}
}
