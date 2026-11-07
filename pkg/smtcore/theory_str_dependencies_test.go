package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func stringDependencyEquality(t *testing.T, theory *SMTStringTheory, left, right ast.ASTHandle, positive bool) {
	t.Helper()
	state := theory.state
	equality, err := state.manager.MkEq(left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	if !positive {
		literal = literal.Negated()
	}
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
}

func stringDependencyConcat(t *testing.T, theory *SMTStringTheory, left, right ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	value, err := theory.state.manager.MkSequenceConcat(left, right)
	astTestMust(t, err)
	astTestMust(t, theory.state.Internalize(value, false))
	return value
}

func stringDependencyPrivateMerge(t *testing.T, theory *SMTStringTheory, left, right ast.ASTHandle) {
	t.Helper()
	l := theory.state.context.enodes.get(left)
	r := theory.state.context.enodes.get(right)
	if l == nil || r == nil || !theory.IsAttachedToVariable(l) || !theory.IsAttachedToVariable(r) {
		t.Fatal("dependency fixture requires two actual String theory variables")
	}
	theory.find.Merge(uint32(l.TheoryVar(theory.FamilyID())), uint32(r.TheoryVar(theory.FamilyID())))
}

func TestSMTStringDependencyClassificationTestersLengthAndEmptyChildren(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "dep-class-x")
	y := stringGroundingWord(t, theory, "dep-class-y")
	testers := []ast.ASTHandle{
		stringGroundingWord(t, theory, "dep-class-length-tester"),
		stringGroundingWord(t, theory, "dep-class-value-tester"),
		stringGroundingWord(t, theory, "dep-class-unroll-tester"),
	}
	theory.lengthTestVariables[testers[0]] = struct{}{}
	theory.valueTestVariables[testers[1]] = struct{}{}
	theory.unrollTestVariables[testers[2]] = struct{}{}
	kinds := newStringExpressionKinds()
	for _, tester := range testers {
		astTestMust(t, theory.classifyStringExpression(tester, kinds))
	}
	if len(kinds.variables) != 0 {
		t.Fatal("classification counted a native tester variable")
	}
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	astTestPin(t, m, length)
	astTestMust(t, theory.classifyStringExpression(length, kinds))
	if len(kinds.variables) != 0 {
		t.Fatal("classification descended through a length term")
	}
	concat := stringDependencyConcat(t, theory, x, y)
	astTestMust(t, theory.classifyStringExpression(concat, kinds))
	if len(kinds.concats) != 1 || len(kinds.variables) != 2 {
		t.Fatal("classification missed concat children")
	}
	empty := astTestString(t, m, "")
	astTestMust(t, state.Internalize(empty, false))
	astTestMust(t, state.PushScope())
	stringDependencyPrivateMerge(t, theory, x, empty)
	kinds = newStringExpressionKinds()
	astTestMust(t, theory.classifyStringExpression(concat, kinds))
	if len(kinds.concats) != 0 || len(kinds.variables) != 2 {
		t.Fatal("empty-child concat was kept, or its variable children were skipped")
	}
	astTestMust(t, state.PopScopes(1))
	kinds = newStringExpressionKinds()
	astTestMust(t, theory.classifyStringExpression(concat, kinds))
	if len(kinds.concats) != 1 {
		t.Fatal("classification used stale empty-child information after pop")
	}
}

func TestSMTStringDependencyPositiveEqualitySelection(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "dep-positive-x")
	y := stringGroundingWord(t, theory, "dep-positive-y")
	z := stringGroundingWord(t, theory, "dep-positive-z")
	positive := stringDependencyConcat(t, theory, x, y)
	negative := stringDependencyConcat(t, theory, z, x)
	stringDependencyEquality(t, theory, positive, y, true)
	stringDependencyEquality(t, theory, negative, z, false)
	// Relevancy is deliberately not a filter in this native classifier.
	kinds := newStringExpressionKinds()
	astTestMust(t, theory.classifyPositiveStringEqualities(kinds))
	if len(kinds.concats) != 1 || kinds.concats[positive] != 1 || len(kinds.variables) != 2 || kinds.variables[z] != 0 {
		t.Fatal("positive-context classifier included a false equality or skipped a true one", kinds)
	}
	length, err := m.MkSequenceLength(z)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	stringDependencyEquality(t, theory, length, zero, true)
	kinds = newStringExpressionKinds()
	astTestMust(t, theory.classifyPositiveStringEqualities(kinds))
	if kinds.variables[z] != 0 {
		t.Fatal("positive arithmetic equality traversed the String length argument")
	}
}

func TestSMTStringDependenciesAliasesCountsAndScopeRecompute(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "dep-count-x")
	y := stringGroundingWord(t, theory, "dep-count-y")
	z := stringGroundingWord(t, theory, "dep-count-z")
	alias := stringGroundingWord(t, theory, "dep-count-alias")
	first := stringGroundingWord(t, theory, "dep-count-first")
	second := stringGroundingWord(t, theory, "dep-count-second")
	fixed := stringGroundingWord(t, theory, "dep-count-fixed")
	constant := astTestString(t, m, "fixed")
	astTestMust(t, state.Internalize(constant, false))
	one := stringDependencyConcat(t, theory, alias, y)
	two := stringDependencyConcat(t, theory, x, z)
	stringDependencyEquality(t, theory, first, one, true)
	stringDependencyEquality(t, theory, second, two, true)
	astTestMust(t, state.PushScope())
	stringDependencyPrivateMerge(t, theory, x, alias)
	stringDependencyPrivateMerge(t, theory, first, one)
	stringDependencyPrivateMerge(t, theory, second, two)
	stringDependencyPrivateMerge(t, theory, fixed, constant)
	analysis, err := theory.analyzeStringDependencies()
	astTestMust(t, err)
	if state.context.enodes.get(x).Root() == state.context.enodes.get(alias).Root() {
		t.Fatal("fixture unexpectedly relied on core aliases instead of private String classes")
	}
	if analysis.variableAliases[alias] != x || analysis.dependencies[first][x] != 2 || analysis.dependencies[first][y] != 2 || analysis.dependencies[second][x] != 2 || analysis.dependencies[second][z] != 2 || analysis.dependencies[fixed][constant] != 1 {
		t.Fatal("dependency analysis lost alias, concat or concrete-value edges", analysis.dependencies)
	}
	if len(analysis.freeVariables) != 3 || analysis.freeVariables[x] != 3 || analysis.freeVariables[y] != 2 || analysis.freeVariables[z] != 2 {
		t.Fatal("source free-variable reference counts changed", analysis.freeVariables)
	}
	astTestMust(t, state.PopScopes(1))
	after, err := theory.analyzeStringDependencies()
	astTestMust(t, err)
	if len(after.variableAliases) != 0 || len(after.dependencies) != 0 || len(after.freeVariables) != 7 {
		t.Fatal("analysis retained scoped private aliases/dependencies", after.freeVariables)
	}
}

func TestSMTStringDependenciesConstantPriorityAndNativeSelfCycle(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "dep-constant-x")
	y := stringGroundingWord(t, theory, "dep-constant-y")
	constant := astTestString(t, m, "ab")
	astTestMust(t, state.Internalize(constant, false))
	concat := stringDependencyConcat(t, theory, x, y)
	stringDependencyEquality(t, theory, concat, constant, true)
	stringDependencyPrivateMerge(t, theory, x, constant)
	stringDependencyPrivateMerge(t, theory, concat, constant)
	analysis, err := theory.analyzeStringDependencies()
	astTestMust(t, err)
	if analysis.dependencies[x][constant] != 1 || analysis.dependencies[y][constant] != 3 || analysis.dependencies[x][x] != 2 || analysis.dependencies[x][y] != 2 {
		t.Fatal("constant/concat dependency precedence changed", analysis.dependencies)
	}
	// The source comment mentions self cycles, but its active selection code
	// excludes variables that have any dependency-map entry. Do not invent SCCs.
	if len(analysis.freeVariables) != 0 {
		t.Fatal("native self/constant dependency was promoted to a free variable", analysis.freeVariables)
	}
}

func TestSMTStringDependenciesUnrollCoreGroupsAndConcreteOperandSkip(t *testing.T) {
	for _, concrete := range []bool{false, true} {
		m, theory, state, _ := stringLibraryFixture(t)
		x := stringGroundingWord(t, theory, "dep-unroll-x")
		pattern := stringGroundingWord(t, theory, "dep-unroll-pattern")
		regex, err := m.MkSequenceToRegex(pattern)
		astTestMust(t, err)
		astTestPin(t, m, regex)
		bound, err := theory.freshUnrollBound()
		astTestMust(t, err)
		first := stringUnrollTerm(t, theory, regex, bound)
		secondBound, err := theory.freshUnrollBound()
		astTestMust(t, err)
		second := stringUnrollTerm(t, theory, regex, secondBound)
		stringConversionCoreEquality(t, state, first, second)
		stringDependencyEquality(t, theory, x, first, true)
		stringDependencyPrivateMerge(t, theory, x, first)
		if concrete {
			constant := astTestString(t, m, "ab")
			astTestMust(t, state.Internalize(constant, false))
			stringDependencyPrivateMerge(t, theory, pattern, constant)
		}
		analysis, err := theory.analyzeStringDependencies()
		astTestMust(t, err)
		if len(analysis.unrollGroups) != 1 {
			t.Fatal("unroll groups did not use the actual core equivalence class", concrete, analysis.unrollGroups)
		}
		for _, group := range analysis.unrollGroups {
			if len(group) != 2 || group[first] != 1 || group[second] != 1 {
				t.Fatal("unroll grouping lost a classified member", group)
			}
		}
		if concrete {
			if analysis.dependencies[x][pattern] != 0 {
				t.Fatal("concrete unroll operand acquired a type-6 dependency")
			}
		} else if analysis.dependencies[x][pattern] != 6 {
			t.Fatal("unroll dependency did not classify its symbolic String operand", analysis.dependencies)
		}
	}
}

func TestSMTStringDependenciesBoundaryGroupsAndEmptyLiteralRule(t *testing.T) {
	for _, side := range []string{"left", "right", "nonempty"} {
		t.Run(side, func(t *testing.T) {
			m, theory, state, _ := stringLibraryFixture(t)
			x := stringGroundingWord(t, theory, "dep-bound-x")
			y := stringGroundingWord(t, theory, "dep-bound-y")
			a := stringGroundingWord(t, theory, "dep-bound-a")
			b := stringGroundingWord(t, theory, "dep-bound-b")
			empty := astTestString(t, m, "")
			astTestMust(t, state.Internalize(empty, false))
			var first, second ast.ASTHandle
			if side == "left" {
				inner := stringDependencyConcat(t, theory, empty, x)
				first = stringDependencyConcat(t, theory, inner, a)
				second = stringDependencyConcat(t, theory, y, b)
			} else if side == "right" {
				inner := stringDependencyConcat(t, theory, x, empty)
				first = stringDependencyConcat(t, theory, a, inner)
				second = stringDependencyConcat(t, theory, b, y)
			} else {
				nonempty := astTestString(t, m, "K")
				astTestMust(t, state.Internalize(nonempty, false))
				first = stringDependencyConcat(t, theory, nonempty, a)
				second = stringDependencyConcat(t, theory, y, b)
			}
			stringDependencyEquality(t, theory, first, second, true)
			stringDependencyPrivateMerge(t, theory, first, second)
			analysis, err := theory.analyzeStringDependencies()
			astTestMust(t, err)
			if side == "nonempty" {
				if len(analysis.dependencies) != 0 || analysis.freeVariables[y] != 1 {
					t.Fatal("nonempty literal incorrectly created native boundary dependency", analysis.dependencies)
				}
			} else {
				kind := 4
				if side == "right" {
					kind = 5
				}
				if analysis.dependencies[y][empty] != kind || analysis.freeVariables[y] != 0 || analysis.freeVariables[x] != 1 {
					t.Fatal("deep empty literal boundary dependency or child traversal changed", side, analysis.dependencies, analysis.freeVariables)
				}
			}
		})
	}
}

func TestSMTStringDependenciesBoundaryGroupsDoNotUnionExistingGroups(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "dep-groups-x")
	y := stringGroundingWord(t, theory, "dep-groups-y")
	z := stringGroundingWord(t, theory, "dep-groups-z")
	w := stringGroundingWord(t, theory, "dep-groups-w")
	for i, pair := range [][2]ast.ASTHandle{{x, y}, {z, w}, {y, z}} {
		literal := astTestString(t, m, []string{"R", "S", "T"}[i])
		astTestMust(t, state.Internalize(literal, false))
		first := stringDependencyConcat(t, theory, pair[0], literal)
		second := stringDependencyConcat(t, theory, pair[1], literal)
		stringDependencyEquality(t, theory, first, second, true)
		stringDependencyPrivateMerge(t, theory, first, second)
	}
	analysis, err := theory.analyzeStringDependencies()
	astTestMust(t, err)
	if len(analysis.dependencies) != 0 || len(analysis.freeVariables) != 2 || analysis.freeVariables[x] != 1 || analysis.freeVariables[w] != 1 {
		t.Fatal("overlapping native boundary groups were incorrectly unioned", analysis.freeVariables)
	}
}

func TestSMTStringDependenciesRunGroundedContainmentConsumer(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "dep-contains-word")
	needle := stringGroundingWord(t, theory, "dep-contains-needle")
	left := stringGroundingWord(t, theory, "dep-contains-left")
	right := stringGroundingWord(t, theory, "dep-contains-right")
	text, find := astTestString(t, m, "ab"), astTestString(t, m, "b")
	astTestMust(t, state.Internalize(text, false))
	astTestMust(t, state.Internalize(find, false))
	concat := stringDependencyConcat(t, theory, left, right)
	stringDependencyEquality(t, theory, word, concat, true)
	stringDependencyPrivateMerge(t, theory, word, concat)
	stringDependencyPrivateMerge(t, theory, left, text)
	stringDependencyPrivateMerge(t, theory, right, text)
	stringDependencyPrivateMerge(t, theory, needle, find)
	relation := stringContainmentRegister(t, theory, word, needle)
	before := len(theory.refs)
	analysis, err := theory.analyzeStringDependencies()
	astTestMust(t, err)
	if analysis.dependencies[word][left] != 2 || len(theory.refs) <= before {
		t.Fatal("dependency analysis bypassed its grounded containment consumer")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	op, err := m.GetDeclKind(axiom)
	astTestMust(t, err)
	if op != ast.ASTOpOr {
		t.Fatal("grounded containment consumer omitted its dependency guard")
	}
	conclusion, err := m.GetArg(axiom, 1)
	astTestMust(t, err)
	if conclusion != relation {
		t.Fatal("grounded containment assertion used a different registered relation")
	}
}
