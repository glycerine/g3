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

// sequenceAutomatonByteSolver decides a label formula over its single fresh
// byte constant exactly, by evaluating it at all 256 values. It stands in for
// the SMT solver theory_seq gives re2automaton (symbolic products and
// complements need one, as m_sa does in the source).
type sequenceAutomatonByteSolver struct{ m *ast.ASTManager }

func (s *sequenceAutomatonByteSolver) CheckSatisfiable(f ast.ASTHandle) (lbool.LBool, error) {
	x, err := s.freshByte(f)
	if err != nil {
		return lbool.Undef, err
	}
	for v := uint64(0); v < 256; v++ {
		value, err := s.eval(f, x, v)
		if err != nil {
			return lbool.Undef, err
		}
		if value != 0 {
			return lbool.True, nil
		}
	}
	return lbool.False, nil
}

func (s *sequenceAutomatonByteSolver) freshByte(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.m
	n, err := m.GetNumArgs(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if n == 0 {
		if numeral, err := m.IsBitVectorNumeral(h); err != nil || numeral {
			return ast.ASTHandle{}, err
		}
		if bv, err := m.IsBitVector(h); err != nil || !bv {
			return ast.ASTHandle{}, err
		}
		return h, nil
	}
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(h, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if x, err := s.freshByte(arg); err != nil || !x.IsNull() {
			return x, err
		}
	}
	return ast.ASTHandle{}, nil
}

// eval returns the value of h with x = v; Booleans are 0 or 1.
func (s *sequenceAutomatonByteSolver) eval(h, x ast.ASTHandle, v uint64) (uint64, error) {
	m := s.m
	switch {
	case h == x:
		return v, nil
	case m.IsTrue(h):
		return 1, nil
	case m.IsFalse(h):
		return 0, nil
	}
	if value, _, matched, err := m.BitVectorNumeralBigInt(h); err != nil || matched {
		if err != nil {
			return 0, err
		}
		return value.Uint64(), nil
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return 0, err
	}
	args := make([]uint64, n)
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(h, i)
		if err != nil {
			return 0, err
		}
		if args[i], err = s.eval(arg, x, v); err != nil {
			return 0, err
		}
	}
	b := boolRewriter{manager: m}
	is := func(op int32) bool { ok, _ := b.isOperator(h, op); return ok }
	truth := func(c bool) uint64 {
		if c {
			return 1
		}
		return 0
	}
	switch {
	case is(ast.ASTOpAnd):
		for _, a := range args {
			if a == 0 {
				return 0, nil
			}
		}
		return 1, nil
	case is(ast.ASTOpOr):
		for _, a := range args {
			if a != 0 {
				return 1, nil
			}
		}
		return 0, nil
	case is(ast.ASTOpNot):
		return truth(args[0] == 0), nil
	case is(ast.ASTOpEq):
		return truth(args[0] == args[1]), nil
	}
	if ule, err := m.IsBitVectorOperation(h, 22); err != nil {
		return 0, err
	} else if ule {
		return truth(args[0] <= args[1]), nil
	}
	return 0, ast.ErrASTDomain
}
