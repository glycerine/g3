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
	if result.Profile.RuleCount != 2 || result.Profile.TupleCount != 2 || len(result.Profile.Relations) != 2 || result.Profile.Relations[1].Name != "Q" || !strings.Contains(result.Output, "Q: 1") {
		t.Fatalf("result %#v output %q", result, result.Output)
	}
}
