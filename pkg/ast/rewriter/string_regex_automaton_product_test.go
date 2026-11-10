package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func stringAutomatonAcceptsWord(t *testing.T, m *ast.ASTManager, rw *TheoryRewriter, owner *StringRegexAutomaton, word string) bool {
	t.Helper()
	graph, err := owner.Graph()
	astTestMust(t, err)
	frontier := graph.EpsilonClosure(graph.Initial())
	for i := 0; i < len(word); i++ {
		var next []uint32
		for _, state := range frontier {
			for _, move := range graph.ExpandedMovesFrom(state, true) {
				condition, err := owner.Condition(move.Condition)
				astTestMust(t, err)
				matches := false
				switch condition.Kind {
				case StringRegexCharacter:
					value, constant, err := m.SequenceUnitByteElementValue(condition.Character)
					astTestMust(t, err)
					if !constant {
						t.Fatal("oracle requires a concrete character")
					}
					matches = value == word[i]
				case StringRegexRange:
					lower, lc, err := m.SequenceUnitByteElementValue(condition.Lower)
					astTestMust(t, err)
					upper, uc, err := m.SequenceUnitByteElementValue(condition.Upper)
					astTestMust(t, err)
					if !lc || !uc {
						t.Fatal("oracle requires concrete range endpoints")
					}
					matches = lower <= word[i] && word[i] <= upper
				case StringRegexPredicate:
					sort, err := m.BitVectorSort(8)
					astTestMust(t, err)
					variable, err := m.MkVar(0, sort)
					astTestMust(t, err)
					replace, err := NewExprSafeReplace(m)
					astTestMust(t, err)
					numeral, err := m.BitVectorNumeralUnsigned(uint32(word[i]), 8)
					astTestMust(t, err)
					astTestMust(t, replace.Insert(variable, numeral))
					out, err := m.NewRef(ast.ASTHandle{})
					astTestMust(t, err)
					astTestMust(t, replace.Apply(condition.Predicate, out))
					expression, err := m.RefGet(out)
					astTestMust(t, err)
					astTestMust(t, rw.Rewrite(expression, out))
					expression, err = m.RefGet(out)
					astTestMust(t, err)
					if !m.IsTrue(expression) && !m.IsFalse(expression) {
						t.Fatal("byte predicate failed to reduce to a Boolean")
					}
					matches = m.IsTrue(expression)
					astTestMust(t, m.DelRef(&out))
					astTestMust(t, replace.Close())
				default:
					t.Fatal("unknown condition kind")
				}
				if matches {
					next = append(next, move.Destination)
				}
			}
		}
		frontier = next
	}
	for _, state := range frontier {
		if graph.IsFinal(state) {
			return true
		}
	}
	return false
}

func TestStringRegexAutomatonProductIndependentLanguages(t *testing.T) {
	m, rw := theoryRewriterFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	literal := func(text string) ast.ASTHandle { return pin(m.MkSequenceToRegex(pin(m.MkString(text)))) }
	a, b, ab, empty := literal("a"), literal("b"), literal("ab"), literal("")
	for _, tc := range []struct {
		name        string
		left, right ast.ASTHandle
	}{
		{"epsilon-only", pin(m.MkRegexUnion(empty, a)), pin(m.MkRegexUnion(empty, b))},
		{"finite", pin(m.MkRegexUnion(a, ab)), pin(m.MkRegexStar(a))},
		{"cyclic", pin(m.MkRegexStar(pin(m.MkRegexUnion(a, b)))), pin(m.MkRegexPlus(a))},
		{"complement", pin(m.MkRegexComplement(a)), pin(m.MkRegexUnion(empty, pin(m.MkRegexUnion(a, pin(m.MkRegexUnion(b, ab))))))},
		{"range-bytes", pin(m.MkRegexRange(pin(m.MkString("\x00")), pin(m.MkString("\xff")))), pin(m.MkRegexUnion(literal("\x00"), literal("\xff")))},
		{"disjoint", a, b},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, err := NewStringRegexAutomaton(m, tc.left, &sequenceAutomatonByteSolver{m})
			astTestMust(t, err)
			right, err := NewStringRegexAutomaton(m, tc.right, &sequenceAutomatonByteSolver{m})
			astTestMust(t, err)
			for _, source := range []*StringRegexAutomaton{left, right} {
				graph, err := source.Graph()
				astTestMust(t, err)
				graph.Compress()
			}
			product, err := NewStringRegexAutomatonProduct(left, right, nil)
			astTestMust(t, err)
			astTestMust(t, left.Close())
			astTestMust(t, right.Close())
			graph, err := product.Graph()
			astTestMust(t, err)
			graph.Compress()
			words := []string{""}
			level := []string{""}
			for length := 0; length < 3; length++ {
				var next []string
				for _, prefix := range level {
					for _, value := range []byte{0, 'a', 'b', 255} {
						next = append(next, prefix+string([]byte{value}))
					}
				}
				words = append(words, next...)
				level = next
			}
			for _, word := range words {
				want := false
				switch tc.name {
				case "epsilon-only":
					want = word == ""
				case "finite":
					want = word == "a"
				case "cyclic":
					want = len(word) > 0
					for i := range word {
						want = want && word[i] == 'a'
					}
				case "complement":
					want = word == "" || word == "b" || word == "ab"
				case "range-bytes":
					want = word == "\x00" || word == "\xff"
				}
				if got := stringAutomatonAcceptsWord(t, m, rw, product, word); got != want {
					t.Fatalf("product language changed for %q: got %v want %v", word, got, want)
				}
			}
			astTestMust(t, product.Close())
		})
	}
}

func TestStringRegexAutomatonProductOwnsRootsAndUsesCurrentGraph(t *testing.T) {
	m, rw := theoryRewriterFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	word, err := m.MkString("a")
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(word)
	astTestMust(t, err)
	left, err := NewStringRegexAutomaton(m, regex, &sequenceAutomatonByteSolver{m})
	astTestMust(t, err)
	right, err := NewStringRegexAutomaton(m, regex, &sequenceAutomatonByteSolver{m})
	astTestMust(t, err)
	product, err := NewStringRegexAutomatonProduct(left, right, nil)
	astTestMust(t, err)
	astTestMust(t, left.Close())
	astTestMust(t, right.Close())
	if _, err := m.GetKind(regex); err != nil {
		t.Fatal("product lost its acquired input root", err)
	}
	if !stringAutomatonAcceptsWord(t, m, rw, product, "a") {
		t.Fatal("product lost labels after both inputs closed")
	}
	graph, err := product.Graph()
	astTestMust(t, err)
	graph.Compress()
	for _, state := range graph.FinalStates() {
		graph.RemoveFinal(state)
	}
	// A stale retained private construction graph would still accept "a".
	last, err := NewStringRegexAutomatonProduct(product, product, nil)
	astTestMust(t, err)
	astTestMust(t, product.Close())
	if stringAutomatonAcceptsWord(t, m, rw, last, "a") {
		t.Fatal("product ignored changes to the current borrowed graph")
	}
	astTestMust(t, last.Close())
	if _, err := m.GetKind(regex); err == nil {
		t.Fatal("product chain leaked its independently acquired input roots")
	}
}

func TestStringRegexAutomatonProductUnknownCleanupAndGuards(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	symbol, err := m.Symbols.Intern("product-symbolic-byte")
	astTestMust(t, err)
	x, err := m.MkConst(symbol, sort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(x)
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(unit)
	astTestMust(t, err)
	// Two different symbolic characters: their mk_and is a predicate that
	// is_sat must send to the (absent) solver, so the product is undefined.
	otherSymbol, err := m.Symbols.Intern("product-symbolic-byte-other")
	astTestMust(t, err)
	y, err := m.MkConst(otherSymbol, sort)
	astTestMust(t, err)
	otherUnit, err := m.MkSequenceUnit(y)
	astTestMust(t, err)
	secondRegex, err := m.MkSequenceToRegex(otherUnit)
	astTestMust(t, err)
	left, err := NewStringRegexAutomaton(m, regex, &sequenceAutomatonByteSolver{m})
	astTestMust(t, err)
	right, err := NewStringRegexAutomaton(m, secondRegex, nil)
	astTestMust(t, err)
	before, err := m.GetRefCount(regex)
	astTestMust(t, err)
	if product, err := NewStringRegexAutomatonProduct(left, right, nil); err == nil || product != nil {
		t.Fatal("undefined symbolic product returned an owning result")
	}
	after, err := m.GetRefCount(regex)
	astTestMust(t, err)
	if after != before {
		t.Fatal("failed product leaked acquired parent roots", before, after)
	}
	if _, err := NewStringRegexAutomatonProduct(nil, right, nil); err == nil {
		t.Fatal("nil left input accepted")
	}
	if _, err := NewStringRegexAutomatonProduct(left, nil, nil); err == nil {
		t.Fatal("nil right input accepted")
	}
	other := astTestManager(t)
	_, err = other.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = other.RegisterBitVectorPlugin()
	astTestMust(t, err)
	word, err := other.MkString("b")
	astTestMust(t, err)
	otherRegex, err := other.MkSequenceToRegex(word)
	astTestMust(t, err)
	foreign, err := NewStringRegexAutomaton(other, otherRegex, nil)
	astTestMust(t, err)
	if _, err := NewStringRegexAutomatonProduct(left, foreign, nil); err == nil {
		t.Fatal("cross-manager product accepted")
	}
	astTestMust(t, foreign.Close())
	astTestMust(t, right.Close())
	if _, err := NewStringRegexAutomatonProduct(left, right, nil); err == nil {
		t.Fatal("closed right input accepted")
	}
	astTestMust(t, left.Close())
	if _, err := NewStringRegexAutomatonProduct(left, left, nil); err == nil {
		t.Fatal("closed left input accepted")
	}
	if _, err := m.GetKind(regex); err == nil {
		t.Fatal("failed product retained roots after parents closed")
	}
}
