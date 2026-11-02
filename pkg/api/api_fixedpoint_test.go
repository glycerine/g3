package api

// Public fixedpoint lifecycle through the retained PDR command integration.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextFixedpointPDRLifecycle(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	references := make([]ast.ASTRef, 0)
	pin := func(handle ast.ASTHandle, x error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, x)
		reference, x := manager.NewRef(handle)
		astTestMust(t, x)
		references = append(references, reference)
		return handle
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	astTestMust(t, err)
	integer := pin(manager.MkFamilySort(arithmetic, 1, nil))
	zero := pin(manager.ArithmeticIntegerNumeralText("0"))
	truth := pin(manager.MkTrue())

	astTestMust(t, context.SetFixedpointEngine("pdr"))
	astTestMust(t, context.SetFixedpointBool(FixedpointTransformSlice, false))
	p, err := context.DeclareRelation("api-fp-P", []ast.ASTHandle{integer})
	astTestMust(t, err)
	q, err := context.DeclareRelation("api-fp-Q", []ast.ASTHandle{integer})
	astTestMust(t, err)
	u, err := context.DeclareRelation("api-fp-U", []ast.ASTHandle{integer})
	astTestMust(t, err)
	pZero := pin(manager.MkApp(p, []ast.ASTHandle{zero}))
	uZero := pin(manager.MkApp(u, []ast.ASTHandle{zero}))
	x := pin(context.DeclareFixedpointVariable("api-fp-x", integer))
	pX := pin(manager.MkApp(p, []ast.ASTHandle{x}))
	qX := pin(manager.MkApp(q, []ast.ASTHandle{x}))
	step := pin(manager.MkImplies(pX, qX))
	astTestMust(t, context.PushFixedpoint())
	astTestMust(t, context.AddFixedpointRuleWithBound(uZero, "api-fp-scoped", 3))
	astTestMust(t, context.AssertFixedpoint(truth))
	assertions, err := context.FixedpointAssertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("scoped fixedpoint assertion enumeration", len(assertions))
	}
	for index := range assertions {
		astTestMust(t, manager.DelRef(&assertions[index]))
	}
	rules, err := context.FixedpointRules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("scoped fixedpoint rule enumeration", len(rules))
	}
	for index := range rules {
		astTestMust(t, manager.DelRef(&rules[index]))
	}
	astTestMust(t, context.PopFixedpoint())
	rules, err = context.FixedpointRules()
	astTestMust(t, err)
	if len(rules) != 0 {
		t.Fatal("popped fixedpoint rule enumeration", len(rules))
	}
	assertions, err = context.FixedpointAssertions()
	astTestMust(t, err)
	if len(assertions) != 0 {
		t.Fatal("popped fixedpoint assertion enumeration", len(assertions))
	}
	astTestMust(t, context.AddFixedpointFact(p, []ast.ASTHandle{zero}))
	astTestMust(t, context.AddFixedpointRule(step, "api-fp-step"))
	astTestMust(t, context.UpdateFixedpointRule(step, "api-fp-step"))
	astTestMust(t, context.AssertFixedpoint(truth))
	rules, err = context.FixedpointRules()
	astTestMust(t, err)
	// UpdateRule before the first flush adds a normalized clause; it does not
	// replace the still-pending raw step. Source enumeration therefore has
	// the fact plus both copies of the step.
	if len(rules) != 3 {
		t.Fatal("fixedpoint rule enumeration", len(rules))
	}
	for index := range rules {
		astTestMust(t, manager.DelRef(&rules[index]))
	}
	if reason, x := context.FixedpointReasonUnknown(); x != nil || reason != "ok" {
		t.Fatal("initial fixedpoint status", reason, x)
	}

	status, err := context.QueryFixedpointRelations([]ast.ASTHandle{q})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable public fixedpoint query", status)
	}
	if reason, x := context.FixedpointReasonUnknown(); x != nil || reason != "ok" {
		t.Fatal("decided fixedpoint status", reason, x)
	}
	answer, err := context.FixedpointAnswer(q)
	astTestMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	astTestMust(t, err)
	if !manager.IsTrue(answerHandle) {
		t.Fatal("public fixedpoint answer is not true")
	}
	astTestMust(t, manager.DelRef(&answer))
	if unexpected, x := context.FixedpointModel(q); x == nil {
		astTestMust(t, unexpected.Close())
		t.Fatal("reachable fixedpoint query exposed an inductive model")
	}

	levels, err := context.FixedpointNumLevels(q)
	astTestMust(t, err)
	if levels == 0 {
		t.Fatal("public fixedpoint query retained no PDR levels")
	}
	bound := pin(manager.MkVar(0, integer))
	coverProperty := pin(manager.MkEq(bound, zero))
	astTestMust(t, context.AddFixedpointCover(-1, q, coverProperty))
	cover, err := context.FixedpointCoverDelta(-1, q)
	astTestMust(t, err)
	coverHandle, err := manager.RefGet(cover)
	astTestMust(t, err)
	if coverHandle != coverProperty {
		t.Fatalf("public fixedpoint invariant cover = %v, want %v", coverHandle, coverProperty)
	}
	astTestMust(t, manager.DelRef(&cover))
	if err = context.AddFixedpointCover(-2, q, coverProperty); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid public fixedpoint cover level", err)
	}

	status, err = context.QueryFixedpoint(u)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable public fixedpoint query", status)
	}
	if reason, x := context.FixedpointReasonUnknown(); x != nil || reason != "ok" {
		t.Fatal("unreachable fixedpoint status", reason, x)
	}
	certificate, err := context.FixedpointAnswer(u)
	astTestMust(t, err)
	certificateHandle, err := manager.RefGet(certificate)
	astTestMust(t, err)
	if manager.IsTrue(certificateHandle) {
		t.Fatal("unreachable public fixedpoint query returned an empty certificate")
	}
	astTestMust(t, manager.DelRef(&certificate))
	inductiveModel, err := context.FixedpointModel(u)
	astTestMust(t, err)
	for _, declaration := range []ast.ASTHandle{p, q} {
		present, x := inductiveModel.Core().HasInterpretation(declaration)
		astTestMust(t, x)
		if !present {
			t.Fatal("fixedpoint model omitted retained relation", declaration)
		}
		interpretation, x := inductiveModel.Core().FunctionInterpretation(declaration)
		astTestMust(t, x)
		if interpretation == nil {
			t.Fatal("fixedpoint model omitted relation function", declaration)
		}
		fallback, x := interpretation.Else()
		astTestMust(t, x)
		if fallback.IsNull() {
			t.Fatal("fixedpoint model relation has no interpretation body", declaration)
		}
	}
	if present, x := inductiveModel.Core().HasInterpretation(u); x != nil || present {
		t.Fatal("fixedpoint model retained the unreachable query relation", present, x)
	}
	astTestMust(t, inductiveModel.Close())
	qZero := pin(manager.MkApp(q, []ast.ASTHandle{zero}))
	for _, application := range []ast.ASTHandle{pZero, qZero} {
		value, x := context.EvaluateFixedpointModel(u, application, true)
		astTestMust(t, x)
		handle, x := manager.RefGet(value)
		astTestMust(t, x)
		if !manager.IsTrue(handle) {
			appText, _ := context.ASTString(application, APIASTPrintSMT2)
			valueText, _ := context.ASTString(handle, APIASTPrintSMT2)
			t.Fatal("fixedpoint model does not satisfy reachable relation", appText, valueText)
		}
		astTestMust(t, manager.DelRef(&value))
	}
	stats, err := context.FixedpointStatistics()
	astTestMust(t, err)
	if stats.Size() == 0 {
		t.Fatal("public fixedpoint statistics are empty")
	}
	astTestMust(t, context.ResetFixedpointStatistics())
	status, err = context.QueryFixedpointRelations([]ast.ASTHandle{p, q})
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("multi-relation PDR query status", status)
	}
	if reason, x := context.FixedpointReasonUnknown(); x != nil || reason != "approximated" {
		t.Fatal("undecided fixedpoint status", reason, x)
	}

	for index := len(references) - 1; index >= 0; index-- {
		astTestMust(t, manager.DelRef(&references[index]))
	}
	astTestMust(t, context.Close())
	if _, err = context.QueryFixedpoint(p); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed public fixedpoint query", err)
	}
}

func TestSMTContextFixedpointSMT2Replay(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	p, err := context.DeclareRelation("fp-replay-P", nil)
	astTestMust(t, err)
	q, err := context.DeclareRelation("fp-replay-Q", nil)
	astTestMust(t, err)
	pAtom, err := manager.MkApp(p, nil)
	astTestMust(t, err)
	qAtom, err := manager.MkApp(q, nil)
	astTestMust(t, err)
	step, err := manager.MkImplies(pAtom, qAtom)
	astTestMust(t, err)
	truth, err := manager.MkTrue()
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointFact(p, nil))
	astTestMust(t, context.AddFixedpointRule(step, "fp-replay-edge"))
	astTestMust(t, context.AssertFixedpoint(truth))
	script, err := context.FixedpointSMT2()
	astTestMust(t, err)
	for _, fragment := range []string{
		"(declare-rel fp-replay-P ())",
		"(declare-rel fp-replay-Q ())",
		"(assert true)",
		":named fp-replay-edge",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("fixedpoint replay omitted %q:\n%s", fragment, script)
		}
	}
	if err = context.WriteFixedpointSMT2(nil); err == nil {
		t.Fatal("nil fixedpoint replay writer succeeded")
	}

	replay, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, replay.Close()) })
	astTestMust(t, replay.LoadFixedpointSMT2(strings.NewReader(script)))
	var output strings.Builder
	astTestMust(t, replay.ExecuteSMT2(strings.NewReader("(query fp-replay-Q)\n"), &output))
	if strings.TrimSpace(output.String()) != "sat" {
		t.Fatalf("fixedpoint replay result %q from:\n%s", output.String(), script)
	}
	if err = replay.LoadFixedpointSMT2(nil); err == nil {
		t.Fatal("nil fixedpoint replay reader succeeded")
	}
}

func TestSMTContextFixedpointPredicateRepresentation(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	predicate, err := context.DeclareRelation("api-fp-representation", []ast.ASTHandle{boolean})
	astTestMust(t, err)
	astTestMust(t, context.SetFixedpointPredicateRepresentation(predicate, "tr_sparse", "tr_hashtable", "tr_bitvector", "tr_lazy_sparse"))
	zero, err := context.DeclareRelation("api-fp-zero-representation", nil)
	astTestMust(t, err)
	if err = context.SetFixedpointPredicateRepresentation(zero, "tr_sparse"); err == nil {
		t.Fatal("sparse representation accepted zero-arity relation")
	}
	if err = context.SetFixedpointPredicateRepresentation(zero, "missing"); err == nil {
		t.Fatal("unknown predicate representation accepted")
	}
	truth, err := manager.MkTrue()
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointFact(predicate, []ast.ASTHandle{truth}))
	status, err := context.QueryFixedpoint(predicate)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("represented fixedpoint relation", status)
	}
	if err = context.SetFixedpointPredicateRepresentation(predicate, "tr_hashtable"); err == nil {
		t.Fatal("predicate representation changed after query")
	}
}

func TestSMTContextFixedpointParameterDescriptors(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	descriptors, err := context.FixedpointParameterDescriptors()
	astTestMust(t, err)
	if len(descriptors) < 50 {
		t.Fatal("incomplete fixedpoint parameter catalog", len(descriptors))
	}
	want := map[string]struct{ kind, value string }{
		"fixedpoint.engine":           {"symbol", "auto-config"},
		"fixedpoint.pdr.farkas":       {"bool", "true"},
		"fixedpoint.spacer.max_level": {"unsigned int", "4294967295"},
	}
	for _, descriptor := range descriptors {
		expected, found := want[descriptor.Name]
		if !found {
			continue
		}
		if descriptor.Kind != expected.kind || descriptor.Default != expected.value || descriptor.Description == "" {
			t.Fatal("fixedpoint parameter descriptor", descriptor)
		}
		delete(want, descriptor.Name)
	}
	if len(want) != 0 {
		t.Fatal("missing fixedpoint parameter descriptors", want)
	}
}

func TestSMTContextFixedpointParameterUpdateRebuildsPDR(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	astTestMust(t, context.SetFixedpointEngine("pdr"))
	predicate, err := context.DeclareRelation("api-fp-parameter-P", nil)
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	fact, err := manager.MkApp(predicate, nil)
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointRule(fact, "api-fp-parameter-fact"))
	status, err := context.QueryFixedpoint(predicate)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("initial fixedpoint parameter query", status)
	}
	astTestMust(t, context.SetUIntOption("fixedpoint.pdr.cache_mode", 3))
	if _, err = context.QueryFixedpoint(predicate); err == nil || !strings.Contains(err.Error(), "invalid pdr.cache_mode 3") {
		t.Fatal("fixedpoint parameter update reused stale PDR context", err)
	}
}

func TestSMTContextFixedpointQueryOptionsAreLocalAndTyped(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	astTestMust(t, context.SetFixedpointEngine("duality"))
	predicate, err := context.DeclareRelation("api-fp-local-P", nil)
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	fact, err := manager.MkApp(predicate, nil)
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointRule(fact, "api-fp-local-fact"))
	options := &FixedpointQueryOptions{
		Bool: map[FixedpointBoolParameter]bool{
			FixedpointDatalogSubsumption: false,
		},
		UInt: map[FixedpointUIntParameter]uint32{
			FixedpointTimeout: 0,
		},
		Symbol: map[FixedpointSymbolParameter]string{
			FixedpointEngine: "datalog",
		},
	}
	status, err := context.QueryFixedpointWithOptions(predicate, options)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query-local fixedpoint options", status)
	}
	status, err = context.QueryFixedpoint(predicate)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("global Duality query", status)
	}
	if answer, err := context.FixedpointAnswer(predicate); err == nil || answer != (ast.ASTRef{}) {
		t.Fatal("query-local Datalog selection leaked into Duality", answer, err)
	}
	proof, err := context.FixedpointProof(predicate)
	astTestMust(t, err)
	proofHandle, err := manager.RefGet(proof)
	astTestMust(t, err)
	conclusion, err := manager.GetProofFact(proofHandle)
	astTestMust(t, err)
	if !manager.IsFalse(conclusion) {
		t.Fatal("Duality query proof conclusion")
	}
	astTestMust(t, manager.DelRef(&proof))
	status, err = context.QueryFixedpointWithOptions(predicate, options)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("query-local fixedpoint options after global Duality query", status)
	}
	bad := &FixedpointQueryOptions{Bool: map[FixedpointBoolParameter]bool{FixedpointBoolParameter(FixedpointEngine): true}}
	if _, err = context.QueryFixedpointWithOptions(predicate, bad); err == nil || !strings.Contains(err.Error(), "has kind symbol, expected bool") {
		t.Fatal("query-local fixedpoint kind guard", err)
	}
}

func TestSMTContextFixedpointPDRProofTrace(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	astTestMust(t, context.SetFixedpointEngine("pdr"))
	astTestMust(t, context.SetBoolOption("fixedpoint.generate_proof_trace", true))
	astTestMust(t, context.SetBoolOption("fixedpoint.xform.inline_eager", false))
	astTestMust(t, context.SetBoolOption("fixedpoint.xform.inline_linear", false))
	manager, err := context.Manager()
	astTestMust(t, err)
	p, err := context.DeclareRelation("api-proof-P", nil)
	astTestMust(t, err)
	q, err := context.DeclareRelation("api-proof-Q", nil)
	astTestMust(t, err)
	pAtom, err := manager.MkApp(p, nil)
	astTestMust(t, err)
	qAtom, err := manager.MkApp(q, nil)
	astTestMust(t, err)
	step, err := manager.MkImplies(pAtom, qAtom)
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointRule(pAtom, "api-proof-fact"))
	astTestMust(t, context.AddFixedpointRule(step, "api-proof-step"))
	status, err := context.QueryFixedpoint(q)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("proof-trace query", status)
	}
	proof, err := context.FixedpointProof(q)
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&proof)) }()
	proofHandle, err := manager.RefGet(proof)
	astTestMust(t, err)
	hyper, err := manager.IsProofKind(proofHandle, ast.ASTPrHyperResolve)
	astTestMust(t, err)
	asserted, err := manager.IsProofKind(proofHandle, ast.ASTPrAsserted)
	astTestMust(t, err)
	parents, err := manager.GetProofNumParents(proofHandle)
	astTestMust(t, err)
	fact, err := manager.GetProofFact(proofHandle)
	astTestMust(t, err)
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	factSort, err := manager.GetSort(fact)
	astTestMust(t, err)
	if !hyper && !asserted || hyper && parents == 0 || factSort != boolean {
		t.Fatal("PDR proof trace", hyper, asserted, parents, factSort, boolean)
	}
	answer, err := context.FixedpointAnswer(q)
	astTestMust(t, err)
	answerHandle, err := manager.RefGet(answer)
	astTestMust(t, err)
	answerIsHyper, err := manager.IsProofKind(answerHandle, ast.ASTPrHyperResolve)
	astTestMust(t, err)
	answerIsAsserted, err := manager.IsProofKind(answerHandle, ast.ASTPrAsserted)
	astTestMust(t, err)
	astTestMust(t, manager.DelRef(&answer))
	if !answerIsHyper && !answerIsAsserted {
		t.Fatal("generate_proof_trace did not select the proof as the reachable answer")
	}
}

func TestSMTContextFixedpointSpacerInductiveExports(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	manager, err := context.Manager()
	astTestMust(t, err)
	astTestMust(t, context.SetFixedpointEngine("spacer"))
	arithmetic, err := manager.ArithmeticFamilyID()
	astTestMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	p, err := context.DeclareRelation("api-spacer-P", []ast.ASTHandle{integer})
	astTestMust(t, err)
	u, err := context.DeclareRelation("api-spacer-U", nil)
	astTestMust(t, err)
	pAtom, err := manager.MkApp(p, []ast.ASTHandle{zero})
	astTestMust(t, err)
	pOne, err := manager.MkApp(p, []ast.ASTHandle{one})
	astTestMust(t, err)
	uAtom, err := manager.MkApp(u, nil)
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointRule(pAtom, "api-spacer-fact"))
	recursive, err := manager.MkImplies(pOne, uAtom)
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointRule(recursive, "api-spacer-recursive"))
	status, err := context.QueryFixedpointFromLevel(u, 1)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("Spacer public unreachable query", status)
	}
	certificate, err := context.FixedpointAnswer(u)
	astTestMust(t, err)
	certificateHandle, err := manager.RefGet(certificate)
	astTestMust(t, err)
	if manager.IsTrue(certificateHandle) {
		t.Fatal("Spacer public certificate omitted retained predicates")
	}
	astTestMust(t, manager.DelRef(&certificate))
	inductiveModel, err := context.FixedpointModel(u)
	astTestMust(t, err)
	present, err := inductiveModel.Core().HasInterpretation(p)
	astTestMust(t, err)
	if !present {
		t.Fatal("Spacer public model omitted retained predicate")
	}
	astTestMust(t, inductiveModel.Close())
	value, err := context.EvaluateFixedpointModel(u, pAtom, true)
	astTestMust(t, err)
	valueHandle, err := manager.RefGet(value)
	astTestMust(t, err)
	if !manager.IsTrue(valueHandle) {
		t.Fatal("Spacer public model does not satisfy fact", valueHandle)
	}
	astTestMust(t, manager.DelRef(&value))
	falsity, err := manager.MkFalse()
	astTestMust(t, err)
	astTestMust(t, context.AddFixedpointInvariant(u, falsity))
	cover, err := context.FixedpointCoverDelta(-1, u)
	astTestMust(t, err)
	coverHandle, err := manager.RefGet(cover)
	astTestMust(t, err)
	if !manager.IsFalse(coverHandle) {
		t.Fatal("Spacer public invariant cover", coverHandle)
	}
	astTestMust(t, manager.DelRef(&cover))
	status, err = context.QueryFixedpoint(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("Spacer public fact query", status)
	}
	reachable, err := context.FixedpointReachable(p)
	astTestMust(t, err)
	reachableHandle, err := manager.RefGet(reachable)
	astTestMust(t, err)
	parts, err := manager.FlattenAnd([]ast.ASTHandle{reachableHandle})
	astTestMust(t, err)
	foundVariable := false
	for _, part := range parts {
		kind, x := manager.GetKind(part)
		astTestMust(t, x)
		if kind != ast.ASTApp {
			continue
		}
		operation, x := manager.GetDeclKind(part)
		astTestMust(t, x)
		if operation != ast.ASTOpEq {
			continue
		}
		for index := uint32(0); index < 2; index++ {
			argument, x := manager.GetArg(part, index)
			astTestMust(t, x)
			argumentKind, x := manager.GetKind(argument)
			astTestMust(t, x)
			foundVariable = foundVariable || argumentKind == ast.ASTVar
		}
	}
	if !foundVariable {
		var text strings.Builder
		astTestMust(t, context.command.WriteExpression(&text, reachableHandle))
		t.Fatal("Spacer reachable summary did not export a column variable", text.String())
	}
	astTestMust(t, manager.DelRef(&reachable))
	ground, err := context.FixedpointGroundSatAnswer(p)
	astTestMust(t, err)
	groundHandle, err := manager.RefGet(ground)
	astTestMust(t, err)
	if groundHandle.IsNull() {
		t.Fatal("Spacer public ground answer is empty")
	}
	astTestMust(t, manager.DelRef(&ground))
	trace, names, err := context.FixedpointRulesAlongTrace()
	astTestMust(t, err)
	if len(trace) != 1 || len(names) != 1 || names[0] != ";api-spacer-fact" {
		t.Fatal("Spacer public trace rules", len(trace), names)
	}
	traceHandle, err := manager.RefGet(trace[0])
	astTestMust(t, err)
	if traceHandle.IsNull() {
		t.Fatal("Spacer public trace formula is empty")
	}
	for index := len(trace) - 1; index >= 0; index-- {
		astTestMust(t, manager.DelRef(&trace[index]))
	}
}
