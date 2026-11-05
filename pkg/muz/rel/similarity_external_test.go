package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestExternalSimilarityProgramBuildsAuxiliarySeedRows(t *testing.T) {
	m := astTestManager(t)
	rules, err := base.NewRuleManager(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	astTestMust(t, err)
	defer func() { astTestMust(t, set.Close()) }()

	domainName, err := m.Symbols.Intern("external-similarity-domain")
	astTestMust(t, err)
	domain, err := m.MkDatalogFiniteSort(domainName, 4)
	astTestMust(t, err)
	astTestPin(t, m, domain)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	predicate, err := m.MkFuncDecl(astTestSymbol(t, m, "external-similarity-p"), []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	astTestPin(t, m, predicate)
	astTestMust(t, rules.RegisterPredicate(predicate))

	want := make(map[ast.ASTHandle]bool)
	for i := uint64(0); i < 3; i++ {
		value, cause := m.MkDatalogNumeral(i, domain)
		astTestMust(t, cause)
		astTestPin(t, m, value)
		want[value] = true
		head, cause := m.MkApp(predicate, []ast.ASTHandle{value})
		astTestMust(t, cause)
		astTestPin(t, m, head)
		rule, cause := rules.NewRule(head, nil, nil, "external-similarity-seed")
		astTestMust(t, cause)
		astTestMust(t, set.Add(rule))
		astTestMust(t, rule.Close())
	}

	program, err := NewExternalSimilarityProgram(m, set, 2, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, program.Close()) }()
	if !program.Modified() {
		t.Fatal("similarity compressor did not merge the seed class")
	}
	auxiliary := program.AuxiliaryPredicates()
	if len(auxiliary) != 1 {
		t.Fatalf("auxiliary predicates = %d, want 1", len(auxiliary))
	}
	rows, found := program.SeedRows(auxiliary[0])
	if !found || len(rows) != len(want) {
		t.Fatalf("auxiliary seeds found=%t rows=%v", found, rows)
	}
	seen := make(map[ast.ASTHandle]bool)
	for _, row := range rows {
		if len(row) != 1 || !want[row[0]] || seen[row[0]] {
			t.Fatalf("unexpected auxiliary seed row %v", row)
		}
		seen[row[0]] = true
	}
	for value := range want {
		if !seen[value] {
			t.Fatalf("missing auxiliary seed for %v", value)
		}
	}
	transformed, err := program.Rules()
	astTestMust(t, err)
	count, err := transformed.Len()
	astTestMust(t, err)
	if count != 1 {
		t.Fatalf("transformed rule count = %d, want 1", count)
	}
}
