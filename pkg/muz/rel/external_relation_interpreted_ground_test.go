package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExecuteExternalGroundInterpretedRules(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle][][]ast.ASTHandle)
	appendUnique := func(target ast.ASTHandle, tuple []ast.ASTHandle) bool {
		for _, existing := range rows[target] {
			if len(existing) != len(tuple) {
				continue
			}
			equal := true
			for index := range tuple {
				equal = equal && existing[index] == tuple[index]
			}
			if equal {
				return false
			}
		}
		rows[target] = append(rows[target], append([]ast.ASTHandle(nil), tuple...))
		return true
	}
	copyRows := func(source [][]ast.ASTHandle) [][]ast.ASTHandle {
		result := make([][]ast.ASTHandle, len(source))
		for index := range source {
			result[index] = append([]ast.ASTHandle(nil), source[index]...)
		}
		return result
	}
	theory := tableRelationTheoryRewriter(t, f.ast)
	complements, filters, nonemptyDeltas, oneOutputUnions := 0, 0, 0, 0
	failComplement := false
	complementFailure := errors.New("external nullary complement refused")
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				rows[outputs[0]] = nil
			case ast.DatalogOpStore:
				appendUnique(outputs[0], arguments[1:])
			case ast.DatalogOpClone:
				rows[outputs[0]] = copyRows(rows[arguments[0]])
			case ast.DatalogOpFilter:
				filters++
				parameters, getErr := f.ast.GetDeclParametersCopy(declaration)
				if getErr != nil || len(parameters) != 1 {
					return getErr
				}
				condition, getErr := f.ast.ParameterAST(parameters[0])
				astTestMust(t, f.ast.DelParameter(&parameters[0]))
				if getErr != nil {
					return getErr
				}
				kept := rows[arguments[0]][:0]
				for _, tuple := range rows[arguments[0]] {
					out, rewriteErr := f.ast.NewRef(ast.ASTHandle{})
					if rewriteErr != nil {
						return rewriteErr
					}
					bindings := make([]ast.ASTHandle, len(tuple))
					for column := range tuple {
						bindings[len(tuple)-column-1] = tuple[column]
					}
					rewriteErr = theory.RewriteWithBindings(condition, bindings, out)
					answer, answerErr := f.ast.RefGet(out)
					rewriteErr = errors.Join(rewriteErr, answerErr)
					if rewriteErr == nil && f.ast.IsTrue(answer) {
						kept = append(kept, tuple)
					}
					rewriteErr = errors.Join(rewriteErr, f.ast.DelRef(&out))
					if rewriteErr != nil {
						return rewriteErr
					}
				}
				rows[outputs[0]] = kept
			case ast.DatalogOpUnion:
				if len(outputs) == 2 {
					rows[outputs[1]] = nil
				} else {
					oneOutputUnions++
				}
				for _, tuple := range rows[arguments[1]] {
					if appendUnique(outputs[0], tuple) && len(outputs) == 2 {
						appendUnique(outputs[1], tuple)
						nonemptyDeltas++
					}
				}
			default:
				t.Fatalf("unexpected ground interpreted assignment %d", kind)
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			if kind == ast.DatalogOpIsEmpty {
				return f.ast.MkBoolVal(len(rows[arguments[0]]) == 0)
			}
			if kind == ast.DatalogOpComplement && failComplement {
				return ast.ASTHandle{}, complementFailure
			}
			sort, cause := f.ast.GetRange(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			result, cause := f.ast.MkFreshConst("external-ground-interpreted", sort)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			switch kind {
			case ast.DatalogOpComplement:
				complements++
				if len(rows[arguments[0]]) == 0 {
					rows[result] = [][]ast.ASTHandle{{}}
				}
			case ast.DatalogOpJoin:
				for _, left := range rows[arguments[0]] {
					for _, right := range rows[arguments[1]] {
						appendUnique(result, append(append([]ast.ASTHandle(nil), left...), right...))
					}
				}
			default:
				t.Fatalf("unexpected ground interpreted reduction %d", kind)
			}
			return result, nil
		},
	})
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	rules, err := base.NewRuleManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, cause := f.ast.Symbols.Intern(name)
		astTestMust(t, cause)
		predicate, cause := f.ast.MkFuncDecl(symbol, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, cause)
		astTestMust(t, rules.RegisterPredicate(predicate))
		return predicate
	}
	application := func(predicate ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		value, cause := f.ast.MkApp(predicate, arguments)
		astTestMust(t, cause)
		return value
	}
	trueCondition, err := f.ast.MkEq(f.truth, f.truth)
	astTestMust(t, err)
	falseCondition, err := f.ast.MkEq(f.truth, f.falsehood)
	astTestMust(t, err)
	accept := declare("external-ground-interpreted-accept", []ast.ASTHandle{f.boolean})
	reject := declare("external-ground-interpreted-reject", []ast.ASTHandle{f.boolean})
	nullary := declare("external-ground-interpreted-nullary", nil)
	set, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()
	for _, item := range []struct {
		head, condition ast.ASTHandle
		name            string
	}{
		{application(accept, f.truth), trueCondition, "ground-interpreted-accept"},
		{application(reject, f.truth), falseCondition, "ground-interpreted-reject"},
		{application(nullary), trueCondition, "ground-interpreted-nullary"},
	} {
		rule, cause := rules.NewRule(item.head, []ast.ASTHandle{item.condition}, nil, item.name)
		astTestMust(t, cause)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}
	check := func(predicate ast.ASTHandle, want [][]ast.ASTHandle) {
		relation, cause := plugin.PredicateRelation(predicate)
		astTestMust(t, cause)
		handle, cause := relation.Relation()
		astTestMust(t, cause)
		if len(rows[handle]) != len(want) {
			t.Fatal("ground interpreted rows", rows[handle], "want", want)
		}
		for i := range want {
			if len(rows[handle][i]) != len(want[i]) {
				t.Fatal("ground interpreted row arity", rows[handle], want)
			}
			for column := range want[i] {
				if rows[handle][i][column] != want[i][column] {
					t.Fatal("ground interpreted row", rows[handle], want)
				}
			}
		}
	}
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	check(accept, [][]ast.ASTHandle{{f.truth}})
	check(reject, nil)
	check(nullary, [][]ast.ASTHandle{{}})
	if complements != 1 || filters == 0 || nonemptyDeltas != 0 || oneOutputUnions != 3 {
		t.Fatal("ground interpreted callbacks", complements, filters, nonemptyDeltas, oneOutputUnions)
	}
	astTestMust(t, ExecuteExternalFactRules(plugin, set))
	check(accept, [][]ast.ASTHandle{{f.truth}})
	check(nullary, [][]ast.ASTHandle{{}})
	if complements != 2 || nonemptyDeltas != 0 || oneOutputUnions != 6 {
		t.Fatal("repeated ground interpreted execution", complements, nonemptyDeltas, oneOutputUnions)
	}

	ground := declare("external-ground-before-complement-failure", nil)
	filtered := declare("external-filtered-complement-failure", nil)
	failing, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, failing.Close()) }()
	for _, item := range []struct {
		head ast.ASTHandle
		tail []ast.ASTHandle
		name string
	}{
		{application(ground), nil, "ground-before-complement-failure"},
		{application(filtered), []ast.ASTHandle{trueCondition}, "complement-failure"},
	} {
		rule, cause := rules.NewRule(item.head, item.tail, nil, item.name)
		astTestMust(t, cause)
		astTestMust(t, failing.Add(rule))
		astTestMust(t, rule.Close())
	}
	failComplement = true
	if cause := ExecuteExternalFactRules(plugin, failing); !errors.Is(cause, complementFailure) {
		t.Fatal("complement preflight failure", cause)
	}
	failComplement = false
	check(ground, nil)

	canceled := declare("external-ground-interpreted-canceled", nil)
	canceledSet, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, canceledSet.Close()) }()
	rule, err := rules.NewRule(application(canceled), []ast.ASTHandle{trueCondition}, nil, "ground-interpreted-canceled")
	astTestMust(t, err)
	astTestMust(t, canceledSet.Add(rule))
	astTestMust(t, rule.Close())
	beforeCanceledComplements := complements
	f.ast.ResourceLimit().Cancel()
	if cause := ExecuteExternalFactRules(plugin, canceledSet); cause == nil {
		t.Fatal("canceled ground interpreted execution continued")
	}
	f.ast.ResourceLimit().ResetCancel()
	if complements != beforeCanceledComplements+1 {
		t.Fatal("cancellation changed source constructor-preflight precedence", beforeCanceledComplements, complements)
	}
	check(canceled, nil)

	canceledFailure := declare("external-ground-interpreted-canceled-complement-failure", nil)
	canceledFailureSet, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, canceledFailureSet.Close()) }()
	rule, err = rules.NewRule(application(canceledFailure), []ast.ASTHandle{trueCondition}, nil, "ground-interpreted-canceled-complement-failure")
	astTestMust(t, err)
	astTestMust(t, canceledFailureSet.Add(rule))
	astTestMust(t, rule.Close())
	failComplement = true
	f.ast.ResourceLimit().Cancel()
	if cause := ExecuteExternalFactRules(plugin, canceledFailureSet); !errors.Is(cause, complementFailure) {
		t.Fatal("constructor preflight no longer precedes cancellation", cause)
	}
	f.ast.ResourceLimit().ResetCancel()
	failComplement = false
	check(canceledFailure, nil)
}
