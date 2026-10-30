package transforms_test

// Source: src/muz/transforms/dl_mk_array_blast.cpp store substitution and
// pairwise Ackermann consistency constraints.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestArrayBlastEliminatesStoreDefinitionAndSelectPairs(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, context.Close()) })
	manager, err := context.Manager()
	coneMust(t, err)
	globals, err := context.GlobalParameters()
	coneMust(t, err)
	rules, err := base.NewRuleManager(manager)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, rules.Close()) })
	set, err := base.NewRuleSet(rules)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, set.Close()) })
	boolean, err := manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalName, err := manager.Symbols.Intern("array-blast-goal")
	coneMust(t, err)
	goalDecl, err := manager.MkFuncDecl(goalName, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, rules.RegisterPredicate(goalDecl))
	goal, err := manager.MkApp(goalDecl, nil)
	coneMust(t, err)
	a, err := manager.MkVar(0, arraySort)
	coneMust(t, err)
	b, err := manager.MkVar(1, arraySort)
	coneMust(t, err)
	i, err := manager.MkVar(2, boolean)
	coneMust(t, err)
	j, err := manager.MkVar(3, boolean)
	coneMust(t, err)
	v, err := manager.MkVar(4, boolean)
	coneMust(t, err)
	x, err := manager.MkVar(5, boolean)
	coneMust(t, err)
	y, err := manager.MkVar(6, boolean)
	coneMust(t, err)
	c, err := manager.MkVar(7, arraySort)
	coneMust(t, err)
	k, err := manager.MkVar(8, boolean)
	coneMust(t, err)
	z, err := manager.MkVar(9, boolean)
	coneMust(t, err)
	var expectedRefs []ast.ASTRef
	for _, expression := range []ast.ASTHandle{i, j, k} {
		reference, refErr := manager.NewRef(expression)
		coneMust(t, refErr)
		expectedRefs = append(expectedRefs, reference)
	}
	t.Cleanup(func() {
		for index := len(expectedRefs) - 1; index >= 0; index-- {
			coneMust(t, manager.DelRef(&expectedRefs[index]))
		}
	})
	family, err := manager.ArrayFamilyID()
	coneMust(t, err)
	store, err := manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{b, i, v})
	coneMust(t, err)
	definition, err := manager.MkEq(a, store)
	coneMust(t, err)
	selectI, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, i})
	coneMust(t, err)
	selectJ, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{b, j})
	coneMust(t, err)
	first, err := manager.MkEq(selectI, x)
	coneMust(t, err)
	second, err := manager.MkEq(selectJ, y)
	coneMust(t, err)
	selectDifferentBase, err := manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{c, k})
	coneMust(t, err)
	differentBaseObservation, err := manager.MkEq(selectDifferentBase, z)
	coneMust(t, err)
	rule, err := rules.NewRule(goal, []ast.ASTHandle{definition, first, second, differentBaseObservation}, nil, "array-blast")
	coneMust(t, err)
	coneMust(t, set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, set.SetOutputPredicate(goalDecl))
	valid, err := set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("array blast source is unstratified")
	}
	plugin, err := transforms.NewArrayBlastPlugin(manager, rules, globals.Context, globals, 35999)
	coneMust(t, err)
	transformer := base.NewRuleTransformer()
	coneMust(t, transformer.Register(plugin))
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	modified, err := transformer.Apply(set)
	coneMust(t, err)
	if !modified {
		t.Fatal("array blast did not modify the rule set")
	}
	transformed, err := set.Rule(0)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, transformed.Close()) })
	tails, err := transformed.TailSize()
	coneMust(t, err)
	if tails == 0 {
		t.Fatal("Ackermann consistency constraint was lost")
	}
	ackermannConstraints := 0
	for index := 0; index < tails; index++ {
		tail, _, getErr := transformed.Tail(index)
		coneMust(t, getErr)
		count, _, findErr := arrayBlastBooleanAckermannCounts(manager, tail)
		coneMust(t, findErr)
		ackermannConstraints += count
		stack := []ast.ASTHandle{tail}
		for len(stack) != 0 {
			last := len(stack) - 1
			current := stack[last]
			stack = stack[:last]
			kind, kindErr := manager.GetKind(current)
			coneMust(t, kindErr)
			if kind != ast.ASTApp {
				continue
			}
			currentFamily, familyErr := manager.GetFamilyID(current)
			coneMust(t, familyErr)
			if currentFamily == family {
				op, opErr := manager.GetDeclKind(current)
				coneMust(t, opErr)
				if op == ast.ASTArrayOpSelect || op == ast.ASTArrayOpStore {
					t.Fatal("array operation survived blasting", op)
				}
			}
			count, countErr := manager.GetNumArgs(current)
			coneMust(t, countErr)
			for child := uint32(0); child < count; child++ {
				argument, argumentErr := manager.GetArg(current, child)
				coneMust(t, argumentErr)
				stack = append(stack, argument)
			}
		}
	}
	if ackermannConstraints != 1 {
		t.Fatalf("got %d Ackermann equalities; want exactly one for the same-base select pair and none across different bases", ackermannConstraints)
	}
}

func arrayBlastBooleanAckermannCounts(manager *ast.ASTManager, root ast.ASTHandle) (int, int, error) {
	basic := manager.BasicFamilyID()
	isVarEquality := func(e ast.ASTHandle) (bool, error) {
		kind, err := manager.GetKind(e)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		family, err := manager.GetFamilyID(e)
		if err != nil || family != basic {
			return false, err
		}
		op, err := manager.GetDeclKind(e)
		if err != nil || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
			return false, err
		}
		left, err := manager.GetArg(e, 0)
		if err != nil {
			return false, err
		}
		right, err := manager.GetArg(e, 1)
		if err != nil {
			return false, err
		}
		leftKind, err := manager.GetKind(left)
		if err != nil || leftKind != ast.ASTVar {
			return false, err
		}
		rightKind, err := manager.GetKind(right)
		if err != nil || rightKind != ast.ASTVar {
			return false, err
		}
		same, err := manager.AreEqual(left, right)
		return !same, err
	}
	isBooleanDisequality := func(e ast.ASTHandle) (bool, error) {
		kind, err := manager.GetKind(e)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		family, err := manager.GetFamilyID(e)
		if err != nil || family != basic {
			return false, err
		}
		op, err := manager.GetDeclKind(e)
		if err != nil {
			return false, err
		}
		if op == ast.ASTOpNot {
			child, err := manager.GetArg(e, 0)
			if err != nil {
				return false, err
			}
			return isVarEquality(child)
		}
		if op != ast.ASTOpIff {
			return false, nil
		}
		left, err := manager.GetArg(e, 0)
		if err != nil {
			return false, err
		}
		right, err := manager.GetArg(e, 1)
		if err != nil {
			return false, err
		}
		for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
			leftKind, err := manager.GetKind(pair[0])
			if err != nil || leftKind != ast.ASTApp {
				if err != nil {
					return false, err
				}
				continue
			}
			leftOp, err := manager.GetDeclKind(pair[0])
			if err != nil || leftOp != ast.ASTOpNot {
				if err != nil {
					return false, err
				}
				continue
			}
			variable, err := manager.GetArg(pair[0], 0)
			if err != nil {
				return false, err
			}
			variableKind, err := manager.GetKind(variable)
			if err != nil {
				return false, err
			}
			otherKind, err := manager.GetKind(pair[1])
			if err != nil {
				return false, err
			}
			if variableKind == ast.ASTVar && otherKind == ast.ASTVar {
				same, err := manager.AreEqual(variable, pair[1])
				return !same, err
			}
		}
		return false, nil
	}
	count, guards := 0, 0
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			if err != nil {
				return 0, 0, err
			}
			continue
		}
		family, err := manager.GetFamilyID(current)
		if err != nil {
			return 0, 0, err
		}
		op, err := manager.GetDeclKind(current)
		if err != nil {
			return 0, 0, err
		}
		numArgs, err := manager.GetNumArgs(current)
		if err != nil {
			return 0, 0, err
		}
		if family == basic && op == ast.ASTOpOr {
			hasGuard, hasValues := false, false
			localGuards := 0
			for i := uint32(0); i < numArgs; i++ {
				arg, err := manager.GetArg(current, i)
				if err != nil {
					return 0, 0, err
				}
				guard, err := isBooleanDisequality(arg)
				if err != nil {
					return 0, 0, err
				}
				valueEquality, err := isVarEquality(arg)
				if err != nil {
					return 0, 0, err
				}
				hasGuard = hasGuard || guard
				if guard {
					localGuards++
				}
				hasValues = hasValues || valueEquality
			}
			if hasGuard && hasValues {
				count++
				guards += localGuards
			}
		}
		for i := uint32(0); i < numArgs; i++ {
			child, err := manager.GetArg(current, i)
			if err != nil {
				return 0, 0, err
			}
			stack = append(stack, child)
		}
	}
	return count, guards, nil
}
