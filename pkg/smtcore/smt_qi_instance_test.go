package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtQIInstanceBuilder(t *testing.T, m *ast.ASTManager, context *SMTCheckerContext) *SMTQIInstanceBuilder {
	t.Helper()
	checker, err := NewSMTChecker(context)
	astTestMust(t, err)
	substitution, err := NewSMTCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, substitution.Close()) })
	costs, err := NewSMTQICostModel(m, "(+ weight generation)", "cost", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	builder, err := NewSMTQIInstanceBuilder(m, checker, substitution, costs)
	astTestMust(t, err)
	return builder
}

func TestSMTQIInstancePrepareSubstituteAndBuildFlattenedLemma(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "qi-instance-x")},
		Body: variable, Weight: 4,
	})
	astTestMust(t, err)
	_, binding := smtEnodeConstant(t, m, 2080, boolean, 0)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	builder := smtQIInstanceBuilder(t, m, context)
	entry := SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: quantifier, args: []*SMTEnode{binding}}, Cost: 7, Generation: 3, Instantiated: true}
	prepared, err := builder.Prepare(entry)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	instance, err := prepared.Instance()
	astTestMust(t, err)
	if instance != binding.owner {
		t.Fatal("cached variable substitution", instance, binding.owner)
	}
	a, err := m.MkConst(astTestSymbol(t, m, "qi-lemma-a"), boolean)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "qi-lemma-b"), boolean)
	astTestMust(t, err)
	disjunction, err := m.MkOr(a, b)
	astTestMust(t, err)
	stat := &SMTQuantifierStat{}
	result, err := builder.Finish(prepared, disjunction, stat, 2)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	lemma, err := result.Lemma()
	astTestMust(t, err)
	if result.Generation() != 7 || stat.NumInstances() != 1 || stat.NumInstancesCurrentSearch() != 1 {
		t.Fatal("instance statistics or generation", result.Generation(), stat)
	}
	if op, err := m.GetDeclKind(lemma); err != nil || op != ast.ASTOpOr {
		t.Fatal("guarded lemma operator", op, err)
	}
	if count, err := m.GetNumArgs(lemma); err != nil || count != 3 {
		t.Fatal("flattened source disjunction", count, err)
	}
	guard, err := m.GetArg(lemma, 0)
	astTestMust(t, err)
	wantGuard, err := m.MkNot(quantifier)
	astTestMust(t, err)
	if guard != wantGuard {
		t.Fatal("quantifier guard", guard, wantGuard)
	}
}

func TestSMTQIInstanceMaterializeInvokesTheoryRewriter(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	negated, err := m.MkNot(variable)
	astTestMust(t, err)
	body, err := m.MkNot(negated)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "qi-materialize-x")}, Body: body,
	})
	astTestMust(t, err)
	bindingOwner, binding := smtEnodeConstant(t, m, 2085, boolean, 4)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	builder := smtQIInstanceBuilder(t, m, context)
	entry := SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: quantifier, args: []*SMTEnode{binding}}, Cost: 2, Generation: 4, Instantiated: true}
	prepared, err := builder.Prepare(entry)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	stat := &SMTQuantifierStat{}
	result, err := builder.Materialize(prepared, smtQuickTheoryRewriter(t, m), stat, 1)
	astTestMust(t, err)
	if result == nil {
		t.Fatal("materialization omitted instance")
	}
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	lemma, err := result.Lemma()
	astTestMust(t, err)
	if count, err := m.GetNumArgs(lemma); err != nil || count != 2 {
		t.Fatal("materialized lemma", count, err)
	}
	rewritten, err := m.GetArg(lemma, 1)
	astTestMust(t, err)
	if rewritten != bindingOwner || stat.NumInstances() != 1 {
		t.Fatal("theory rewrite or statistics", rewritten, bindingOwner, stat.NumInstances())
	}
}

func TestSMTQIInstanceSatisfiedAndReducedTrueShortCircuits(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(astTestSymbol(t, m, "QIInstanceSort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	xOwner, x := smtEnodeConstant(t, m, 2090, sort, 0)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{}, false)
	astTestMust(t, err)
	predicate, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "qi-instance-p"), []ast.ASTHandle{sort}, boolean, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	pxOwner, err := m.MkApp(predicate, []ast.ASTHandle{xOwner})
	astTestMust(t, err)
	px, err := NewSMTEnode(m, pxOwner, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{xOwner: x}), SMTEnodeOptions{CongruenceEnabled: true})
	astTestMust(t, err)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	body, err := m.MkApp(predicate, []ast.ASTHandle{variable})
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []symbol.Symbol{astTestSymbol(t, m, "qi-instance-y")}, Body: body, Weight: 1,
	})
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	astTestMust(t, context.RegisterEnode(x, true))
	astTestMust(t, context.RegisterEnode(px, true))
	astTestMust(t, context.SetAssignment(pxOwner, lbool.True))
	builder := smtQIInstanceBuilder(t, m, context)
	entry := SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: quantifier, args: []*SMTEnode{x}}, Cost: 1, Generation: 0, Instantiated: true}
	prepared, err := builder.Prepare(entry)
	astTestMust(t, err)
	if !prepared.AlreadySatisfied() {
		t.Fatal("checker did not suppress satisfied instance")
	}
	astTestMust(t, prepared.Close())

	emptyContext, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	otherBuilder := smtQIInstanceBuilder(t, m, emptyContext)
	prepared, err = otherBuilder.Prepare(entry)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	truth, err := m.MkTrue()
	astTestMust(t, err)
	stat := &SMTQuantifierStat{}
	result, err := otherBuilder.Finish(prepared, truth, stat, 0)
	astTestMust(t, err)
	if !result.ReducedTrue() || stat.NumInstances() != 0 {
		t.Fatal("true rewrite should not publish an instance", result.ReducedTrue(), stat.NumInstances())
	}
	astTestMust(t, result.Close())
}

func TestSMTQIInstanceQuantifierInstantiationProof(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, "qi-proof-x")}, Body: variable, Weight: 2,
	})
	astTestMust(t, err)
	_, binding := smtEnodeConstant(t, m, 2100, boolean, 0)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	builder := smtQIInstanceBuilder(t, m, context)
	entry := SMTQIQueueEntry{Fingerprint: &SMTFingerprint{data: quantifier, args: []*SMTEnode{binding}}, Cost: 2, Generation: 1, Instantiated: true}
	prepared, err := builder.Prepare(entry)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	instance, err := prepared.Instance()
	astTestMust(t, err)
	stat := &SMTQuantifierStat{}
	result, err := builder.FinishWithProof(prepared, instance, ast.ASTHandle{}, stat, 0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	lemma, err := result.Lemma()
	astTestMust(t, err)
	proof, err := result.Proof()
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if proof.IsNull() || fact != lemma {
		t.Fatal("quantifier instantiation proof fact", proof, fact, lemma)
	}
	var proofLemma ast.ASTHandle
	var bindings []ast.ASTHandle
	matched, err := m.MatchProofQuantInstantiation(proof, &proofLemma, &bindings)
	astTestMust(t, err)
	if !matched || proofLemma != lemma || len(bindings) != 1 || bindings[0] != binding.owner {
		t.Fatal("quantifier instantiation proof parameters", matched, proofLemma, bindings)
	}
	negated, err := m.MkNot(instance)
	astTestMust(t, err)
	simplified, err := m.MkNot(negated)
	astTestMust(t, err)
	rewriteProof, err := m.MkProofRewrite(instance, simplified)
	astTestMust(t, err)
	rewritten, err := builder.FinishWithProof(prepared, simplified, rewriteProof, &SMTQuantifierStat{}, 0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rewritten.Close()) })
	rewrittenLemma, err := rewritten.Lemma()
	astTestMust(t, err)
	rewrittenProof, err := rewritten.Proof()
	astTestMust(t, err)
	rewrittenFact, err := m.GetProofFact(rewrittenProof)
	astTestMust(t, err)
	if rewrittenFact != rewrittenLemma {
		t.Fatal("rewrite congruence proof chain", rewrittenFact, rewrittenLemma)
	}
}
