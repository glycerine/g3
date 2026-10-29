package cmd_context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatalogParserRelativeInclude(t *testing.T) {
	directory := t.TempDir()
	domains := filepath.Join(directory, "domains.dl")
	rules := filepath.Join(directory, "rules.dl")
	main := filepath.Join(directory, "main.dl")
	contextMust(t, os.WriteFile(domains, []byte("D 2\n\n"), 0600))
	contextMust(t, os.WriteFile(rules, []byte("P(x:D)\nP(\"v\").\n"), 0600))
	contextMust(t, os.WriteFile(main, []byte(".include \"domains.dl\"\n\n.include \"rules.dl\"\n"), 0600))
	context := contextFixture(t)
	contextMust(t, context.ParseDatalogFile(main))
	engine, err := context.fixedpointContext()
	contextMust(t, err)
	predicates, err := engine.Predicates()
	contextMust(t, err)
	if len(predicates) != 1 {
		t.Fatal("included predicate count", len(predicates))
	}
}
