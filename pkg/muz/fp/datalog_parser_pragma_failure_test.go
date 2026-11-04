package fp_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source parse_pred publishes argument names only after output pragmas.
// Exhaust reference slots after admitting the sort and predicate owners.
// Sweep the allocation allowance until the parser can construct the finite
// sort and predicate but the output-predicate reference is refused.
func TestDatalogParserPragmaFailurePrecedesArgumentNames(t *testing.T) {
	foundBoundary := false
	for allowance := int64(0); allowance < 64 && !foundBoundary; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			foundBoundary = datalogParserPragmaFailureAttempt(t, allowance)
		})
	}
	if !foundBoundary {
		t.Fatal("allocation sweep did not reach output pragma after predicate registration")
	}
}

func datalogParserPragmaFailureAttempt(t *testing.T, allowance int64) bool {
	t.Helper()
	f := newDatalogParserFixture(t)
	manager, err := f.context.Manager()
	fpMust(t, err)
	sortName, err := manager.Symbols.Intern("N")
	fpMust(t, err)
	sort, err := manager.MkDatalogFiniteSort(sortName, 2)
	fpMust(t, err)
	sortRef, err := manager.NewRef(sort)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&sortRef)) }()
	boolean, err := manager.MkBoolSort()
	fpMust(t, err)
	name, err := manager.Symbols.Intern("P")
	fpMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	fpMust(t, err)
	declRef, err := manager.NewRef(declaration)
	fpMust(t, err)
	defer func() { fpMust(t, manager.DelRef(&declRef)) }()
	var padding []ast.ASTRef
	defer func() {
		manager.BudgetField.FailAfter(-1)
		for i := range padding {
			fpMust(t, manager.DelRef(&padding[i]))
		}
	}()
	for i := 0; i < 3; i++ {
		ref, e := manager.NewRef(ast.ASTHandle{})
		fpMust(t, e)
		padding = append(padding, ref)
	}
	manager.BudgetField.FailAfter(0)
	for i := 0; ; i++ {
		ref, e := manager.NewRef(ast.ASTHandle{})
		if errors.Is(e, arena.ErrInjectedFailure) {
			break
		}
		fpMust(t, e)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference exhaustion not reached")
		}
	}
	for i := 0; i < 3; i++ {
		last := len(padding) - 1
		fpMust(t, manager.DelRef(&padding[last]))
		padding = padding[:last]
	}
	manager.BudgetField.FailAfter(allowance)
	err = fp.ParseDatalogString(f.context, "N 2\n\nP(x:N) outputtuples\n")
	manager.BudgetField.FailAfter(-1)
	if err == nil {
		return false
	}
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected injected allocation refusal", err)
	}
	got, found, err := f.context.Predicate(name)
	fpMust(t, err)
	if !found || got != declaration {
		return false
	}
	arg, err := f.context.ArgumentName(declaration, 0)
	fpMust(t, err)
	text, err := manager.Symbols.Display(arg)
	fpMust(t, err)
	if text != "#0" {
		return false
	}
	rules, err := f.context.Rules()
	fpMust(t, err)
	output, err := rules.IsOutputPredicate(declaration)
	fpMust(t, err)
	if output {
		return false
	}
	fpMust(t, f.context.SetOutputPredicate(declaration))
	x, err := manager.Symbols.Intern("x")
	fpMust(t, err)
	fpMust(t, f.context.SetArgumentNames(declaration, []symbol.Symbol{x}))
	arg, err = f.context.ArgumentName(declaration, 0)
	fpMust(t, err)
	output, err = rules.IsOutputPredicate(declaration)
	fpMust(t, err)
	if arg != x || !output {
		t.Fatal("pragma recovery did not publish metadata", arg, output)
	}
	return true
}
