package smtcore

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtArrayRefusingTraceWriter struct{}

func (smtArrayRefusingTraceWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSMTArrayModelDefaultPartitionAndStoreSelectPropagation(t *testing.T) {
	for _, infinite := range []bool{false, true} {
		for _, overwritten := range []bool{false, true} {
			t.Run(fmt.Sprint(infinite, "/", overwritten), func(t *testing.T) {
				var trace bytes.Buffer
				tags := diagnostic.NewTraceTagsWithWriter(&trace)
				tags.Enable("array")
				m, cfg := smtModelGeneratorFixture(t)
				family, err := m.RegisterArrayPlugin()
				astTestMust(t, err)
				context, _, state := smtInternalizerFixture(t, m, nil)
				theory, err := NewSMTArrayTheory(family)
				astTestMust(t, err)
				config := theory.arrayConfig
				config.Weak = true
				astTestMust(t, theory.ConfigureArray(config))
				astTestMust(t, state.RegisterTheory(theory))
				astTestMust(t, state.SetupTheories())
				indexSort, indices := smtArrayQueueIndices(t, m, "model-propagation-index-", 3)
				rangeSort, err := m.MkBoolSort()
				astTestMust(t, err)
				baseValue, err := m.MkTrue()
				astTestMust(t, err)
				storedValue, err := m.MkFalse()
				astTestMust(t, err)
				if infinite {
					rangeSort, err = m.MkUninterpretedSort(astTestSymbol(t, m, "model-propagation-range"), nil)
					astTestMust(t, err)
					baseValue, err = m.MkConst(astTestSymbol(t, m, "model-propagation-base-value"), rangeSort)
					astTestMust(t, err)
					storedValue, err = m.MkConst(astTestSymbol(t, m, "model-propagation-store-value"), rangeSort)
					astTestMust(t, err)
				}
				sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), rangeSort)
				astTestMust(t, err)
				a, err := m.MkConst(astTestSymbol(t, m, "model-propagation-base"), sort)
				astTestMust(t, err)
				first, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, indices[0], storedValue})
				astTestMust(t, err)
				lastIndex := indices[2]
				if overwritten {
					lastIndex = indices[1]
				}
				last, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{first, lastIndex, storedValue})
				astTestMust(t, err)
				read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, indices[1]})
				astTestMust(t, err)
				observed, err := m.MkEq(read, baseValue)
				astTestMust(t, err)
				astTestMust(t, state.Assert(observed, ast.ASTHandle{}, 0))
				astTestMust(t, state.Internalize(last, false))
				astTestMust(t, state.markRelevant(last))
				_, err = state.PropagateCore()
				astTestMust(t, err)
				generator, err := NewSMTModelGenerator(m)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, generator.Close()) })
				astTestMust(t, generator.SetContext(context))
				astTestMust(t, generator.InitializeModel(cfg))
				generator.trace = tags
				astTestMust(t, theory.InitializeModel(generator))
				data := theory.modelData
				if data == nil || data.unspecified {
					t.Fatal("store terms did not initialize the shared-default partition")
				}
				nodes := []*SMTEnode{state.context.enodes.get(a), state.context.enodes.get(first), state.context.enodes.get(last)}
				root := data.find(int(nodes[0].TheoryVar(family)))
				for _, node := range nodes[1:] {
					if data.find(int(node.TheoryVar(family))) != root {
						t.Fatal("store chain did not share its default class")
					}
				}
				containsRead := func(node *SMTEnode) bool {
					for _, entry := range data.selects[node.root].entries() {
						if entry.owner == read {
							return true
						}
					}
					return false
				}
				if !containsRead(nodes[1]) || containsRead(nodes[2]) == overwritten {
					t.Fatal("observed read did not propagate along non-overwriting store edges")
				}
				firstProcedure, err := theory.ModelValue(nodes[0], generator)
				astTestMust(t, err)
				if tags.ShouldTrace("array") {
					for _, want := range []string{
						"-------- [array] mk_value src/smt/theory_array_base.cpp:944 ---------\n",
						"model-propagation-base\n",
					} {
						if !strings.Contains(trace.String(), want) {
							t.Fatalf("missing array model trace %q in %q", want, trace.String())
						}
					}
				} else if trace.Len() != 0 {
					t.Fatal("compiled-out array trace executed", trace.String())
				}
				closedTags := diagnostic.NewTraceTagsWithWriter(smtArrayRefusingTraceWriter{})
				closedTags.Enable("array")
				generator.trace = closedTags
				lastProcedure, err := theory.ModelValue(nodes[2], generator)
				astTestMust(t, err)
				if infinite {
					firstDependencies, lastDependencies := firstProcedure.Dependencies(), lastProcedure.Dependencies()
					if !firstDependencies[0].IsFreshValue() || !lastDependencies[0].IsFreshValue() {
						t.Fatal("infinite range did not use a shared extra-fresh fallback")
					}
					left, err := firstDependencies[0].FreshValue()
					astTestMust(t, err)
					right, err := lastDependencies[0].FreshValue()
					astTestMust(t, err)
					if left != right {
						t.Fatal("same default class allocated independent fallback values")
					}
				} else if data.elseValues[root].expression.IsNull() || data.elseValues[root].fresh != nil {
					t.Fatal("finite range assumed a fresh fallback")
				}
				astTestMust(t, theory.FinalizeModel(generator))
				if theory.modelData != nil {
					t.Fatal("model preparation retained its temporary select tables")
				}
			})
		}
	}
}

func TestSMTFullArrayConstructorModelsDefaultAndMapPropagation(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	solver.kernel.preprocess = false
	astTestMust(t, solver.SetProduceModels(true))
	family, err := m.RegisterArrayPlugin()
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	astTestMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "full-model-a"), sort)
	astTestMust(t, err)
	b, err := m.MkConst(astTestSymbol(t, m, "full-model-b"), sort)
	astTestMust(t, err)
	c, err := m.MkConst(astTestSymbol(t, m, "full-model-c"), sort)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "full-model-f"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	yes, err := m.MkTrue()
	astTestMust(t, err)
	no, err := m.MkFalse()
	astTestMust(t, err)
	fTrue, err := m.MkApp(f, []ast.ASTHandle{yes})
	astTestMust(t, err)
	notFTrue, err := m.MkNot(fTrue)
	astTestMust(t, err)
	fFalse, err := m.MkApp(f, []ast.ASTHandle{no})
	astTestMust(t, err)
	observed, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, no})
	astTestMust(t, err)
	mapped, err := m.MkArrayMap(f, ast.SliceArguments([]ast.ASTHandle{a}))
	astTestMust(t, err)
	mapEquality, err := m.MkEq(b, mapped)
	astTestMust(t, err)
	constant, err := m.MkConstArray(sort, yes)
	astTestMust(t, err)
	constEquality, err := m.MkEq(c, constant)
	astTestMust(t, err)
	for _, term := range []ast.ASTHandle{mapEquality, constEquality, observed, notFTrue, fFalse} {
		astTestPin(t, m, term)
		astTestMust(t, solver.Assert(term))
	}
	read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, no})
	astTestMust(t, err)
	astTestPin(t, m, read)
	for attempt := 0; attempt < 2; attempt++ {
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("full constructor model", status)
		}
		theory := smtArrayCore(solver.kernel.internalizer.theories[family])
		if theory.simple || theory.statistics.defaultConst == 0 || theory.statistics.defaultMap == 0 || theory.statistics.mapAxiom == 0 {
			t.Fatal("constructor defaults or upward map reads were not emitted")
		}
		func() {
			generated, err := solver.AcquireModel()
			astTestMust(t, err)
			defer func() { astTestMust(t, generated.DecRef()) }()
			evaluator, err := model.NewModelEvaluator(generated, cfg)
			astTestMust(t, err)
			defer func() { astTestMust(t, evaluator.Close()) }()
			evaluateBoolean := func(term ast.ASTHandle) bool {
				t.Helper()
				ref, err := evaluator.EvaluateValue(term)
				astTestMust(t, err)
				defer func() { astTestMust(t, m.DelRef(&ref)) }()
				value, err := m.RefGet(ref)
				astTestMust(t, err)
				if !m.IsTrue(value) && !m.IsFalse(value) {
					t.Fatal("model query did not reduce to a Boolean value", term, value)
				}
				return m.IsTrue(value)
			}
			for _, term := range []ast.ASTHandle{constEquality, observed, notFTrue, fFalse} {
				if !evaluateBoolean(term) {
					t.Fatal("full constructor model violates an equation or observation", term)
				}
			}
			if evaluateBoolean(read) {
				t.Fatal("mapped read did not reflect f(true) = false")
			}
			// Native incremental model evaluation may leave map equality
			// symbolic. Exhaust its finite domain to verify the equation.
			for _, index := range []ast.ASTHandle{no, yes} {
				input, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, index})
				astTestMust(t, err)
				mappedInput, err := m.MkApp(f, []ast.ASTHandle{input})
				astTestMust(t, err)
				output, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, index})
				astTestMust(t, err)
				if evaluateBoolean(output) != evaluateBoolean(mappedInput) {
					t.Fatal("map equation fails at a Boolean index", index)
				}
			}
		}()
		astTestMust(t, solver.Push())
		astTestMust(t, solver.Assert(read))
		status, err = solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("mapped read contradiction", status)
		}
		astTestMust(t, solver.Pop(1))
	}
}
