package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatalogFrontendProfileRendersRelationSizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.dl")
	if err := os.WriteFile(path, []byte("\nP\nQ\nP.\nQ :- P.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	profile, err := frontend.Profile()
	if err != nil || profile.RuleCount != 2 || profile.RelationCount != 2 || profile.TupleCount != 2 {
		t.Fatalf("profile %#v error %v", profile, err)
	}
	var output bytes.Buffer
	if err = frontend.WriteProfile(&output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"relation sizes\n", "P: 1\n", "Q: 1\n", "rules: 2\n", "relations: 2\n", "tuples: 2\n", "compiled: true\n"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("profile missing %q: %q", want, output.String())
		}
	}
}
