package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIProfileDatalogPathReturnsNamedRelationSummary(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	path := filepath.Join(t.TempDir(), "profile.dl")
	astTestMust(t, os.WriteFile(path, []byte("\nP\nQ\nP.\nQ :- P.\n"), 0o600))
	result, err := context.ProfileDatalogPath(path)
	astTestMust(t, err)
	// The fact P. goes to context::add_fact (dparser::add_rule), so the
	// rule set holds only Q :- P.
	if result.Profile.RuleCount != 1 || result.Profile.TupleCount != 2 || len(result.Profile.Relations) != 2 || result.Profile.Relations[1].Name != "Q" || !strings.Contains(result.Output, "Q: 1") {
		t.Fatalf("result %#v output %q", result, result.Output)
	}
}
