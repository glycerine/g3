package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestExternalRuleExecutionCleanupReceiptRetriesNativeFrontier(t *testing.T) {
	f := newTableRelationFixture(t)
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	first, err := plugin.NewEmpty(nil)
	astTestMust(t, err)
	second, err := plugin.NewEmpty(nil)
	astTestMust(t, err)

	// A consumed ref deterministically reproduces a native Close refusal after
	// the relation's remaining live refs have advanced their cleanup frontier.
	staleHandle, err := f.ast.MkTrue()
	astTestMust(t, err)
	staleRef, err := f.ast.NewRef(staleHandle)
	astTestMust(t, err)
	first.emptyDecl = staleRef
	astTestMust(t, f.ast.DelRef(&staleRef))

	owner := &externalRuleExecutionOwner{runtime: f.ast.NumeralService().RuntimeOwner()}
	owner.adopt(first)
	owner.adopt(second)
	origin := errors.New("external rule evaluation failed")
	failure := finishExternalRuleExecution(owner, origin)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != owner.runtime {
		t.Fatal("rule execution cleanup refusal lost typed receipt", failure)
	}
	if owner.relations[1] != nil || owner.relations[0] != first || plugin.users != 1 {
		t.Fatal("rule execution cleanup did not preserve exact reverse frontier", owner.relations, plugin.users)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	if !errors.Is(receipt.OriginalCause(), origin) || receipt.CleanupCause() == nil {
		t.Fatal("rule execution receipt lost operation or cleanup cause", receipt.OriginalCause(), receipt.CleanupCause())
	}
	if err = receipt.Cleanup(); err == nil || receipt.Error() != shown || receipt.Unwrap() != exposed {
		t.Fatal("rule execution retry changed stable refusal", err)
	}
	first.emptyDecl = ast.ASTRef{}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || len(owner.relations) != 0 || plugin.users != 0 {
		t.Fatal("rule execution cleanup retry left owners", receipt.CleanupCause(), len(owner.relations), plugin.users)
	}
}

func TestExternalRuleExecutionPreservesDeltaAfterNativeCloseRefusal(t *testing.T) {
	f := newTableRelationFixture(t)
	nonempty := make(map[ast.ASTHandle]bool)
	poisoned := false
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				nonempty[outputs[0]] = false
			case ast.DatalogOpStore:
				nonempty[outputs[0]] = true
			case ast.DatalogOpClone:
				nonempty[outputs[0]] = nonempty[arguments[0]]
			case ast.DatalogOpUnion, ast.DatalogOpWiden:
				added := nonempty[arguments[1]] && !nonempty[outputs[0]]
				nonempty[outputs[0]] = nonempty[outputs[0]] || nonempty[arguments[1]]
				if len(outputs) == 2 {
					nonempty[outputs[1]] = added
				}
			default:
				return ErrCompilerUnsupported
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			if kind != ast.DatalogOpIsEmpty {
				return ast.ASTHandle{}, ErrCompilerUnsupported
			}
			if !poisoned && nonempty[arguments[0]] {
				// Consume the delta's borrowed expression handle. Its owning ASTRef
				// now reports a deterministic native refusal when Close reaches it.
				_, cause = f.ast.DecRefConsumed(arguments[0])
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
				poisoned = true
			}
			return f.ast.MkBoolVal(!nonempty[arguments[0]])
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	rules, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	name, err := f.ast.Symbols.Intern("external-rule-delta-cleanup")
	astTestMust(t, err)
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, f.ast, predicate)
	astTestMust(t, rules.RegisterPredicate(predicate))
	fact, err := f.ast.MkApp(predicate, []ast.ASTHandle{f.truth})
	astTestMust(t, err)
	x, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	astTestPin(t, f.ast, x)
	recursive, err := f.ast.MkApp(predicate, []ast.ASTHandle{x})
	astTestMust(t, err)
	set, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	groundRule, err := rules.NewRule(fact, nil, nil, "delta-cleanup-fact")
	astTestMust(t, err)
	astTestMust(t, set.Add(groundRule))
	astTestMust(t, groundRule.Close())
	recursiveRule, err := rules.NewRule(recursive, []ast.ASTHandle{recursive}, nil, "delta-cleanup-self")
	astTestMust(t, err)
	astTestMust(t, set.Add(recursiveRule))
	astTestMust(t, recursiveRule.Close())

	failure := ExecuteExternalFactRules(plugin, set)
	var executionFailure *externalRuleExecutionCleanupFailure
	if !poisoned || !errors.As(failure, &executionFailure) {
		t.Fatal("recursive delta cleanup refusal lost execution receipt", poisoned, failure)
	}
	target, err := plugin.PredicateRelation(predicate)
	astTestMust(t, err)
	if _, err = target.Relation(); err != nil {
		t.Fatal("delta cleanup refusal damaged published predicate", err)
	}
	shown, exposed := failure.Error(), errors.Unwrap(failure)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != executionFailure.owner.runtime {
		t.Fatal("recursive delta cleanup lost aggregate runtime receipt", failure)
	}
	for _, relation := range executionFailure.owner.relations {
		if relation != nil && relation.relation != (ast.ASTRef{}) {
			relation.relation = ast.ASTRef{}
		}
	}
	astTestMust(t, receipt.Cleanup())
	astTestMust(t, receipt.Cleanup())
	if failure.Error() != shown || errors.Unwrap(failure) != exposed || plugin.users != 1 {
		t.Fatal("recursive delta cleanup retry changed failure or published ownership", plugin.users)
	}
}
