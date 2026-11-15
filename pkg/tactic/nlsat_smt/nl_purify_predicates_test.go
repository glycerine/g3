package nlsatsmt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestNLPurifyPredicateRewriteRespectsPolarity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pol     polarity
		clauses int
	}{
		{"positive", positive, 1}, {"negative", negative, 1}, {"dual", dual, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x := analysisConst(t, m, real, "predicate-x")
			h, err := m.ArithmeticRealNumeralText("0")
			zero := analysisKeep(t, m, h, err)
			h, err = m.MkArithmeticLE(x, zero)
			atom := analysisKeep(t, m, h, err)
			goal, err := tactic.NewGoal(m, false, false, false)
			astTestMust(t, err)
			defer func() { astTestMust(t, goal.Close()) }()
			polarities := map[ast.ASTHandle]polarity{atom: tc.pol}
			r, err := newNLPredicateRewriter(m, goal, polarities)
			astTestMust(t, err)
			defer func() { astTestMust(t, r.Close()) }()
			// A second reference makes the atom shared, so must_cache lets
			// the second rewrite reuse the cached predicate.
			shared, err := m.NewRef(atom)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&shared)) }()
			pred, err := r.rewrite(atom)
			astTestMust(t, err)
			if len(r.newPredicates) != 1 || r.newPredicates[0] != pred || polarities[pred] != tc.pol {
				t.Fatal("interface predicate", r.newPredicates, polarities[pred])
			}
			if n, err := goal.Size(); err != nil || int(n) != tc.clauses {
				t.Fatal("clause count", n, err)
			}
			index := uint32(0)
			if tc.pol != negative {
				neg, err := m.MkNot(pred)
				astTestMust(t, err)
				want, err := m.MkOr(neg, atom)
				astTestMust(t, err)
				got, err := goal.Form(index)
				astTestMust(t, err)
				if got != want {
					t.Fatal("positive implication", got, want)
				}
				index++
			}
			if tc.pol != positive {
				neg, err := m.MkNot(atom)
				astTestMust(t, err)
				want, err := m.MkOr(pred, neg)
				astTestMust(t, err)
				got, err := goal.Form(index)
				astTestMust(t, err)
				if got != want {
					t.Fatal("negative implication", got, want)
				}
			}
			again, err := r.rewrite(atom)
			astTestMust(t, err)
			if again != pred || len(r.newPredicates) != 1 {
				t.Fatal("shared predicate", again, pred)
			}
		})
	}
}

func TestNLPurifyInterfaceAndPredicateGoalRewrites(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x := analysisConst(t, m, real, "mixed-x")
	p := analysisConst(t, m, boolean, "mixed-p")
	h, err := m.ArithmeticRealNumeralText("1")
	one := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticAdd(x, one)
	sum := analysisKeep(t, m, h, err)
	decl, err := m.MkFuncDecl(astTestSymbol(t, m, "mixed-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	h, err = m.MkApp(decl, []ast.ASTHandle{sum})
	fx := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticGE(fx, one)
	bound := analysisKeep(t, m, h, err)
	h, err = m.MkOr(bound, p)
	root := analysisKeep(t, m, h, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	nonlinear, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, nonlinear.Close()) }()
	astTestMust(t, goal.AssertWithProof(root, ast.ASTHandle{}, ast.ASTDependency{}))
	interfaces, err := newNLInterfaceRewriter(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, interfaces.Close()) }()
	astTestMust(t, interfaces.rewriteGoal(goal, nonlinear))
	rewritten, err := goal.Form(0)
	astTestMust(t, err)
	polarities, err := purificationPolarities(m, []ast.ASTHandle{rewritten})
	astTestMust(t, err)
	predicates, err := newNLPredicateRewriter(m, nonlinear, polarities)
	astTestMust(t, err)
	defer func() { astTestMust(t, predicates.Close()) }()
	astTestMust(t, predicates.rewriteGoal(goal))
	if len(interfaces.smtDefinitions) != 1 || len(predicates.newPredicates) != 1 {
		t.Fatal("interface results", len(interfaces.smtDefinitions), len(predicates.newPredicates))
	}
	if n, err := nonlinear.Size(); err != nil || n != 2 {
		t.Fatal("nonlinear definitions and implication", n, err)
	}
	result, err := goal.Form(0)
	astTestMust(t, err)
	left, err := m.GetArg(result, 0)
	astTestMust(t, err)
	if left != predicates.newPredicates[0] {
		t.Fatal("goal atom was not abstracted", left, predicates.newPredicates[0])
	}
}

// AUDIT.md tactic_small_01-F4: rewriter_core::must_cache never caches the
// root of a rewrite, so an atom asserted on its own and also shared inside
// another formula is abstracted twice.
func TestNLPurifyRootAtomGetsFreshPredicateLikeRewriterTpl(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	x := analysisConst(t, m, real, "root-x")
	y := analysisConst(t, m, real, "root-y")
	p := analysisConst(t, m, boolean, "root-p")
	h, err := m.MkArithmeticMul(x, x)
	square := analysisKeep(t, m, h, err)
	h, err = m.MkArithmeticLT(square, y)
	atom := analysisKeep(t, m, h, err)
	h, err = m.MkOr(atom, p)
	clause := analysisKeep(t, m, h, err)
	goal, err := tactic.NewGoal(m, false, true, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	astTestMust(t, goal.AssertWithProof(atom, ast.ASTHandle{}, ast.ASTDependency{}))
	astTestMust(t, goal.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}))
	prep, err := prepareNLPurification(goal)
	astTestMust(t, err)
	defer func() { astTestMust(t, prep.Close()) }()
	if len(prep.predicates.newPredicates) != 2 {
		t.Fatal("root atom shared the cached predicate", len(prep.predicates.newPredicates))
	}
}
