package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestIntervalSimilarityProgramBuildsArithmeticAuxiliaryWithoutSourceMutation(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	rules, err := base.NewRuleManager(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rules.Close()) })
	source, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, source.Close()) })
	declare := func(name string) ast.ASTHandle {
		declaration, err := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		astTestMust(t, rules.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("interval-similarity-P"), declare("interval-similarity-Q")
	for _, text := range []string{"0", "2", "4"} {
		value, err := m.ArithmeticIntegerNumeralText(text)
		astTestMust(t, err)
		head, err := m.MkApp(p, []ast.ASTHandle{value})
		astTestMust(t, err)
		rule, err := rules.NewRule(head, nil, nil, "interval-source-fact")
		astTestMust(t, err)
		astTestMust(t, source.Add(rule))
		astTestMust(t, rule.Close())
	}
	// Unchanged transformations must preserve source precedence: the source
	// compressor returns nullptr rather than publishing comparator order. This
	// order is observable by the subsequent widening/evaluation schedule.
	for _, unchanged := range []struct {
		threshold uint32
		outputs   []ast.ASTHandle
	}{{3, []ast.ASTHandle{q}}, {2, []ast.ASTHandle{p}}} {
		program, err := NewIntervalSimilarityProgram(m, source, unchanged.threshold, unchanged.outputs)
		astTestMust(t, err)
		if program.Modified() || len(program.AuxiliaryPredicates()) != 0 {
			t.Fatal("threshold/output protected program was changed")
		}
		unchangedRules, err := program.Rules()
		astTestMust(t, err)
		for index, want := range []string{"0", "2", "4"} {
			rule, err := unchangedRules.Rule(index)
			astTestMust(t, err)
			head, err := rule.Head()
			astTestMust(t, err)
			constant, err := m.GetArg(head, 0)
			astTestMust(t, err)
			text, _, matched, err := m.ArithmeticNumeralText(constant)
			astTestMust(t, err)
			if !matched || text != want {
				t.Fatal("unchanged interval program reordered source rules", index, text, want)
			}
			astTestMust(t, rule.Close())
		}
		astTestMust(t, program.Close())
	}
	program, err := NewIntervalSimilarityProgram(m, source, 2, []ast.ASTHandle{q})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, program.Close()) })
	auxiliaries := program.AuxiliaryPredicates()
	if !program.Modified() || len(auxiliaries) != 1 {
		t.Fatal("arithmetic similarity class was not compressed", program.Modified(), auxiliaries)
	}
	auxiliary := auxiliaries[0]
	if registered, err := rules.IsRegisteredPredicate(auxiliary); err != nil || registered {
		t.Fatal("private auxiliary escaped into the original rule manager", registered, err)
	}
	if program.origins[auxiliary] != p {
		t.Fatal("auxiliary lost its source head")
	}
	seed, found := program.AuxiliaryRelation(auxiliary)
	if !found {
		t.Fatal("missing arithmetic auxiliary facts")
	}
	lower, lowerOpen, upper, upperOpen, err := seed.Bounds(0)
	astTestMust(t, err)
	if lower == nil || upper == nil || lower.String() != "0/1" || upper.String() != "4/1" || lowerOpen || upperOpen {
		t.Fatal("auxiliary did not use source interval fact abstraction", lower, upper, lowerOpen, upperOpen)
	}
	transformed, err := program.Rules()
	astTestMust(t, err)
	if count, err := transformed.Len(); err != nil || count != 1 {
		t.Fatal("arithmetic class representative count", count, err)
	}
	astTestMust(t, source.Close())
	astTestMust(t, rules.Close())
	rule, err := transformed.Rule(0)
	astTestMust(t, err)
	head, err := rule.Head()
	astTestMust(t, err)
	variable, err := m.GetArg(head, 0)
	astTestMust(t, err)
	tail, negative, err := rule.Tail(0)
	astTestMust(t, err)
	argument, err := m.GetArg(tail, 0)
	astTestMust(t, err)
	declaration, err := m.GetDecl(tail)
	astTestMust(t, err)
	if variable != argument || declaration != auxiliary || negative {
		t.Fatal("arithmetic representative lost its shared auxiliary column")
	}
	astTestMust(t, rule.Close())
}
