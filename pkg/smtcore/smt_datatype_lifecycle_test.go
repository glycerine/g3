package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtDatatypePolicySort(t *testing.T, m *ast.ASTManager, infinite bool) ast.ASTHandle {
	t.Helper()
	second := ast.DatatypeConstructorDefinition{Name: astTestSymbol(t, m, "policy-next"), Recognizer: astTestSymbol(t, m, "policy-is-next")}
	if infinite {
		second.Accessors = []ast.DatatypeAccessorDefinition{{Name: astTestSymbol(t, m, "policy-prev"), Recursive: true}}
	}
	sorts, err := m.MkDatatypes([]ast.DatatypeDefinition{{Name: astTestSymbol(t, m, "policy-sort"), Constructors: []ast.DatatypeConstructorDefinition{
		{Name: astTestSymbol(t, m, "policy-base"), Recognizer: astTestSymbol(t, m, "policy-is-base")}, second,
	}}})
	astTestMust(t, err)
	return sorts[0]
}

func TestSMTDatatypeLazyPolicyAndFinalCheckWithoutInputRecognizer(t *testing.T) {
	for _, infinite := range []bool{false, true} {
		for _, policy := range []uint32{0, 1, 2} {
			t.Run(fmt.Sprint(infinite, "/", policy), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				s, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, s.Close()) })
				s.kernel.smtParams.datatype.LazySplits = policy
				family, err := m.DatatypeFamilyID()
				astTestMust(t, err)
				dt := s.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
				sort := smtDatatypePolicySort(t, m, infinite)
				x, err := m.MkConst(astTestSymbol(t, m, "policy-x"), sort)
				astTestMust(t, err)
				state := s.kernel.internalizer
				astTestMust(t, state.Internalize(x, false))
				astTestMust(t, dt.ensureVariable(state.context.enodes.get(x)))
				eager := policy == 0 || policy == 1 && !infinite
				slots, err := dt.recognizerSlots(x)
				astTestMust(t, err)
				if (len(slots) != 0) != eager {
					t.Fatal("wrong eager split policy", policy, infinite)
				}
				if !eager {
					// x has no input recognizer; it still belongs to the source
					// final-check traversal because it has a theory variable.
					if len(slots) != 0 {
						t.Fatal("fixture already has recognizers")
					}
					status, err := dt.FinalCheck()
					astTestMust(t, err)
					slots, err = dt.recognizerSlots(x)
					astTestMust(t, err)
					if status != SMTFinalCheckContinue || len(slots) != 1 {
						t.Fatal("lazy root was omitted by final check")
					}
				}
				if len(slots) != 1 {
					t.Fatal("split expanded all constructors", len(slots))
				}
				preferred := 0
				for constructor, slot := range slots {
					data, err := state.BooleanData(slot.literal.Var())
					astTestMust(t, err)
					if !data.TryTrueFirst() {
						continue
					}
					preferred++
					fields, err := m.DatatypeConstructorAccessors(constructor)
					astTestMust(t, err)
					if !state.relevancy.IsRelevant(slot.expression) || infinite && len(fields) != 0 {
						t.Fatal("preferred split is not a relevant base constructor")
					}
				}
				if preferred != 1 {
					t.Fatal("expected one preferred nonrecursive split", preferred)
				}
			})
		}
	}
}

func TestSMTDatatypeTranslationAndAuxiliaryPreserveParameterOwner(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(fmt.Sprint(configured), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			target, targetCfg := smtModelGeneratorFixture(t)
			s, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg, TargetConfig: func(manager *ast.ASTManager) (SMTSolverConfig, error) {
				if manager != target {
					return SMTSolverConfig{}, ast.ErrASTHandle
				}
				return SMTSolverConfig{Params: targetCfg.Context, ModelEvaluator: targetCfg}, nil
			}})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			s.kernel.smtParams.datatype.LazySplits = 2
			owner := &s.kernel.smtParams.datatype
			if configured {
				status, err := s.CheckSatCore(nil)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("source setup", status)
				}
			}
			copy, err := s.Translate(target, nil)
			astTestMust(t, err)
			a := copy.(*SMTSolver)
			t.Cleanup(func() { astTestMust(t, a.Close()) })
			family, err := target.DatatypeFamilyID()
			astTestMust(t, err)
			dt := a.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
			if configured {
				if dt.parameterOwner != owner || dt.lazySplits() != 2 {
					t.Fatal("fresh datatype lost source owner")
				}
				checker := s.kernel.internalizer.quantifierManager.modelChecker
				astTestMust(t, checker.initAuxiliaryContext())
				family, err := m.DatatypeFamilyID()
				astTestMust(t, err)
				aux := checker.auxiliaryContext.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
				if aux.parameterOwner != owner || aux.lazySplits() != 2 {
					t.Fatal("auxiliary datatype lost source owner")
				}
			} else if dt.parameterOwner == owner || dt.lazySplits() != 1 {
				t.Fatal("unconfigured placeholder became a fixed source plugin")
			}
			status, err := a.CheckSatCore(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("translated setup", status)
			}
			chained, err := a.Translate(target, nil)
			astTestMust(t, err)
			b := chained.(*SMTSolver)
			t.Cleanup(func() { astTestMust(t, b.Close()) })
			bdt := b.kernel.internalizer.theories[family].(*SMTDatatypeTheory)
			if bdt == dt || bdt.parameterOwner != dt.parameterOwner || bdt.NumVariables() != 0 {
				t.Fatal("chained copy shared state or lost parameter owner")
			}
			astTestMust(t, s.Close())
			astTestMust(t, a.Close())
			if configured {
				owner.LazySplits = 0
				if bdt.lazySplits() != 0 {
					t.Fatal("fresh datatype no longer observes retained owner")
				}
			}
		})
	}
}

func TestSMTDatatypeNegativeRecognizerAcrossEqualArguments(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_DT", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	s.kernel.smtParams.datatype.LazySplits = 2
	astTestMust(t, s.SetProduceModels(true))
	sort := smtDatatypePolicySort(t, m, false)
	constructors, err := m.DatatypeConstructors(sort)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "recognizer-alias-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "recognizer-alias-y"), sort)
	astTestMust(t, err)
	equal, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestMust(t, s.Assert(equal))
	negative := func(constructor, value ast.ASTHandle) ast.ASTHandle {
		d, err := m.DatatypeConstructorRecognizer(constructor)
		astTestMust(t, err)
		r, err := m.MkApp(d, []ast.ASTHandle{value})
		astTestMust(t, err)
		n, err := m.MkNot(r)
		astTestMust(t, err)
		astTestPin(t, m, n)
		return n
	}
	astTestMust(t, s.Assert(negative(constructors[0], x)))
	status, err := s.CheckSatCore(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("survivor conflict", status)
	}
	md, err := s.AcquireModel()
	astTestMust(t, err)
	if md == nil {
		t.Fatal("missing survivor model")
	}
	for _, expression := range []ast.ASTHandle{x, y} {
		d, err := m.GetDecl(expression)
		astTestMust(t, err)
		value, err := md.ConstantInterpretation(d)
		astTestMust(t, err)
		constructor, found, err := m.DatatypeConstructorApp(value)
		astTestMust(t, err)
		if !found || constructor != constructors[1] {
			t.Fatal("model ignored negative recognizer", expression, value)
		}
	}
	astTestMust(t, md.DecRef())
	astTestMust(t, s.Push())
	astTestMust(t, s.Assert(negative(constructors[1], y)))
	status, err = s.CheckSatCore(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("equal arguments escaped all constructors", status)
	}
	astTestMust(t, s.Pop(1))
	status, err = s.CheckSatCore(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("recognizer propagation did not recover after pop", status)
	}
}
