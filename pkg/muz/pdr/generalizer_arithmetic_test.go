package pdr

// Source: core_arith_inductive_generalizer bound matching, alias substitution,
// and candidate adoption in src/muz/pdr/pdr_generalizers.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func arithmeticGeneralizerFixture(t *testing.T, integer bool) (*ast.ASTManager, *PredicateTransformer, ast.ASTHandle, func()) {
	t.Helper()
	manager, owner, initialTransformer, initialTerm, closeBase := closureFixture(t)
	transformer := initialTransformer
	term := initialTerm
	if integer {
		closureMust(t, initialTransformer.Close())
		family, err := manager.ArithmeticFamilyID()
		closureMust(t, err)
		integerSort, err := manager.MkFamilySort(family, 1, nil)
		closureMust(t, err)
		boolean, err := manager.MkBoolSort()
		closureMust(t, err)
		name, err := manager.Symbols.Intern("arithmetic_inductive_predicate")
		closureMust(t, err)
		head, err := manager.MkFuncDecl(name, []ast.ASTHandle{integerSort}, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, err)
		transformer, err = NewPredicateTransformer(owner, head, ReachableNoCache)
		closureMust(t, err)
		signature, err := transformer.Signature()
		closureMust(t, err)
		declaration, err := owner.OldToNew(signature[0], 0)
		closureMust(t, err)
		term, err = manager.MkApp(declaration, nil)
		closureMust(t, err)
	}
	closeAll := func() {
		closureMust(t, transformer.Close())
		closureMust(t, owner.Close())
		closeBase()
	}
	return manager, transformer, term, closeAll
}

func arithmeticGeneralizerRelation(t *testing.T, manager *ast.ASTManager, operator int32, left, right ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	family, err := manager.ArithmeticFamilyID()
	closureMust(t, err)
	result, err := manager.MkFamilyApp(family, operator, nil, []ast.ASTHandle{left, right})
	closureMust(t, err)
	return result
}

func arithmeticGeneralizerWantRelation(t *testing.T, manager *ast.ASTManager, expression ast.ASTHandle, operator int32, left, right ast.ASTHandle) {
	t.Helper()
	arguments := closureOperation(t, manager, expression, operator)
	if arguments.Count() != 2 {
		t.Fatal("relation arity", arguments.Count())
	}
	gotLeft, err := arguments.At(0)
	closureMust(t, err)
	gotRight, err := arguments.At(1)
	closureMust(t, err)
	if gotLeft != left || gotRight != right {
		t.Fatalf("relation got left=%v right=%v, want left=%v right=%v", gotLeft, gotRight, left, right)
	}
}

func arithmeticGeneralizerWantMinusOne(t *testing.T, manager *ast.ASTManager, expression, term ast.ASTHandle) {
	t.Helper()
	arguments := closureOperation(t, manager, expression, 7)
	if arguments.Count() != 2 {
		t.Fatal("subtraction arity", arguments.Count())
	}
	left, err := arguments.At(0)
	closureMust(t, err)
	right, err := arguments.At(1)
	closureMust(t, err)
	text, integer, numeral, err := manager.ArithmeticNumeralText(right)
	closureMust(t, err)
	if left != term || !numeral || !integer || text != "1" {
		t.Fatal("expected term - 1", left, term, text, integer, numeral)
	}
}

func TestArithmeticInductiveCandidateRewritesAliasesAndIntegerGrid(t *testing.T) {
	manager, transformer, x, closeAll := arithmeticGeneralizerFixture(t, true)
	integerSort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("arithmetic_inductive_y", integerSort)
	closureMust(t, err)
	z, err := manager.MkFreshConst("arithmetic_inductive_z", integerSort)
	closureMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	closureMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	closureMust(t, err)
	three, err := manager.ArithmeticIntegerNumeralText("3")
	closureMust(t, err)
	input := []ast.ASTHandle{
		arithmeticGeneralizerRelation(t, manager, closureArithLE, y, two),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, y, one),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, y, three),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, z, two),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, z, three),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, z, one),
		// Source map::insert overwrites same-magnitude entries, so the exact
		// x bounds must occur last to become the selected alias pair.
		arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two),
	}
	candidate, references, changed, err := arithmeticInductiveCandidate(transformer, input)
	closureMust(t, err)
	if !changed || len(candidate) != len(input) {
		t.Fatal("candidate publication", changed, len(candidate), len(input))
	}
	parityArgs, err := manager.AppArgumentView(candidate[6])
	closureMust(t, err)
	if operator, err := manager.GetDeclKind(candidate[6]); err != nil || operator != ast.ASTOpEq || parityArgs.Count() != 2 {
		t.Fatal("parity equality", operator, parityArgs.Count(), err)
	}
	modulo, err := parityArgs.At(0)
	closureMust(t, err)
	moduloArgs := closureOperation(t, manager, modulo, arithmeticInductiveMod)
	moduloTerm, err := moduloArgs.At(0)
	closureMust(t, err)
	modulus, err := moduloArgs.At(1)
	closureMust(t, err)
	modulusText, _, modulusNumeral, err := manager.ArithmeticNumeralText(modulus)
	closureMust(t, err)
	if moduloTerm != x || !modulusNumeral || modulusText != "2" {
		t.Fatal("source parity modulus", moduloTerm, x, modulusText, modulusNumeral)
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	closureMust(t, err)
	arithmeticGeneralizerWantRelation(t, manager, candidate[7], closureArithLE, x, zero)
	arithmeticGeneralizerWantRelation(t, manager, candidate[0], closureArithLE, y, x)
	arithmeticGeneralizerWantRelation(t, manager, candidate[1], closureArithLT, y, x)
	arguments := closureOperation(t, manager, candidate[2], closureArithLE)
	left, err := arguments.At(0)
	closureMust(t, err)
	right, err := arguments.At(1)
	closureMust(t, err)
	if left != y {
		t.Fatal("upper adjacent alias", left, y)
	}
	arithmeticGeneralizerWantMinusOne(t, manager, right, x)
	arithmeticGeneralizerWantRelation(t, manager, candidate[3], closureArithGE, z, x)
	arithmeticGeneralizerWantRelation(t, manager, candidate[4], closureArithGT, z, x)
	arguments = closureOperation(t, manager, candidate[5], closureArithGE)
	left, err = arguments.At(0)
	closureMust(t, err)
	right, err = arguments.At(1)
	closureMust(t, err)
	if left != z {
		t.Fatal("lower adjacent alias", left, z)
	}
	arithmeticGeneralizerWantMinusOne(t, manager, right, x)
	closureMust(t, closeASTRefs(manager, references))
	closeAll()
}

func TestArithmeticInductiveCandidateNormalizesNegationAndOverwritesMagnitude(t *testing.T) {
	manager, transformer, x, closeAll := arithmeticGeneralizerFixture(t, true)
	sort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("arithmetic_inductive_overwrite", sort)
	closureMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	closureMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	closureMust(t, err)
	upperOne := arithmeticGeneralizerRelation(t, manager, closureArithLE, x, one)
	notUpperOne, err := manager.MkNot(upperOne)
	closureMust(t, err)
	input := []ast.ASTHandle{notUpperOne, arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two)}
	candidate, references, changed, err := arithmeticInductiveCandidate(transformer, input)
	closureMust(t, err)
	if !changed || len(references) == 0 || len(candidate) != 2 {
		t.Fatal("negated integer bound did not form exact value", changed, len(references), len(candidate))
	}
	closureMust(t, closeASTRefs(manager, references))

	overwritten := []ast.ASTHandle{
		arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, y, two),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two),
	}
	candidate, references, changed, err = arithmeticInductiveCandidate(transformer, overwritten)
	closureMust(t, err)
	if changed || len(references) != 0 {
		t.Fatal("same-magnitude source overwrite was not preserved", changed, len(references))
	}
	for index := range overwritten {
		if candidate[index] != overwritten[index] {
			t.Fatal("no-match candidate changed", index)
		}
	}
	closeAll()
}

func TestArithmeticInductiveCandidateRealAliasOmitsIntegerGrid(t *testing.T) {
	manager, transformer, x, closeAll := arithmeticGeneralizerFixture(t, false)
	sort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("arithmetic_inductive_real_y", sort)
	closureMust(t, err)
	two, err := manager.ArithmeticRealNumeralText("2")
	closureMust(t, err)
	input := []ast.ASTHandle{
		arithmeticGeneralizerRelation(t, manager, closureArithLE, y, two),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two),
	}
	candidate, references, changed, err := arithmeticInductiveCandidate(transformer, input)
	closureMust(t, err)
	if !changed || !manager.IsTrue(candidate[1]) || !manager.IsTrue(candidate[2]) {
		t.Fatal("real exact bounds should be removed without integer grid", changed, manager.IsTrue(candidate[1]), manager.IsTrue(candidate[2]))
	}
	arithmeticGeneralizerWantRelation(t, manager, candidate[0], closureArithLE, y, x)
	closureMust(t, closeASTRefs(manager, references))
	closeAll()
}

func TestArithmeticInductiveCoreOwnsFallbackAndAdoptedSubset(t *testing.T) {
	manager, transformer, x, closeAll := arithmeticGeneralizerFixture(t, true)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	closureMust(t, err)
	lower := arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two)
	upper := arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two)
	input, err := pinGeneralizedCore(manager, []ast.ASTHandle{lower}, true)
	closureMust(t, err)
	fallback, err := generalizeArithmeticInductiveCore(transformer, 0, input)
	closureMust(t, err)
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{input}))
	if len(fallback.literals) != 1 || fallback.literals[0] != lower {
		t.Fatal("no-match fallback", fallback.literals)
	}
	if _, err = manager.GetKind(fallback.literals[0]); err != nil {
		t.Fatal("fallback did not own its literal", err)
	}
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{fallback}))

	falsity, err := manager.MkFalse()
	closureMust(t, err)
	truth, err := manager.MkTrue()
	closureMust(t, err)
	closureMust(t, transformer.InitializeSystem(falsity, truth))
	lower = arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two)
	upper = arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two)
	matched, err := pinGeneralizedCore(manager, []ast.ASTHandle{lower, upper}, true)
	closureMust(t, err)
	adopted, err := generalizeArithmeticInductiveCore(transformer, 0, matched)
	closureMust(t, err)
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{matched}))
	for _, literal := range adopted.literals {
		if literal == lower || literal == upper {
			t.Fatal("inductive candidate was not adopted", adopted.literals)
		}
	}
	for _, literal := range adopted.literals {
		if _, err = manager.GetKind(literal); err != nil {
			t.Fatal("adopted subset did not own its literal", err)
		}
	}
	closureMust(t, closeGeneralizedCores(manager, []generalizedCore{adopted}))
	closeAll()
}

// AUDIT.md muz_pdr_01-F5: substitute_alias only rewrites bounds at r, r-1
// and r+1; a distant bound such as (<= y 5) with r = 2 is kept verbatim.
func TestArithmeticInductiveCandidateLeavesDistantBoundsUnchanged(t *testing.T) {
	manager, transformer, x, closeAll := arithmeticGeneralizerFixture(t, true)
	integerSort, err := manager.GetSort(x)
	closureMust(t, err)
	y, err := manager.MkFreshConst("arithmetic_inductive_distant_y", integerSort)
	closureMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	closureMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	closureMust(t, err)
	five, err := manager.ArithmeticIntegerNumeralText("5")
	closureMust(t, err)
	input := []ast.ASTHandle{
		arithmeticGeneralizerRelation(t, manager, closureArithLE, y, five),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, y, zero),
		arithmeticGeneralizerRelation(t, manager, closureArithGE, x, two),
		arithmeticGeneralizerRelation(t, manager, closureArithLE, x, two),
	}
	candidate, references, changed, err := arithmeticInductiveCandidate(transformer, input)
	closureMust(t, err)
	if !changed || len(candidate) != len(input) {
		t.Fatal("candidate publication", changed, len(candidate), len(input))
	}
	if candidate[0] != input[0] || candidate[1] != input[1] {
		t.Fatal("distant bounds were rewritten")
	}
	arithmeticGeneralizerWantRelation(t, manager, candidate[3], closureArithLE, x, zero)
	closureMust(t, closeASTRefs(manager, references))
	closeAll()
}
