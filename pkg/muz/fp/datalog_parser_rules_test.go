package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserFactsPositiveAndNegativeRules(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\n\nP(x:N)\nQ(x:N)\nP(\"a\").\nQ(X) :- P(X).\nP(X) :- !Q(X).\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	if count := f.ruleCount(t); count != 3 {
		t.Fatal("rule count", count)
	}
	rules, err := f.context.Rules()
	fpMust(t, err)
	rule, err := rules.Rule(2)
	fpMust(t, err)
	defer rule.Close()
	if size, e := rule.TailSize(); e != nil || size != 1 {
		t.Fatal(size, e)
	}
	_, negative, err := rule.Tail(0)
	fpMust(t, err)
	if !negative {
		t.Fatal("negative predicate polarity lost")
	}
}

func TestDatalogParserInfixEqualityAndOrdering(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "I int\n\nP(x:I)\nQ(x:I)\nQ(X) :- P(X), X < 3.\nP(X) :- Q(X), X != 2.\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	if count := f.ruleCount(t); count != 2 {
		t.Fatal(count)
	}
}

func TestDatalogParserInfixConstantLeftOperands(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\nI int\n\nP(x:N)\nQ(x:N)\nA(x:I)\nB(x:I)\nQ(X) :- P(X), \"a\" = X.\nQ(X) :- P(X), 3 != X.\nB(X) :- A(X), 2 < X.\nB(X) :- A(X), \"7\" > X.\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	rules, err := f.context.Rules()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	wantKinds := []int32{ast.ASTOpEq, ast.ASTOpNot, 4, 4} // arithmetic OP_LT is source kind 4
	wantLeftVariable := []bool{false, false, false, true}
	for i, want := range wantKinds {
		rule, err := rules.Rule(i)
		fpMust(t, err)
		tail, negative, err := rule.Tail(1)
		fpMust(t, err)
		if negative {
			t.Fatal("unexpected negated infix tail", i)
		}
		kind, err := manager.GetDeclKind(tail)
		fpMust(t, err)
		if kind != want {
			t.Fatalf("rule %d infix kind %v, want %v", i, kind, want)
		}
		comparison := tail
		if kind == ast.ASTOpNot {
			comparison, err = manager.GetArg(tail, 0)
			fpMust(t, err)
		}
		left, err := manager.GetArg(comparison, 0)
		fpMust(t, err)
		leftKind, err := manager.GetKind(left)
		fpMust(t, err)
		if (leftKind == ast.ASTVar) != wantLeftVariable[i] {
			t.Fatalf("rule %d left-variable orientation %v, want %v", i, leftKind == ast.ASTVar, wantLeftVariable[i])
		}
		fpMust(t, rule.Close())
	}
}

func TestDatalogParserInfixRequiresVariableOperand(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\n\nP(x:N)\nQ(x:N)\nQ(X) :- P(X), \"a\" = \"b\".\n"
	if err := fp.ParseDatalogString(f.context, input); err == nil {
		t.Fatal("constant-only infix relation accepted")
	}
}
