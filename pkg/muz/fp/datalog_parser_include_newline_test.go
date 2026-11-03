package fp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserDomainIncludeRequiresParentNewline(t *testing.T) {
	directory := t.TempDir()
	included := filepath.Join(directory, "domains.dl")
	fpMust(t, os.WriteFile(included, []byte("N 2\n\nP(x:N)\n"), 0o600))
	main := filepath.Join(directory, "main.dl")
	fpMust(t, os.WriteFile(main, []byte(".include \"domains.dl\" Q(x:N)\n"), 0o600))

	fixture := newDatalogParserFixture(t)
	if err := fp.ParseDatalogFile(fixture.context, main); err == nil {
		t.Fatal("domain include accepted trailing parent tokens")
	}
	// The source checks the parent terminator after parsing the child stream.
	// Preserve that mutation order even though the parent directive is invalid.
	predicates, err := fixture.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 1 {
		t.Fatal("included declarations were not parsed before terminator rejection", len(predicates))
	}
}
