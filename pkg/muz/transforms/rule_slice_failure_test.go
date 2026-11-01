package transforms_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	proofcheck "github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestRuleSliceProofConverterColdFailureAndRetry(t *testing.T) {
	stops := make([]int64, 128)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(128); stop <= 16384; stop *= 2 {
		stops = append(stops, stop)
	}
	failures, succeeded := 0, false
	for _, stop := range stops {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, converter, proof, expected := newSliceFailureFixture(t)
			before := m.Stats().LiveReferences
			inputCount, err := m.GetRefCount(proof)
			coneMust(t, err)
			mode, err := m.ProofMode()
			coneMust(t, err)
			m.Budget().FailAfter(stop)
			result, err := converter.Convert(proof)
			m.Budget().FailAfter(-1)
			if result != (ast.ASTRef{}) {
				coneMust(t, m.DelRef(&result))
			}
			if err != nil {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					coneMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatalf("checkpoint %d: %v", stop, err)
				}
				failures++
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("checkpoint %d: converter references = %d, want %d", stop, got, before)
			}
			count, err := m.GetRefCount(proof)
			coneMust(t, err)
			if count != inputCount {
				t.Fatalf("checkpoint %d: converter kept its input proof", stop)
			}
			if current, err := m.ProofMode(); err != nil || current != mode {
				t.Fatalf("checkpoint %d: proof mode = %v, %v", stop, current, err)
			}
			retry, err := converter.Convert(proof)
			coneMust(t, err)
			node, err := m.RefGet(retry)
			coneMust(t, err)
			valid, err := m.IsProof(node)
			coneMust(t, err)
			if !valid {
				t.Fatal("retry returned an expression instead of a proof")
			}
			fact, err := m.GetProofFact(node)
			coneMust(t, err)
			if fact != expected {
				t.Fatal("retry proved the wrong conclusion")
			}

			if node == proof {
				t.Fatal("retry passed through a sliced derivation")
			}
			checker, err := proofcheck.NewChecker(m)
			coneMust(t, err)
			checked, side, err := checker.Check(node)
			coneMust(t, err)
			coneMust(t, checker.ReleaseSideConditions(side))
			if !checked {
				t.Fatal("converted derivation failed proof checking")
			}
			coneMust(t, m.DelRef(&retry))
			if got := m.Stats().LiveReferences; got != before {
				t.Fatalf("checkpoint %d: retry references = %d, want %d", stop, got, before)
			}
		})
		if t.Failed() || succeeded {
			break
		}
	}
	if !succeeded || failures < 8 {
		t.Fatal("missing slice proof failure/success frontier", failures, succeeded)
	}
}

func newSliceFailureFixture(t *testing.T) (*ast.ASTManager, *transforms.SliceProofConverter, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	f := newConeFixture(t, "Goal")
	m := f.manager
	boolean, err := m.MkBoolSort()
	coneMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	coneMust(t, err)
	integer, err := m.GetSort(zero)
	coneMust(t, err)
	x, err := m.MkVar(0, integer)
	coneMust(t, err)
	declarations := make([]ast.ASTHandle, 8)
	applications := make([]ast.ASTHandle, 8)
	add := func(head ast.ASTHandle, tail []ast.ASTHandle) {
		rule, err := f.rules.NewRule(head, tail, nil, "")
		coneMust(t, err)
		coneMust(t, f.rules.MkRuleAssertedProof(rule))
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	for i := range declarations {
		name, err := m.Symbols.Intern(fmt.Sprintf("slice-cold-P%d", i))
		coneMust(t, err)
		declarations[i], err = m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, err)
		coneMust(t, f.rules.RegisterPredicate(declarations[i]))
		applications[i], err = m.MkApp(declarations[i], []ast.ASTHandle{x})
		coneMust(t, err)
		var tail []ast.ASTHandle
		if i != 0 {
			tail = []ast.ASTHandle{applications[i-1]}
		}
		add(applications[i], tail)
	}
	goal := f.applications["Goal"]
	add(goal, []ast.ASTHandle{applications[len(applications)-1]})
	coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("invalid slice failure fixture")
	}
	plugin := transforms.NewRuleSlicePlugin(m, f.rules, 1)
	plugin.SetGenerateProofTrace(true)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	changed, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !changed {
		t.Fatal("fixture was not sliced")
	}
	derived := make([]ast.ASTHandle, len(declarations))
	for i, decl := range declarations {
		var found bool
		derived[i], found = plugin.DerivedPredicate(decl)
		if !found {
			t.Fatal("missing sliced predicate", i)
		}
	}
	converter := plugin.TakeProofConverter()
	coneMust(t, transformer.Close())
	if converter == nil {
		t.Fatal("missing slice proof converter")
	}
	t.Cleanup(func() { coneMust(t, converter.Close()) })
	proofs := make(map[ast.ASTHandle]ast.ASTHandle)
	count, err := f.set.Len()
	coneMust(t, err)
	for i := 0; i < count; i++ {
		rule, err := f.set.Rule(i)
		coneMust(t, err)
		proof, found, err := rule.Proof()
		coneMust(t, err)
		if !found {
			t.Fatal("missing sliced rule proof")
		}
		head, err := rule.HeadDeclaration()
		coneMust(t, err)
		proofs[head] = proof
		coneMust(t, rule.Close())
	}
	scope, err := ast.NewScopedProof(m)
	coneMust(t, err)
	proof := proofs[derived[0]]
	heads := append(derived[1:], f.declarations["Goal"])
	for _, head := range heads {
		conclusion, err := m.MkApp(head, nil)
		coneMust(t, err)
		proof, err = m.MkProofHyperResolve([]ast.ASTHandle{proofs[head], proof}, conclusion, []ast.ASTProofPosition{{First: 1, Second: 0}}, [][]ast.ASTHandle{nil, nil})
		coneMust(t, err)
	}
	pin, err := m.NewRef(proof)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, m.DelRef(&pin)) })
	coneMust(t, scope.Close())
	return m, converter, proof, goal
}
