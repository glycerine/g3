package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtRecursiveTestDefinition(t *testing.T, m *ast.ASTManager, name string) (q, f ast.ASTHandle) {
	t.Helper()
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, b)
	astTestMust(t, err)
	f, err = m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	lhs, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	rhs, err := m.MkNot(x)
	astTestMust(t, err)
	body, err := m.MkEq(lhs, rhs)
	astTestMust(t, err)
	left, err := m.MkPattern([]ast.ASTHandle{lhs})
	astTestMust(t, err)
	right, err := m.MkPattern([]ast.ASTHandle{rhs})
	astTestMust(t, err)
	qid, err := m.RecursiveFunctionQID()
	astTestMust(t, err)
	q, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")}, Body: body, QID: qid, Patterns: []ast.ASTHandle{left, right}})
	astTestMust(t, err)
	astTestPin(t, m, q)
	return q, f
}

func TestSMTRecursiveDefinitionActivatesOnlyFunctionTrigger(t *testing.T) {
	m, state, _, manager := smtQuantifierManagerFixture(t, 10, 20, SMTQuantifierManagerConfig{Ematching: true, MaxEagerMultipatterns: 10, MaxLazyMultipatternMatching: 2})
	q, _ := smtRecursiveTestDefinition(t, m, "recursive-trigger")
	astTestMust(t, state.Internalize(q, true))
	astTestMust(t, state.context.SetRelevant(q, true))
	astTestMust(t, state.context.SetAssignment(q, lbool.True))
	astTestMust(t, manager.AssignQuantifier(q))
	astTestMust(t, manager.Propagate())
	first, err := m.GetPattern(q, 0)
	astTestMust(t, err)
	eager := smtMAMTestPatterns(t, manager.mam)
	if len(eager) != 1 || eager[0] != first || len(smtMAMTestPatterns(t, manager.lazyMAM)) != 0 {
		t.Fatal("recursive body trigger was installed as an ordinary trigger")
	}
}

func TestSMTRecursiveCheckerUsesRelevantCallsAndDirectGeneration(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	state := s.kernel.internalizer
	q, f := smtRecursiveTestDefinition(t, m, "recursive-check")
	astTestMust(t, state.SetGeneration(17))
	astTestMust(t, state.Internalize(q, true))
	yes, err := m.MkTrue()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	var calls []ast.ASTHandle
	for _, value := range []ast.ASTHandle{yes, no} {
		call, err := m.MkApp(f, []ast.ASTHandle{value})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(call, false))
		astTestMust(t, state.context.SetRelevant(call, true))
		calls = append(calls, call)
	}
	proto, err := protomodel.NewProtoModel(m, cfg)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	fi, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, fi.SetElse(no))
	astTestMust(t, proto.Core().RegisterFunction(f, fi))
	checker := state.quantifierManager.modelChecker
	satisfied, err := checker.checkRecursiveFunction(proto, q, false)
	astTestMust(t, err)
	if satisfied || len(checker.newInstances) != 1 || checker.newInstances[0].bindings[0] != no || checker.newInstances[0].generation != 0 {
		t.Fatal("recursive checker did not retain the first false ground call with source generation zero", satisfied, checker.newInstances)
	}
	astTestMust(t, checker.ResetNewInstances())
	state.relevancy.enabled = true
	astTestMust(t, state.context.SetRelevant(calls[1], false))
	satisfied, err = checker.checkRecursiveFunction(proto, q, false)
	astTestMust(t, err)
	if !satisfied || checker.HasNewInstances() {
		t.Fatal("recursive checker did not skip irrelevant ground call")
	}
}

func TestSMTRecursiveModelPublicationAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 256 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			k := smtTestKernel(t, m)
			q, f := smtRecursiveTestDefinition(t, m, "recursive-stopped-model")
			proto, err := protomodel.NewProtoModel(m, cfg)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, proto.Close()) })
			m.Budget().FailAfter(stop)
			err = k.registerRecursiveFunctionModel(proto.Core(), q)
			m.Budget().FailAfter(-1)
			if err != nil {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("unexpected stopped recursive model owner", err)
				}
			} else {
				succeeded = true
				fi, err := proto.Core().FunctionInterpretation(f)
				astTestMust(t, err)
				if fi == nil {
					t.Fatal("successful recursive function model not published")
				}
			}
		})
		if t.Failed() {
			t.Fatalf("recursive model frontier %d", stop)
		}
	}
	if !succeeded {
		t.Fatal("no successful recursive model publication frontier")
	}
}

func TestSMTRecursiveCheckerStrictFallbackAndGroundModelValidation(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "AUFLIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	state := s.kernel.internalizer
	q, f := smtRecursiveTestDefinition(t, m, "recursive-strict")
	astTestMust(t, state.Internalize(q, true))
	b, err := m.MkBoolSort()
	astTestMust(t, err)
	x, err := m.MkVar(0, b)
	astTestMust(t, err)
	unknown, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []symbol.Symbol{astTestSymbol(t, m, "recursive-symbolic-value")}, Body: x})
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	call, err := m.MkApp(f, []ast.ASTHandle{yes})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(call, false))
	astTestMust(t, state.context.SetRelevant(call, true))
	proto, err := protomodel.NewProtoModel(m, cfg)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, proto.Close()) })
	fi, err := model.NewFunctionInterpretation(m, 1)
	astTestMust(t, err)
	astTestMust(t, fi.SetElse(unknown))
	astTestMust(t, proto.Core().RegisterFunction(f, fi))
	checker := state.quantifierManager.modelChecker
	satisfied, err := checker.checkRecursiveFunction(proto, q, false)
	astTestMust(t, err)
	if !satisfied || checker.HasNewInstances() {
		t.Fatal("non-strict source check rejected a symbolic evaluation")
	}
	satisfied, err = checker.checkRecursiveFunction(proto, q, true)
	astTestMust(t, err)
	if satisfied || !checker.HasNewInstances() {
		t.Fatal("strict source check accepted a symbolic evaluation")
	}
	astTestMust(t, checker.ResetNewInstances())
	a := smtRelevancyBool(t, m, "recursive-invalid-ground-model")
	astTestMust(t, state.Assert(a, ast.ASTHandle{}, 0))
	declaration, err := m.GetDecl(a)
	astTestMust(t, err)
	astTestMust(t, proto.Core().RegisterConstant(declaration, no))
	satisfied, err = checker.Check(proto, []ast.ASTHandle{q})
	astTestMust(t, err)
	if satisfied || !checker.HasNewInstances() {
		t.Fatal("invalid ground candidate did not trigger strict recursive checking")
	}
}
