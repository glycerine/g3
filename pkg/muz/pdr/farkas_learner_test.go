package pdr_test

// Source: src/muz/pdr/pdr_farkas_learner.cpp Farkas-lemma recognition and
// combine_constraints coefficient and literal modes.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func farkasFixture(t *testing.T) (*cmd_context.Context, *ast.ASTManager, *pdr.FarkasLearner) {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	pdrMust(t, manager.ToggleProofMode(ast.ASTProofEnabled))
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	learner, err := pdr.NewFarkasLearner(manager, globals.Context, globals)
	pdrMust(t, err)
	return command, manager, learner
}

func TestFarkasLearnerRecognizesArithmeticTheoryLemma(t *testing.T) {
	command, manager, learner := farkasFixture(t)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	pdrMust(t, err)
	xName, err := manager.Symbols.Intern("farkas-x")
	pdrMust(t, err)
	x, err := manager.MkConst(xName, integer)
	pdrMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	pdrMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	pdrMust(t, err)
	le, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{x, zero})
	pdrMust(t, err)
	ge, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{x, one})
	pdrMust(t, err)
	premise0, err := manager.MkProofAsserted(le)
	pdrMust(t, err)
	premise1, err := manager.MkProofAsserted(ge)
	pdrMust(t, err)
	falsity, err := manager.MkFalse()
	pdrMust(t, err)
	farkasName, err := manager.Symbols.Intern("farkas")
	pdrMust(t, err)
	farkas, err := manager.NewParameterSymbol(farkasName)
	pdrMust(t, err)
	coefficientValue, err := manager.NumeralService().NewValue()
	pdrMust(t, err)
	pdrMust(t, manager.NumeralService().SetString(&coefficientValue, "-3/2"))
	coefficient0, err := manager.NewParameterRational(&coefficientValue)
	pdrMust(t, err)
	pdrMust(t, manager.NumeralService().SetString(&coefficientValue, "5"))
	coefficient1, err := manager.NewParameterRational(&coefficientValue)
	pdrMust(t, err)
	pdrMust(t, manager.NumeralService().SetString(&coefficientValue, "7"))
	conclusionCoefficient, err := manager.NewParameterRational(&coefficientValue)
	pdrMust(t, err)
	pdrMust(t, manager.NumeralService().Del(&coefficientValue))
	lemma, err := manager.MkProofTheoryLemma(arithmetic, falsity, []ast.ASTHandle{premise0, premise1}, []ast.ASTParameter{farkas, coefficient0, coefficient1})
	pdrMust(t, err)
	lemmaWithConclusion, err := manager.MkProofTheoryLemma(arithmetic, le, []ast.ASTHandle{premise0, premise1}, []ast.ASTParameter{farkas, coefficient0, coefficient1, conclusionCoefficient})
	pdrMust(t, err)
	pdrMust(t, manager.DelParameter(&conclusionCoefficient))
	pdrMust(t, manager.DelParameter(&coefficient1))
	pdrMust(t, manager.DelParameter(&coefficient0))
	pdrMust(t, manager.DelParameter(&farkas))

	matched, err := learner.IsFarkasLemma(lemma)
	pdrMust(t, err)
	if !matched {
		t.Fatal("arithmetic Farkas theory lemma was not recognized")
	}
	facts, coefficients, matched, err := learner.FarkasPremises(lemma)
	pdrMust(t, err)
	if !matched || len(facts) != 2 || facts[0] != le || facts[1] != ge {
		t.Fatal("Farkas parent facts were not extracted in proof order", matched, facts)
	}
	q := manager.NumeralService().Manager
	threeHalves := numeral.Rational{Num: numeral.NewInt(3), Den: numeral.NewInt(2)}
	five := numeral.Rational{Num: numeral.NewInt(5), Den: numeral.NewInt(1)}
	if c0, e0 := q.Compare(&coefficients[0], &threeHalves); e0 != nil || c0 != 0 {
		t.Fatal("Farkas coefficients were not extracted and normalized", coefficients)
	}
	if c1, e1 := q.Compare(&coefficients[1], &five); e1 != nil || c1 != 0 {
		t.Fatal("Farkas coefficients were not extracted and normalized", coefficients)
	}
	pdrMust(t, learner.DelCoefficients(coefficients))
	combined, produced, err := learner.CombineFarkasLemma(lemma, []bool{true, false})
	pdrMust(t, err)
	if !produced {
		t.Fatal("B-pure Farkas parent did not produce a combined lemma")
	}
	combinedHandle, err := manager.RefGet(combined)
	pdrMust(t, err)
	if combinedHandle.IsNull() {
		t.Fatal("Farkas theory-lemma reduction returned a null lemma")
	}
	pdrMust(t, manager.DelRef(&combined))
	combined, produced, err = learner.CombineFarkasLemma(lemma, []bool{false, false})
	pdrMust(t, err)
	if produced || combined != (ast.ASTRef{}) {
		t.Fatal("Farkas theory lemma without a B-pure parent produced output")
	}
	combined, produced, err = learner.CombineFarkasLemma(lemmaWithConclusion, []bool{true, false})
	pdrMust(t, err)
	if !produced {
		t.Fatal("Farkas conclusion coefficient did not produce a combined lemma")
	}
	pdrMust(t, manager.DelRef(&combined))
	lemmaRoot, err := manager.NewRef(lemma)
	pdrMust(t, err)
	lemmas, err := learner.GetLemmas(lemma, []ast.ASTHandle{le})
	pdrMust(t, err)
	if len(lemmas) != 1 {
		t.Fatal("proof traversal did not recover the Farkas lemma", len(lemmas))
	}
	pdrMust(t, manager.DelRef(&lemmas[0]))
	lemmas, err = learner.GetLemmas(lemma, nil)
	pdrMust(t, err)
	if len(lemmas) != 0 {
		t.Fatal("proof traversal produced a lemma without a B dependency", len(lemmas))
	}
	consequences, err := learner.GetConsequences(lemma, []ast.ASTHandle{le})
	pdrMust(t, err)
	if len(consequences) != 1 {
		t.Fatal("consequence traversal did not recover the Farkas consequence", len(consequences))
	}
	consequence, err := manager.RefGet(consequences[0])
	pdrMust(t, err)
	isNot, argument, err := manager.IsNot(consequence)
	pdrMust(t, err)
	if !isNot || argument != ge {
		t.Fatal("consequence mode did not negate the remaining literal", isNot, argument, ge)
	}
	pdrMust(t, manager.DelRef(&consequences[0]))
	pdrMust(t, manager.DelRef(&lemmaRoot))
	matched, err = learner.IsFarkasLemma(premise0)
	pdrMust(t, err)
	if matched {
		t.Fatal("asserted proof was recognized as a Farkas lemma")
	}

	pdrMust(t, learner.Close())
	pdrMust(t, command.Close())
}

func TestFarkasLearnerCombinesCoefficientsAndNegatedLiterals(t *testing.T) {
	command, manager, learner := farkasFixture(t)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	pdrMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	pdrMust(t, err)
	xName, err := manager.Symbols.Intern("combine-x")
	pdrMust(t, err)
	x, err := manager.MkConst(xName, integer)
	pdrMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	pdrMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	pdrMust(t, err)
	le, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{x, zero})
	pdrMust(t, err)
	ge, err := manager.MkFamilyApp(arithmetic, 3, nil, []ast.ASTHandle{x, one})
	pdrMust(t, err)
	combined, err := learner.CombineConstraints([]ast.ASTHandle{le, ge}, pdrOnes(2))
	pdrMust(t, err)
	combinedHandle, err := manager.RefGet(combined)
	pdrMust(t, err)
	if combinedHandle.IsNull() {
		t.Fatal("coefficient combination returned a null constraint")
	}
	pdrMust(t, manager.DelRef(&combined))

	xSplitName, err := manager.Symbols.Intern("combine-split-x")
	pdrMust(t, err)
	xSplit, err := manager.MkConst(xSplitName, integer)
	pdrMust(t, err)
	yName, err := manager.Symbols.Intern("combine-y")
	pdrMust(t, err)
	y, err := manager.MkConst(yName, integer)
	pdrMust(t, err)
	splitZero, err := manager.ArithmeticIntegerNumeralText("0")
	pdrMust(t, err)
	yLE, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{y, splitZero})
	pdrMust(t, err)
	xLE, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{xSplit, splitZero})
	pdrMust(t, err)
	pdrMust(t, learner.SetSplitLiterals(true))
	split, err := learner.CombineConstraints([]ast.ASTHandle{xLE, yLE}, pdrOnes(2))
	pdrMust(t, err)
	splitHandle, err := manager.RefGet(split)
	pdrMust(t, err)
	if family, err := manager.GetFamilyID(splitHandle); err != nil || family != manager.BasicFamilyID() {
		t.Fatalf("split result family = %d, err %v", family, err)
	}
	if op, err := manager.GetDeclKind(splitHandle); err != nil || op != ast.ASTOpOr {
		t.Fatalf("split result op = %d, err %v", op, err)
	}
	if count, err := manager.GetNumArgs(splitHandle); err != nil || count != 2 {
		t.Fatalf("split result count = %d, err %v", count, err)
	}
	pdrMust(t, manager.DelRef(&split))

	pdrMust(t, learner.SetCombineCoefficients(false))
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	pName, err := manager.Symbols.Intern("combine-p")
	pdrMust(t, err)
	qName, err := manager.Symbols.Intern("combine-q")
	pdrMust(t, err)
	p, err := manager.MkConst(pName, boolean)
	pdrMust(t, err)
	q, err := manager.MkConst(qName, boolean)
	pdrMust(t, err)
	negated, err := learner.CombineConstraints([]ast.ASTHandle{p, q}, pdrOnes(2))
	pdrMust(t, err)
	negatedHandle, err := manager.RefGet(negated)
	pdrMust(t, err)
	isNot, _, err := manager.IsNot(negatedHandle)
	pdrMust(t, err)
	if !isNot {
		t.Fatal("literal combination did not produce not(or(literals))")
	}
	pdrMust(t, manager.DelRef(&negated))

	pdrMust(t, learner.Close())
	pdrMust(t, command.Close())
}

func TestFarkasLearnerRecoversGeneralBPureUnits(t *testing.T) {
	command, manager, learner := farkasFixture(t)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	pName, err := manager.Symbols.Intern("unit-b")
	pdrMust(t, err)
	p, err := manager.MkConst(pName, boolean)
	pdrMust(t, err)
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	bProof, err := manager.MkProofAsserted(p)
	pdrMust(t, err)
	aProof, err := manager.MkProofAsserted(notP)
	pdrMust(t, err)
	root, err := manager.MkProofUnitResolution([]ast.ASTHandle{bProof, aProof})
	pdrMust(t, err)
	lemmas, err := learner.GetLemmas(root, []ast.ASTHandle{p})
	pdrMust(t, err)
	if len(lemmas) != 1 {
		t.Fatal("mixed proof did not recover its B-pure unit", len(lemmas))
	}
	lemma, err := manager.RefGet(lemmas[0])
	pdrMust(t, err)
	if lemma != p {
		t.Fatal("recovered the wrong B-pure unit", lemma, p)
	}
	pdrMust(t, manager.DelRef(&lemmas[0]))
	pdrMust(t, learner.Close())
	pdrMust(t, command.Close())
}

// pdrOnes is n coefficients rational(1).
func pdrOnes(n int) []numeral.Rational {
	ones := make([]numeral.Rational, n)
	for i := range ones {
		ones[i] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
	}
	return ones
}
