package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalRelationPluginClonePredicatesTransactionalAndIndependent(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	cloneCalls := 0
	failCloneAt := 0
	cloneFailure := errors.New("external predicate clone callback failed")
	callbacks := ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
		kind, err := f.ast.GetDeclKind(declaration)
		if err != nil {
			return err
		}
		switch kind {
		case ast.DatalogOpEmpty:
			rows[outputs[0]] = make(map[ast.ASTHandle]bool)
		case ast.DatalogOpStore:
			rows[outputs[0]][arguments[1]] = true
		case ast.DatalogOpClone:
			cloneCalls++
			if failCloneAt != 0 && cloneCalls == failCloneAt {
				return cloneFailure
			}
			rows[outputs[0]] = make(map[ast.ASTHandle]bool)
			for value := range rows[arguments[0]] {
				rows[outputs[0]][value] = true
			}
		default:
			t.Fatalf("unexpected clone assignment %d", kind)
		}
		return nil
	}}
	source, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	destination, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, destination.Close()) }()
	declare := func(name string) ast.ASTHandle {
		symbol, cause := f.ast.Symbols.Intern(name)
		astTestMust(t, cause)
		predicate, cause := f.ast.MkFuncDecl(symbol, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, cause)
		astTestPin(t, f.ast, predicate)
		return predicate
	}
	firstPredicate, secondPredicate := declare("external-clone-first"), declare("external-clone-second")
	first, err := source.PredicateRelation(firstPredicate)
	astTestMust(t, err)
	second, err := source.PredicateRelation(secondPredicate)
	astTestMust(t, err)
	astTestMust(t, first.AddFact([]ast.ASTHandle{f.truth}))
	astTestMust(t, second.AddFact([]ast.ASTHandle{f.falsehood}))
	// Arbitrary user relations are not part of the predicate snapshot.
	user, err := source.NewEmpty([]ast.ASTHandle{f.boolean})
	astTestMust(t, err)

	cloneCalls, failCloneAt = 0, 2
	failed, err := source.ClonePredicatesTo(destination)
	if failed != nil || !errors.Is(err, cloneFailure) || cloneCalls != 2 || source.users != 3 {
		t.Fatal("partial predicate clone was published or changed source ownership", failed, err, cloneCalls, source.users)
	}

	cloneCalls, failCloneAt = 0, 0
	cloned, err := source.CloneForManager(destination)
	astTestMust(t, err)
	if cloneCalls != 2 || cloned == nil || cloned.autoClose || cloned.users != 2 || len(cloned.predicates) != 2 {
		t.Fatal("predicate clone did not own exactly the source predicate snapshot", cloneCalls, cloned.users, len(cloned.predicates))
	}
	clonedFirst, err := cloned.PredicateRelation(firstPredicate)
	astTestMust(t, err)
	clonedSecond, err := cloned.PredicateRelation(secondPredicate)
	astTestMust(t, err)
	sourceFirstHandle, err := first.Relation()
	astTestMust(t, err)
	cloneFirstHandle, err := clonedFirst.Relation()
	astTestMust(t, err)
	sourceSecondHandle, err := second.Relation()
	astTestMust(t, err)
	cloneSecondHandle, err := clonedSecond.Relation()
	astTestMust(t, err)
	if sourceFirstHandle == cloneFirstHandle || sourceSecondHandle == cloneSecondHandle || !rows[cloneFirstHandle][f.truth] || !rows[cloneSecondHandle][f.falsehood] {
		t.Fatal("predicate clone shared expression ownership or lost callback rows")
	}
	astTestMust(t, first.AddFact([]ast.ASTHandle{f.falsehood}))
	if rows[cloneFirstHandle][f.falsehood] {
		t.Fatal("source mutation changed independently cloned predicate rows")
	}
	astTestMust(t, clonedFirst.AddFact([]ast.ASTHandle{f.falsehood}))
	if !rows[cloneFirstHandle][f.falsehood] || !rows[sourceFirstHandle][f.falsehood] {
		t.Fatal("independent clone mutation did not reach its own callback row")
	}
	astTestMust(t, clonedSecond.AddFact([]ast.ASTHandle{f.truth}))
	cloneCallsBeforeReject := cloneCalls
	wrongSymbol, err := f.ast.Symbols.Intern("external-clone-wrong-signature")
	astTestMust(t, err)
	wrongPredicate, err := f.ast.MkFuncDecl(wrongSymbol, nil, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.ast, wrongPredicate)
	if err = source.ReplacePredicateFrom(wrongPredicate, clonedSecond); !errors.Is(err, ErrTableRelation) || cloneCalls != cloneCallsBeforeReject {
		t.Fatal("predicate replacement accepted incompatible signature or invoked callback", err, cloneCalls, cloneCallsBeforeReject)
	}
	beforeReplaceUsers := source.users
	astTestMust(t, source.ReplacePredicateFrom(secondPredicate, clonedSecond))
	replacedSecond, err := source.PredicateRelation(secondPredicate)
	astTestMust(t, err)
	replacedHandle, err := replacedSecond.Relation()
	astTestMust(t, err)
	if replacedHandle == sourceSecondHandle || replacedHandle == cloneSecondHandle || source.users != beforeReplaceUsers || !rows[replacedHandle][f.falsehood] || !rows[replacedHandle][f.truth] {
		t.Fatal("predicate replacement was not an independent transactional clone", source.users, beforeReplaceUsers)
	}

	astTestMust(t, source.DropPredicateRelation(firstPredicate))
	if _, err = clonedFirst.Relation(); err != nil {
		t.Fatal("source predicate retirement consumed clone owner", err)
	}
	astTestMust(t, user.Close())
	astTestMust(t, source.Close())
	if _, err = clonedSecond.Relation(); err != nil {
		t.Fatal("source plugin close consumed clone predicate owner", err)
	}
	astTestMust(t, cloned.Close())
}

func TestExternalRelationPluginCloneRejectsDifferentASTManager(t *testing.T) {
	first := newTableRelationFixture(t)
	second := newTableRelationFixture(t)
	plugin, err := NewExternalRelationPlugin(first.relations, ExternalRelationCallbacks{})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	cloned, err := plugin.ClonePredicatesTo(second.relations)
	if cloned != nil || !errors.Is(err, ErrTableRelation) {
		t.Fatal("cross-AST external predicate clone accepted", cloned, err)
	}
}

func TestExternalRelationPluginCloneCleanupRefusalIsRetryable(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *externalPluginCloneOwner
	var refused error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && refused == nil {
			refused = finishExternalPluginClone(owner, errors.New("predicate clone failed"))
		}
		return value
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	sourceManager, err := NewRelationManager(manager)
	must(err)
	targetManager, err := NewRelationManager(manager)
	must(err)
	source, err := NewExternalRelationPlugin(sourceManager, ExternalRelationCallbacks{})
	must(err)
	target, err := NewExternalRelationPlugin(targetManager, ExternalRelationCallbacks{})
	must(err)
	published, err := target.NewEmpty(nil)
	must(err)
	name, err := manager.Symbols.Intern("clone-cleanup-predicate")
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(manager.IncRef(predicate))
	target.predicates[predicate] = published
	pending, err := target.NewEmpty(nil)
	must(err)
	owner = &externalPluginCloneOwner{source: source, target: target, runtime: runtime, pending: pending}

	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || refused == nil || owner.pending != pending || owner.target != target || target.users != 2 || target.predicates[predicate] != published {
		t.Fatal("clone cleanup refusal lost pending or published destination owners", body, refused, target.users)
	}
	// The receipt's runtime identity must not depend on the borrowed source
	// plugin remaining open through a later cleanup retry.
	must(source.Close())
	var receipt memory.ConstructionFailure
	if !errors.As(refused, &receipt) || receipt.RuntimeOwner() != runtime {
		t.Fatal("clone cleanup lost typed runtime receipt", refused, receipt)
	}
	message, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil || owner.pending != nil || owner.target != nil || target.users != 0 {
		t.Fatal("clone cleanup retry changed identity or left owners")
	}
	must(receipt.Cleanup())

	// Refusal while retiring an existing predicate must leave the map on the
	// independently usable replacement, never on the partially consumed old
	// relation.
	replacementSource, err := NewExternalRelationPlugin(sourceManager, ExternalRelationCallbacks{})
	must(err)
	sourceRelation, err := replacementSource.NewEmpty(nil)
	must(err)
	replacementTarget, err := NewExternalRelationPlugin(targetManager, ExternalRelationCallbacks{})
	must(err)
	oldRelation, err := replacementTarget.NewEmpty(nil)
	must(err)
	// Install a consumed native ref ahead of the live relation ref. Close then
	// reports the native refusal after consuming relation, reproducing the
	// partial-retirement frontier that made the old map entry unusable.
	staleHandle, err := manager.MkTrue()
	must(err)
	staleRef, err := manager.NewRef(staleHandle)
	must(err)
	oldRelation.emptyDecl = staleRef
	must(manager.DelRef(&staleRef))
	replaceName, err := manager.Symbols.Intern("clone-replace-cleanup-predicate")
	must(err)
	replacePredicate, err := manager.MkFuncDecl(replaceName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(manager.IncRef(replacePredicate))
	replacementTarget.predicates[replacePredicate] = oldRelation
	replacementErr := replacementTarget.ReplacePredicateFrom(replacePredicate, sourceRelation)
	if replacementErr == nil {
		t.Fatal("predicate replacement retirement did not expose native refusal")
	}
	mapped, lookupErr := replacementTarget.PredicateRelation(replacePredicate)
	must(lookupErr)
	if mapped == oldRelation || mapped.open() != nil || replacementTarget.users < 1 {
		t.Fatal("failed replacement left predicate mapped to retired owner", mapped == oldRelation, replacementTarget.users)
	}
	var replaceReceipt memory.ConstructionFailure
	if !errors.As(replacementErr, &replaceReceipt) || replaceReceipt.RuntimeOwner() != runtime {
		t.Fatal("replacement retirement refusal lost typed cleanup owner", replacementErr)
	}
	replaceMessage, replaceExposed := replaceReceipt.Error(), replaceReceipt.Unwrap()
	if err = replaceReceipt.Cleanup(); err == nil || replaceReceipt.Error() != replaceMessage || replaceReceipt.Unwrap() != replaceExposed {
		t.Fatal("replacement retirement retry lost stable refusal", err)
	}
	oldRelation.emptyDecl = ast.ASTRef{}
	must(replaceReceipt.Cleanup())
	must(replaceReceipt.Cleanup())
	if mappedAgain, cause := replacementTarget.PredicateRelation(replacePredicate); cause != nil || mappedAgain != mapped {
		t.Fatal("replacement cleanup retry disturbed published predicate", mappedAgain, cause)
	}
	must(replacementTarget.Close())
	must(sourceRelation.Close())
	must(replacementSource.Close())
	must(targetManager.Close())
	must(sourceManager.Close())
	must(numbers.Del(&number))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("clone cleanup fixture leaked bytes", used)
	}
}
