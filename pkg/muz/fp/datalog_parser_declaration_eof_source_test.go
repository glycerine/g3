package fp_test

import (
	"github.com/glycerine/g3/pkg/muz/fp"
	"os"
	"path/filepath"
	"testing"
)

// Source parse_args ends on EOS even without ')'; parse_rule accepts EOS
// after publishing the declaration and its successfully parsed arguments.
func TestDatalogParserDeclarationEOFPublishesParsedArguments(t *testing.T) {
	for _, tc := range []struct {
		suffix string
		arity  uint32
	}{{"P(", 0}, {"P(x:N", 1}, {"P(x:N,", 1}, {"P(x:N y:N", 2}} {
		t.Run(tc.suffix, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\n"+tc.suffix))
			predicates, err := f.context.Predicates()
			fpMust(t, err)
			if len(predicates) != 1 {
				t.Fatal("predicate count", len(predicates))
			}
			manager, err := f.context.Manager()
			fpMust(t, err)
			arity, err := manager.GetArity(predicates[0])
			fpMust(t, err)
			if arity != tc.arity || f.ruleCount(t) != 0 {
				t.Fatal("published declaration", arity, tc.arity)
			}
			for i := uint32(0); i < arity; i++ {
				name, err := f.context.ArgumentName(predicates[0], i)
				fpMust(t, err)
				text, err := manager.Symbols.Display(name)
				fpMust(t, err)
				if want := []string{"x", "y"}[i]; text != want {
					t.Fatal("argument name", i, text, want)
				}
			}
		})
	}
	f := newDatalogParserFixture(t)
	if err := fp.ParseDatalogString(f.context, "N 2\n\nP(x:"); err == nil {
		t.Fatal("missing sort accepted")
	}
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 0 {
		t.Fatal("incomplete argument published predicate")
	}
}

func TestDatalogParserFileSyntheticNewlineRejectsUnfinishedDeclaration(t *testing.T) {
	f := newDatalogParserFixture(t)
	path := filepath.Join(t.TempDir(), "input.dl")
	fpMust(t, os.WriteFile(path, []byte("N 2\n\nP(x:N"), 0600))
	if err := fp.ParseDatalogFile(f.context, path); err == nil {
		t.Fatal("synthetic file newline accepted as EOS")
	}
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 0 {
		t.Fatal("unfinished file declaration published")
	}
}
