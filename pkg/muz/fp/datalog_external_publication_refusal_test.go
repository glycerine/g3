package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPublishDatalogExternalConstructionRefusalDoesNotPublishPrefix(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var decoded *externalDefaultDecodedOwner
	var construction error
	active := false
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && construction == nil {
			construction = decoded.finish(arena.ErrInjectedFailure)
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: native, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	parent, err := NewPropositionalEngine(manager)
	must(err)
	clone, err := NewPropositionalEngine(manager)
	must(err)

	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("external-publication-refusal")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(parent.RegisterPredicate(predicate))
	must(clone.RegisterPredicate(predicate))
	root, err := manager.NewRef(predicate)
	must(err)
	decoded = &externalDefaultDecodedOwner{manager: manager, refs: [][]ast.ASTRef{{root}}}

	value, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	parent.external, err = rel.NewExternalRelationPlugin(parent.relations, rel.ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
		kind, cause := manager.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		if kind != ast.DatalogOpClone {
			return nil
		}
		must(native.SetMaxCount(uint64(native.State().GlobalCount) + 1))
		active = true
		var invocation ast.ASTArithmeticNumeralInvocation
		_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		active = false
		must(native.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil || construction == nil {
			return arena.ErrInjectedFailure
		}
		return construction
	}})
	must(err)
	clone.external, err = rel.NewExternalRelationPlugin(clone.relations, rel.ExternalRelationCallbacks{})
	must(err)
	source, err := clone.external.PredicateRelation(predicate)
	must(err)
	clone.predicates[predicate].external = source
	clone.predicates[predicate].table = nil

	publishErr := parent.publishDatalogQueryRelations(clone)
	var receipt memory.ConstructionFailure
	if publishErr == nil || !errors.As(publishErr, &receipt) || receipt.RuntimeOwner() != native {
		t.Fatal("external publication lost native construction receipt", publishErr)
	}
	if parent.predicates[predicate].external != nil || parent.predicates[predicate].table == nil {
		t.Fatal("failed construction published or refreshed an external prefix")
	}
	if relation, found, cause := parent.external.TryGetPredicateRelation(predicate); cause != nil || found || relation != nil {
		t.Fatal("failed construction inserted a predicate owner", relation, found, cause)
	}
	if _, cause := source.Relation(); cause != nil {
		t.Fatal("parent publication failure consumed clone source", cause)
	}
	must(receipt.Cleanup())
	must(receipt.Cleanup())

	must(clone.Close())
	must(parent.Close())
	must(numbers.Del(&value))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(native.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("external publication refusal fixture leaked bytes", used)
	}
}
