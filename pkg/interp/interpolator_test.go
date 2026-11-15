package interp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestInterpolationOptionsErrorsAndLinearParents(t *testing.T) {
	_, manager := interpProofManagerFixture(t)
	base, err := NewBase(manager, nil, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	options := NewOptions()
	options.Set("weak", "1")
	interpMust(t, options.Apply(base))
	if weak, e := base.WeakMode(); e != nil || !weak {
		t.Fatal("weak option", weak, e)
	}
	if !IsLinearParents(nil) || !IsLinearParents([]int{1, 2, ScopeTop}) || IsLinearParents([]int{2, 2, ScopeTop}) {
		t.Fatal("linear parent recognition")
	}
	if (InterpolationBadTreeError{}).Error() != "iz3_bad_tree" || (IncompletenessError{}).Error() != "iz3_incompleteness" || (InterpolationFailure{Message: "failed"}).Error() != "failed" || (InterpolationError{}).Error() != "theory not supported by interpolation or bad proof" {
		t.Fatal("interpolation error contract")
	}
	if err = options.Apply(nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil base guard", err)
	}
}

func TestInterpolationProofAssumptionsDeduplicateDAG(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("assumption-p", boolSort)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("assumption-q", boolSort)
	interpMust(t, err)
	q = interpPin(t, m, q)
	pp, err := m.MkProofAsserted(p)
	interpMust(t, err)
	pp = interpPin(t, m, pp)
	qp, err := m.MkProofAsserted(q)
	interpMust(t, err)
	qp = interpPin(t, m, qp)
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	arithmetic, err := m.ArithmeticFamilyID()
	interpMust(t, err)
	root := interpTheoryLemma(t, m, arithmetic, "farkas", falsehood, []ast.ASTHandle{pp, qp, pp}, "1", "1", "1")
	assumptions, err := ProofAssumptions(manager, root)
	interpMust(t, err)
	if len(assumptions) != 2 || assumptions[0] != p || assumptions[1] != q {
		t.Fatal("proof assumptions", assumptions)
	}
}

func TestInterpolationCollectAndAssertConjuncts(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	makeVar := func(name string) ast.ASTHandle {
		h, e := manager.MakeVariable(name, boolSort)
		interpMust(t, e)
		return interpPin(t, m, h)
	}
	p, q, r, s := makeVar("collect-p"), makeVar("collect-q"), makeVar("collect-r"), makeVar("collect-s")
	qr, err := manager.Make(OperatorAnd, q, r)
	interpMust(t, err)
	marked, err := manager.Make(OperatorInterp, qr)
	interpMust(t, err)
	tree, err := manager.Make(OperatorAnd, p, marked, s)
	interpMust(t, err)
	tree = interpPin(t, m, tree)
	constraints, err := CollectConjuncts(manager, tree)
	interpMust(t, err)
	if len(constraints) != 3 || constraints[0] != p || constraints[1] != qr || constraints[2] != s {
		t.Fatal("collected conjuncts", constraints)
	}
	target, err := newInterpolationScriptSolver(m, lbool.Undef)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, target.Close()) })
	asserted, err := AssertConjuncts(manager, target, tree)
	interpMust(t, err)
	if len(asserted) != 3 || len(target.assertions) != 3 {
		t.Fatal("asserted conjuncts", asserted, target.assertions)
	}
}

func TestInterpolationFrameReducerTreeAndRestoration(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	parts := make([][]ast.ASTHandle, 4)
	for i, name := range []string{"frame-a", "frame-b", "frame-c", "frame-d"} {
		formula, e := manager.MakeVariable(name, boolSort)
		interpMust(t, e)
		parts[i] = []ast.ASTHandle{interpPin(t, m, formula)}
	}
	proof, err := m.MkProofAsserted(parts[0][0])
	interpMust(t, err)
	proof = interpPin(t, m, proof)
	reducer, err := NewFrameReducer(manager)
	interpMust(t, err)
	reduced, parents, err := reducer.Reduce(parts, []int{2, 2, 3, ScopeTop}, proof)
	interpMust(t, err)
	if len(reduced) != 2 || reduced[0][0] != parts[0][0] || reduced[1][0] != parts[3][0] {
		t.Fatal("reduced frames", reduced)
	}
	if len(parents) != 2 || parents[0] != ScopeTop || parents[1] != ScopeTop {
		t.Fatal("reduced parents", parents)
	}
	fixed, err := reducer.FixInterpolants([]ast.ASTHandle{parts[1][0]})
	interpMust(t, err)
	defer func() {
		for i := range fixed {
			interpMust(t, m.DelRef(&fixed[i]))
		}
	}()
	if len(fixed) != 3 {
		t.Fatal("fixed interpolant count", len(fixed))
	}
	got0, err := m.RefGet(fixed[0])
	interpMust(t, err)
	got1, err := m.RefGet(fixed[1])
	interpMust(t, err)
	got2, err := m.RefGet(fixed[2])
	interpMust(t, err)
	if got0 != parts[1][0] || got2 != parts[1][0] {
		t.Fatal("restored interpolants", got0, got2)
	}
	if truth, e := manager.IsTrue(got1); e != nil || !truth {
		t.Fatal("removed sibling interpolant", truth, e)
	}
	if _, err = reducer.FixInterpolants(nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("fixed interpolant length guard", err)
	}
}

func TestInterpolationFrameReducerSequenceAndGuards(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	truth = interpPin(t, m, truth)
	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	falsehood = interpPin(t, m, falsehood)
	proof, err := m.MkProofAsserted(truth)
	interpMust(t, err)
	proof = interpPin(t, m, proof)
	reducer, err := NewFrameReducer(manager)
	interpMust(t, err)
	parts := [][]ast.ASTHandle{{truth}, {falsehood}}
	reduced, parents, err := reducer.Reduce(parts, nil, proof)
	interpMust(t, err)
	if len(reduced) != 2 || parents != nil {
		t.Fatal("sequence reduction", reduced, parents)
	}
	if _, _, err = reducer.Reduce(nil, nil, proof); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("empty frame guard", err)
	}
	if _, _, err = reducer.Reduce(parts, []int{ScopeTop}, proof); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("parent length guard", err)
	}
}

func TestInterpolationProofTreeAndSolverOrchestration(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("orchestration-p", boolean)
	interpMust(t, err)
	p = interpPin(t, m, p)
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	notP = interpPin(t, m, notP)
	positive, err := m.MkProofAsserted(p)
	interpMust(t, err)
	positive = interpPin(t, m, positive)
	negative, err := m.MkProofAsserted(notP)
	interpMust(t, err)
	negative = interpPin(t, m, negative)
	proof, err := m.MkProofUnitResolution([]ast.ASTHandle{positive, negative})
	interpMust(t, err)
	proof = interpPin(t, m, proof)

	flat, err := ProofInterpolantsFlat(manager, proof, []ast.ASTHandle{p, notP}, nil, nil, nil)
	interpMust(t, err)
	if len(flat) != 1 {
		t.Fatal("flat interpolants", len(flat))
	}
	for i := range flat {
		interpMust(t, m.DelRef(&flat[i]))
	}
	markedP, err := manager.Make(OperatorInterp, p)
	interpMust(t, err)
	markedNotP, err := manager.Make(OperatorInterp, notP)
	interpMust(t, err)
	tree, err := manager.Make(OperatorAnd, markedP, markedNotP)
	interpMust(t, err)
	tree = interpPin(t, m, tree)
	treeResults, err := TreeInterpolants(manager, proof, nil, tree, nil)
	interpMust(t, err)
	if len(treeResults) != 2 {
		t.Fatal("tree interpolants", len(treeResults))
	}
	for i := range treeResults {
		interpMust(t, m.DelRef(&treeResults[i]))
	}

	target, err := newInterpolationScriptSolver(m, lbool.False)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, target.Close()) })
	interpMust(t, target.SetProof(proof))
	status, constraints, solved, mdl, err := SolveTree(manager, target, tree, true, nil)
	interpMust(t, err)
	if status != lbool.False || len(constraints) != 2 || len(solved) != 2 || mdl != nil {
		t.Fatal("solver interpolation", status, len(constraints), len(solved), mdl)
	}
	for i := range solved {
		interpMust(t, m.DelRef(&solved[i]))
	}
	satisfiable, err := newInterpolationScriptSolver(m, lbool.True)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, satisfiable.Close()) })
	modelValue, err := model.NewModelCore(m)
	interpMust(t, err)
	interpMust(t, satisfiable.SetModel(modelValue))
	status, constraints, solved, mdl, err = SolveTree(manager, satisfiable, tree, true, nil)
	interpMust(t, err)
	if status != lbool.True || len(constraints) != 2 || solved != nil || mdl == nil {
		t.Fatal("solver model", status, len(constraints), solved, mdl)
	}
	interpMust(t, mdl.DecRef())
}

func TestFullTranslationProofAnalysisAndResolution(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("translation-a", boolean)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("translation-b", boolean)
	interpMust(t, err)
	q = interpPin(t, m, q)
	base, err := NewBase(manager, []ast.ASTHandle{p, q}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })

	falsehood, err := manager.Make(OperatorFalse)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	hypothesis, err := m.MkProofHypothesis(falsehood)
	interpMust(t, err)
	lemma, err := m.MkProofLemma(hypothesis, truth)
	interpMust(t, err)
	hypotheses, err := translation.ProofHypotheses(lemma)
	interpMust(t, err)
	if len(hypotheses) != 0 {
		t.Fatal("discharged hypotheses", hypotheses)
	}

	mixed, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	equivalence, err := manager.Make(OperatorIff, mixed, p)
	interpMust(t, err)
	equivalenceProof, err := m.MkProofAsserted(equivalence)
	interpMust(t, err)
	interpMust(t, translation.FindEquivalences(equivalenceProof))
	local, proof, found, err := translation.EquivalentLocal(mixed)
	if err != nil || !found || local != p || proof != equivalenceProof {
		t.Fatal("local equivalence", local, proof, found, err)
	}

	clause, err := manager.Make(OperatorOr, p, q)
	interpMust(t, err)
	clauseProof, err := m.MkProofAsserted(clause)
	interpMust(t, err)
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	unitProof, err := m.MkProofAsserted(notP)
	interpMust(t, err)
	resolution, err := m.MkProofUnitResolution([]ast.ASTHandle{clauseProof, unitProof})
	interpMust(t, err)
	literals, err := translation.UnitResolutionLiterals(resolution, 1)
	interpMust(t, err)
	if len(literals) != 1 || literals[0] != q {
		t.Fatal("unit resolution literals", literals)
	}
	empty, err := translation.ClauseLiterals(falsehood)
	if err != nil || len(empty) != 0 {
		t.Fatal("false clause", empty, err)
	}
	assertedP, err := m.MkProofAsserted(p)
	interpMust(t, err)
	if locality, e := translation.Locality(assertedP); e != nil || locality != 0 {
		t.Fatal("asserted locality", locality, e)
	}
	assertedQ, err := m.MkProofAsserted(q)
	interpMust(t, err)
	if locality, e := translation.Locality(assertedQ); e != nil || locality != 1 {
		t.Fatal("second asserted locality", locality, e)
	}
	interpMust(t, translation.ScanSkolems(assertedP))
	if quantified, e := translation.Quantify(p, DownwardScopeRange(0)); e != nil || quantified != p {
		t.Fatal("identity quantification", quantified, e)
	}
	mixedHypothesis, err := m.MkProofHypothesis(mixed)
	interpMust(t, err)
	destination, err := NewInterpolationProof(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, destination.Close()) })
	node, err := translation.Translate(mixedHypothesis, destination)
	interpMust(t, err)
	interpolant, err := destination.Interpolate(node, DownwardScopeRange(0), false)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
}

func TestFullTranslationEliminatesCommutativityWrappers(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	p, err := manager.MakeVariable("commute-p", boolean)
	interpMust(t, err)
	p = interpPin(t, m, p)
	q, err := manager.MakeVariable("commute-q", boolean)
	interpMust(t, err)
	q = interpPin(t, m, q)
	base, err := NewBase(manager, []ast.ASTHandle{p, q}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })

	formula, err := manager.Make(OperatorIff, p, q)
	interpMust(t, err)
	formula = interpPin(t, m, formula)
	commutativity, err := m.MkProofCommutativity(formula)
	interpMust(t, err)
	commutativity = interpPin(t, m, commutativity)
	memo := newTranslationMemo()
	commuted, err := translation.translateMain(commutativity, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(commuted); e != nil || !marked {
		t.Fatal("commutativity marker", marked, e)
	}

	symmetry, err := m.MkProofSymmetry(commutativity)
	interpMust(t, err)
	symmetry = interpPin(t, m, symmetry)
	symmetric, err := translation.translateMain(symmetry, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(symmetric); e != nil || !marked {
		t.Fatal("symmetric commutativity marker", marked, e)
	}

	transitivity, err := m.MkProofTransitivity(commutativity, symmetry)
	interpMust(t, err)
	transitivity = interpPin(t, m, transitivity)
	transitive, err := translation.translateMain(transitivity, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(transitive); e != nil || !marked {
		t.Fatal("transitive commutativity marker", marked, e)
	}

	commutedConclusion, err := manager.Conclusion(commutativity)
	interpMust(t, err)
	left, err := manager.Arg(commutedConclusion, 0)
	interpMust(t, err)
	right, err := manager.Arg(commutedConclusion, 1)
	interpMust(t, err)
	notLeft, err := manager.MkNot(left)
	interpMust(t, err)
	notRight, err := manager.MkNot(right)
	interpMust(t, err)
	iffDeclaration, err := m.GetDecl(commutedConclusion)
	interpMust(t, err)
	monotonicity, err := m.MkProofMonotonicity(iffDeclaration, notLeft, notRight, []ast.ASTHandle{commutativity})
	interpMust(t, err)
	monotonicity = interpPin(t, m, monotonicity)
	monotone, err := translation.translateMain(monotonicity, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(monotone); e != nil || !marked {
		t.Fatal("monotone commutativity marker", marked, e)
	}
	negativeHypothesis, err := m.MkProofHypothesis(notLeft)
	interpMust(t, err)
	negativeHypothesis = interpPin(t, m, negativeHypothesis)
	negativeSymmetry, err := m.MkProofModusPonens(negativeHypothesis, monotonicity)
	interpMust(t, err)
	negativeSymmetry = interpPin(t, m, negativeSymmetry)
	negativeTranslated, err := translation.translateMain(negativeSymmetry, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(negativeTranslated); e != nil || marked {
		t.Fatal("disequality symmetry must consume marker", marked, e)
	}

	hypothesis, err := m.MkProofHypothesis(left)
	interpMust(t, err)
	hypothesis = interpPin(t, m, hypothesis)
	modusPonens, err := m.MkProofModusPonens(hypothesis, commutativity)
	interpMust(t, err)
	modusPonens = interpPin(t, m, modusPonens)
	translated, err := translation.translateMain(modusPonens, false, direct, memo)
	interpMust(t, err)
	if marked, e := translation.isCommuted(translated); e != nil || marked {
		t.Fatal("modus ponens must consume marker", marked, e)
	}
}

func TestFullTranslationRewritesClauseByMonotonicity(t *testing.T) {
	m, manager := interpRewritingManagerFixture(t)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	makeVariable := func(name string) ast.ASTHandle {
		value, e := manager.MakeVariable(name, boolean)
		interpMust(t, e)
		return interpPin(t, m, value)
	}
	p := makeVariable("rewrite-clause-p")
	q := makeVariable("rewrite-clause-q")
	r := makeVariable("rewrite-clause-r")
	frame0, err := manager.Make(OperatorAnd, p, q)
	interpMust(t, err)
	frame1, err := manager.Make(OperatorAnd, r, q)
	interpMust(t, err)
	base, err := NewBase(manager, []ast.ASTHandle{frame0, frame1}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	firstClause, err := manager.Make(OperatorOr, p, q)
	interpMust(t, err)
	secondClause, err := manager.Make(OperatorOr, r, q)
	interpMust(t, err)
	equivalence, err := manager.Make(OperatorIff, p, r)
	interpMust(t, err)
	equivalenceProof, err := m.MkProofHypothesis(equivalence)
	interpMust(t, err)
	equivalenceProof = interpPin(t, m, equivalenceProof)
	congruence, err := m.MkProofCongruence(firstClause, secondClause, []ast.ASTHandle{equivalenceProof})
	interpMust(t, err)
	congruence = interpPin(t, m, congruence)
	clauseProof, err := m.MkProofHypothesis(firstClause)
	interpMust(t, err)
	clauseProof = interpPin(t, m, clauseProof)
	modusPonens, err := m.MkProofModusPonens(clauseProof, congruence)
	interpMust(t, err)
	modusPonens = interpPin(t, m, modusPonens)
	result, err := translation.translateMain(modusPonens, true, direct, newTranslationMemo())
	interpMust(t, err)
	interpolant, err := direct.Interpolate(result)
	interpMust(t, err)
	interpMust(t, m.DelRef(&interpolant))
}

// AUDIT.md interp_02-F8: scan_skolems_rec calls ast_scope on the skolemized
// formula right after add_frame_range. ast_scope memoizes, so a later
// skolemization that adds another frame to the same subterms does not
// change the scope already computed.
func TestFullTranslationScanSkolemsFreezesSkolemScope(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	boolean, err := manager.BoolType()
	interpMust(t, err)
	c, err := manager.MakeVariable("skolem-scope-c", integer)
	interpMust(t, err)
	c = interpPin(t, m, c)
	q, err := manager.MakeVariable("skolem-scope-q", boolean)
	interpMust(t, err)
	q = interpPin(t, m, q)
	sk, err := manager.MakeVariable("skolem-scope-sk", integer)
	interpMust(t, err)
	sk = interpPin(t, m, sk)
	name, err := m.Symbols.Intern("skolem-scope-p")
	interpMust(t, err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	interpMust(t, err)
	p = interpPin(t, m, p)
	pc, err := m.MkApp(p, []ast.ASTHandle{c})
	interpMust(t, err)
	pc = interpPin(t, m, pc)
	psk, err := m.MkApp(p, []ast.ASTHandle{sk})
	interpMust(t, err)
	psk = interpPin(t, m, psk)
	base, err := NewBase(manager, []ast.ASTHandle{pc, q}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	first, err := m.MkProofSkolemization(pc, psk)
	interpMust(t, err)
	first = interpPin(t, m, first)
	second, err := m.MkProofSkolemization(q, psk)
	interpMust(t, err)
	second = interpPin(t, m, second)
	interpMust(t, translation.ScanSkolems(first))
	interpMust(t, translation.ScanSkolems(second))
	scope, err := base.ASTScope(psk)
	interpMust(t, err)
	if scope.Lo != 0 || scope.Hi != 0 {
		t.Fatal("skolemized formula scope was not frozen after the first skolemization", scope)
	}
}

// AUDIT.md interp_02-F7: get_locality_rec calls iproof->make_axiom(lits) for
// every PR_QUANT_INST it visits, which localizes the instance's mixed
// literals in the current iproof before any rule is translated.
func TestFullTranslationLocalityLocalizesQuantInstLiterals(t *testing.T) {
	m, manager := interpProofManagerFixture(t)
	integer, err := manager.IntType()
	interpMust(t, err)
	a, err := manager.MakeVariable("locality-qi-a", integer)
	interpMust(t, err)
	a = interpPin(t, m, a)
	b, err := manager.MakeVariable("locality-qi-b", integer)
	interpMust(t, err)
	b = interpPin(t, m, b)
	zero, err := manager.MakeIntText("0")
	interpMust(t, err)
	aFrame, err := manager.Make(OperatorLE, a, zero)
	interpMust(t, err)
	aFrame = interpPin(t, m, aFrame)
	bFrame, err := manager.Make(OperatorLE, b, zero)
	interpMust(t, err)
	bFrame = interpPin(t, m, bFrame)
	base, err := NewBase(manager, []ast.ASTHandle{aFrame, bFrame}, nil, nil)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, base.Close()) })
	translation, err := NewFullTranslation(base)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, translation.Close()) })
	direct, err := NewDirectProofInterpolator(base, DownwardScopeRange(0), false)
	interpMust(t, err)
	t.Cleanup(func() { interpMust(t, direct.Close()) })
	// quant-inst of (or (not (forall x (<= x a))) (<= b a)): the instance
	// literal (<= b a) mixes the A symbol a and the B symbol b.
	variable, err := m.MkVar(0, integer)
	interpMust(t, err)
	body, err := manager.Make(OperatorLE, variable, a)
	interpMust(t, err)
	symbol, err := m.Symbols.Intern("x")
	interpMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{symbol}, Body: body, Weight: 1})
	interpMust(t, err)
	quantifier = interpPin(t, m, quantifier)
	notQuantifier, err := manager.MkNot(quantifier)
	interpMust(t, err)
	instance, err := manager.Make(OperatorLE, b, a)
	interpMust(t, err)
	instance = interpPin(t, m, instance)
	clause, err := manager.Make(OperatorOr, notQuantifier, instance)
	interpMust(t, err)
	clause = interpPin(t, m, clause)
	proof, err := m.MkProofQuantInstantiation(clause, []ast.ASTHandle{b})
	interpMust(t, err)
	proof = interpPin(t, m, proof)
	translation.direct = direct
	defer func() { translation.direct = nil }()
	if _, err = translation.Locality(proof); err != nil {
		t.Fatal(err)
	}
	// make_axiom narrows the range to the first literal's (frame A), so b,
	// the B symbol of the instance, gets the first localization variable.
	if len(direct.localizationVariables) != 1 || direct.localizationVariables[0].term != b {
		t.Fatal("quant-inst literals were not localized by get_locality", len(direct.localizationVariables))
	}
}
