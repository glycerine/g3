package fp_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDatalogParserMissingOptionalDomainMapContinues(t *testing.T) {
	directory := t.TempDir()
	main := filepath.Join(directory, "main.dl")
	input := "Color 3 absent.map\n\nP(x:Color)\nP(\"red\").\n"
	fpMust(t, os.WriteFile(main, []byte(input), 0o600))
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogFile(f.context, main))
	if count := f.ruleCount(t); count != 1 {
		t.Fatal("fact count after missing optional map", count)
	}
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	sort, err := manager.GetDomain(predicates[0], 0)
	fpMust(t, err)
	if count, err := f.context.ConstantCount(sort); err != nil || count != 1 {
		t.Fatal("dynamic constant after missing optional map", count, err)
	}
}

func TestDatalogParserDomainMapAcceptsSourceSizedLines(t *testing.T) {
	directory := t.TempDir()
	name := strings.Repeat("mapped-symbol-", 6000)
	fpMust(t, os.WriteFile(filepath.Join(directory, "names.map"), []byte(name+"\n"), 0o600))
	main := filepath.Join(directory, "main.dl")
	input := "Color 2 names.map\n\nP(x:Color)\nP(\"" + name + "\").\n"
	fpMust(t, os.WriteFile(main, []byte(input), 0o600))
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogFile(f.context, main))
	if count := f.ruleCount(t); count != 1 {
		t.Fatal("large mapped-symbol fact count", count)
	}
}
