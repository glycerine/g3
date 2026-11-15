package cmd_context

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogParserDeclarationsFactsRulesAndInfix(t *testing.T) {
	context := contextFixture(t)
	program := "D 3\n\nP(x:D)\nQ(x:D)\nP(\"a\").\nQ(X) :- P(X), X != \"b\".\n"
	contextMust(t, context.ParseDatalogString(program))
	q, err := context.FindFunctionDeclaration("Q", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	answer, err := context.QueryFixedpoint(q)
	contextMust(t, err)
	if answer != lbool.True {
		t.Fatal("parsed program query", answer)
	}
	engine, err := context.fixedpointContext()
	contextMust(t, err)
	rules, err := engine.Rules()
	contextMust(t, err)
	// The ground fact goes to context::add_fact, not to the rule set.
	if len(rules) != 1 {
		t.Fatalf("parsed rules %d, want 1", len(rules))
	}
	manager, err := context.Manager()
	contextMust(t, err)
	for i := range rules {
		err = errors.Join(err, manager.DelRef(&rules[i]))
	}
	contextMust(t, err)
}

func TestDatalogParserDomainSuffixWildcardAndBounds(t *testing.T) {
	context := contextFixture(t)
	contextMust(t, context.ParseDatalogString("Node12 2\n\nR(x:Node99)\nR(_).\n"))

	numeric := contextFixture(t)
	contextMust(t, numeric.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nQ(x:Tiny)\nT(999).\nQ(x) :- T(x), x = 999.\n"))
	declaration, err := numeric.FindFunctionDeclaration("Q", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	status, err := numeric.QueryFixedpoint(declaration)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("numeric symbolic finite fact", status)
	}

	overflow := contextFixture(t)
	if err := overflow.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nT(2).\nT(3).\n"); err == nil {
		t.Fatal("accepted two distinct numeric symbols in a one-element domain")
	}

	wide := contextFixture(t)
	if err := wide.ParseDatalogString("Tiny 1\n\nT(x:Tiny)\nT(18446744073709551616).\n"); err == nil {
		t.Fatal("accepted a finite-domain numeric token outside uint64")
	}
}

func TestDatalogParserCommentOnlyLinesDoNotEndDomains(t *testing.T) {
	context := contextFixture(t)
	input := "# header\nN 2\n# between domains\nM 3\n\nP(x:N,y:M)\nP(\"n\",\"m\").\n"
	contextMust(t, context.ParseDatalogString(input))
	declaration, err := context.FindFunctionDeclaration("P", nil, nil, ast.ASTHandle{})
	contextMust(t, err)
	manager, err := context.Manager()
	contextMust(t, err)
	for i, want := range []uint64{2, 3} {
		sort, err := manager.GetDomain(declaration, uint32(i))
		contextMust(t, err)
		size, found, err := manager.DatalogFiniteSortSize(sort)
		contextMust(t, err)
		if !found || size != want {
			t.Fatalf("domain %d size %d found %v, want %d", i, size, found, want)
		}
	}
}

// AUDIT.md cmd_context_01-F9: parse_domain returns the newline after `int`
// unconsumed, so parse_domains ends the domain section there and `W 5` is
// parsed as a rule.
func TestDatalogIntDomainEndsDomainSection(t *testing.T) {
	context := contextFixture(t)
	var printed bytes.Buffer
	ok, err := context.ParseDatalogStringTo("V int\nW 5\n\nedge(a:W, b:W)\n", &printed)
	contextMust(t, err)
	if want := "unexpected token at line 2 '5' found '<num>'\n\n"; ok || printed.String() != want {
		t.Fatalf("ok %v printed %q, want %q", ok, printed.String(), want)
	}
}

// The parser messages of dparser and the exceptions parse_stream catches.
func TestDatalogParserMessagesMatchCPP(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"V 10\n\nedge(a:V, b:V)\nedge(1,2)", "'.' expected at the end of rule at line 4 '' found '<eos>'\n\n"},
		{"V 0\n", "Domain size of sort 'V' may not be 0\n"},
		{"V 2\n\nedge(a:V)\nedge(1).\nedge(2).\nedge(3).\n", "sort V contains more constants than its declared size 2\n"},
		{"V 2\nV1 3\n", "sort V already declared\n"},
		{"V 2\n\nP(x:W)\n", "unknown sort \"W\"\n"},
		{"P\nQ\n", "numeral or 'int' at line 2 '' found '\\n'\n\n"},
		{"V 3\n\nP(x:V)\nQ(x:V)\nP(1).\nQ(X) :- P(X), R(y:V), X < Y.\n", "identifier at line 6 'Y' found '<id>'\n\n"},
		{"I int\n\nP(x:I)\nP(-3).\n", "integer expected at line 4 '-3' found '<num>'\n\n"},
		{"I int\n\nP(x:I)\nP(\"abc\").\n", "Invalid integer: \"abc\"\n"},
		{"V 3\nW 3\n\nP(x:V)\nQ(x:W)\nR(x:V)\nR(X) :- P(X), Q(X).\n", "sort: W expected, but got: V\n\n"},
		{"V 3\n\nP(x:V)\nP(1,2).\n", "too many arguments passed to predicate at line 4 '2' found '<num>'\n\n"},
		{"V 3\n\nP(x:V)\nP().\n", "too many arguments passed to predicate at line 4 '.' found '.'\n\n"},
		{"V 3\n\nP(x:V)\nP(X) :- !.\n", "expected predicate or relation at line 4 '.' found '.'\n\n"},
		// A string on the last line of a string input is read like any
		// other; a file's last line fails (see the -dl vectors).
		{"V 3\n\nP(x:V)\nP(\"a\n", "too few arguments passed to predicate at line 4 'a' found '<error>'\n\n"},
	} {
		context := contextFixture(t)
		var printed bytes.Buffer
		ok, err := context.ParseDatalogStringTo(tc.input, &printed)
		contextMust(t, err)
		if ok || printed.String() != tc.want {
			t.Errorf("%q: ok %v printed %q, want %q", tc.input, ok, printed.String(), tc.want)
		}
	}
}

// parse_pred declares a predicate first used in a rule body.
func TestDatalogParserBodyDeclaresPredicate(t *testing.T) {
	context := contextFixture(t)
	contextMust(t, context.ParseDatalogString("V 3\n\nP(x:V)\nQ(x:V)\nP(1).\nQ(X) :- P(X), R(y:V).\n"))
	if _, err := context.FindFunctionDeclaration("R", nil, nil, ast.ASTHandle{}); err != nil {
		t.Fatal("body predicate was not declared", err)
	}
}
