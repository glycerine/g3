package fp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserRelativeIncludeAndDomainMap(t *testing.T) {
	directory := t.TempDir()
	fpMust(t, os.WriteFile(filepath.Join(directory, "names.map"), []byte("red\nblue\n"), 0o600))
	fpMust(t, os.WriteFile(filepath.Join(directory, "rules.dl"), []byte("P(\"red\").\n"), 0o600))
	main := filepath.Join(directory, "main.dl")
	input := "Color 3 names.map\n\nP(x:Color)\n.include \"rules.dl\"\n"
	fpMust(t, os.WriteFile(main, []byte(input), 0o600))
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogFile(f.context, main))
	if count := f.ruleCount(t); count != 1 {
		t.Fatal("included rule count", count)
	}
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	sort, err := manager.GetDomain(predicates[0], 0)
	fpMust(t, err)
	if count, e := f.context.ConstantCount(sort); e != nil || count != 2 {
		t.Fatal("mapped constants", count, e)
	}
}

func TestDatalogParserMissingFileAndIncludeFail(t *testing.T) {
	f := newDatalogParserFixture(t)
	if err := fp.ParseDatalogFile(f.context, filepath.Join(t.TempDir(), "missing.dl")); err == nil {
		t.Fatal("missing file accepted")
	}
	directory := t.TempDir()
	main := filepath.Join(directory, "main.dl")
	fpMust(t, os.WriteFile(main, []byte("N 2\n\n.include \"absent.dl\"\n"), 0o600))
	if err := fp.ParseDatalogFile(f.context, main); err == nil {
		t.Fatal("missing include accepted")
	}
}
