package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type sequenceAutomatonTestSolver struct {
	answer lbool.LBool
	calls  int
}

func (s *sequenceAutomatonTestSolver) CheckSatisfiable(ast.ASTHandle) (lbool.LBool, error) {
	s.calls++
	return s.answer, nil
}

func TestSequenceRewriterAutomatonMembership(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		astTestPin(t, m, h)
		return h
	}
	str := func(text string) ast.ASTHandle { return pin(m.MkString(text)) }
	toRegex := func(text string) ast.ASTHandle { return pin(m.MkSequenceToRegex(str(text))) }
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	check := func(name string, input, regex ast.ASTHandle, want ast.ASTHandle) {
		t.Helper()
		status, e := rw.RewriteStringInRegex(input, regex, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteFull || result != want {
			t.Fatalf("%s: status=%d result=%v want=%v", name, status, result, want)
		}
	}

	singleton := toRegex("ab")
	status, err := rw.RewriteStringInRegex(str("input"), singleton, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != RewriteFull || kind != ast.ASTOpEq {
		t.Fatalf("singleton language: status=%d kind=%d error=%v", status, kind, e)
	}

	union := pin(m.MkRegexUnion(toRegex("a"), toRegex("b")))
	check("union accepts", str("b"), union, truth)
	check("union rejects", str("c"), union, falsehood)
	star := pin(m.MkRegexStar(toRegex("a")))
	check("star empty", str(""), star, truth)
	check("star repeated", str("aaa"), star, truth)
	check("star rejects", str("aab"), star, falsehood)
	plus := pin(m.MkRegexPlus(toRegex("a")))
	check("plus accepts", str("aa"), plus, truth)
	check("plus rejects empty", str(""), plus, falsehood)
	option := pin(m.MkRegexOption(toRegex("a")))
	check("option accepts empty", str(""), option, truth)
	check("option rejects repeated", str("aa"), option, falsehood)

	rangeRegex := pin(m.MkRegexRange(str("a"), str("z")))
	check("range accepts", str("m"), rangeRegex, truth)
	check("range rejects", str("0"), rangeRegex, falsehood)
	regexSort, err := m.GetSort(rangeRegex)
	astTestMust(t, err)
	fullCharacter := pin(m.MkRegexFullChar(regexSort))
	check("full character accepts", str("x"), fullCharacter, truth)
	check("full character rejects pair", str("xy"), fullCharacter, falsehood)

	loop := pin(m.MkRegexLoop(toRegex("a"), 2, 3))
	check("bounded loop lower", str("aa"), loop, truth)
	check("bounded loop upper", str("aaa"), loop, truth)
	check("bounded loop below", str("a"), loop, falsehood)
	check("bounded loop above", str("aaaa"), loop, falsehood)

	byteZero := pin(m.BitVectorNumeralUnsigned(0, 8))
	byteSort, err := m.GetSort(byteZero)
	astTestMust(t, err)
	symbolicByte := pin(m.MkConst(astTestSymbol(t, m, "automaton-byte"), byteSort))
	symbolicUnit := pin(m.MkSequenceUnit(symbolicByte))
	status, err = rw.RewriteStringInRegex(symbolicUnit, rangeRegex, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != RewriteFull || kind != ast.ASTOpAnd {
		t.Fatalf("symbolic range: status=%d kind=%d error=%v", status, kind, e)
	}
}

func TestSequenceRewriterShortestRegexByteString(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	a, err := m.MkString("a")
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	for _, test := range []struct {
		name, want string
		make       func() (ast.ASTHandle, error)
	}{
		{name: "star", make: func() (ast.ASTHandle, error) { return m.MkRegexStar(aRegex) }},
		{name: "plus", want: "a", make: func() (ast.ASTHandle, error) { return m.MkRegexPlus(aRegex) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			regex, makeErr := test.make()
			astTestMust(t, makeErr)
			actual, found, witnessErr := rw.ShortestRegexByteString(regex)
			astTestMust(t, witnessErr)
			if !found || actual != test.want {
				t.Fatal("shortest regex witness", actual, found, "want", test.want)
			}
		})
	}
	b, err := m.MkString("b")
	astTestMust(t, err)
	bRegex, err := m.MkSequenceToRegex(b)
	astTestMust(t, err)
	finite, err := m.MkRegexUnion(aRegex, bRegex)
	astTestMust(t, err)
	words, exhaustive, err := rw.RegexByteStringCandidates(finite, 3)
	astTestMust(t, err)
	if !exhaustive || len(words) != 2 || !((words[0] == "a" && words[1] == "b") || (words[0] == "b" && words[1] == "a")) {
		t.Fatal("finite regex candidates", words, exhaustive)
	}
	zero, err := m.BitVectorNumeralUnsigned(0, 8)
	astTestMust(t, err)
	byteSort, err := m.GetSort(zero)
	astTestMust(t, err)
	symbolicByte, err := m.MkConst(astTestSymbol(t, m, "candidate-byte"), byteSort)
	astTestMust(t, err)
	symbolicUnit, err := m.MkSequenceUnit(symbolicByte)
	astTestMust(t, err)
	symbolicRegex, err := m.MkSequenceToRegex(symbolicUnit)
	astTestMust(t, err)
	words, exhaustive, err = rw.RegexByteStringCandidates(symbolicRegex, 1)
	astTestMust(t, err)
	if exhaustive || len(words) != 0 {
		t.Fatal("symbolic regex candidates", words, exhaustive)
	}
	symbolicPlus, err := m.MkRegexPlus(symbolicRegex)
	astTestMust(t, err)
	lengths, lengthExhaustive, err := rw.RegexAcceptedLengths(symbolicPlus, 3)
	astTestMust(t, err)
	if lengthExhaustive || len(lengths) != 3 || lengths[0] != 1 || lengths[1] != 2 || lengths[2] != 3 {
		t.Fatal("symbolic regex lengths", lengths, lengthExhaustive)
	}
	symbolicStar, err := m.MkRegexStar(symbolicRegex)
	astTestMust(t, err)
	symbolicComplement, err := m.MkRegexComplement(symbolicStar)
	astTestMust(t, err)
	lengths, lengthExhaustive, err = rw.RegexAcceptedLengths(symbolicComplement, 3)
	astTestMust(t, err)
	if lengthExhaustive || len(lengths) != 3 || lengths[0] != 1 || lengths[1] != 2 || lengths[2] != 3 {
		t.Fatal("symbolic complement lengths", lengths, lengthExhaustive)
	}
	lengths, lengthExhaustive, err = rw.RegexAcceptedLengths(symbolicRegex, 2)
	astTestMust(t, err)
	if !lengthExhaustive || len(lengths) != 1 || lengths[0] != 1 {
		t.Fatal("finite symbolic regex lengths", lengths, lengthExhaustive)
	}
	complement, err := m.MkRegexComplement(aRegex)
	astTestMust(t, err)
	words, exhaustive, err = rw.RegexByteStringCandidates(complement, 2)
	astTestMust(t, err)
	if exhaustive || len(words) != 2 || words[0] != "" || words[1] == "a" {
		t.Fatal("complement regex candidates", words, exhaustive)
	}
}

func TestSequenceRewriterAutomatonIntersectionSolver(t *testing.T) {
	m, context, globals, local := boolRewriterConfigFixture(t)
	full, err := NewTheoryRewriter(m, context, globals, local)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, full.Close()) })
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	solver := &sequenceAutomatonTestSolver{answer: lbool.True}
	astTestMust(t, rw.SetSolver(solver))
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		astTestPin(t, m, h)
		return h
	}
	str := func(text string) ast.ASTHandle { return pin(m.MkString(text)) }
	leftRange := pin(m.MkRegexRange(str("a"), str("m")))
	rightRange := pin(m.MkRegexRange(str("h"), str("z")))
	intersection := pin(m.MkRegexIntersect(leftRange, rightRange))
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	for _, tc := range []struct {
		input string
		want  ast.ASTHandle
	}{{"i", truth}, {"a", falsehood}, {"z", falsehood}} {
		status, e := rw.RewriteStringInRegex(str(tc.input), intersection, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		// BR_REWRITE_FULL: the rewriter folds the accepted range conditions.
		astTestMust(t, full.Rewrite(result, out))
		result, e = m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteFull || result != tc.want {
			t.Fatalf("range intersection %q: status=%d result=%v want=%v", tc.input, status, result, tc.want)
		}
	}
	// sym_expr_boolean_algebra::is_sat sends the intersected range
	// predicate to the expression solver; it does not decide it concretely.
	if solver.calls == 0 {
		t.Fatal("range intersection did not consult the solver")
	}
	toA := pin(m.MkSequenceToRegex(str("a")))
	complement := pin(m.MkRegexComplement(toA))
	for _, tc := range []struct {
		input string
		want  ast.ASTHandle
	}{{"a", falsehood}, {"", truth}, {"b", truth}, {"aa", truth}} {
		status, e := rw.RewriteStringInRegex(str(tc.input), complement, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		astTestMust(t, full.Rewrite(result, out))
		result, e = m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteFull || result != tc.want {
			t.Fatalf("complement %q: status=%d result=%v want=%v", tc.input, status, result, tc.want)
		}
	}

	byteZero := pin(m.BitVectorNumeralUnsigned(0, 8))
	byteSort, err := m.GetSort(byteZero)
	astTestMust(t, err)
	x := pin(m.MkConst(astTestSymbol(t, m, "intersection-x"), byteSort))
	y := pin(m.MkConst(astTestSymbol(t, m, "intersection-y"), byteSort))
	unitX := pin(m.MkSequenceUnit(x))
	unitY := pin(m.MkSequenceUnit(y))
	reX := pin(m.MkSequenceToRegex(unitX))
	reY := pin(m.MkSequenceToRegex(unitY))
	symbolicIntersection := pin(m.MkRegexIntersect(reX, reY))
	status, err := rw.RewriteStringInRegex(unitX, symbolicIntersection, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteFull || solver.calls == 0 {
		t.Fatalf("symbolic intersection: status=%d solver calls=%d result=%v", status, solver.calls, result)
	}

	solver.answer = lbool.Undef
	status, err = rw.RewriteStringInRegex(unitX, symbolicIntersection, out)
	astTestMust(t, err)
	if status != RewriteFailed {
		t.Fatalf("undefined symbolic intersection status: %d", status)
	}
}
