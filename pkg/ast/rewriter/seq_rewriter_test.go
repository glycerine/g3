package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSequenceRewriterConcatSourceBranches(t *testing.T) {
	m := astTestManager(t)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	str := func(value string) ast.ASTHandle {
		h, e := m.MkString(value)
		astTestMust(t, e)
		return h
	}
	a, b, empty := str("a"), str("b"), str("")
	astTestPin(t, m, a)
	astTestPin(t, m, b)
	astTestPin(t, m, empty)
	checkString := func(want string, status RewriteStatus) {
		t.Helper()
		if status != RewriteDone {
			t.Fatalf("status %d, want done", status)
		}
		h, e := m.RefGet(out)
		astTestMust(t, e)
		got, matched, e := m.SequenceStringValue(h)
		astTestMust(t, e)
		if !matched || got != want {
			t.Fatalf("string result: %q matched=%v, want %q", got, matched, want)
		}
	}
	status, err := rw.RewriteConcat(a, b, out)
	astTestMust(t, err)
	checkString("ab", status)

	status, err = rw.RewriteConcat(empty, b, out)
	astTestMust(t, err)
	checkString("b", status)
	status, err = rw.RewriteConcat(a, empty, out)
	astTestMust(t, err)
	checkString("a", status)

	stringSort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "seq-x"), stringSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	astTestPin(t, m, left)
	status, err = rw.RewriteConcat(left, b, out)
	astTestMust(t, err)
	if status != Rewrite2 {
		t.Fatalf("right association status: %d", status)
	}
	result, err := m.RefGet(out)
	astTestMust(t, err)
	leaves, err := m.SequenceFlattenConcat(result)
	astTestMust(t, err)
	if len(leaves) != 3 || leaves[0] != x || leaves[1] != a || leaves[2] != b {
		t.Fatalf("right-associated leaves: %v", leaves)
	}

	tail, err := m.MkSequenceConcat(b, x)
	astTestMust(t, err)
	astTestPin(t, m, tail)
	status, err = rw.RewriteConcat(a, tail, out)
	astTestMust(t, err)
	if status != RewriteDone {
		t.Fatalf("adjacent string fold status: %d", status)
	}
	result, err = m.RefGet(out)
	astTestMust(t, err)
	leaves, err = m.SequenceFlattenConcat(result)
	astTestMust(t, err)
	if len(leaves) != 2 || leaves[1] != x {
		t.Fatalf("adjacent string leaves: %v", leaves)
	}
	text, matched, err := m.SequenceStringValue(leaves[0])
	astTestMust(t, err)
	if !matched || text != "ab" {
		t.Fatalf("adjacent string prefix: %q matched=%v", text, matched)
	}

	status, err = rw.RewriteConcat(x, x, out)
	astTestMust(t, err)
	if status != RewriteFailed {
		t.Fatalf("unreduced concat status: %d", status)
	}
}

func TestSequenceRewriterByteUnit(t *testing.T) {
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
	for _, value := range []uint32{0, 65, 255} {
		literal, e := m.BitVectorNumeralUnsigned(value, 8)
		astTestMust(t, e)
		astTestPin(t, m, literal)
		status, e := rw.RewriteUnit(literal, out)
		astTestMust(t, e)
		if status != RewriteDone {
			t.Fatalf("byte %d status: %d", value, status)
		}
		result, e := m.RefGet(out)
		astTestMust(t, e)
		text, matched, e := m.SequenceStringValue(result)
		astTestMust(t, e)
		if !matched || text != string([]byte{byte(value)}) {
			t.Fatalf("byte %d result: %q matched=%v", value, text, matched)
		}
	}
	wide, err := m.BitVectorNumeralUnsigned(65, 16)
	astTestMust(t, err)
	astTestPin(t, m, wide)
	status, err := rw.RewriteUnit(wide, out)
	astTestMust(t, err)
	if status != RewriteFailed {
		t.Fatalf("wide unit status: %d", status)
	}
}

func TestSequenceRewriterConstantPredicates(t *testing.T) {
	m := astTestManager(t)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	values := map[string]ast.ASTHandle{}
	for _, text := range []string{"", "ab", "abc", "bc", "z"} {
		values[text], err = m.MkString(text)
		astTestMust(t, err)
		astTestPin(t, m, values[text])
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	check := func(name string, rewrite func(ast.ASTHandle, ast.ASTHandle, ast.ASTRef) (RewriteStatus, error), a, b ast.ASTHandle, want ast.ASTHandle) {
		t.Helper()
		status, e := rewrite(a, b, out)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteDone || got != want {
			t.Fatalf("%s: status=%d result=%v want=%v", name, status, got, want)
		}
	}
	check("contains", rw.RewriteContains, values["abc"], values["bc"], truth)
	check("not-contains", rw.RewriteContains, values["ab"], values["bc"], falsehood)
	check("prefix", rw.RewritePrefix, values["ab"], values["abc"], truth)
	check("not-prefix", rw.RewritePrefix, values["abc"], values["ab"], falsehood)
	check("empty-prefix", rw.RewritePrefix, values[""], values["z"], truth)
	check("suffix", rw.RewriteSuffix, values["bc"], values["abc"], truth)
	check("not-suffix", rw.RewriteSuffix, values["abc"], values["bc"], falsehood)
	check("identity-suffix", rw.RewriteSuffix, values["ab"], values["ab"], truth)
	status, err := rw.RewriteSuffix(values["ab"], values[""], out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite3 || kind != ast.ASTOpEq {
		t.Fatalf("empty target suffix: status=%d kind=%d error=%v", status, kind, e)
	}
}

func TestSequenceRewriterPrefixSuffixConcatReductions(t *testing.T) {
	m := astTestManager(t)
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
	concat := func(left, right ast.ASTHandle) ast.ASTHandle {
		return pin(m.MkSequenceConcat(left, right))
	}
	stringSort, err := m.GetSort(str("sort witness"))
	astTestMust(t, err)
	constant := func(name string) ast.ASTHandle {
		return pin(m.MkConst(astTestSymbol(t, m, name), stringSort))
	}
	x, y, z := constant("predicate-x"), constant("predicate-y"), constant("predicate-z")

	status, err := rw.RewritePrefix(concat(str("ab"), x), concat(str("abc"), y), out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	left, right, matched, err := m.SequenceBinaryOperationArguments(result, 3)
	astTestMust(t, err)
	if status != RewriteFull || !matched || left != x {
		t.Fatalf("split prefix: status=%d matched=%v left=%v", status, matched, left)
	}
	parts, err := m.SequenceFlattenConcat(right)
	astTestMust(t, err)
	if len(parts) != 2 || parts[1] != y {
		t.Fatalf("split prefix right parts: %v", parts)
	}
	text, stringValue, err := m.SequenceStringValue(parts[0])
	astTestMust(t, err)
	if !stringValue || text != "c" {
		t.Fatalf("split prefix remainder: %q matched=%v", text, stringValue)
	}

	status, err = rw.RewritePrefix(concat(x, y), concat(x, z), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	left, right, matched, err = m.SequenceBinaryOperationArguments(result, 3)
	astTestMust(t, err)
	if status != RewriteDone || !matched || left != y || right != z {
		t.Fatalf("shared prefix: status=%d matched=%v left=%v right=%v", status, matched, left, right)
	}

	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	trueUnit := pin(m.MkSequenceUnit(truth))
	falseUnit := pin(m.MkSequenceUnit(falsehood))
	status, err = rw.RewritePrefix(trueUnit, falseUnit, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite3 || kind != ast.ASTOpEq {
		t.Fatalf("unit prefix equality: status=%d kind=%d error=%v", status, kind, e)
	}

	status, err = rw.RewriteSuffix(concat(x, str("bc")), concat(y, str("abc")), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	left, right, matched, err = m.SequenceBinaryOperationArguments(result, 4)
	astTestMust(t, err)
	if status != RewriteDone || !matched || left != x {
		t.Fatalf("split suffix: status=%d matched=%v left=%v", status, matched, left)
	}
	parts, err = m.SequenceFlattenConcat(right)
	astTestMust(t, err)
	if len(parts) != 2 || parts[0] != y {
		t.Fatalf("split suffix right parts: %v", parts)
	}
	text, stringValue, err = m.SequenceStringValue(parts[1])
	astTestMust(t, err)
	if !stringValue || text != "a" {
		t.Fatalf("split suffix remainder: %q matched=%v", text, stringValue)
	}

	tail := constant("predicate-tail")
	status, err = rw.RewriteSuffix(concat(x, tail), concat(y, tail), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	left, right, matched, err = m.SequenceBinaryOperationArguments(result, 4)
	astTestMust(t, err)
	if status != RewriteDone || !matched || left != x || right != y {
		t.Fatalf("shared suffix: status=%d matched=%v left=%v right=%v", status, matched, left, right)
	}

	status, err = rw.RewriteSuffix(concat(x, tail), tail, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite3 || kind != ast.ASTOpEq {
		t.Fatalf("suffix empty residual: status=%d kind=%d error=%v", status, kind, e)
	}
	status, err = rw.RewriteSuffix(trueUnit, falseUnit, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != falsehood {
		t.Fatalf("distinct value suffix: status=%d result=%v", status, result)
	}
}

func TestSequenceRewriterContainsConcatReductions(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
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
	concat := func(left, right ast.ASTHandle) ast.ASTHandle {
		return pin(m.MkSequenceConcat(left, right))
	}
	stringSort, err := m.GetSort(str("contains sort witness"))
	astTestMust(t, err)
	stringConstant := func(name string) ast.ASTHandle {
		return pin(m.MkConst(astTestSymbol(t, m, name), stringSort))
	}
	x, y, z := stringConstant("contains-x"), stringConstant("contains-y"), stringConstant("contains-z")
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)

	status, err := rw.RewriteContains(concat(x, y), y, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != truth {
		t.Fatalf("exact subsequence: status=%d result=%v", status, result)
	}

	status, err = rw.RewriteContains(concat(str("a"), str("b")), concat(str("b"), str("a")), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != falsehood {
		t.Fatalf("all-value miss: status=%d result=%v", status, result)
	}

	status, err = rw.RewriteContains(concat(x, str("abc")), str("b"), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != truth {
		t.Fatalf("contained string segment: status=%d result=%v", status, result)
	}

	empty := str("")
	status, err = rw.RewriteContains(empty, x, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite2 || kind != ast.ASTOpEq {
		t.Fatalf("empty base: status=%d kind=%d error=%v", status, kind, e)
	}

	needle := concat(str("q"), z)
	status, err = rw.RewriteContains(concat(str("x"), y), needle, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	base, retainedNeedle, matched, err := m.SequenceBinaryOperationArguments(result, 5)
	astTestMust(t, err)
	if status != Rewrite2 || !matched || base != y || retainedNeedle != needle {
		t.Fatalf("trimmed base: status=%d matched=%v base=%v needle=%v", status, matched, base, retainedNeedle)
	}

	zero := pin(rw.makeSmallInteger(0))
	one := pin(rw.makeSmallInteger(1))
	extract := pin(m.MkSequenceExtract(x, zero, one))
	status, err = rw.RewriteContains(x, extract, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != truth {
		t.Fatalf("self extract: status=%d result=%v", status, result)
	}

	boolSort, err := m.MkBoolSort()
	astTestMust(t, err)
	p := pin(m.MkConst(astTestSymbol(t, m, "contains-p"), boolSort))
	q := pin(m.MkConst(astTestSymbol(t, m, "contains-q"), boolSort))
	r := pin(m.MkConst(astTestSymbol(t, m, "contains-r"), boolSort))
	s := pin(m.MkConst(astTestSymbol(t, m, "contains-s"), boolSort))
	unitP := pin(m.MkSequenceUnit(p))
	unitQ := pin(m.MkSequenceUnit(q))
	unitR := pin(m.MkSequenceUnit(r))
	unitS := pin(m.MkSequenceUnit(s))
	units := concat(unitP, concat(unitQ, concat(unitR, unitS)))
	status, err = rw.RewriteContains(units, concat(unitP, unitS), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != RewriteFull || kind != ast.ASTOpOr {
		t.Fatalf("unit expansion: status=%d kind=%d error=%v", status, kind, e)
	}
	status, err = rw.RewriteContains(unitP, concat(unitQ, unitR), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != falsehood {
		t.Fatalf("minimum length: status=%d result=%v", status, result)
	}
}

func TestSequenceRewriterReplaceSourceBranches(t *testing.T) {
	m := astTestManager(t)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	str := func(text string) ast.ASTHandle {
		h, e := m.MkString(text)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	base, pattern, replacement := str("ababa"), str("ba"), str("X")
	status, err := rw.RewriteReplace(base, pattern, replacement, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	text, matched, err := m.SequenceStringValue(result)
	astTestMust(t, err)
	if status != RewriteDone || !matched || text != "aXba" {
		t.Fatalf("constant replace: status=%d text=%q matched=%v", status, text, matched)
	}

	status, err = rw.RewriteReplace(base, pattern, pattern, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != base {
		t.Fatalf("identity replace: status=%d result=%v", status, result)
	}

	empty := str("")
	stringSort, err := m.GetSort(base)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "replace-x"), stringSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	status, err = rw.RewriteReplace(base, empty, x, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	left, right, concat, err := m.SequenceConcatParts(result)
	astTestMust(t, err)
	if status != Rewrite1 || !concat || left != x || right != base {
		t.Fatalf("empty-pattern replace: status=%d concat=%v left=%v right=%v", status, concat, left, right)
	}
	status, err = rw.RewriteReplace(empty, pattern, x, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != empty {
		t.Fatalf("empty-base replace: status=%d result=%v", status, result)
	}
}

func TestSequenceRewriterConstantLengthExtractAtAndIndex(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	str := func(text string) ast.ASTHandle {
		h, e := m.MkString(text)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	num := func(text string) ast.ASTHandle { return arithRewriterNumeral(t, m, text, true) }
	readInteger := func(want string, status RewriteStatus) {
		t.Helper()
		if status != RewriteDone {
			t.Fatalf("integer status: %d", status)
		}
		h, e := m.RefGet(out)
		astTestMust(t, e)
		text, integer, matched, e := m.ArithmeticNumeralText(h)
		astTestMust(t, e)
		if !matched || !integer || text != want {
			t.Fatalf("integer result: %q integer=%v matched=%v want=%q", text, integer, matched, want)
		}
	}
	readString := func(want string, status RewriteStatus) {
		t.Helper()
		if status != RewriteDone {
			t.Fatalf("string status: %d", status)
		}
		h, e := m.RefGet(out)
		astTestMust(t, e)
		text, matched, e := m.SequenceStringValue(h)
		astTestMust(t, e)
		if !matched || text != want {
			t.Fatalf("string result: %q matched=%v want=%q", text, matched, want)
		}
	}
	base, needle := str("ababa"), str("ba")
	status, err := rw.RewriteLength(base, out)
	astTestMust(t, err)
	readInteger("5", status)

	status, err = rw.RewriteExtract(base, num("1"), num("20"), out)
	astTestMust(t, err)
	readString("baba", status)
	status, err = rw.RewriteExtract(base, num("-1"), num("2"), out)
	astTestMust(t, err)
	readString("", status)
	status, err = rw.RewriteAt(base, num("2"), out)
	astTestMust(t, err)
	readString("a", status)
	status, err = rw.RewriteAt(base, num("999999999999999999999999"), out)
	astTestMust(t, err)
	readString("", status)
	status, err = rw.RewriteIndex(base, needle, num("0"), out)
	astTestMust(t, err)
	readInteger("1", status)
	status, err = rw.RewriteIndex(base, needle, num("2"), out)
	astTestMust(t, err)
	readInteger("3", status)
	status, err = rw.RewriteIndex(base, str("z"), num("0"), out)
	astTestMust(t, err)
	readInteger("-1", status)
	status, err = rw.RewriteIndex(base, needle, num("-2"), out)
	astTestMust(t, err)
	readInteger("-1", status)

	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	byteValue, err := m.BitVectorNumeralUnsigned(65, 8)
	astTestMust(t, err)
	astTestPin(t, m, byteValue)
	unit, err := m.MkSequenceUnit(byteValue)
	astTestMust(t, err)
	astTestPin(t, m, unit)
	concat, err := m.MkSequenceConcat(str("xy"), unit)
	astTestMust(t, err)
	astTestPin(t, m, concat)
	status, err = rw.RewriteLength(concat, out)
	astTestMust(t, err)
	readInteger("3", status)

	stringSort, err := m.GetSort(base)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "length-x"), stringSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	symbolicConcat, err := m.MkSequenceConcat(x, str("pq"))
	astTestMust(t, err)
	astTestPin(t, m, symbolicConcat)
	status, err = rw.RewriteLength(symbolicConcat, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite2 || kind != 6 {
		t.Fatalf("symbolic length sum: status=%d kind=%d error=%v", status, kind, e)
	}

	units := make([]ast.ASTHandle, 3)
	for i, value := range []uint32{65, 66, 67} {
		literal, e := m.BitVectorNumeralUnsigned(value, 8)
		astTestMust(t, e)
		astTestPin(t, m, literal)
		units[i], e = m.MkSequenceUnit(literal)
		astTestMust(t, e)
		astTestPin(t, m, units[i])
	}
	tail, err := m.MkSequenceConcat(units[1], units[2])
	astTestMust(t, err)
	astTestPin(t, m, tail)
	unitSequence, err := m.MkSequenceConcat(units[0], tail)
	astTestMust(t, err)
	astTestPin(t, m, unitSequence)
	status, err = rw.RewriteExtract(unitSequence, num("1"), num("2"), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	parts, err := m.SequenceFlattenConcat(result)
	astTestMust(t, err)
	if status != RewriteDone || len(parts) != 2 || parts[0] != units[1] || parts[1] != units[2] {
		t.Fatalf("unit extraction: status=%d parts=%v", status, parts)
	}
	status, err = rw.RewriteAt(unitSequence, num("1"), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != units[1] {
		t.Fatalf("unit at: status=%d result=%v", status, result)
	}
	// AUDIT.md ast_rewriter_05-F2: mk_seq_at bounds a only through
	// min_length(1, &a, len), so a concatenation of units is unbounded and
	// an index past its end is left alone.
	status, err = rw.RewriteAt(tail, num("5"), out)
	astTestMust(t, err)
	if status != RewriteFailed {
		t.Fatalf("out-of-range at over a unit concat: status=%d", status)
	}
	status, err = rw.RewriteAt(units[0], num("1"), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if empty, e := m.IsSequenceEmpty(result); e != nil || status != RewriteDone || !empty {
		t.Fatalf("out-of-range at over a single unit: status=%d empty=%v error=%v", status, empty, e)
	}
}

func TestSequenceRewriterIntegerStringConversions(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	checkString := func(input ast.ASTHandle, want string) {
		t.Helper()
		status, e := rw.RewriteIntToString(input, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		text, matched, e := m.SequenceStringValue(result)
		astTestMust(t, e)
		if status != RewriteDone || !matched || text != want {
			t.Fatalf("int.to.str: status=%d text=%q matched=%v want=%q", status, text, matched, want)
		}
	}
	checkString(arithRewriterNumeral(t, m, "123456789012345678901234567890", true), "123456789012345678901234567890")
	checkString(arithRewriterNumeral(t, m, "12", false), "12")
	checkString(arithRewriterNumeral(t, m, "-1", true), "")
	checkString(arithRewriterNumeral(t, m, "1/2", false), "")

	checkInteger := func(text, want string) {
		t.Helper()
		value, e := m.MkString(text)
		astTestMust(t, e)
		astTestPin(t, m, value)
		status, e := rw.RewriteStringToInt(value, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		got, integer, matched, e := m.ArithmeticNumeralText(result)
		astTestMust(t, e)
		if status != RewriteDone || !matched || !integer || got != want {
			t.Fatalf("str.to.int %q: status=%d got=%q integer=%v matched=%v want=%q", text, status, got, integer, matched, want)
		}
	}
	checkInteger("987654321098765432109876543210", "987654321098765432109876543210")
	checkInteger("", "-1")
	checkInteger("12x", "-1")
	checkInteger("\xff", "-1")

	zero := arithRewriterNumeral(t, m, "0", true)
	intSort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "stoi-x"), intSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	itos, err := m.MkIntToString(x)
	astTestMust(t, err)
	astTestPin(t, m, itos)
	status, err := rw.RewriteStringToInt(itos, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != RewriteDone || kind != ast.ASTOpIte {
		t.Fatalf("stoi(itos(x)): status=%d kind=%d error=%v", status, kind, e)
	}
}

func TestSequenceRewriterRegexAlgebraicBranches(t *testing.T) {
	m := astTestManager(t)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	a, err := m.MkString("a")
	astTestMust(t, err)
	astTestPin(t, m, a)
	emptyString, err := m.MkString("")
	astTestMust(t, err)
	astTestPin(t, m, emptyString)
	ra, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	astTestPin(t, m, ra)
	b, err := m.MkString("b")
	astTestMust(t, err)
	astTestPin(t, m, b)
	rb, err := m.MkSequenceToRegex(b)
	astTestMust(t, err)
	astTestPin(t, m, rb)
	epsilon, err := m.MkSequenceToRegex(emptyString)
	astTestMust(t, err)
	astTestPin(t, m, epsilon)
	regexSort, err := m.GetSort(ra)
	astTestMust(t, err)
	empty, err := m.MkRegexEmpty(regexSort)
	astTestMust(t, err)
	astTestPin(t, m, empty)
	full, err := m.MkRegexFull(regexSort)
	astTestMust(t, err)
	astTestPin(t, m, full)
	check := func(name string, call func(ast.ASTHandle, ast.ASTHandle, ast.ASTRef) (RewriteStatus, error), left, right, want ast.ASTHandle) {
		t.Helper()
		status, e := call(left, right, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteDone || result != want {
			t.Fatalf("%s: status=%d result=%v want=%v", name, status, result, want)
		}
	}
	check("concat-left-empty", rw.RewriteRegexConcat, empty, ra, empty)
	check("concat-epsilon", rw.RewriteRegexConcat, epsilon, ra, ra)
	check("union-empty", rw.RewriteRegexUnion, empty, ra, ra)
	check("union-full", rw.RewriteRegexUnion, ra, full, full)
	check("intersection-full", rw.RewriteRegexIntersect, full, ra, ra)
	check("intersection-empty", rw.RewriteRegexIntersect, ra, empty, empty)

	status, err := rw.RewriteRegexPlus(ra, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	if status != Rewrite2 {
		t.Fatalf("regex plus expansion status: %d", status)
	}
	if yes, e := m.IsSequenceOperation(result, 17); e != nil || !yes {
		t.Fatalf("regex plus expansion: concat=%v error=%v", yes, e)
	}
	star, err := m.MkRegexStar(ra)
	astTestMust(t, err)
	astTestPin(t, m, star)
	status, err = rw.RewriteRegexPlus(star, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != RewriteDone || result != star {
		t.Fatalf("star plus identity: status=%d result=%v", status, result)
	}
	status, err = rw.RewriteRegexOptional(ra, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if status != Rewrite1 {
		t.Fatalf("regex optional status: %d", status)
	}
	if yes, e := m.IsSequenceOperation(result, 18); e != nil || !yes {
		t.Fatalf("regex optional: union=%v error=%v", yes, e)
	}
	status, err = rw.RewriteRegexComplement(empty, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, e := m.IsSequenceOperation(result, 23); e != nil || status != RewriteDone || !yes {
		t.Fatalf("complement empty: status=%d full=%v error=%v", status, yes, e)
	}
	intersection, err := m.MkRegexIntersect(ra, rb)
	astTestMust(t, err)
	astTestPin(t, m, intersection)
	status, err = rw.RewriteRegexComplement(intersection, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, e := m.IsSequenceOperation(result, 18); e != nil || status != Rewrite2 || !yes {
		t.Fatalf("De Morgan complement: status=%d union=%v error=%v", status, yes, e)
	}
	status, err = rw.RewriteRegexStar(empty, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, e := m.IsRegexEpsilon(result); e != nil || status != RewriteDone || !yes {
		t.Fatalf("star empty: status=%d epsilon=%v error=%v", status, yes, e)
	}
	plus, err := m.MkRegexPlus(ra)
	astTestMust(t, err)
	astTestPin(t, m, plus)
	status, err = rw.RewriteRegexStar(plus, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, e := m.IsSequenceOperation(result, 14); e != nil || status != RewriteDone || !yes {
		t.Fatalf("star plus: status=%d star=%v error=%v", status, yes, e)
	}
	starA, err := m.MkRegexStar(ra)
	astTestMust(t, err)
	astTestPin(t, m, starA)
	union, err := m.MkRegexUnion(starA, rb)
	astTestMust(t, err)
	astTestPin(t, m, union)
	status, err = rw.RewriteRegexStar(union, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if yes, e := m.IsSequenceOperation(result, 14); e != nil || status != Rewrite2 || !yes {
		t.Fatalf("star union: status=%d star=%v error=%v", status, yes, e)
	}
}

func TestSequenceRewriterRegexLoopAndMembershipGuards(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	text, err := m.MkString("x")
	astTestMust(t, err)
	astTestPin(t, m, text)
	regex, err := m.MkSequenceToRegex(text)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	lower := arithRewriterNumeral(t, m, "2", true)
	upper := arithRewriterNumeral(t, m, "5", true)
	for _, args := range [][]ast.ASTHandle{{regex, lower}, {regex, lower, upper}} {
		status, e := rw.RewriteRegexLoop(args, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		body, lo, hi, bounded, matched, e := m.RegexLoopParameters(result)
		astTestMust(t, e)
		if status != RewriteDone || !matched || body != regex || lo != 2 || bounded != (len(args) == 3) || bounded && hi != 5 {
			t.Fatalf("regex loop: status=%d body=%v lo=%d hi=%d bounded=%v matched=%v", status, body, lo, hi, bounded, matched)
		}
	}
	regexSort, err := m.GetSort(regex)
	astTestMust(t, err)
	empty, err := m.MkRegexEmpty(regexSort)
	astTestMust(t, err)
	astTestPin(t, m, empty)
	full, err := m.MkRegexFull(regexSort)
	astTestMust(t, err)
	astTestPin(t, m, full)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsehood, err := m.MkFalse()
	astTestMust(t, err)
	for _, tc := range []struct {
		regex, want ast.ASTHandle
	}{
		{empty, falsehood},
		{full, truth},
	} {
		status, e := rw.RewriteStringInRegex(text, tc.regex, out)
		astTestMust(t, e)
		result, e := m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteDone || result != tc.want {
			t.Fatalf("membership guard: status=%d result=%v want=%v", status, result, tc.want)
		}
	}
}

func TestSequenceRewriterDeclarationDispatch(t *testing.T) {
	m := astTestManager(t)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	a, err := m.MkString("a")
	astTestMust(t, err)
	astTestPin(t, m, a)
	b, err := m.MkString("b")
	astTestMust(t, err)
	astTestPin(t, m, b)
	concat, err := m.MkSequenceConcat(a, b)
	astTestMust(t, err)
	astTestPin(t, m, concat)
	decl, err := m.GetDecl(concat)
	astTestMust(t, err)
	status, err := rw.RewriteApp(decl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	text, matched, err := m.SequenceStringValue(result)
	astTestMust(t, err)
	if status != RewriteDone || !matched || text != "ab" {
		t.Fatalf("concat dispatch: status=%d text=%q matched=%v", status, text, matched)
	}

	indexDecl, err := m.MkFamilyFuncDecl(rw.family, 10, nil, func() []ast.ASTHandle {
		sort, e := m.GetSort(a)
		astTestMust(t, e)
		return []ast.ASTHandle{sort, sort}
	}(), ast.ASTHandle{})
	astTestMust(t, err)
	status, err = rw.RewriteApp(indexDecl, ast.SliceArguments([]ast.ASTHandle{a, b}), out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if kind, e := m.GetDeclKind(result); e != nil || status != Rewrite1 || kind != 10 {
		t.Fatalf("two-argument index dispatch: status=%d kind=%d error=%v", status, kind, e)
	}

	// AUDIT.md ast_rewriter_05-F1: mk_app_core returns the sole argument
	// of a unary re.++ / re.union.
	toRe, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	astTestPin(t, m, toRe)
	for name, build := range map[string]func(l, r ast.ASTHandle) (ast.ASTHandle, error){
		"re.++":    m.MkRegexConcat,
		"re.union": m.MkRegexUnion,
	} {
		binary, e := build(toRe, toRe)
		astTestMust(t, e)
		astTestPin(t, m, binary)
		regexDecl, e := m.GetDecl(binary)
		astTestMust(t, e)
		status, err = rw.RewriteApp(regexDecl, ast.SliceArguments([]ast.ASTHandle{toRe}), out)
		astTestMust(t, err)
		result, err = m.RefGet(out)
		astTestMust(t, err)
		if status != RewriteDone || result != toRe {
			t.Fatalf("unary %s dispatch: status=%d result=%v want %v", name, status, result, toRe)
		}
	}

	truth, err := m.MkTrue()
	astTestMust(t, err)
	truthDecl, err := m.GetDecl(truth)
	astTestMust(t, err)
	status, err = rw.RewriteApp(truthDecl, ast.SliceArguments(nil), out)
	astTestMust(t, err)
	if status != RewriteFailed {
		t.Fatalf("foreign declaration status: %d", status)
	}
}

// GCC evaluates nested call arguments right to left: str.to_int(int.to.str
// x) creates -1 before 0, and complement(inter(r1, r2)) creates
// complement(r2) before complement(r1).
func TestSequenceRewriterConstructionOrder(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	rw, err := NewSequenceRewriter(m)
	astTestMust(t, err)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	id := func(h ast.ASTHandle, path ...uint32) uint32 {
		t.Helper()
		for _, i := range path {
			var e error
			h, e = m.GetArg(h, i)
			astTestMust(t, e)
		}
		v, e := m.GetASTID(h)
		astTestMust(t, e)
		return v
	}
	intSort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "order-stoi-x"), intSort)
	astTestMust(t, err)
	astTestPin(t, m, x)
	itos, err := m.MkIntToString(x)
	astTestMust(t, err)
	astTestPin(t, m, itos)
	_, err = rw.RewriteStringToInt(itos, out)
	astTestMust(t, err)
	result, err := m.RefGet(out)
	astTestMust(t, err)
	astTestPin(t, m, result)
	if minusOne, zero := id(result, 2), id(result, 0, 1); minusOne >= zero {
		t.Fatal("stoi: -1 must be created before 0", minusOne, zero)
	}

	c, err := m.MkString("order-c")
	astTestMust(t, err)
	d, err := m.MkString("order-d")
	astTestMust(t, err)
	rc, err := m.MkSequenceToRegex(c)
	astTestMust(t, err)
	rd, err := m.MkSequenceToRegex(d)
	astTestMust(t, err)
	intersection, err := m.MkRegexIntersect(rc, rd)
	astTestMust(t, err)
	astTestPin(t, m, intersection)
	_, err = rw.RewriteRegexComplement(intersection, out)
	astTestMust(t, err)
	result, err = m.RefGet(out)
	astTestMust(t, err)
	if left, right := id(result, 0), id(result, 1); right >= left {
		t.Fatal("complement: right complement must be created first", left, right)
	}
}
