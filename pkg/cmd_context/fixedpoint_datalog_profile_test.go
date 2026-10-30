package cmd_context_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestCommandContextDatalogProfileNamesRelations(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.ParseDatalogString("\nP\nQ\nP.\nQ :- P.\n"); err != nil {
		t.Fatal(err)
	}
	predicates, err := context.FixedpointPredicates()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = context.QueryFixedpointRelations(predicates); err != nil {
		t.Fatal(err)
	}
	profile, err := context.FixedpointDatalogProfile()
	if err != nil || len(profile.Relations) != 2 || profile.Relations[0].Name != "P" || profile.Relations[1].Name != "Q" || profile.TupleCount != 2 {
		t.Fatalf("profile %#v error %v", profile, err)
	}
}
