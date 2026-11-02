package rel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRelationManagerDisplaysFiniteExplanationAndMissingOutputs(t *testing.T) {
	f := newTableRelationFixture(t)
	context, err := base.NewDatalogContext(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, context.Close()) }()
	rules, err := context.Rules()
	astTestMust(t, err)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		predicate, e := f.ast.MkFuncDecl(sym, domain, f.boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestMust(t, context.RegisterPredicate(predicate, true))
		astTestMust(t, context.SetOutputPredicate(predicate))
		return predicate
	}
	finite := declare("display-output-finite", []ast.ASTHandle{f.boolean})
	missing := declare("display-output-missing", []ast.ASTHandle{f.boolean})
	explained := declare("display-output-explained", []ast.ASTHandle{f.boolean, ruleSort})
	argument, err := f.ast.Symbols.Intern("flag")
	astTestMust(t, err)
	astTestMust(t, context.SetArgumentNames(finite, []symbol.Symbol{argument}))
	table, err := f.relations.GetRelation(finite)
	astTestMust(t, err)
	_, err = table.Add(Fact{1})
	astTestMust(t, err)
	explanationName, err := f.ast.Symbols.Intern("display-output-proof")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(explanationName, ruleSort)
	astTestMust(t, err)
	product, err := f.relations.GetExplanationRelation(explained)
	astTestMust(t, err)
	astTestMust(t, product.AddFact([]ast.ASTHandle{f.truth}, explanation))

	var output bytes.Buffer
	astTestMust(t, f.relations.DisplayOutputTables(context, rules, &output))
	text := output.String()
	for _, fragment := range []string{
		"Tuples in display-output-finite: \n\t(flag=1(1))\n",
		"Tuples in display-output-missing: \n",
		"Tuples in display-output-explained: \nfinite_product_relation:\n",
		"display-output-proof",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("output tables missing %q: %q", fragment, text)
		}
	}
	refusal := errors.New("output table writer refusal")
	if err = f.relations.DisplayOutputTables(context, rules, productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("writer error = %v, want %v", err, refusal)
	}
	if err = f.relations.DisplayOutputTables(context, rules, nil); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("nil writer error = %v", err)
	}
	if _, found, err := f.relations.TryGetRelation(missing); err != nil || found {
		t.Fatalf("display created missing relation: %v %v", found, err)
	}
}
