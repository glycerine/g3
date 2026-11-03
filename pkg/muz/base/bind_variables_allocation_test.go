package base

// Source: bind_variables::operator() and expr_abstractor own every generated
// variable/application until the bound result has acquired its reference.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type variableBinderAllocationFixture struct {
	manager    *ast.ASTManager
	binder     *VariableBinder
	body       ast.ASTHandle
	input      ast.ASTRef
	variables  []ast.ASTHandle
	references []ast.ASTRef
	names      []symbol.Symbol
}

func variableBinderAllocationMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newVariableBinderAllocationFixture(t *testing.T) *variableBinderAllocationFixture {
	t.Helper()
	must := func(err error) { t.Helper(); variableBinderAllocationMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	binder, err := NewVariableBinder(manager)
	must(err)
	f := &variableBinderAllocationFixture{manager: manager, binder: binder}
	t.Cleanup(func() {
		budget.FailAfter(-1)
		if f.input != (ast.ASTRef{}) {
			must(manager.DelRef(&f.input))
		}
		must(binder.Close())
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		if budget.Used() != 0 {
			t.Fatal("binding allocation campaign leaked budget", budget.Used())
		}
	})
	boolean, err := manager.MkBoolSort()
	must(err)
	// Cold 512-variable fixtures cross independent AST-node and owning-ref
	// page boundaries without warming the generated variables or quantifier.
	f.variables = make([]ast.ASTHandle, 512)
	f.names = make([]symbol.Symbol, len(f.variables))
	domain := make([]ast.ASTHandle, len(f.variables))
	for i := range f.variables {
		f.variables[i], err = manager.MkFreshConst(fmt.Sprintf("binding-input-%d", i), boolean)
		must(err)
		must(binder.AddVariable(f.variables[i]))
		declaration, err := manager.GetDecl(f.variables[i])
		must(err)
		f.names[i], err = manager.GetASTName(declaration)
		must(err)
		domain[i] = boolean
	}
	f.references = append([]ast.ASTRef(nil), binder.references...)
	name, err := symbols.Intern("binding-allocation-wide")
	must(err)
	declaration, err := manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	f.body, err = manager.MkApp(declaration, f.variables)
	must(err)
	f.input, err = manager.NewRef(f.body)
	must(err)
	return f
}

func variableBinderAllocationCounts(stats ast.ASTManagerStats) [4]uint32 {
	return [4]uint32{stats.LiveNodes, stats.LiveInfos, stats.LiveParameters, stats.LiveReferences}
}

func (f *variableBinderAllocationFixture) checkInput(t *testing.T) {
	t.Helper()
	must := func(err error) { t.Helper(); variableBinderAllocationMust(t, err) }
	if f.binder.closed || f.binder.closing || len(f.binder.references) != len(f.references) || len(f.binder.variables) != len(f.variables) {
		t.Fatal("binding failure changed registered owner state")
	}
	body, err := f.manager.RefGet(f.input)
	must(err)
	if body != f.body {
		t.Fatal("binding changed the original input")
	}
	for i, ref := range f.references {
		value, err := f.manager.RefGet(ref)
		must(err)
		if f.binder.references[i] != ref || f.binder.variables[i] != f.variables[i] || value != f.variables[i] {
			t.Fatal("binding changed a registered constant or its owner", i)
		}
		argument, err := f.manager.GetArg(body, uint32(i))
		must(err)
		if argument != value {
			t.Fatal("binding rewrote the original application", i)
		}
	}
}

func (f *variableBinderAllocationFixture) checkResult(t *testing.T, result ast.ASTRef, forall bool) {
	t.Helper()
	must := func(err error) { t.Helper(); variableBinderAllocationMust(t, err) }
	formula, err := f.manager.RefGet(result)
	must(err)
	actualForall, err := f.manager.IsForall(formula)
	must(err)
	count, err := f.manager.GetNumDeclarations(formula)
	must(err)
	if actualForall != forall || count != uint32(len(f.variables)) {
		t.Fatal("wrong quantifier kind or declaration count", actualForall, count)
	}
	body, err := f.manager.GetQuantifierBody(formula)
	must(err)
	declaration, err := f.manager.GetDecl(body)
	must(err)
	originalDeclaration, err := f.manager.GetDecl(f.body)
	must(err)
	if declaration != originalDeclaration {
		t.Fatal("binding changed the predicate declaration")
	}
	for i, expectedName := range f.names {
		name, err := f.manager.GetDeclarationName(formula, uint32(i))
		must(err)
		argument, err := f.manager.GetArg(body, uint32(i))
		must(err)
		index, err := f.manager.GetVarIndex(argument)
		must(err)
		if name != expectedName || index != uint32(len(f.variables)-i-1) {
			t.Fatal("source binder name/de Bruijn order changed", i, name, index)
		}
	}
}

func TestVariableBinderAllocationExactASTOwnership(t *testing.T) {
	for _, forall := range []bool{false, true} {
		t.Run(fmt.Sprintf("forall=%t", forall), func(t *testing.T) {
			completed, failures := false, 0
			for allowance := int64(0); allowance < 256 && !completed; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					f := newVariableBinderAllocationFixture(t)
					must := func(err error) { t.Helper(); variableBinderAllocationMust(t, err) }
					before := variableBinderAllocationCounts(f.manager.Stats())
					f.manager.Budget().FailAfter(allowance)
					result, cause := f.binder.Bind(f.body, forall)
					f.manager.Budget().FailAfter(-1)
					var pending memory.ConstructionFailure
					if errors.As(cause, &pending) {
						must(pending.Cleanup())
					}
					if cause == nil {
						f.checkResult(t, result, forall)
						completed = true
					} else {
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("allocation boundary returned an unrelated error", cause)
						}
						failures++
					}
					if result != (ast.ASTRef{}) {
						must(f.manager.DelRef(&result))
					}
					f.checkInput(t)
					if after := variableBinderAllocationCounts(f.manager.Stats()); after != before {
						t.Fatal("binding leaked AST owners", allowance, cause, before, after)
					}
					// Retry the actual operation after every boundary, including
					// first success, and release the owning quantified snapshot.
					retry, err := f.binder.Bind(f.body, forall)
					must(err)
					f.checkResult(t, retry, forall)
					must(f.manager.DelRef(&retry))
					f.checkInput(t)
					if after := variableBinderAllocationCounts(f.manager.Stats()); after != before {
						t.Fatal("successful binding retry leaked AST owners", allowance, before, after)
					}
				}) {
					return
				}
			}
			if !completed || failures == 0 {
				t.Fatal("binding campaign missed allocation failure or completion", failures, completed)
			}
		})
	}
}
