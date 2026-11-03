package base

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The source numeral allocation failure transport calls cleanup while the AST
// invocation owns the manager. Its real admission refusal must preserve every
// owner until the invocation ends and cleanup can be retried.
func TestRuleManagerAndHornNormalizerCleanupNativePending(t *testing.T) {
	for _, operation := range []string{"manager-close", "normalizer-close", "normalizer-reset", "normalizer-body"} {
		t.Run(operation, func(t *testing.T) { testHornCleanupNativePending(t, operation) })
	}
}

func TestHornNormalizationResultCleanupNativePending(t *testing.T) {
	for _, operation := range []string{"result-close", "proof-result-close"} {
		t.Run(operation, func(t *testing.T) { testHornCleanupNativePending(t, operation) })
	}
}

func testHornCleanupNativePending(t *testing.T, operation string) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var rules *RuleManager
	var normalized *HornNormalization
	resultClose := operation == "result-close" || operation == "proof-result-close"
	var input ast.ASTRef
	active, callbacks, intact := false, 0, true
	var observed []string
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if !active {
			return f
		}
		callbacks++
		n := rules.normalizer
		predicates := make(map[ast.ASTHandle]ast.ASTRef, len(rules.predicates))
		for declaration, ref := range rules.predicates {
			predicates[declaration] = ref
		}
		work := append([]ast.ASTRef(nil), n.workRefs...)
		fresh := append([]ast.ASTRef(nil), n.freshPredicates...)
		todo := append([]ast.ASTRef(nil), n.todoRefs...)
		body := append([]ast.ASTRef(nil), n.bodyRefs...)
		var resultProofs, resultFormulas, resultFresh []ast.ASTRef
		if resultClose {
			resultProofs = append(resultProofs, normalized.Proofs...)
			resultFormulas = append(resultFormulas, normalized.Formulas...)
			resultFresh = append(resultFresh, normalized.FreshPredicates...)
		}
		observed = append(observed, fmt.Sprintf("before callback %d: predicates=%d work=%d fresh=%d todo=%d body=%d", callbacks, len(predicates), len(work), len(fresh), len(todo), len(body)))
		for call := 1; call <= 2; call++ {
			var failure error
			switch operation {
			case "manager-close":
				failure = rules.Close()
			case "normalizer-close":
				failure = n.Close()
			case "normalizer-reset":
				failure = n.Reset()
			case "normalizer-body":
				failure = n.beginHornStep()
			case "result-close", "proof-result-close":
				failure = normalized.Close()
			}
			same := !rules.closed && !n.closed && reflect.DeepEqual(rules.predicates, predicates) && reflect.DeepEqual(n.workRefs, work) && reflect.DeepEqual(n.freshPredicates, fresh) && reflect.DeepEqual(n.todoRefs, todo) && reflect.DeepEqual(n.bodyRefs, body)
			if resultClose {
				same = same && !normalized.closed && reflect.DeepEqual(normalized.Proofs, resultProofs) && reflect.DeepEqual(normalized.Formulas, resultFormulas) && reflect.DeepEqual(normalized.FreshPredicates, resultFresh)
			}
			intact = intact && errors.Is(failure, ast.ErrASTPending) && same
			observed = append(observed, fmt.Sprintf("close %d: pending=%v err=%v closed=%v normalizer_closed=%v predicates=%d work=%d fresh=%d todo=%d body=%d owners_intact=%v", call, errors.Is(failure, ast.ErrASTPending), failure, rules.closed, n.closed, len(rules.predicates), len(n.workRefs), len(n.freshPredicates), len(n.todoRefs), len(n.bodyRefs), same))
		}
		if operation == "normalizer-reset" || operation == "normalizer-body" {
			_, normalizeErr := n.Normalize(ast.ASTHandle{})
			_, proofErr := n.NormalizeWithProof(ast.ASTHandle{}, ast.ASTHandle{})
			_, snapshotErr := n.FreshPredicates()
			intact = intact && errors.Is(normalizeErr, ErrHornNormalizationOwner) && errors.Is(proofErr, ErrHornNormalizationOwner) && errors.Is(snapshotErr, ErrHornNormalizationOwner) && errors.Is(n.SetName(symbol.Symbol{}), ErrHornNormalizationOwner)
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
	rules, err = NewRuleManager(m)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if normalized != nil {
			must(normalized.Close())
		}
		if input != (ast.ASTRef{}) {
			must(m.DelRef(&input))
		}
		must(rules.Close())
		must(rules.Close())
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Errorf("cleanup leaked %d bytes", budget.Used())
		}
		t.Logf("final budget=%d", budget.Used())
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	var apps [4]ast.ASTHandle
	for i, name := range []string{"P", "Q", "R", "H"} {
		apps[i], err = m.MkFreshConst(name, boolean)
		must(err)
		declaration, cause := m.GetDecl(apps[i])
		must(cause)
		must(rules.RegisterPredicate(declaration))
	}
	disjunction, err := m.MkOr(apps[0], apps[1])
	must(err)
	conjunction, err := m.MkAnd(disjunction, apps[2])
	must(err)
	formula, err := m.MkImplies(conjunction, apps[3])
	must(err)
	input, err = m.NewRef(formula)
	must(err)
	if operation == "proof-result-close" {
		must(m.ToggleProofMode(ast.ASTProofEnabled))
		proof, cause := m.MkProofAsserted(formula)
		must(cause)
		must(m.ToggleProofMode(ast.ASTProofDisabled))
		normalized, err = rules.normalizer.NormalizeWithProof(formula, proof)
	} else {
		normalized, err = rules.normalizer.Normalize(formula)
	}
	must(err)
	if resultClose {
		if len(normalized.Formulas) != 3 || len(normalized.FreshPredicates) != 1 || operation == "proof-result-close" && len(normalized.Proofs) != 3 {
			t.Fatal("normalization did not produce the expected owning result", len(normalized.Formulas), len(normalized.Proofs), len(normalized.FreshPredicates))
		}
	} else {
		must(normalized.Close())
	}
	if len(rules.normalizer.workRefs) == 0 || len(rules.normalizer.freshPredicates) == 0 || len(rules.normalizer.bodyRefs) == 0 {
		t.Fatal("normalization did not populate actual root owners")
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	beforeRefs := m.Stats().LiveReferences
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, failure := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	for _, line := range observed {
		t.Log(line)
	}
	if failure == nil || callbacks == 0 {
		t.Fatalf("missing native allocation refusal: failure=%v callbacks=%d", failure, callbacks)
	}
	if !intact {
		t.Fatalf("cleanup discarded real owners during native callback: callbacks=%d", callbacks)
	}
	if resultClose {
		if m.Stats().LiveReferences != beforeRefs {
			t.Fatal("refused result cleanup changed AST root counts", beforeRefs, m.Stats().LiveReferences)
		}
		owned := uint64(len(normalized.Proofs) + len(normalized.Formulas) + len(normalized.FreshPredicates))
		for _, group := range [][]ast.ASTRef{normalized.Proofs, normalized.Formulas, normalized.FreshPredicates} {
			for _, ref := range group {
				_, cause := m.RefGet(ref)
				must(cause)
			}
		}
		must(normalized.Close())
		must(normalized.Close())
		if !normalized.closed || normalized.Proofs != nil || normalized.Formulas != nil || normalized.FreshPredicates != nil || uint64(m.Stats().LiveReferences)+owned != uint64(beforeRefs) {
			t.Fatal("result cleanup retry lost or repeated a root release", owned, beforeRefs, m.Stats().LiveReferences)
		}
	}
	// Reset must finish a refused reset without changing the source current
	// body, but finish a refused body release before allowing new work.
	if operation == "normalizer-reset" || operation == "normalizer-body" {
		body := append([]ast.ASTRef(nil), rules.normalizer.bodyRefs...)
		must(rules.normalizer.Reset())
		if rules.normalizer.cleanupPending != hornCleanupNone {
			t.Fatal("cleanup retry left a pending phase")
		}
		if operation == "normalizer-reset" && !reflect.DeepEqual(rules.normalizer.bodyRefs, body) {
			t.Fatal("reset retry changed the current body")
		}
		if operation == "normalizer-body" && len(rules.normalizer.bodyRefs) != 0 {
			t.Fatal("reset retry did not finish interrupted body cleanup")
		}
		result, cause := rules.normalizer.Normalize(formula)
		must(cause)
		must(result.Close())
	}
	must(rules.Close())
	must(rules.Close())
	if !rules.closed || rules.cleanupPending || !rules.normalizer.closed || rules.normalizer.cleanupPending != hornCleanupNone || len(rules.predicates) != 0 {
		t.Fatal("retry did not finish every owner")
	}
}
