package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestStringRegexAutomatonOwnedGraphLabelsAndClose(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	word, err := m.MkString("\x00\xff")
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(word)
	astTestMust(t, err)
	input, err := m.NewRef(regex)
	astTestMust(t, err)
	owner, err := NewStringRegexAutomaton(m, regex, nil)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&input))
	if _, err := m.GetKind(regex); err != nil {
		t.Fatal("automaton owner did not keep its input/label ASTs alive", err)
	}
	graph, err := owner.Graph()
	astTestMust(t, err)
	graph.Compress()
	frontier := graph.EpsilonClosure(graph.Initial())
	for _, want := range []byte{0, 255} {
		var next []uint32
		for _, state := range frontier {
			for _, move := range graph.ExpandedMovesFrom(state, true) {
				condition, err := owner.Condition(move.Condition)
				astTestMust(t, err)
				value, constant, err := m.SequenceUnitByteElementValue(condition.Character)
				astTestMust(t, err)
				if condition.Kind != StringRegexCharacter || !constant || value != want {
					t.Fatal("borrowed byte label changed", condition.Kind, value, want)
				}
				next = append(next, move.Destination)
			}
		}
		frontier = next
	}
	accepts := false
	for _, state := range frontier {
		accepts = accepts || graph.IsFinal(state)
	}
	if !accepts {
		t.Fatal("converted owned graph lost the two-byte accepting path")
	}
	if _, err := owner.Condition(^uint32(0)); err == nil {
		t.Fatal("invalid label ID bypassed the condition-table guard")
	}
	astTestMust(t, owner.Close())
	astTestMust(t, owner.Close())
	if _, err := owner.Graph(); err == nil {
		t.Fatal("closed owner exposed a borrowed graph")
	}
	if _, err := owner.Condition(0); err == nil {
		t.Fatal("closed owner exposed a borrowed AST label")
	}
	if _, err := m.GetKind(regex); err == nil {
		t.Fatal("automaton close leaked its independently owned regex root")
	}
}

func TestStringRegexAutomatonRangeAndPredicateLabels(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	lo, err := m.MkString("a")
	astTestMust(t, err)
	hi, err := m.MkString("z")
	astTestMust(t, err)
	rangeRE, err := m.MkRegexRange(lo, hi)
	astTestMust(t, err)
	astTestPin(t, m, rangeRE)
	sort, err := m.GetSort(rangeRE)
	astTestMust(t, err)
	any, err := m.MkRegexFullChar(sort)
	astTestMust(t, err)
	astTestPin(t, m, any)
	for _, regex := range []ast.ASTHandle{rangeRE, any} {
		owner, err := NewStringRegexAutomaton(m, regex, nil)
		astTestMust(t, err)
		graph, err := owner.Graph()
		astTestMust(t, err)
		moves := graph.ExpandedMovesFrom(graph.Initial(), true)
		if len(moves) == 0 {
			t.Fatal("range/any constructor omitted its labeled move")
		}
		for _, move := range moves {
			condition, err := owner.Condition(move.Condition)
			astTestMust(t, err)
			if regex == rangeRE {
				lower, lc, err := m.SequenceUnitByteElementValue(condition.Lower)
				astTestMust(t, err)
				upper, uc, err := m.SequenceUnitByteElementValue(condition.Upper)
				astTestMust(t, err)
				if condition.Kind != StringRegexRange || !lc || !uc || lower != 'a' || upper != 'z' {
					t.Fatal("range bounds did not survive the concrete label table")
				}
			} else if condition.Kind != StringRegexPredicate || !m.IsTrue(condition.Predicate) {
				t.Fatal("full-character move did not retain the native true predicate")
			}
		}
		astTestMust(t, owner.Close())
	}
}

func TestStringRegexAutomatonComposedPredicates(t *testing.T) {
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
	rangeRE := pin(m.MkRegexRange(pin(m.MkString("a")), pin(m.MkString("z"))))
	intersection := pin(m.MkRegexIntersect(rangeRE, literal("m")))
	complement := pin(m.MkRegexComplement(literal("a")))
	for _, regex := range []ast.ASTHandle{intersection, complement} {
		owner, err := NewStringRegexAutomaton(m, regex, nil)
		astTestMust(t, err)
		graph, err := owner.Graph()
		astTestMust(t, err)
		graph.Compress()
		sort, err := m.BitVectorSort(8)
		astTestMust(t, err)
		variable := pin(m.MkVar(0, sort))
		predicateMoves := 0
		for _, value := range []byte{0, 'a', 'm', 'z', 255} {
			accepted := false
			for _, move := range graph.ExpandedMovesFrom(graph.Initial(), true) {
				condition, err := owner.Condition(move.Condition)
				astTestMust(t, err)
				// A composed label exposes a predicate AST. As in the
				// source, a minterm equal to one original character keeps
				// that character label.
				predicate := condition.Predicate
				switch condition.Kind {
				case StringRegexPredicate:
				case StringRegexCharacter:
					predicate = pin(m.MkEq(variable, condition.Character))
				default:
					t.Fatal("composed label did not expose a predicate or character")
				}
				predicateMoves++
				replace, err := NewExprSafeReplace(m)
				astTestMust(t, err)
				numeral := pin(m.BitVectorNumeralUnsigned(uint32(value), 8))
				astTestMust(t, replace.Insert(variable, numeral))
				out, err := m.NewRef(ast.ASTHandle{})
				astTestMust(t, err)
				astTestMust(t, replace.Apply(predicate, out))
				expression, err := m.RefGet(out)
				astTestMust(t, err)
				astTestMust(t, rw.Rewrite(expression, out))
				expression, err = m.RefGet(out)
				astTestMust(t, err)
				if !m.IsTrue(expression) && !m.IsFalse(expression) {
					t.Fatal("concrete composed byte predicate did not reduce to Boolean")
				}
				accepted = accepted || m.IsTrue(expression) && graph.IsFinal(move.Destination)
				astTestMust(t, m.DelRef(&out))
				astTestMust(t, replace.Close())
			}
			want := value == 'm'
			if regex == complement {
				want = value != 'a'
			}
			if accepted != want {
				t.Fatal("composed automaton predicate changed concrete byte acceptance", regex == complement, value, accepted, want)
			}
		}
		if predicateMoves == 0 {
			t.Fatal("composed predicate regression exercised no actual labeled move")
		}
		astTestMust(t, owner.Close())
	}
}

func TestStringRegexAutomatonRejectsNonStringRegex(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(zero)
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(unit)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	if owner, err := NewStringRegexAutomaton(m, regex, nil); err == nil || owner != nil {
		t.Fatal("concrete byte-string bridge accepted an integer-sequence regex")
	}
	word, err := m.MkString("a")
	astTestMust(t, err)
	astTestPin(t, m, word)
	if owner, err := NewStringRegexAutomaton(m, word, nil); err == nil || owner != nil {
		t.Fatal("concrete byte-string bridge accepted a non-regex input")
	}
	stringSort, err := m.GetSort(word)
	astTestMust(t, err)
	name, err := m.Symbols.Intern("unsupported-automaton-symbolic-word")
	astTestMust(t, err)
	symbolicWord, err := m.MkConst(name, stringSort)
	astTestMust(t, err)
	symbolicRegex, err := m.MkSequenceToRegex(symbolicWord)
	astTestMust(t, err)
	input, err := m.NewRef(symbolicRegex)
	astTestMust(t, err)
	if owner, err := NewStringRegexAutomaton(m, symbolicRegex, nil); err == nil || owner != nil {
		t.Fatal("unsupported symbolic construction returned an owning automaton")
	}
	astTestMust(t, m.DelRef(&input))
	if _, err := m.GetKind(symbolicRegex); err == nil {
		t.Fatal("constructor failure leaked its acquired input root")
	}
}
