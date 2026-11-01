package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func formulaCheckerConfig(t *testing.T, m *ast.ASTManager) smtcore.SMTSolverConfig {
	t.Helper()
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	astTestMust(t, err)
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	astTestMust(t, err)
	astTestMust(t, globals.InitializeSource())
	t.Cleanup(func() {
		astTestMust(t, globals.Close())
		astTestMust(t, catalog.Close())
		astTestMust(t, context.Close())
	})
	return smtcore.SMTSolverConfig{Params: context, Globals: globals}
}

func TestFormulaCheckedTableFactsAndReset(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("initial empty", empty, err)
	}
	if empty, err := checked.FastEmpty(); err != nil || !empty {
		t.Fatal("initial fast empty", empty, err)
	}
	added, err := checked.AddFact([]ast.ASTHandle{f.truth})
	if err != nil || !added {
		t.Fatal("first fact", added, err)
	}
	if err = checked.AddNewFact([]ast.ASTHandle{f.truth}); err != nil {
		t.Fatal("new fact", err)
	}
	added, err = checked.AddFact([]ast.ASTHandle{f.truth})
	if err != nil || added {
		t.Fatal("duplicate fact", added, err)
	}
	if empty, err := checked.Empty(); err != nil || empty {
		t.Fatal("populated relation", empty, err)
	}
	if empty, err := checked.FastEmpty(); err != nil || empty {
		t.Fatal("populated fast empty", empty, err)
	}
	if found, err := checked.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("checked membership", found, err)
	}
	if found, err := checked.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || found {
		t.Fatal("checked non-membership", found, err)
	}
	clone, err := checked.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	// An underlying mutation without the wrapper makes its recorded formula
	// disagree with emptiness; the solver must find that counterexample.
	astTestMust(t, table.Reset())
	if _, err = checked.ContainsFact([]ast.ASTHandle{f.truth}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent membership", err)
	}
	if found, err := clone.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("clone changed with source", found, err)
	}
	if _, err = checked.Empty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent empty relation", err)
	}
	if _, err = checked.FastEmpty(); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed inconsistent fast empty relation", err)
	}
	astTestMust(t, checked.Reset())
	if empty, err := checked.Empty(); err != nil || !empty {
		t.Fatal("reset", empty, err)
	}
}

func TestFormulaCheckedTableFilters(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	for _, fact := range [][]ast.ASTHandle{
		{f.truth, f.truth}, {f.truth, f.falsehood}, {f.falsehood, f.falsehood},
	} {
		_, err := checked.AddFact(fact)
		astTestMust(t, err)
	}
	astTestMust(t, checked.FilterIdentical([]int{0, 1}))
	if table.NumRows() != 2 {
		t.Fatal("identical filter", table.NumRows())
	}
	astTestMust(t, checked.FilterEqual(f.truth, 0))
	if table.NumRows() != 1 {
		t.Fatal("equal filter", table.NumRows())
	}
	variable, err := f.ast.MkVar(1, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, variable)
	condition, err := f.ast.MkEq(variable, f.truth)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	theory := tableRelationTheoryRewriter(t, f.ast)
	fused, err := checked.FilterInterpretedAndProject(theory, condition, []int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, fused.Close()) }()
	if found, err := fused.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("interpreted filter and projection", found, err)
	}
	if found, err := fused.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || found {
		t.Fatal("interpreted filter and projection excluded tuple", found, err)
	}
	astTestMust(t, checked.FilterInterpreted(theory, condition))
	if table.NumRows() != 1 {
		t.Fatal("interpreted filter", table.NumRows())
	}
	astTestMust(t, table.Reset())
	if err = checked.FilterIdentical([]int{0, 1}); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed filter counterexample", err)
	}
}

func TestFormulaCheckedTableUnionAndDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	create := func() *FormulaCheckedTable {
		table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
		astTestMust(t, err)
		r, err := NewFormulaCheckedTable(table, config)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		return r
	}
	target, source, delta := create(), create(), create()
	_, err := target.AddFact([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	_, err = source.AddFact([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	_, err = delta.AddFact([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	astTestMust(t, target.UnionFrom(source, delta))
	for _, r := range []*FormulaCheckedTable{target, delta} {
		for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
			found, err := r.ContainsFact([]ast.ASTHandle{value})
			if err != nil || !found {
				t.Fatal("union or delta omitted fact", found, err)
			}
		}
	}
	astTestMust(t, target.UnionFrom(source, nil))
	if err = target.UnionFrom(source, target); err == nil {
		t.Fatal("aliased delta accepted")
	}
	if err = target.verifyUnion(f.falsehood, f.truth, f.truth, f.falsehood, f.falsehood, true); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed incomplete delta", err)
	}
}

func TestFormulaCheckedTableRename(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean, f.boolean})
	astTestMust(t, err)
	r, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	_, err = r.AddFact([]ast.ASTHandle{f.truth, f.falsehood, f.falsehood})
	astTestMust(t, err)
	renamed, err := r.RenameCycle([]int{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	if found, err := renamed.ContainsFact([]ast.ASTHandle{f.falsehood, f.falsehood, f.truth}); err != nil || !found {
		t.Fatal("renamed fact", found, err)
	}
	if found, err := r.ContainsFact([]ast.ASTHandle{f.truth, f.falsehood, f.falsehood}); err != nil || !found {
		t.Fatal("rename changed source", found, err)
	}
	if _, err = r.RenameCycle([]int{0, 0}); err == nil {
		t.Fatal("duplicate cycle accepted")
	}
}

func TestFormulaCheckedTableProjection(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean, f.boolean})
	astTestMust(t, err)
	r, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	_, err = r.AddFact([]ast.ASTHandle{f.truth, f.falsehood, f.truth})
	astTestMust(t, err)
	_, err = r.AddFact([]ast.ASTHandle{f.falsehood, f.falsehood, f.truth})
	astTestMust(t, err)
	projected, err := r.Project([]int{0, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if found, err := projected.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || !found {
		t.Fatal("projected fact", found, err)
	}
	if found, err := projected.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || found {
		t.Fatal("unexpected projected fact", found, err)
	}
	sourceFormula, err := r.table.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&sourceFormula)) }()
	sourceHandle, err := f.ast.RefGet(sourceFormula)
	astTestMust(t, err)
	existential, err := r.projectFormula(sourceHandle, []int{0, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&existential)) }()
	expected, err := f.ast.RefGet(existential)
	astTestMust(t, err)
	if err = projected.verifyGrounded(expected, f.falsehood, false); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed existential projection counterexample", err)
	}
	all, err := r.Project([]int{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, all.Close()) }()
	if found, err := all.ContainsFact(nil); err != nil || !found {
		t.Fatal("zero-column projection", found, err)
	}
	if _, err = r.Project([]int{1, 1}); err == nil {
		t.Fatal("duplicate projected columns accepted")
	}
}

func TestFormulaCheckedTableJoin(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	create := func() *FormulaCheckedTable {
		table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
		astTestMust(t, err)
		r, err := NewFormulaCheckedTable(table, config)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		return r
	}
	left, right := create(), create()
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		_, err := left.AddFact([]ast.ASTHandle{value})
		astTestMust(t, err)
	}
	_, err := right.AddFact([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	joined, err := left.Join(right, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, joined.Close()) }()
	if found, err := joined.ContainsFact([]ast.ASTHandle{f.falsehood, f.falsehood}); err != nil || !found {
		t.Fatal("joined tuple", found, err)
	}
	if found, err := joined.ContainsFact([]ast.ASTHandle{f.truth, f.falsehood}); err != nil || found {
		t.Fatal("join equality", found, err)
	}
	projected, err := joined.Project([]int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if found, err := projected.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || !found {
		t.Fatal("join projection", found, err)
	}
	combined, err := left.JoinProject(right, []int{0}, []int{0}, []int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, combined.Close()) }()
	if found, err := combined.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || !found {
		t.Fatal("combined join projection", found, err)
	}
	if found, err := combined.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || found {
		t.Fatal("combined join projection excluded tuple", found, err)
	}
	formula, err := left.joinFormula(right, []int{0}, []int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	expected, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	if err = joined.verifyGrounded(expected, f.falsehood, false); !errors.Is(err, ErrRelationVerification) {
		t.Fatal("missed join counterexample", err)
	}
	if _, err = left.Join(right, []int{1}, []int{0}); err == nil {
		t.Fatal("invalid join column accepted")
	}
}

func TestFormulaCheckedTableFilterByNegation(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	create := func(sorts []ast.ASTHandle) *FormulaCheckedTable {
		table, err := NewTableRelation(f.relations, sorts)
		astTestMust(t, err)
		checked, err := NewFormulaCheckedTable(table, config)
		astTestMust(t, err)
		return checked
	}
	target := create([]ast.ASTHandle{f.boolean})
	defer func() { astTestMust(t, target.Close()) }()
	negative := create([]ast.ASTHandle{f.boolean, f.boolean})
	defer func() { astTestMust(t, negative.Close()) }()
	_, err := target.AddFact([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	_, err = target.AddFact([]ast.ASTHandle{f.falsehood})
	astTestMust(t, err)
	_, err = negative.AddFact([]ast.ASTHandle{f.falsehood, f.truth})
	astTestMust(t, err)
	astTestMust(t, target.FilterByNegation(negative, []int{0}, []int{0}))
	if found, err := target.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || !found {
		t.Fatal("unmatched target row", found, err)
	}
	if found, err := target.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || found {
		t.Fatal("removed target row", found, err)
	}
}

func TestFormulaCheckedTableComplement(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(table, config)
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	_, err = checked.AddFact([]ast.ASTHandle{f.truth})
	astTestMust(t, err)
	complement, err := checked.Complement()
	astTestMust(t, err)
	defer func() { astTestMust(t, complement.Close()) }()
	if found, err := complement.ContainsFact([]ast.ASTHandle{f.falsehood}); err != nil || !found {
		t.Fatal("complement member", found, err)
	}
	if found, err := complement.ContainsFact([]ast.ASTHandle{f.truth}); err != nil || found {
		t.Fatal("complement nonmember", found, err)
	}
	formula, err := complement.ToFormula()
	astTestMust(t, err)
	astTestMust(t, f.ast.DelRef(&formula))
}
