package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func newArrayEqRewriteCoveragePlugin(t *testing.T, f *coneFixture) *transforms.ArrayEqRewritePlugin {
	t.Helper()
	plugin, err := transforms.NewArrayEqRewritePlugin(f.manager, f.rules, f.globals.Context, f.globals, 34998)
	coneMust(t, err)
	t.Cleanup(func() { coneMust(t, plugin.Close()) })
	return plugin
}

func TestArrayEqRewritePluginCloseAndClosedTransform(t *testing.T) {
	f := newConeFixture(t)
	plugin, err := transforms.NewArrayEqRewritePlugin(f.manager, f.rules, f.globals.Context, f.globals, 12345)
	coneMust(t, err)
	if plugin.Priority() != 12345 || plugin.CanDestratifyNegation() {
		t.Fatal("array equality rewrite plugin capability contract differs from the source")
	}
	plugin.Cancel()
	coneMust(t, plugin.Close())
	coneMust(t, plugin.Close())
	if result, transformErr := plugin.Transform(f.set); transformErr == nil || result != nil {
		if result != nil {
			coneMust(t, result.Close())
		}
		t.Fatalf("closed plugin transform returned %v, %v", result, transformErr)
	}
}

func TestArrayEqRewriteExpandsSelectStoreInsidePredicateTail(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	array, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	index, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	value, err := f.manager.MkVar(2, boolean)
	coneMust(t, err)
	arrayFamily, err := f.manager.ArrayFamilyID()
	coneMust(t, err)
	stored, err := f.manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array, index, value})
	coneMust(t, err)
	selected, err := f.manager.MkFamilyApp(arrayFamily, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{stored, index})
	coneMust(t, err)
	name, err := f.manager.Symbols.Intern("array-eq-predicate-tail")
	coneMust(t, err)
	predicate, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(predicate))
	predicateTail, err := f.manager.MkApp(predicate, []ast.ASTHandle{selected})
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-eq-predicate-goal", []ast.ASTHandle{boolean})
	goal, err := f.manager.MkApp(goalDecl, []ast.ASTHandle{value})
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{predicateTail}, nil, "array-eq-predicate-tail")
	result, err := newArrayEqRewriteCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	tail, _, err := rule.Tail(0)
	coneMust(t, err)
	got, err := f.manager.GetArg(tail, 0)
	coneMust(t, err)
	head, err := rule.Head()
	coneMust(t, err)
	want, err := f.manager.GetArg(head, 0)
	coneMust(t, err)
	if got != want {
		t.Fatalf("predicate tail argument after select-store expansion %v, want head variable %v", got, want)
	}
}

func TestArrayEqRewriteMergesTransitiveClassesAndKeepsOtherEqualities(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-eq-transitive", []ast.ASTHandle{arraySort})
	a, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	b, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	c, err := f.manager.MkVar(2, arraySort)
	coneMust(t, err)
	leftBool, err := f.manager.MkVar(3, boolean)
	coneMust(t, err)
	rightBool, err := f.manager.MkVar(4, boolean)
	coneMust(t, err)
	first, err := f.manager.MkEq(a, b)
	coneMust(t, err)
	second, err := f.manager.MkEq(b, c)
	coneMust(t, err)
	ordinaryEquality, err := f.manager.MkEq(leftBool, rightBool)
	coneMust(t, err)
	goal, err := f.manager.MkApp(goalDecl, []ast.ASTHandle{a})
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{first, second, ordinaryEquality}, nil, "array-eq-transitive")
	if err = f.set.SetOutputPredicate(goalDecl); err != nil {
		t.Fatal(err)
	}
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("transitive array equality fixture is unstratified")
	}
	result, err := newArrayEqRewriteCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	count, err := rule.TailSize()
	coneMust(t, err)
	if count != 4 {
		t.Fatalf("rewritten tail size %d, want ordinary equality and all three class equalities", count)
	}
	seenOrdinary := false
	seenClass := map[[2]ast.ASTHandle]bool{}
	reflexive := 0
	for index := 0; index < count; index++ {
		tail, _, getErr := rule.Tail(index)
		coneMust(t, getErr)
		if tail == ordinaryEquality {
			seenOrdinary = true
			continue
		}
		if f.manager.IsTrue(tail) {
			reflexive++
			continue
		}
		left, right, equality, getErr := arrayEqCoverageOperands(f.manager, tail)
		coneMust(t, getErr)
		if !equality {
			t.Fatalf("rewritten tail %d is neither retained condition nor array equality: %v", index, tail)
		}
		if !((left == b && right == c) || (left == c && right == b) ||
			(left == a && right == c) || (left == c && right == a)) {
			t.Fatalf("unexpected restored transitive class equality: (%v, %v)", left, right)
		}
		seenClass[orderedASTPair(left, right)] = true
	}
	if !seenOrdinary || reflexive != 1 || len(seenClass) != 2 ||
		!(seenClass[orderedASTPair(a, c)] && seenClass[orderedASTPair(b, c)]) {
		t.Fatalf("rewritten conditions missing ordinary or transitive class equalities: ordinary=%v reflexive=%d class=%v", seenOrdinary, reflexive, seenClass)
	}
}

func TestArrayEqRewriteRestoresIndependentClassesAndUsesSourceRoots(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-eq-independent", []ast.ASTHandle{arraySort})
	a, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	b, err := f.manager.MkVar(1, arraySort)
	coneMust(t, err)
	c, err := f.manager.MkVar(2, arraySort)
	coneMust(t, err)
	d, err := f.manager.MkVar(3, arraySort)
	coneMust(t, err)
	first, err := f.manager.MkEq(a, b)
	coneMust(t, err)
	second, err := f.manager.MkEq(c, d)
	coneMust(t, err)
	goal, err := f.manager.MkApp(goalDecl, []ast.ASTHandle{a})
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{first, second}, nil, "array-eq-independent")
	if err = f.set.SetOutputPredicate(goalDecl); err != nil {
		t.Fatal(err)
	}
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("independent array equality fixture is unstratified")
	}
	result, err := newArrayEqRewriteCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	count, err := rule.TailSize()
	coneMust(t, err)
	if count != 4 {
		t.Fatalf("rewritten tail size %d, want both member equalities for each independent class", count)
	}
	seen := map[[2]ast.ASTHandle]bool{}
	reflexive := 0
	for index := 0; index < count; index++ {
		tail, _, getErr := rule.Tail(index)
		coneMust(t, getErr)
		if f.manager.IsTrue(tail) {
			reflexive++
			continue
		}
		left, right, equality, getErr := arrayEqCoverageOperands(f.manager, tail)
		coneMust(t, getErr)
		if !equality {
			t.Fatalf("independent class tail %d is not an equality", index)
		}
		seen[orderedASTPair(left, right)] = true
	}
	if reflexive != 2 || len(seen) != 2 || !seen[orderedASTPair(a, b)] || !seen[orderedASTPair(c, d)] {
		t.Fatalf("independent classes were not restored: reflexive=%d equalities=%v", reflexive, seen)
	}
}

func TestArrayEqRewriteMergesPreviouslySeparateClasses(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-eq-join-classes", []ast.ASTHandle{arraySort})
	variables := make([]ast.ASTHandle, 4)
	for i := range variables {
		variables[i], err = f.manager.MkVar(uint32(i), arraySort)
		coneMust(t, err)
	}
	first, err := f.manager.MkEq(variables[0], variables[1])
	coneMust(t, err)
	second, err := f.manager.MkEq(variables[2], variables[3])
	coneMust(t, err)
	join, err := f.manager.MkEq(variables[1], variables[2])
	coneMust(t, err)
	goal, err := f.manager.MkApp(goalDecl, []ast.ASTHandle{variables[0]})
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{first, second, join}, nil, "array-eq-join-classes")
	if err = f.set.SetOutputPredicate(goalDecl); err != nil {
		t.Fatal(err)
	}
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("class-joining array equality fixture is unstratified")
	}
	result, err := newArrayEqRewriteCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	count, err := rule.TailSize()
	coneMust(t, err)
	if count != len(variables) {
		t.Fatalf("joined class tail size %d, want one restored equality per member", count)
	}
	seen := map[[2]ast.ASTHandle]bool{}
	reflexive := 0
	for index := 0; index < count; index++ {
		tail, _, getErr := rule.Tail(index)
		coneMust(t, getErr)
		if f.manager.IsTrue(tail) {
			reflexive++
			continue
		}
		left, right, equality, getErr := arrayEqCoverageOperands(f.manager, tail)
		coneMust(t, getErr)
		if !equality {
			t.Fatalf("joined class tail %d is not an equality", index)
		}
		seen[orderedASTPair(left, right)] = true
	}
	if reflexive != 1 || len(seen) != len(variables)-1 {
		t.Fatalf("joined class restored %d reflexive and %d nonreflexive equalities, want 1 and 3", reflexive, len(seen))
	}
	parent := make(map[ast.ASTHandle]ast.ASTHandle, len(variables))
	for _, variable := range variables {
		parent[variable] = variable
	}
	var root func(ast.ASTHandle) ast.ASTHandle
	root = func(variable ast.ASTHandle) ast.ASTHandle {
		if parent[variable] != variable {
			parent[variable] = root(parent[variable])
		}
		return parent[variable]
	}
	for pair := range seen {
		parent[root(pair[0])] = root(pair[1])
	}
	for _, variable := range variables[1:] {
		if root(variable) != root(variables[0]) {
			t.Fatalf("restored class does not connect all variables: %v", seen)
		}
	}
}

func TestArrayEqRewriteHandlesReflexiveArrayEqualityOnce(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	coneMust(t, err)
	goalDecl := declareArrayBlastPredicate(t, f, "array-eq-reflexive", []ast.ASTHandle{arraySort})
	array, err := f.manager.MkVar(0, arraySort)
	coneMust(t, err)
	reflexive, err := f.manager.MkEq(array, array)
	coneMust(t, err)
	goal, err := f.manager.MkApp(goalDecl, []ast.ASTHandle{array})
	coneMust(t, err)
	addTransformRule(t, f, goal, []ast.ASTHandle{reflexive}, nil, "array-eq-reflexive")
	if err = f.set.SetOutputPredicate(goalDecl); err != nil {
		t.Fatal(err)
	}
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("reflexive array equality fixture is unstratified")
	}
	result, err := newArrayEqRewriteCoveragePlugin(t, f).Transform(f.set)
	coneMust(t, err)
	defer func() { coneMust(t, result.Close()) }()
	rule, err := result.Rule(0)
	coneMust(t, err)
	defer func() { coneMust(t, rule.Close()) }()
	count, err := rule.TailSize()
	coneMust(t, err)
	if count != 1 {
		t.Fatalf("reflexive class restored %d equalities, want exactly one", count)
	}
	tail, _, err := rule.Tail(0)
	coneMust(t, err)
	if !f.manager.IsTrue(tail) {
		t.Fatalf("reflexive class equality was not simplified to true: %v", tail)
	}
}

func orderedASTPair(left, right ast.ASTHandle) [2]ast.ASTHandle {
	if right.Origin() < left.Origin() || right.Origin() == left.Origin() &&
		(right.Slot() < left.Slot() || right.Slot() == left.Slot() && right.Generation() < left.Generation()) {
		left, right = right, left
	}
	return [2]ast.ASTHandle{left, right}
}

func arrayEqCoverageOperands(manager *ast.ASTManager, expression ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if manager.IsTrue(expression) || manager.IsFalse(expression) {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	if count, getErr := manager.GetNumArgs(expression); getErr != nil || count != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, getErr
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if family != manager.BasicFamilyID() {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	operation, err := manager.GetDeclKind(expression)
	if err != nil || operation != ast.ASTOpEq {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := manager.GetArg(expression, 1)
	return left, right, err == nil, err
}
