package fp_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// Source parse_body checks for period at the start of every iteration,
// including immediately after ':-' or a comma.
func TestDatalogParserPeriodTerminatesEmptyAndTrailingCommaBodies(t *testing.T) {
	f := newDatalogParserFixture(t)
	err := fp.ParseDatalogString(f.context, "\nP\nQ\nP :- .\nP :- Q, .\n")
	fpMust(t, err)
	rules, err := f.context.Rules()
	fpMust(t, err)
	count, err := rules.Len()
	fpMust(t, err)
	if count != 2 {
		t.Fatal("rule count", count)
	}
	for i, want := range []int{0, 1} {
		rule, e := rules.Rule(i)
		fpMust(t, e)
		size, e := rule.TailSize()
		fpMust(t, e)
		fpMust(t, rule.Close())
		if size != want {
			t.Fatal("tail size", i, size, want)
		}
	}
}

func TestDatalogParserBodyReferenceFailureRetiresHeadAndRetries(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "\nP\nQ\n"))
	manager, err := f.context.Manager()
	fpMust(t, err)
	var padding []ast.ASTRef
	defer func() {
		manager.BudgetField.FailAfter(-1)
		for i := range padding {
			fpMust(t, manager.DelRef(&padding[i]))
		}
	}()
	seed, err := manager.NewRef(ast.ASTHandle{})
	fpMust(t, err)
	padding = append(padding, seed)
	manager.BudgetField.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := manager.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		fpMust(t, cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	last := len(padding) - 1
	fpMust(t, manager.DelRef(&padding[last]))
	padding = padding[:last]
	before := manager.Stats()
	err = fp.ParseDatalogString(f.context, "\nP :- Q,")
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected temporary body reference refusal", err)
	}
	after := manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatalf("failed body leaked nodes or references: %+v -> %+v", before, after)
	}
	manager.BudgetField.FailAfter(-1)
	fpMust(t, fp.ParseDatalogString(f.context, "\nP :- Q,"))
	after = manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || f.ruleCount(t) != 0 {
		t.Fatal("EOF retry did not release temporary owners", before, after)
	}
}

func TestDatalogParserHeadReferenceFailureRetiresFreshVariable(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nP(x:N)\n"))
	manager, err := f.context.Manager()
	fpMust(t, err)
	var padding []ast.ASTRef
	defer func() {
		manager.BudgetField.FailAfter(-1)
		for i := range padding {
			fpMust(t, manager.DelRef(&padding[i]))
		}
	}()
	manager.BudgetField.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := manager.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		fpMust(t, cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	before := manager.Stats()
	err = fp.ParseDatalogString(f.context, "\nP(X) :-")
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected reference refusal", err)
	}
	after := manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatalf("failed head leaked variable: nodes %d -> %d, references %d -> %d", before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences)
	}
	manager.BudgetField.FailAfter(-1)
	fpMust(t, fp.ParseDatalogString(f.context, "\nP(X) :-"))
	after = manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || f.ruleCount(t) != 0 {
		t.Fatal("EOF retry leaked variable or published rule", before, after)
	}
}

func TestDatalogParserBodyArgumentFailureRetiresTemporaryNodes(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nP\nQ(x:N)\n"))
	manager, err := f.context.Manager()
	fpMust(t, err)
	var padding []ast.ASTRef
	defer func() {
		manager.BudgetField.FailAfter(-1)
		for i := range padding {
			fpMust(t, manager.DelRef(&padding[i]))
		}
	}()
	for i := 0; i < 2; i++ {
		ref, e := manager.NewRef(ast.ASTHandle{})
		fpMust(t, e)
		padding = append(padding, ref)
	}
	manager.BudgetField.FailAfter(0)
	for i := 0; ; i++ {
		ref, cause := manager.NewRef(ast.ASTHandle{})
		if errors.Is(cause, arena.ErrInjectedFailure) {
			break
		}
		fpMust(t, cause)
		padding = append(padding, ref)
		if i >= 256 {
			t.Fatal("reference page exhaustion not reached")
		}
	}
	for i := 0; i < 2; i++ {
		last := len(padding) - 1
		fpMust(t, manager.DelRef(&padding[last]))
		padding = padding[:last]
	}
	before := manager.Stats()
	err = fp.ParseDatalogString(f.context, "\nP :- Q(X),")
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected body argument reference refusal", err)
	}
	after := manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatalf("failed body leaked temporary ASTs: nodes %d -> %d, references %d -> %d", before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences)
	}
	manager.BudgetField.FailAfter(-1)
	fpMust(t, fp.ParseDatalogString(f.context, "\nP :- Q(X),"))
	after = manager.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || f.ruleCount(t) != 0 {
		t.Fatal("body EOF retry leaked or published rule", before, after)
	}
}

func TestDatalogParserRejectsPeriodAfterNegation(t *testing.T) {
	f := newDatalogParserFixture(t)
	if err := fp.ParseDatalogString(f.context, "\nP\nP :- !.\n"); err == nil {
		t.Fatal("period after negation accepted")
	}
	if count := f.ruleCount(t); count != 0 {
		t.Fatal("malformed rule published", count)
	}
}

func TestDatalogParserBodyEOFDoesNotPublishRule(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		accepted   bool
	}{
		{"after arrow", "P :-", true},
		{"after comma", "P :- Q,", true},
		{"after expression", "P :- Q", false},
		{"after negation", "P :- !", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			err := fp.ParseDatalogString(f.context, "\nP\nQ\n"+tc.body)
			if (err == nil) != tc.accepted {
				t.Fatal("EOF acceptance", err)
			}
			if count := f.ruleCount(t); count != 0 {
				t.Fatal("incomplete rule published", count)
			}
		})
	}
}

func TestDatalogParserBodyEOFRetiresTemporaryASTs(t *testing.T) {
	for _, body := range []string{"P :-", "P :- Q,"} {
		t.Run(body, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			fpMust(t, fp.ParseDatalogString(f.context, "\nP\nQ\n"))
			manager, err := f.context.Manager()
			fpMust(t, err)
			before := manager.Stats()
			fpMust(t, fp.ParseDatalogString(f.context, "\n"+body))
			after := manager.Stats()
			if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
				t.Fatalf("temporary AST leak: nodes %d -> %d, references %d -> %d", before.LiveNodes, after.LiveNodes, before.LiveReferences, after.LiveReferences)
			}
		})
	}
}
