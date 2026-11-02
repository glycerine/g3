package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestHornPreprocessClosePreservesPendingRuleAndProofOwners(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var result, conversionResult *hornPreprocessResult
	var coneSteps []transforms.ConeModelStep
	var backend, proofBackend *PropositionalEngine
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for _, target := range []*PropositionalEngine{backend, proofBackend} {
				pdrProof := target.pdrProof
				conversions := len(target.pdrConversions)
				for i := 0; i < 2; i++ {
					cause := target.clearPDRPreprocessing()
					intact = intact && errors.Is(cause, ast.ErrASTPending) && target.pdrProof == pdrProof && len(target.pdrConversions) == conversions && len(target.pdrPredicates) == 1
				}
			}

			coneErr := transforms.CloseConeModelSteps(coneSteps)
			intact = intact && errors.Is(coneErr, ast.ErrASTPending)
			for _, step := range coneSteps {
				for _, definition := range step {
					intact = intact && definition.Formula != (ast.ASTRef{})
				}
			}

			for i := 0; i < 2; i++ {
				cause := conversionResult.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !conversionResult.closed && len(conversionResult.modelConversions) == 1 && conversionResult.modelConversions[0].inline[0].Head != (ast.ASTRef{}) && conversionResult.modelConversions[0].inline[0].Body != (ast.ASTRef{})
			}
			rules, proof := result.rules, result.sliceProof
			for i := 0; i < 2; i++ {
				cause := result.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !result.closed && result.rules == rules && result.sliceProof == proof && len(result.slicedPredicates) == 1 && len(result.modelConversions) == 1
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if result != nil {
			must(result.Close())
		}
		if conversionResult != nil {
			must(conversionResult.Close())
		}
		must(set.Close())
		must(rules.Close())
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("pending preprocessing fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		n, e := symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(n, domain, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rules.RegisterPredicate(d))
		return d
	}
	p, goal := declare("pending-preprocess-P", []ast.ASTHandle{boolean}), declare("pending-preprocess-goal", nil)
	unused := declare("pending-unused", nil)
	unusedApp, err := m.MkApp(unused, nil)
	must(err)
	x, err := m.MkVar(0, boolean)
	must(err)
	pa, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	ga, err := m.MkApp(goal, nil)
	must(err)
	var original, originalProof ast.ASTHandle
	for i, spec := range []struct {
		head ast.ASTHandle
		tail []ast.ASTHandle
	}{{pa, nil}, {ga, []ast.ASTHandle{pa}}, {unusedApp, nil}} {
		rule, e := rules.NewRule(spec.head, spec.tail, nil, "")
		must(e)
		must(rules.MkRuleAssertedProof(rule))
		if i == 0 {
			proof, found, e := rule.Proof()
			must(e)
			if !found {
				t.Fatal("missing original proof")
			}
			originalProof = proof
			original, e = m.GetProofFact(proof)
			must(e)
		}
		must(set.Add(rule))
		must(rule.Close())
	}
	must(set.SetOutputPredicate(goal))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("invalid fixture")
	}
	cone := transforms.NewConePlugin(1)
	filtered, e := cone.Transform(set)
	must(e)
	if filtered == nil {
		t.Fatal("expected cone model conversion")
	}
	coneSteps, e = cone.TakeModelSteps()
	must(e)
	definitions := 0
	for _, step := range coneSteps {
		definitions += len(step)
	}
	if definitions == 0 {
		t.Fatal("missing cone model roots")
	}
	defer func() { must(transforms.CloseConeModelSteps(coneSteps)) }()
	must(filtered.Close())
	must(cone.Close())
	inliner := transforms.NewRuleInlinerPlugin(m, rules, []ast.ASTHandle{goal}, 1)
	inlined, e := inliner.Transform(set)
	must(e)
	if inlined == nil {
		t.Fatal("expected inlined program")
	}
	modelRules, e := inliner.TakeModelRules()
	must(e)
	if len(modelRules) == 0 {
		t.Fatal("missing inline model roots")
	}
	conversionResult = &hornPreprocessResult{modelConversions: []hornModelConversion{{kind: hornConvertInline, inline: modelRules}}}
	must(inlined.Close())
	must(inliner.Close())
	plugin := transforms.NewRuleSlicePlugin(m, rules, 1)
	plugin.SetGenerateProofTrace(true)
	transformer := base.NewRuleTransformer()
	must(transformer.Register(plugin))
	changed, err := transformer.Apply(set)
	must(err)
	if !changed {
		t.Fatal("expected sliced program")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("missing sliced predicate")
	}
	result = &hornPreprocessResult{rules: set, sliceProof: plugin.TakeProofConverter(), slicedPredicates: map[ast.ASTHandle]ast.ASTHandle{p: derived}, modelConversions: []hornModelConversion{{kind: hornConvertSlice}}}
	must(transformer.Close())
	ledgerPlugin := transforms.NewRuleInlinerPlugin(m, rules, []ast.ASTHandle{goal}, 1)
	ledgerSet, e := ledgerPlugin.Transform(set)
	must(e)
	if ledgerSet == nil {
		t.Fatal("missing backend model conversion")
	}
	ledgerRules, e := ledgerPlugin.TakeModelRules()
	must(e)
	if len(ledgerRules) == 0 {
		t.Fatal("missing backend model roots")
	}
	defer func() { must(transforms.CloseInlineModelRules(ledgerRules)) }()
	must(ledgerSet.Close())
	must(ledgerPlugin.Close())
	backend = &PropositionalEngine{manager: m}
	backend.installPDRPreprocessing(&hornPreprocessResult{slicedPredicates: map[ast.ASTHandle]ast.ASTHandle{p: derived}, modelConversions: []hornModelConversion{{kind: hornConvertInline, inline: ledgerRules}}})
	backend.pdrProof, err = m.NewRef(originalProof)
	must(err)
	defer func() { must(backend.clearPDRPreprocessing()) }()
	proofBackend = &PropositionalEngine{manager: m}
	proofBackend.installPDRPreprocessing(&hornPreprocessResult{slicedPredicates: map[ast.ASTHandle]ast.ASTHandle{p: derived}})
	proofBackend.pdrProof, err = m.NewRef(originalProof)
	must(err)
	defer func() { must(proofBackend.clearPDRPreprocessing()) }()

	if result.sliceProof == nil {
		t.Fatal("missing slice converter")
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("pending preprocessing close discarded owners", body, callbacks, intact)
	}
	preserved, e := m.RefGet(backend.pdrProof)
	must(e)
	factBefore, e := m.GetProofFact(preserved)
	must(e)
	if factBefore != original || backend.pdrPredicate(p) != derived {
		t.Fatal("refused PDR cleanup changed proof or predicate mapping")
	}
	preservedProof, e := m.RefGet(proofBackend.pdrProof)
	must(e)
	if preservedProof != originalProof || proofBackend.pdrPredicate(p) != derived {
		t.Fatal("refused proof-only cleanup changed proof or predicate mapping")
	}
	must(proofBackend.clearPDRPreprocessing())
	must(proofBackend.clearPDRPreprocessing())
	if proofBackend.pdrProof != (ast.ASTRef{}) || proofBackend.pdrPredicates != nil {
		t.Fatal("proof-only cleanup retry did not retire owners")
	}
	must(backend.clearPDRPreprocessing())
	must(backend.clearPDRPreprocessing())
	if backend.pdrProof != (ast.ASTRef{}) || backend.pdrConversions != nil || backend.pdrPredicates != nil {
		t.Fatal("PDR cleanup retry did not retire owners")
	}
	if got := result.takeModelConversions(); got != nil || len(result.modelConversions) != 1 {
		t.Fatal("closing owner published model conversions")
	}
	if got := conversionResult.takeModelConversions(); got != nil || len(conversionResult.modelConversions) != 1 {
		t.Fatal("closing conversion owner published model conversions")
	}
	// A completed component release is safe to revisit during owner cleanup.
	must(transforms.CloseInlineModelRules(conversionResult.modelConversions[0].inline))
	must(transforms.CloseInlineModelRules(conversionResult.modelConversions[0].inline))
	must(conversionResult.Close())
	must(conversionResult.Close())
	must(transforms.CloseConeModelSteps(coneSteps))
	must(transforms.CloseConeModelSteps(coneSteps))
	candidates, err := result.rules.PredicateRules(derived)
	must(err)
	if len(candidates) != 1 {
		t.Fatal("missing sliced rule after refused close")
	}
	proof, found, err := candidates[0].Proof()
	must(err)
	if !found {
		t.Fatal("missing sliced proof")
	}
	converted, err := result.sliceProof.Convert(proof)
	must(err)
	node, err := m.RefGet(converted)
	must(err)
	fact, err := m.GetProofFact(node)
	must(err)
	if fact != original {
		t.Fatal("refused preprocessing close changed proof mapping")
	}
	must(m.DelRef(&converted))
	must(candidates[0].Close())
	must(result.Close())
	must(result.Close())
}
