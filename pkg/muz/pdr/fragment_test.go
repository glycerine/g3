package pdr

// Source: test_diff_logic in src/muz/pdr/pdr_util.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestPDRDifferenceAndUTVPIFragmentClassification(t *testing.T) {
	manager, owner, transformer, x, closeBase := closureFixture(t)
	sort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("fragment-y", sort)
	closureMust(t, err)
	z, err := manager.MkFreshConst("fragment-z", sort)
	closureMust(t, err)
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	condition, err := manager.MkFreshConst("fragment-condition", boolean)
	closureMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	closureMust(t, err)
	one, err := manager.ArithmeticRealNumeralText("1")
	closureMust(t, err)
	two, err := manager.ArithmeticRealNumeralText("2")
	closureMust(t, err)
	three, err := manager.ArithmeticRealNumeralText("3")
	closureMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	closureMust(t, err)

	negativeY, err := manager.MkArithmeticMul(minusOne, y)
	closureMust(t, err)
	difference, err := manager.MkArithmeticAdd(x, negativeY)
	closureMust(t, err)
	differenceBound, err := manager.MkArithmeticLE(difference, three)
	closureMust(t, err)

	sum, err := manager.MkArithmeticAdd(x, y)
	closureMust(t, err)
	sumBound, err := manager.MkArithmeticLE(sum, three)
	closureMust(t, err)
	threeWay, err := manager.MkArithmeticAdd(sum, z)
	closureMust(t, err)
	threeWayBound, err := manager.MkArithmeticLE(threeWay, three)
	closureMust(t, err)
	twiceX, err := manager.MkArithmeticMul(two, x)
	closureMust(t, err)
	twiceBound, err := manager.MkArithmeticLE(twiceX, zero)
	closureMust(t, err)

	choice, err := manager.MkIte(condition, one, two)
	closureMust(t, err)
	choiceBound, err := manager.MkArithmeticLE(x, choice)
	closureMust(t, err)
	equality, err := manager.MkEq(x, y)
	closureMust(t, err)
	negativeEquality, err := manager.MkEq(negativeY, x)
	closureMust(t, err)

	rightNegativeY, err := manager.MkArithmeticMul(y, minusOne)
	closureMust(t, err)
	rightDifference, err := manager.MkArithmeticAdd(x, rightNegativeY)
	closureMust(t, err)
	rightDifferenceBound, err := manager.MkArithmeticLE(rightDifference, zero)
	closureMust(t, err)

	functionName, err := manager.Symbols.Intern("fragment-function")
	closureMust(t, err)
	function, err := manager.MkFuncDecl(functionName, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	application, err := manager.MkApp(function, []ast.ASTHandle{x})
	closureMust(t, err)

	checks := []struct {
		name       string
		expression ast.ASTHandle
		difference bool
		utvpi      bool
	}{
		{"difference", differenceBound, true, true},
		{"two-variable-positive", sumBound, false, true},
		{"three-variable", threeWayBound, false, false},
		{"coefficient-two", twiceBound, false, false},
		{"numeric-ite", choiceBound, true, true},
		{"variable-equality", equality, true, true},
		{"product-equality", negativeEquality, false, false},
		{"right-minus-one", rightDifferenceBound, false, true},
		{"nonboolean-uninterpreted-application", application, false, false},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			difference, classifyErr := IsDifferenceLogic(manager, []ast.ASTHandle{check.expression})
			closureMust(t, classifyErr)
			if difference != check.difference {
				t.Fatalf("difference logic = %t, want %t", difference, check.difference)
			}
			utvpi, classifyErr := IsUTVPILogic(manager, []ast.ASTHandle{check.expression})
			closureMust(t, classifyErr)
			if utvpi != check.utvpi {
				t.Fatalf("UTVPI = %t, want %t", utvpi, check.utvpi)
			}
		})
	}

	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func TestPDRContextSelectsArithmeticFragmentBeforeSolverSetup(t *testing.T) {
	for _, test := range []struct {
		name          string
		terms         int
		useUTVPI      bool
		convex        bool
		termITE       bool
		wantMode      uint32
		wantEqToIneq  bool
		wantDL, wantU bool
	}{
		{"difference", 2, true, false, false, smtcore.SMTArithmeticDifferenceLogic, true, true, false},
		{"difference-term-ite", 2, true, false, true, smtcore.SMTArithmeticDifferenceLogic, true, true, false},
		{"utvpi", -2, true, false, false, smtcore.SMTArithmeticUTVPI, true, false, true},
		{"general", 3, true, false, false, smtcore.SMTArithmeticSimplex, false, false, false},
		{"disabled", -2, false, false, false, smtcore.SMTArithmeticSimplex, false, false, false},
		{"convex-suppresses-specialization", -2, true, true, false, smtcore.SMTArithmeticSimplex, false, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, oldOwner, transformer, _, closeBase := closureFixture(t)
			config := oldOwner.contexts.config
			closureMust(t, transformer.Close())
			closureMust(t, oldOwner.Close())
			context, err := NewContext(manager, nil, 2, ReachableNoCache, config)
			closureMust(t, err)
			closureMust(t, context.SetUseUTVPI(test.useUTVPI))
			if test.convex {
				closureMust(t, context.SetUseConvexClosureGeneralizer(true))
			}
			arithmetic, err := manager.ArithmeticFamilyID()
			closureMust(t, err)
			realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
			closureMust(t, err)
			boolean, err := manager.MkBoolSort()
			closureMust(t, err)
			name, err := manager.Symbols.Intern("fragment-context-" + test.name)
			closureMust(t, err)
			declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{realSort, realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
			closureMust(t, err)
			rules, err := base.NewRuleManager(manager)
			closureMust(t, err)
			closureMust(t, rules.RegisterPredicate(declaration))
			set, err := base.NewRuleSet(rules)
			closureMust(t, err)
			x, err := manager.MkVar(0, realSort)
			closureMust(t, err)
			y, err := manager.MkVar(1, realSort)
			closureMust(t, err)
			z, err := manager.MkVar(2, realSort)
			closureMust(t, err)
			head, err := manager.MkApp(declaration, []ast.ASTHandle{x, y, z})
			closureMust(t, err)
			zero, err := manager.ArithmeticRealNumeralText("0")
			closureMust(t, err)
			var sum ast.ASTHandle
			if test.termITE {
				condition, xerr := manager.MkFreshConst("fragment-ite-condition", boolean)
				closureMust(t, xerr)
				one, xerr := manager.ArithmeticRealNumeralText("1")
				closureMust(t, xerr)
				two, xerr := manager.ArithmeticRealNumeralText("2")
				closureMust(t, xerr)
				choice, xerr := manager.MkIte(condition, one, two)
				closureMust(t, xerr)
				sum, err = manager.MkArithmeticAdd(x, choice)
			} else if test.terms < 0 {
				sum, err = manager.MkArithmeticAdd(x, y)
			} else {
				minusOne, xerr := manager.ArithmeticRealNumeralText("-1")
				closureMust(t, xerr)
				negativeY, xerr := manager.MkArithmeticMul(minusOne, y)
				closureMust(t, xerr)
				sum, err = manager.MkArithmeticAdd(x, negativeY)
				if test.terms == 3 {
					sum, err = manager.MkArithmeticAdd(sum, z)
				}
			}
			closureMust(t, err)
			bound, err := manager.MkArithmeticLE(sum, zero)
			closureMust(t, err)
			rule, err := rules.NewRule(head, []ast.ASTHandle{bound}, nil, "fragment")
			closureMust(t, err)
			closureMust(t, set.Add(rule))
			closureMust(t, rule.Close())
			closureMust(t, context.Initialize(set))
			if len(context.owner.contexts.solvers) == 0 {
				t.Fatal("context created no arithmetic solver")
			}
			state, err := context.owner.contexts.solvers[0].ArithmeticState()
			closureMust(t, err)
			if state.Mode != test.wantMode || state.Eq2Ineq != test.wantEqToIneq || state.BoundProp != 0 || !state.AutoConfigSimplex || state.PropagateEqs || state.EagerEqAxioms {
				t.Fatal("PDR arithmetic tuning", state)
			}
			difference, err := context.IsDifferenceLogic()
			closureMust(t, err)
			utvpi, err := context.IsUTVPI()
			closureMust(t, err)
			if difference != test.wantDL || utvpi != test.wantU || !context.proofActive || context.farkasActive {
				t.Fatal("PDR fragment/proof state", difference, utvpi, context.proofActive, context.farkasActive)
			}
			if test.termITE {
				transformer, found, xerr := context.Transformer(declaration)
				closureMust(t, xerr)
				if !found {
					t.Fatal("term-ITE transformer missing")
				}
				transition, xerr := transformer.Transition()
				closureMust(t, xerr)
				features, xerr := ast.NewStaticFeatures(manager)
				closureMust(t, xerr)
				closureMust(t, features.Collect(transition))
				if features.NumITETerms != 0 {
					t.Fatal("PDR special arithmetic retained a term ITE", features.NumITETerms)
				}
				closureMust(t, features.Close())
			}
			closureMust(t, context.Close())
			closureMust(t, set.Close())
			closureMust(t, rules.Close())
			closeBase()
		})
	}
}

func TestPropSolverRejectsFarkasLemmasOutsideSelectedFragment(t *testing.T) {
	manager, owner, transformer, x, closeBase := closureFixture(t)
	sort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("fragment-farkas-y", sort)
	closureMust(t, err)
	z, err := manager.MkFreshConst("fragment-farkas-z", sort)
	closureMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	closureMust(t, err)
	sum, err := manager.MkArithmeticAdd(x, y)
	closureMust(t, err)
	sumBound, err := manager.MkArithmeticLE(sum, zero)
	closureMust(t, err)
	threeWay, err := manager.MkArithmeticAdd(sum, z)
	closureMust(t, err)
	threeWayBound, err := manager.MkArithmeticLE(threeWay, zero)
	closureMust(t, err)
	sumRoot, err := manager.NewRef(sumBound)
	closureMust(t, err)
	threeWayRoot, err := manager.NewRef(threeWayBound)
	closureMust(t, err)
	for _, test := range []struct {
		name       string
		mode       uint32
		expression ast.ASTHandle
		want       bool
	}{
		{"difference-rejects-sum", smtcore.SMTArithmeticDifferenceLogic, sumBound, false},
		{"utvpi-accepts-sum", smtcore.SMTArithmeticUTVPI, sumBound, true},
		{"utvpi-rejects-three-way", smtcore.SMTArithmeticUTVPI, threeWayBound, false},
		{"simplex-unrestricted", smtcore.SMTArithmeticSimplex, threeWayBound, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner.contexts.arithmetic.effective = test.mode
			reference, err := manager.NewRef(test.expression)
			closureMust(t, err)
			inside, err := transformer.solver.theoryResultsInsideFragment([]ast.ASTRef{reference})
			closureMust(t, err)
			if inside != test.want {
				t.Fatal("fragment result", inside, "want", test.want)
			}
			closureMust(t, manager.DelRef(&reference))
		})
	}
	closureMust(t, manager.DelRef(&threeWayRoot))
	closureMust(t, manager.DelRef(&sumRoot))
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}
