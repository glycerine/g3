package interp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func directProofFixture(t *testing.T) (*ast.ASTManager, *Manager, *Base, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, manager := interpProofManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("direct-a", boolean)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("direct-b", boolean)
	interpMust(t, err)
	q = interpPin(t, m, q)
	base, err := NewBase(manager, []ast.ASTHandle{p, q}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	return m, manager, base, p, q
}

func TestDirectProofTermSidesAndAssumptions(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	mixed, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), true)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	for _, test := range []struct {
		term ast.ASTHandle
		want TermSide
	}{{p, TermA}, {q, TermB}, {mixed, TermMixed}} {
		if got, e := direct.TermSide(test.term); e != nil || got != test.want {
			t.Fatalf("term side = %v, want %v: %v", got, test.want, e)
		}
	}
	inA, err := direct.MakeAssumption(0, []ast.ASTHandle{p, q, mixed})
	interpMust(t, err)
	if op, e := manager.Operator(inA); e != nil || op != OperatorOr {
		t.Fatal("A assumption", op, e)
	}
	args, err := manager.Args(inA)
	interpMust(t, err)
	if len(args) != 2 || args[0] != q || args[1] != mixed {
		t.Fatal("A assumption clause", args)
	}
	inB, err := direct.MakeAssumption(1, []ast.ASTHandle{p, q, mixed})
	interpMust(t, err)
	if truth, e := manager.IsTrue(inB); e != nil || !truth {
		t.Fatal("B assumption", truth, e)
	}
}

func TestDirectProofHypothesisReflexivityAndContra(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	hypA, err := direct.MakeHypothesis(p)
	interpMust(t, err)
	if no, e := manager.IsFalse(hypA); e != nil || !no {
		t.Fatal("A hypothesis", no, e)
	}
	hypB, err := direct.MakeHypothesis(q)
	interpMust(t, err)
	if yes, e := manager.IsTrue(hypB); e != nil || !yes {
		t.Fatal("B hypothesis", yes, e)
	}
	mixed, err := manager.Make(OperatorEqual, p, q)
	interpMust(t, err)
	hypMixed, err := direct.MakeHypothesis(mixed)
	interpMust(t, err)
	hypMixedAgain, err := direct.MakeHypothesis(mixed)
	interpMust(t, err)
	if hypMixed != hypMixedAgain {
		t.Fatal("mixed hypothesis did not reuse placeholders", hypMixed, hypMixedAgain)
	}
	if op, e := manager.Operator(hypMixed); e != nil || op != OperatorAnd {
		t.Fatal("mixed hypothesis", op, e)
	}
	reflA, err := direct.MakeReflexivity(p)
	interpMust(t, err)
	if no, _ := manager.IsFalse(reflA); !no {
		t.Fatal("A reflexivity")
	}
	reflB, err := direct.MakeReflexivity(q)
	interpMust(t, err)
	if yes, _ := manager.IsTrue(reflB); !yes {
		t.Fatal("B reflexivity")
	}
	reflMixed, err := direct.MakeReflexivity(mixed)
	interpMust(t, err)
	if op, e := manager.Operator(reflMixed); e != nil || op != OperatorAnd {
		t.Fatal("mixed reflexivity", op, e)
	}
	contra, err := direct.MakeContra(hypMixed, []ast.ASTHandle{mixed})
	interpMust(t, err)
	if contra != hypMixed {
		t.Fatal("contra changed proof term", contra, hypMixed)
	}
}

func TestDirectProofArithmeticHypothesisAndGuards(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("direct-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("direct-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	xFrame, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	xFrame = interpPin(t, m, xFrame)
	yFrame, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	yFrame = interpPin(t, m, yFrame)
	base, err := NewBase(manager, []ast.ASTHandle{xFrame, yFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	mixed, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	hypothesis, err := direct.MakeHypothesis(mixed)
	interpMust(t, err)
	if op, e := manager.Operator(hypothesis); e != nil || op != OperatorAnd {
		t.Fatal("mixed arithmetic hypothesis", op, e)
	}
	copy := *direct
	if err = copy.Close(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copy guard", err)
	}
	interpMust(t, direct.Close())
	if _, err = direct.TermSide(mixed); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed guard", err)
	}
}

func TestDirectProofLocalResolutionAndSymmetry(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	resolvedA, err := direct.MakeResolution(p, nil, falsehood, q)
	interpMust(t, err)
	if resolvedA != q {
		t.Fatal("A resolution", resolvedA, q)
	}
	resolvedB, err := direct.MakeResolution(q, nil, truth, p)
	interpMust(t, err)
	if resolvedB != p {
		t.Fatal("B resolution", resolvedB, p)
	}
	if symmetric, e := direct.MakeSymmetry(p, p, q); e != nil || symmetric != q {
		t.Fatal("local symmetry", symmetric, e)
	}
	mixed, err := manager.Make(OperatorEqual, p, q)
	interpMust(t, err)
	if resolved, e := direct.MakeResolution(mixed, nil, truth, falsehood); e != nil || resolved != falsehood {
		t.Fatal("mixed resolution traversal", resolved, e)
	}
	reversed, err := manager.Make(OperatorEqual, q, p)
	interpMust(t, err)
	mixedPremise, err := direct.MakeHypothesis(mixed)
	interpMust(t, err)
	mixedSymmetry, err := direct.MakeSymmetry(reversed, mixed, mixedPremise)
	interpMust(t, err)
	if mixedSymmetry.IsNull() {
		t.Fatal("mixed symmetry returned null proof")
	}
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	notQ, err := manager.MkNot(q)
	interpMust(t, err)
	congruentConclusion, err := manager.Make(OperatorEqual, notP, notQ)
	interpMust(t, err)
	mixedCongruence, err := direct.MakeCongruence(mixed, congruentConclusion, mixedPremise)
	interpMust(t, err)
	if mixedCongruence.IsNull() {
		t.Fatal("mixed congruence returned null proof")
	}
}

func TestDirectProofLocalArithmeticRules(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("rules-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("rules-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	xLeqZero, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	xLeqZero = interpPin(t, m, xLeqZero)
	yLeqZero, err := manager.Make(OperatorLE, y, zero)
	interpMust(t, err)
	yLeqZero = interpPin(t, m, yLeqZero)
	base, err := NewBase(manager, []ast.ASTHandle{xLeqZero, yLeqZero}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	farkas, err := direct.MakeFarkas(falsehood, []ast.ASTHandle{falsehood, truth}, []ast.ASTHandle{xLeqZero, yLeqZero}, []ast.ASTHandle{one, one})
	interpMust(t, err)
	want, err := manager.SumInequalities([]ast.ASTHandle{one}, []ast.ASTHandle{xLeqZero}, true)
	interpMust(t, err)
	if farkas != want {
		t.Fatal("local Farkas", farkas, want)
	}
	leqEqA, err := direct.MakeLeqToEq(x, zero, xLeqZero, xLeqZero)
	interpMust(t, err)
	if no, _ := manager.IsFalse(leqEqA); !no {
		t.Fatal("A leq-to-eq")
	}
	leqEqB, err := direct.MakeLeqToEq(y, zero, yLeqZero, yLeqZero)
	interpMust(t, err)
	if yes, _ := manager.IsTrue(leqEqB); !yes {
		t.Fatal("B leq-to-eq")
	}
	eqLeqA, err := direct.MakeEqToLeq(x, zero, xLeqZero)
	interpMust(t, err)
	if no, _ := manager.IsFalse(eqLeqA); !no {
		t.Fatal("A eq-to-leq")
	}
	eqLeqB, err := direct.MakeEqToLeq(y, zero, yLeqZero)
	interpMust(t, err)
	if yes, _ := manager.IsTrue(eqLeqB); !yes {
		t.Fatal("B eq-to-leq")
	}
	cutA, err := direct.MakeCutRule(xLeqZero, one, xLeqZero, yLeqZero)
	interpMust(t, err)
	if cutA != yLeqZero {
		t.Fatal("A cut", cutA, yLeqZero)
	}
	cutB, err := direct.MakeCutRule(yLeqZero, one, yLeqZero, xLeqZero)
	interpMust(t, err)
	if cutB != xLeqZero {
		t.Fatal("B cut", cutB, xLeqZero)
	}
	if _, err = direct.MakeFarkas(falsehood, nil, []ast.ASTHandle{xLeqZero}, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("Farkas length guard", err)
	}
	mixed, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	mixedEqToLeq, err := direct.MakeEqToLeq(x, y, mixed)
	interpMust(t, err)
	if op, e := manager.Operator(mixedEqToLeq); e != nil || op != OperatorAnd {
		t.Fatal("mixed eq-to-leq", op, e)
	}
	yLeqX, err := manager.Make(OperatorLE, y, x)
	interpMust(t, err)
	mixedLeqToEq, err := direct.MakeLeqToEq(x, y, mixed, yLeqX)
	interpMust(t, err)
	if op, e := manager.Operator(mixedLeqToEq); e != nil || op != OperatorAnd {
		t.Fatal("mixed leq-to-eq", op, e)
	}
	rotatedContra, err := manager.Arg(mixedLeqToEq, 1)
	interpMust(t, err)
	rotatedProof, err := manager.Arg(rotatedContra, 0)
	interpMust(t, err)
	declaration, err := manager.Symbol(rotatedProof)
	interpMust(t, err)
	if declaration != direct.rotateSum {
		t.Fatal("mixed leq-to-eq rotation", declaration, direct.rotateSum)
	}
	mixedPremise, err := direct.MakeHypothesis(mixed)
	interpMust(t, err)
	mixedFarkas, err := direct.MakeFarkas(falsehood, []ast.ASTHandle{mixedPremise}, []ast.ASTHandle{mixed}, []ast.ASTHandle{one})
	interpMust(t, err)
	if mixedFarkas.IsNull() {
		t.Fatal("mixed Farkas returned null proof")
	}
	mixedCut, err := direct.MakeCutRule(mixed, one, mixed, mixedPremise)
	interpMust(t, err)
	if mixedCut.IsNull() {
		t.Fatal("mixed cut returned null proof")
	}
}

func TestDirectProofFinalSimplificationOwnership(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("final-proof-p", boolean)
	interpMust(t, err)
	p = interpPin(t, m, p)
	base, err := NewBase(manager, []ast.ASTHandle{p}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	proof, err := manager.Make(OperatorAnd, p, truth)
	interpMust(t, err)
	result, err := direct.Interpolate(proof)
	interpMust(t, err)
	localized, err := direct.freshLocalizationVariable(p, 0)
	interpMust(t, err)
	localizedResult, err := direct.Interpolate(localized)
	interpMust(t, err)
	localizedHandle, err := m.RefGet(localizedResult)
	interpMust(t, err)
	if op, e := manager.Operator(localizedHandle); e != nil || op != OperatorExists {
		t.Fatal("final localization quantifier", op, e)
	}
	interpMust(t, m.DelRef(&localizedResult))
	interpMust(t, direct.Close())
	got, err := m.RefGet(result)
	interpMust(t, err)
	if got != p {
		t.Fatal("final simplified proof", got, p)
	}
	interpMust(t, m.DelRef(&result))
	if _, err = direct.Interpolate(proof); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed finalizer guard", err)
	}
}

func TestDirectProofLocalizationQuantifierInsertion(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	localA, err := direct.freshLocalizationVariable(p, 0)
	interpMust(t, err)
	localB, err := direct.freshLocalizationVariable(q, 1)
	interpMust(t, err)
	body, err := manager.Make(OperatorAnd, localA, localB)
	interpMust(t, err)
	quantified, err := direct.addLocalizationQuantifiers(body)
	interpMust(t, err)
	if op, e := manager.Operator(quantified); e != nil || op != OperatorExists {
		t.Fatal("outer localization quantifier", op, e)
	}
	if len(direct.localizationVariables) != 2 || direct.localizationVariables[0].term != p || direct.localizationVariables[1].term != q {
		t.Fatal("localization metadata", direct.localizationVariables)
	}
}

func TestDirectProofRecursiveAtomicLocalization(t *testing.T) {
	_, manager, base, _, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	localized, proof, err := direct.localizeTerm(q, DownwardScopeRange(0))
	interpMust(t, err)
	if localized == q || localized.IsNull() || proof.IsNull() {
		t.Fatal("atomic localization", localized, proof, q)
	}
	again, againProof, err := direct.localizeTerm(q, DownwardScopeRange(0))
	interpMust(t, err)
	if again != localized || againProof != proof {
		t.Fatal("localization memo", again, againProof, localized, proof)
	}
	if len(direct.localizationVariables) != 1 || direct.localizationVariables[0].term != q {
		t.Fatal("atomic localization metadata", direct.localizationVariables)
	}
	notQ, err := manager.MkNot(q)
	interpMust(t, err)
	localizedNot, compositeProof, err := direct.localizeTerm(notQ, DownwardScopeRange(0))
	interpMust(t, err)
	if compositeProof.IsNull() {
		t.Fatal("composite localization proof")
	}
	argument, err := manager.Arg(localizedNot, 0)
	interpMust(t, err)
	if argument != localized || len(direct.localizationVariables) != 1 {
		t.Fatal("recursive localization", localizedNot, argument, localized, direct.localizationVariables)
	}
}

func TestDirectProofTermSubstitutionAndBarriers(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	conjunction, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	replaced, err := direct.SubstituteProofTerm(p, q, conjunction)
	interpMust(t, err)
	replacedArgs, err := manager.Args(replaced)
	interpMust(t, err)
	if len(replacedArgs) != 2 || replacedArgs[0] != q || replacedArgs[1] != q {
		t.Fatal("conjunction substitution", replacedArgs, q)
	}
	equality, err := manager.Make(OperatorEqual, p, p)
	interpMust(t, err)
	equality, err = direct.SubstituteProofTerm(p, q, equality)
	interpMust(t, err)
	if truth, e := manager.IsTrue(equality); e != nil || !truth {
		t.Fatal("reflexive equality substitution", truth, e)
	}
	chain, err := direct.makeSymbol(direct.concat, p, q)
	interpMust(t, err)
	unchangedChain, err := direct.SubstituteProofTerm(p, q, chain)
	interpMust(t, err)
	if unchangedChain != chain {
		t.Fatal("rewrite chain barrier", unchangedChain, chain)
	}
	pivotPlaceholder, err := direct.placeholder(p)
	interpMust(t, err)
	proof, err := manager.Make(OperatorAnd, pivotPlaceholder, q)
	interpMust(t, err)
	rotation, err := direct.makeSymbol(direct.rotateSum, p, proof)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	unchangedRotation, err := direct.SubstituteProofTerm(pivotPlaceholder, truth, rotation)
	interpMust(t, err)
	if unchangedRotation != rotation {
		t.Fatal("rotation pivot barrier", unchangedRotation, rotation)
	}
	changedRotation, err := direct.SubstituteProofTerm(q, truth, rotation)
	interpMust(t, err)
	if changedRotation == rotation {
		t.Fatal("rotation proof was not substituted")
	}
}

func TestDirectProofContraRecognitionAndResolvents(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	equality, err := manager.Make(OperatorEqual, p, q)
	interpMust(t, err)
	negativeEquality, err := manager.MkNot(equality)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	conjunction, err := direct.makeContraNode(truth, []ast.ASTHandle{negativeEquality, p}, 0)
	interpMust(t, err)
	pivot, found, err := direct.contraPivot(negativeEquality, conjunction)
	interpMust(t, err)
	if !found {
		t.Fatal("negative equality pivot not found")
	}
	if literal, e := manager.Arg(pivot, 1); e != nil || literal != negativeEquality {
		t.Fatal("recognized pivot", literal, e)
	}
	entries, err := manager.Args(conjunction)
	interpMust(t, err)
	if count, e := direct.countNegativeEqualities(entries[1:]); e != nil || count != 1 {
		t.Fatal("negative equality count", count, e)
	}
	var resolvents []ast.ASTHandle
	interpMust(t, direct.collectContraResolvents(0, entries[1], entries[2], conjunction, &resolvents))
	if len(resolvents) != 2 {
		t.Fatal("resolvent count", len(resolvents))
	}
	for _, resolvent := range resolvents {
		proof, e := manager.Arg(resolvent, 0)
		interpMust(t, e)
		if proof != direct.noProof {
			t.Fatal("no-proof resolvent", proof, direct.noProof)
		}
	}
	positiveConjunction, err := direct.makeContraNode(truth, []ast.ASTHandle{equality, p}, 1)
	interpMust(t, err)
	negativeConjunction, err := direct.makeContraNode(truth, []ast.ASTHandle{negativeEquality, q}, 0)
	interpMust(t, err)
	positiveEntries, err := manager.Args(positiveConjunction)
	interpMust(t, err)
	negativeEntries, err := manager.Args(negativeConjunction)
	interpMust(t, err)
	merged, err := direct.resolveContradictions(positiveEntries[1], positiveConjunction, negativeEntries[1], negativeConjunction)
	interpMust(t, err)
	if op, e := manager.Operator(merged); e != nil || op != OperatorAnd {
		t.Fatal("normal-form contradiction merge", op, e)
	}
	if count, e := manager.NumArgs(merged); e != nil || count != 3 {
		t.Fatal("normal-form resolvent size", count, e)
	}
	singleLeft, err := direct.makeContraNode(truth, []ast.ASTHandle{equality}, 1)
	interpMust(t, err)
	singleRight, err := direct.makeContraNode(truth, []ast.ASTHandle{negativeEquality}, 0)
	interpMust(t, err)
	leftEntries, err := manager.Args(singleLeft)
	interpMust(t, err)
	rightEntries, err := manager.Args(singleRight)
	interpMust(t, err)
	single, err := direct.resolveContradictions(leftEntries[1], singleLeft, rightEntries[1], singleRight)
	interpMust(t, err)
	if yes, e := manager.IsTrue(single); e != nil || !yes {
		t.Fatal("single resolvent simplification", yes, e)
	}
}

func TestDirectProofGenericSymbolicSimplification(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	term, err := manager.Make(OperatorAnd, p, truth)
	interpMust(t, err)
	term, err = direct.SimplifyProofTerm(term)
	interpMust(t, err)
	if term != p {
		t.Fatal("boolean proof simplification", term, p)
	}
	equality, err := manager.Make(OperatorEqual, q, q)
	interpMust(t, err)
	equality, err = direct.SimplifyProofTerm(equality)
	interpMust(t, err)
	if yes, e := manager.IsTrue(equality); e != nil || !yes {
		t.Fatal("equality proof simplification", yes, e)
	}
}

func TestDirectProofOrdinarySumSimplification(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("sum-proof-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	first, err := manager.Make(OperatorLE, zero, x)
	interpMust(t, err)
	first = interpPin(t, m, first)
	second, err := manager.Make(OperatorLE, zero, one)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{first}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	proof, err := direct.makeSymbol(direct.sum, first, one, second)
	interpMust(t, err)
	got, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	want, err := manager.LinearCombination(first, one, second, false)
	interpMust(t, err)
	if got != want {
		t.Fatal("ordinary sum simplification", got, want)
	}
}

func TestDirectProofLocalEqualityRules(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	equivalence, err := manager.Make(OperatorIff, p, p)
	interpMust(t, err)
	modusPonens, err := direct.MakeModusPonens(equivalence, truth, truth)
	interpMust(t, err)
	if yes, e := manager.IsTrue(modusPonens); e != nil || !yes {
		t.Fatal("local modus ponens", yes, e)
	}
	congruence, err := direct.MakeCongruence(equivalence, equivalence, truth)
	interpMust(t, err)
	if yes, e := manager.IsTrue(congruence); e != nil || !yes {
		t.Fatal("local congruence", yes, e)
	}
	transitivity, err := direct.MakeTransitivity(p, p, p, truth, truth)
	interpMust(t, err)
	if yes, e := manager.IsTrue(transitivity); e != nil || !yes {
		t.Fatal("local transitivity", yes, e)
	}
	application, err := manager.Make(OperatorAnd, p, p)
	interpMust(t, err)
	conclusion, err := manager.Make(OperatorIff, application, application)
	interpMust(t, err)
	multi, err := direct.MakeCongruences([]ast.ASTHandle{equivalence, equivalence}, conclusion, []ast.ASTHandle{truth, truth})
	interpMust(t, err)
	if yes, e := manager.IsTrue(multi); e != nil || !yes {
		t.Fatal("local multi-congruence", yes, e)
	}
}

func TestDirectProofTrivialInterpolationRewriteChains(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	mixed, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	negatedQ, err := manager.MkNot(q)
	interpMust(t, err)
	interpolation, err := direct.trivialInterpolation(direct.modusPonens, []ast.ASTHandle{p, mixed, negatedQ}, 1<<1)
	interpMust(t, err)
	entries, err := manager.Args(interpolation)
	interpMust(t, err)
	if len(entries) != 2 {
		t.Fatal("mixed conjunct count", len(entries))
	}
	proof, err := manager.Arg(entries[0], 0)
	interpMust(t, err)
	if declaration, e := manager.Symbol(proof); e != nil || declaration != direct.modusPonens {
		t.Fatal("mixed rule", declaration, e)
	}
	proofArgs, err := manager.Args(proof)
	interpMust(t, err)
	for i, want := range []ast.ASTHandle{direct.rewriteA, ast.ASTHandle{}, direct.rewriteB} {
		if i == 1 {
			placeholder, e := direct.isPlaceholder(proofArgs[i])
			if e != nil || !placeholder {
				t.Fatal("mixed premise placeholder", placeholder, e)
			}
			continue
		}
		chainDeclaration, e := manager.Symbol(proofArgs[i])
		interpMust(t, e)
		if chainDeclaration != direct.concat {
			t.Fatal("local rewrite chain", i, chainDeclaration)
		}
		rewrite, e := manager.Arg(proofArgs[i], 1)
		interpMust(t, e)
		rewriteDeclaration, e := manager.Symbol(rewrite)
		interpMust(t, e)
		if rewriteDeclaration != want {
			t.Fatal("local rewrite side", i, rewriteDeclaration, want)
		}
	}
	rotated, err := manager.Arg(entries[1], 0)
	interpMust(t, err)
	if declaration, e := manager.Symbol(rotated); e != nil || declaration != direct.rotateSum {
		t.Fatal("compressed rotation mask", declaration, e)
	}
}

func TestDirectProofMixedModusPonens(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	equivalence, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	premise1, err := direct.MakeHypothesis(p)
	interpMust(t, err)
	premise2, err := direct.MakeHypothesis(equivalence)
	interpMust(t, err)
	result, err := direct.MakeModusPonens(equivalence, premise1, premise2)
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorAnd {
		t.Fatal("mixed modus ponens", op, e)
	}
}

func TestDirectProofLocalToMixedCongruenceAndTransitivity(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	localEquality, err := manager.Make(OperatorIff, p, p)
	interpMust(t, err)
	application, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	mixedConclusion, err := manager.Make(OperatorIff, application, application)
	interpMust(t, err)
	localPremise, err := direct.MakeHypothesis(localEquality)
	interpMust(t, err)
	congruence, err := direct.MakeCongruence(localEquality, mixedConclusion, localPremise)
	interpMust(t, err)
	if op, e := manager.Operator(congruence); e != nil || op != OperatorAnd {
		t.Fatal("local-to-mixed congruence", op, e)
	}
	left, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	right, err := manager.Make(OperatorIff, q, q)
	interpMust(t, err)
	leftPremise, err := direct.MakeHypothesis(left)
	interpMust(t, err)
	rightPremise, err := direct.MakeHypothesis(right)
	interpMust(t, err)
	transitivity, err := direct.MakeTransitivity(p, q, q, leftPremise, rightPremise)
	interpMust(t, err)
	if op, e := manager.Operator(transitivity); e != nil || op != OperatorAnd {
		t.Fatal("mixed transitivity", op, e)
	}
}

func TestDirectProofLocalizedAxioms(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	aAxiom, err := direct.MakeAxiom([]ast.ASTHandle{p})
	interpMust(t, err)
	if no, e := manager.IsFalse(aAxiom); e != nil || !no {
		t.Fatal("A axiom", no, e)
	}
	bAxiom, err := direct.MakeAxiom([]ast.ASTHandle{q})
	interpMust(t, err)
	if yes, e := manager.IsTrue(bAxiom); e != nil || !yes {
		t.Fatal("B axiom", yes, e)
	}
	localized, err := direct.MakeAxiomInRange([]ast.ASTHandle{q}, DownwardScopeRange(0))
	interpMust(t, err)
	if localized.IsNull() || len(direct.localizationVariables) != 1 {
		t.Fatal("localized axiom", localized, direct.localizationVariables)
	}
}

func TestDirectProofNoProofResolutionLocalization(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	pivot, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	negativePivot, err := manager.MkNot(pivot)
	interpMust(t, err)
	positiveConjunction, err := direct.makeContraNode(direct.noProof, []ast.ASTHandle{pivot}, 0)
	interpMust(t, err)
	negativeConjunction, err := direct.makeContraNode(direct.noProof, []ast.ASTHandle{negativePivot}, 0)
	interpMust(t, err)
	positiveEntries, err := manager.Args(positiveConjunction)
	interpMust(t, err)
	negativeEntries, err := manager.Args(negativeConjunction)
	interpMust(t, err)
	for _, test := range []struct {
		firstPivot, firstConjunction, secondPivot, secondConjunction ast.ASTHandle
	}{
		{positiveEntries[1], positiveConjunction, negativeEntries[1], negativeConjunction},
		{negativeEntries[1], negativeConjunction, positiveEntries[1], positiveConjunction},
	} {
		result, e := direct.resolveContradictions(test.firstPivot, test.firstConjunction, test.secondPivot, test.secondConjunction)
		interpMust(t, e)
		if result.IsNull() {
			t.Fatal("localized no-proof resolution")
		}
	}
	if len(direct.localizationVariables) == 0 {
		t.Fatal("no localization variable recorded")
	}
}

func TestDirectProofEquivalenceChainSymmetrySimplification(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	equality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	chain, err := direct.makeLocalRewrite(TermA, equality)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.symmetry, chain)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	_, rewrite, err := direct.chainParts(result)
	interpMust(t, err)
	reversed, err := manager.Arg(rewrite, 2)
	interpMust(t, err)
	left, err := manager.Arg(reversed, 0)
	interpMust(t, err)
	right, err := manager.Arg(reversed, 1)
	interpMust(t, err)
	if left != negativeP || right != p {
		t.Fatal("reversed equivalence chain", left, right, negativeP, p)
	}
}

func TestDirectProofNegationChainSymmetrySimplification(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	equality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	negativeEquality, err := manager.MkNot(equality)
	interpMust(t, err)
	chain, err := direct.makeLocalRewrite(TermA, negativeEquality)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.symmetry, chain)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	_, rewrite, err := direct.chainParts(result)
	interpMust(t, err)
	rewritten, err := manager.Arg(rewrite, 2)
	interpMust(t, err)
	right, err := manager.Arg(rewritten, 1)
	interpMust(t, err)
	body, err := manager.Arg(right, 0)
	interpMust(t, err)
	left, err := manager.Arg(body, 0)
	interpMust(t, err)
	other, err := manager.Arg(body, 1)
	interpMust(t, err)
	if left != negativeP || other != p {
		t.Fatal("commuted negation chain", left, other, negativeP, p)
	}
}

func TestDirectProofRewriteChainAbsorption(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	forward, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	backward, err := manager.Make(OperatorIff, negativeP, p)
	interpMust(t, err)
	forward, err = direct.makeRewrite(TermA, direct.topPosition, truth, forward)
	interpMust(t, err)
	backward, err = direct.makeRewrite(TermA, direct.topPosition, truth, backward)
	interpMust(t, err)
	chain, err := direct.chainCons(truth, forward)
	interpMust(t, err)
	second, err := direct.chainCons(truth, backward)
	interpMust(t, err)
	chain, err = direct.concatRewriteChains(chain, second)
	interpMust(t, err)
	rest, rewrite, err := direct.chainParts(chain)
	interpMust(t, err)
	if empty, e := manager.IsTrue(rest); e != nil || !empty {
		t.Fatal("absorbed chain rest", empty, e)
	}
	left, right, err := direct.rewriteEndpoints(rewrite)
	interpMust(t, err)
	if left != p || right != p {
		t.Fatal("absorbed rewrite", left, right, p)
	}
}

func TestDirectProofRewriteApplicationAndSideSplit(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	forwardEquality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	backwardEquality, err := manager.Make(OperatorIff, negativeP, p)
	interpMust(t, err)
	forward, err := direct.makeRewrite(TermA, direct.topPosition, truth, forwardEquality)
	interpMust(t, err)
	backward, err := direct.makeRewrite(TermB, direct.topPosition, truth, backwardEquality)
	interpMust(t, err)
	chain, err := direct.chainCons(truth, forward)
	interpMust(t, err)
	chain, err = direct.chainCons(chain, backward)
	interpMust(t, err)
	result, err := direct.applyRewriteChain(p, chain)
	interpMust(t, err)
	if result != p {
		t.Fatal("rewrite application", result, p)
	}
	kept, remainder, err := direct.dropRewrites(TermB, chain)
	interpMust(t, err)
	_, keptLast, err := direct.chainParts(kept)
	interpMust(t, err)
	_, remainderLast, err := direct.chainParts(remainder)
	interpMust(t, err)
	keptSide, _, _, _, err := direct.rewriteFields(keptLast)
	interpMust(t, err)
	remainderSide, _, _, _, err := direct.rewriteFields(remainderLast)
	interpMust(t, err)
	if keptSide != TermA || remainderSide != TermB {
		t.Fatal("rewrite side split", keptSide, remainderSide)
	}
}

func TestDirectProofPredicateChainContradiction(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	negativeChain, err := direct.makeLocalRewrite(TermA, negativeP)
	interpMust(t, err)
	positiveChain, err := direct.makeLocalRewrite(TermB, p)
	interpMust(t, err)
	result, err := direct.contradictChains(negativeChain, positiveChain)
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorIff {
		t.Fatal("predicate chain contradiction", op, e)
	}
}

func TestDirectProofEqualityChainContradiction(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	equality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	negativeEquality, err := manager.MkNot(equality)
	interpMust(t, err)
	negativeChain, err := direct.makeLocalRewrite(TermA, negativeEquality)
	interpMust(t, err)
	positiveChain, err := direct.makeLocalRewrite(TermB, equality)
	interpMust(t, err)
	result, err := direct.contradictChains(negativeChain, positiveChain)
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorNot {
		t.Fatal("equality chain contradiction", op, e)
	}
}

func TestDirectProofExcludedMiddleAndCongruenceSimplification(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	equality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	negativeEquality, err := manager.MkNot(equality)
	interpMust(t, err)
	positiveChain, err := direct.makeLocalRewrite(TermB, equality)
	interpMust(t, err)
	negativeChain, err := direct.makeLocalRewrite(TermA, negativeEquality)
	interpMust(t, err)
	excludedMiddle, err := direct.makeSymbol(direct.excludedMid, equality, positiveChain, negativeChain)
	interpMust(t, err)
	if result, e := direct.SimplifyProofTerm(excludedMiddle); e != nil || result.IsNull() {
		t.Fatal("excluded-middle simplification", result, e)
	}
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	congruence, err := direct.makeSymbol(direct.congruence, positiveChain, zero, negativeChain)
	interpMust(t, err)
	if result, e := direct.SimplifyProofTerm(congruence); e != nil || result.IsNull() {
		t.Fatal("congruence simplification", result, e)
	}
}

func TestDirectProofPredicateModusPonensSimplification(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	first, err := direct.makeLocalRewrite(TermA, p)
	interpMust(t, err)
	second, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	third, err := direct.makeLocalRewrite(TermB, negativeP)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.modusPonens, first, second, third)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if result.IsNull() {
		t.Fatal("predicate modus-ponens simplification")
	}
}

func TestDirectProofUnsplitModusPonensSimplification(t *testing.T) {
	_, manager, base, p, _ := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	firstFormula, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	secondFormula, err := manager.Make(OperatorIff, negativeP, p)
	interpMust(t, err)
	first, err := direct.makeLocalRewrite(TermA, firstFormula)
	interpMust(t, err)
	implication, err := manager.Make(OperatorIff, firstFormula, secondFormula)
	interpMust(t, err)
	second, err := direct.makeLocalRewrite(TermB, implication)
	interpMust(t, err)
	negativeSecond, err := manager.MkNot(secondFormula)
	interpMust(t, err)
	third, err := direct.makeLocalRewrite(TermA, negativeSecond)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.modusPonens, first, second, third)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if result.IsNull() {
		t.Fatal("unsplit modus-ponens simplification")
	}
}

func TestDirectProofNestedSumCoefficientDistribution(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("nested-sum-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	two, err := manager.MakeIntText("2")
	interpMust(t, err)
	inequality, err := manager.Make(OperatorLE, zero, x)
	interpMust(t, err)
	inequality = interpPin(t, m, inequality)
	base, err := NewBase(manager, []ast.ASTHandle{inequality}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	inner, err := direct.makeSymbol(direct.sum, inequality, one, truth)
	interpMust(t, err)
	outer, err := direct.makeSymbol(direct.sum, inequality, two, inner)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(outer)
	interpMust(t, err)
	if declaration, e := manager.Symbol(result); e != nil || declaration != direct.sum {
		t.Fatal("distributed outer sum", declaration, e)
	}
	coefficient, err := manager.Arg(result, 1)
	interpMust(t, err)
	if coefficient != one {
		t.Fatal("distributed outer coefficient", coefficient, one)
	}
}

func TestDirectProofRotatedOrdinarySumSimplification(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	literal, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	placeholder, err := direct.placeholder(literal)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	zeroInequality, err := manager.Make(OperatorLE, zero, zero)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.sum, zeroInequality, one, placeholder)
	interpMust(t, err)
	proof, err = direct.makeSymbol(direct.rotateSum, literal, proof)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorLE {
		t.Fatal("rotated ordinary sum", op, e)
	}
}

func TestDirectProofRotatedCongruenceSimplification(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	literal, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	negativeP, err := manager.MkNot(p)
	interpMust(t, err)
	equality, err := manager.Make(OperatorIff, p, negativeP)
	interpMust(t, err)
	chain, err := direct.makeLocalRewrite(TermA, equality)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	placeholder, err := direct.placeholder(literal)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.congruence, chain, zero, placeholder)
	interpMust(t, err)
	proof, err = direct.makeSymbol(direct.rotateSum, literal, proof)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if declaration, e := manager.Symbol(result); e != nil || declaration != direct.concat {
		t.Fatal("rotated congruence", declaration, e)
	}
}

func TestDirectProofRotatedModusPonensSimplification(t *testing.T) {
	_, manager, base, p, q := directProofFixture(t)
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	literal, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	placeholder, err := direct.placeholder(literal)
	interpMust(t, err)
	chain, err := direct.makeLocalRewrite(TermA, p)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	for _, arguments := range [][3]ast.ASTHandle{
		{chain, truth, placeholder},
		{placeholder, truth, chain},
	} {
		proof, e := direct.makeSymbol(direct.modusPonens, arguments[:]...)
		interpMust(t, e)
		proof, e = direct.makeSymbol(direct.rotateSum, literal, proof)
		interpMust(t, e)
		result, e := direct.SimplifyProofTerm(proof)
		interpMust(t, e)
		if declaration, symbolErr := manager.Symbol(result); symbolErr != nil || declaration != direct.concat {
			t.Fatal("rotated modus ponens", declaration, symbolErr)
		}
	}
}

func TestDirectProofArithmeticNormalVocabulary(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("normal-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("normal-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	step, err := direct.makeNormalStep(x, y, truth)
	interpMust(t, err)
	chain, err := direct.consNormal(step, truth)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	inequality, err := manager.Make(OperatorLE, zero, x)
	interpMust(t, err)
	normal, err := direct.makeNormal(inequality, chain)
	interpMust(t, err)
	if yes, e := direct.isNormalInequality(normal); e != nil || !yes {
		t.Fatal("normal inequality", yes, e)
	}
	gotInequality, gotChain, err := direct.destructNormal(normal)
	interpMust(t, err)
	if gotInequality != inequality || gotChain != chain {
		t.Fatal("normal destruct", gotInequality, gotChain, inequality, chain)
	}
	left, right, proof, err := direct.normalStepParts(step)
	interpMust(t, err)
	if left != x || right != y || proof != truth {
		t.Fatal("normal step", left, right, proof)
	}
}

func TestDirectProofDisjointNormalChainMerge(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("normal-merge-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("normal-merge-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	firstStep, err := direct.makeNormalStep(x, zero, truth)
	interpMust(t, err)
	secondStep, err := direct.makeNormalStep(y, one, truth)
	interpMust(t, err)
	first, err := direct.consNormal(firstStep, truth)
	interpMust(t, err)
	second, err := direct.consNormal(secondStep, truth)
	interpMust(t, err)
	aProof, bProof := truth, truth
	merged, err := direct.mergeNormalChains(first, second, &aProof, &bProof)
	interpMust(t, err)
	step, rest, err := direct.normalChainParts(merged)
	interpMust(t, err)
	left, _, _, err := direct.normalStepParts(step)
	interpMust(t, err)
	next, _, err := direct.normalChainParts(rest)
	interpMust(t, err)
	nextLeft, _, _, err := direct.normalStepParts(next)
	interpMust(t, err)
	leftID, err := manager.ASTID(left)
	interpMust(t, err)
	nextID, err := manager.ASTID(nextLeft)
	interpMust(t, err)
	if leftID >= nextID || aProof != truth || bProof != truth {
		t.Fatal("normal merge order or conditions", leftID, nextID, aProof, bProof)
	}
}

func TestDirectProofNormalChainMixedRepresentativeMerge(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("normal-mixed-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("normal-mixed-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	left, err := manager.Make(OperatorPlus, x, y)
	interpMust(t, err)
	mixedRight, err := manager.Make(OperatorSub, x, y)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	firstEquality, err := manager.Make(OperatorEqual, left, mixedRight)
	interpMust(t, err)
	firstProof, err := direct.makeRewrite(TermA, direct.topPosition, truth, firstEquality)
	interpMust(t, err)
	firstProof, err = direct.chainCons(truth, firstProof)
	interpMust(t, err)
	secondEquality, err := manager.Make(OperatorEqual, left, x)
	interpMust(t, err)
	secondProof, err := direct.makeRewrite(TermB, direct.topPosition, truth, secondEquality)
	interpMust(t, err)
	secondProof, err = direct.chainCons(truth, secondProof)
	interpMust(t, err)
	firstStep, err := direct.makeNormalStep(left, mixedRight, firstProof)
	interpMust(t, err)
	secondStep, err := direct.makeNormalStep(left, x, secondProof)
	interpMust(t, err)
	first, err := direct.consNormal(firstStep, truth)
	interpMust(t, err)
	second, err := direct.consNormal(secondStep, truth)
	interpMust(t, err)
	aProof, bProof := truth, truth
	merged, err := direct.mergeNormalChains(first, second, &aProof, &bProof)
	interpMust(t, err)
	step, _, err := direct.normalChainParts(merged)
	interpMust(t, err)
	gotLeft, gotRight, _, err := direct.normalStepParts(step)
	interpMust(t, err)
	if gotLeft != left || gotRight != x {
		t.Fatal("mixed representative merge", gotLeft, gotRight, left, x)
	}
}

func TestDirectProofNormalInequalitySumAndFinalSubstitution(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("normal-sum-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("normal-sum-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	firstStep, err := direct.makeNormalStep(x, zero, truth)
	interpMust(t, err)
	secondStep, err := direct.makeNormalStep(y, one, truth)
	interpMust(t, err)
	firstChain, err := direct.consNormal(firstStep, truth)
	interpMust(t, err)
	secondChain, err := direct.consNormal(secondStep, truth)
	interpMust(t, err)
	firstInequality, err := manager.Make(OperatorLE, zero, x)
	interpMust(t, err)
	secondInequality, err := manager.Make(OperatorLE, zero, y)
	interpMust(t, err)
	firstNormal, err := direct.makeNormal(firstInequality, firstChain)
	interpMust(t, err)
	secondNormal, err := direct.makeNormal(secondInequality, secondChain)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.sum, firstNormal, one, secondNormal)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if op, e := manager.Operator(result); e != nil || op != OperatorLE {
		t.Fatal("normalized inequality sum", op, e)
	}
}

func TestDirectProofRotatedLeqToEqSimplification(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("rot-leq2eq-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("rot-leq2eq-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	equality, err := manager.Make(OperatorEqual, x, y)
	interpMust(t, err)
	negativeEquality, err := manager.MkNot(equality)
	interpMust(t, err)
	placeholder, err := direct.placeholder(negativeEquality)
	interpMust(t, err)
	xLeqY, err := manager.Make(OperatorLE, x, y)
	interpMust(t, err)
	yLeqX, err := manager.Make(OperatorLE, y, x)
	interpMust(t, err)
	proof, err := direct.makeSymbol(direct.leqToEq, placeholder, xLeqY, yLeqX)
	interpMust(t, err)
	proof, err = direct.makeSymbol(direct.rotateSum, negativeEquality, proof)
	interpMust(t, err)
	result, err := direct.SimplifyProofTerm(proof)
	interpMust(t, err)
	if declaration, e := manager.Symbol(result); e != nil || declaration != direct.concat {
		t.Fatal("rotated leq2eq", declaration, e)
	}
}

func TestDirectProofRotatedEqToLeqSimplification(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("rot-eq2leq-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("rot-eq2leq-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)

	for _, test := range []struct {
		name  string
		left  ast.ASTHandle
		right ast.ASTHandle
	}{
		{name: "unmixed", left: x, right: y},
		{name: "mixed-left", left: func() ast.ASTHandle {
			mixed, e := manager.Make(OperatorPlus, x, y)
			interpMust(t, e)
			return mixed
		}(), right: x},
	} {
		t.Run(test.name, func(t *testing.T) {
			equality, e := manager.Make(OperatorEqual, test.left, test.right)
			interpMust(t, e)
			rewrite, e := direct.makeRewrite(TermA, direct.topPosition, truth, equality)
			interpMust(t, e)
			chain, e := direct.chainCons(truth, rewrite)
			interpMust(t, e)
			inequality, e := manager.Make(OperatorLE, test.left, test.right)
			interpMust(t, e)
			negativeInequality, e := manager.MkNot(inequality)
			interpMust(t, e)
			placeholder, e := direct.placeholder(negativeInequality)
			interpMust(t, e)
			proof, e := direct.makeSymbol(direct.eqToLeq, chain, placeholder)
			interpMust(t, e)
			proof, e = direct.makeSymbol(direct.rotateSum, negativeInequality, proof)
			interpMust(t, e)
			result, e := direct.SimplifyProofTerm(proof)
			interpMust(t, e)
			valid, e := direct.isInequality(result)
			if e != nil || !valid {
				t.Fatal("rotated eq2leq result", result, e)
			}
		})
	}
}

func TestDirectProofRewriteChainArithmeticReduction(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	x, err := manager.MakeVariable("chain-arith-x", integer)
	interpMust(t, err)
	x = interpPin(t, m, x)
	y, err := manager.MakeVariable("chain-arith-y", integer)
	interpMust(t, err)
	y = interpPin(t, m, y)
	base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	one, err := manager.MakeIntText("1")
	interpMust(t, err)
	firstInequality, err := manager.Make(OperatorLE, x, zero)
	interpMust(t, err)
	headEquality, err := manager.Make(OperatorIff, truth, firstInequality)
	interpMust(t, err)
	head, err := direct.makeRewrite(TermA, direct.topPosition, truth, headEquality)
	interpMust(t, err)
	chain, err := direct.chainCons(truth, head)
	interpMust(t, err)
	position, err := direct.positionAdd(0, direct.topPosition)
	interpMust(t, err)
	tailEquality, err := manager.Make(OperatorEqual, x, y)
	interpMust(t, err)
	tail, err := direct.makeRewrite(TermA, position, truth, tailEquality)
	interpMust(t, err)
	chain, err = direct.chainCons(chain, tail)
	interpMust(t, err)

	aProof, bProof := truth, truth
	converted, err := direct.rewriteChainToNormalInequality(chain, &aProof, &bProof)
	interpMust(t, err)
	valid, err := direct.isNormalInequality(converted)
	if err != nil || !valid {
		t.Fatal("rewrite-chain inequality", converted, err)
	}
	baseInequality, err := manager.Make(OperatorLE, zero, zero)
	interpMust(t, err)
	sum, err := direct.simplifySum([]ast.ASTHandle{baseInequality, one, chain})
	interpMust(t, err)
	valid, err = direct.isNormalInequality(sum)
	if err != nil || !valid {
		t.Fatal("rewrite-chain sum", sum, err)
	}

	headOnly, err := direct.chainCons(truth, head)
	interpMust(t, err)
	modusPonens, err := direct.simplifyModusPonens([]ast.ASTHandle{headOnly, truth, baseInequality})
	interpMust(t, err)
	valid, err = direct.isNormalInequality(modusPonens)
	if err != nil || !valid {
		t.Fatal("arithmetic modus ponens", modusPonens, err)
	}

	firstTerm, err := manager.Make(OperatorSub, x, x)
	interpMust(t, err)
	structuralInequality, err := manager.Make(OperatorLE, firstTerm, zero)
	interpMust(t, err)
	structuralHeadEquality, err := manager.Make(OperatorIff, truth, structuralInequality)
	interpMust(t, err)
	structuralHead, err := direct.makeRewrite(TermB, direct.topPosition, truth, structuralHeadEquality)
	interpMust(t, err)
	structuralChain, err := direct.chainCons(truth, structuralHead)
	interpMust(t, err)
	innerPosition, err := direct.positionAdd(1, direct.topPosition)
	interpMust(t, err)
	innerPosition, err = direct.positionAdd(0, innerPosition)
	interpMust(t, err)
	structuralTail, err := direct.makeRewrite(TermA, innerPosition, truth, tailEquality)
	interpMust(t, err)
	structuralChain, err = direct.chainCons(structuralChain, structuralTail)
	interpMust(t, err)
	aProof, bProof = truth, truth
	converted, err = direct.rewriteChainToNormalInequality(structuralChain, &aProof, &bProof)
	interpMust(t, err)
	normal, err := direct.isNormalWrapper(converted)
	if err != nil || !normal {
		t.Fatal("structural normal chain", converted, err)
	}
}

// AUDIT.md interp_01-F2: eq_from_ineq compares arg(ineq,1) with the Int
// make_int(0) node, so a Real 0.0 keeps the sum.
func TestDirectProofEqFromIneqRealZeroKeepsSum(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	for _, real := range []bool{true, false} {
		sort, err := manager.IntType()
		interpMust(t, err)
		zeroText, minusOneText := manager.MakeIntText, manager.MakeIntText
		suffix := "int"
		if real {
			sort, err = manager.RealType()
			interpMust(t, err)
			zeroText, minusOneText = manager.MakeRealText, manager.MakeRealText
			suffix = "real"
		}
		x, err := manager.MakeVariable("eq-from-ineq-x-"+suffix, sort)
		interpMust(t, err)
		x = interpPin(t, m, x)
		y, err := manager.MakeVariable("eq-from-ineq-y-"+suffix, sort)
		interpMust(t, err)
		y = interpPin(t, m, y)
		base, err := NewBase(manager, []ast.ASTHandle{x, y}, nil, nil)
		interpMust(t, err)
		t.Cleanup(func() { interpMust(t, base.Close()) })
		direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
		interpMust(t, err)
		t.Cleanup(func() { interpMust(t, direct.Close()) })
		minusOne, err := minusOneText("-1")
		interpMust(t, err)
		product, err := manager.Make(OperatorTimes, minusOne, y)
		interpMust(t, err)
		sum, err := manager.Make(OperatorPlus, x, product)
		interpMust(t, err)
		zero, err := zeroText("0")
		interpMust(t, err)
		inequality, err := manager.Make(OperatorLE, sum, zero)
		interpMust(t, err)
		inequality = interpPin(t, m, inequality)
		left, right, err := direct.equalityEndpointsFromInequality(inequality)
		interpMust(t, err)
		if real && (left != sum || right != zero) {
			t.Fatal("Real 0.0 split the sum", left, right)
		}
		if !real && (left != x || right != y) {
			t.Fatal("Int 0 did not split the sum", left, right)
		}
	}
}
