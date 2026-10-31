package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestNLPurificationPreparesGoalsAndClauses(t *testing.T) {
	for _, cores := range []bool{false, true} {
		name := "without-cores"
		if cores {
			name = "with-cores"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			x := analysisConst(t, m, real, "prepare-x")
			p := analysisConst(t, m, boolean, "prepare-p")
			h, err := m.ArithmeticRealNumeralText("0")
			zero := analysisKeep(t, m, h, err)
			h, err = m.MkArithmeticGE(x, zero)
			pure := analysisKeep(t, m, h, err)
			decl, err := m.MkFuncDecl(astTestSymbol(t, m, "prepare-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			h, err = m.MkApp(decl, []ast.ASTHandle{x})
			fx := analysisKeep(t, m, h, err)
			h, err = m.MkArithmeticLE(fx, zero)
			atom := analysisKeep(t, m, h, err)
			h, err = m.MkOr(atom, p)
			mixed := analysisKeep(t, m, h, err)
			goal, err := tactic.NewGoal(m, false, true, cores)
			astTestMust(t, err)
			defer func() { astTestMust(t, goal.Close()) }()
			astTestMust(t, goal.AssertWithProof(pure, ast.ASTHandle{}, ast.ASTDependency{}))
			astTestMust(t, goal.AssertWithProof(mixed, ast.ASTHandle{}, ast.ASTDependency{}))
			prep, err := prepareNLPurification(goal)
			astTestMust(t, err)
			defer func() { astTestMust(t, prep.Close()) }()
			if n, err := prep.nonlinear.Size(); err != nil || n != 2 {
				t.Fatal("nonlinear goal", n, err)
			}
			if len(prep.interfaces.smtDefinitions) != 1 {
				t.Fatal("SMT definitions", len(prep.interfaces.smtDefinitions))
			}
			wantPredicates := 1
			if cores {
				wantPredicates = 2
			}
			if len(prep.predicates.newPredicates) != wantPredicates {
				t.Fatal("predicates", len(prep.predicates.newPredicates), wantPredicates)
			}
			if len(prep.extraction.Clauses) != 2 {
				t.Fatal("SMT clauses", len(prep.extraction.Clauses))
			}
			first, err := goal.Form(0)
			astTestMust(t, err)
			if m.IsTrue(first) == cores {
				t.Fatal("pure arithmetic transfer", cores, first)
			}
			if depth, err := goal.Depth(); err != nil || depth != 1 {
				t.Fatal("goal depth", depth, err)
			}
		})
	}
}

func TestNLPurificationRejectsQuantifierAndCleansUp(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	h, err := m.MkVar(0, boolean)
	v := analysisKeep(t, m, h, err)
	h, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "prepare-bound")}, Body: v})
	quantifier := analysisKeep(t, m, h, err)
	goal, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	astTestMust(t, goal.AssertWithProof(quantifier, ast.ASTHandle{}, ast.ASTDependency{}))
	if prep, err := prepareNLPurification(goal); err == nil || prep != nil {
		t.Fatal("quantifier accepted", prep, err)
	}
}
