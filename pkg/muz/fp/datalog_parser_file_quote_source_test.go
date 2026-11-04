package fp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

// The file parser uses char_reader, which synthesizes a newline after every
// line, whereas the stream parser exposes the input bytes directly.
func TestDatalogParserFileEOFInsideBarIdentifier(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		file              bool
	}{
		{"stream", "\n|abc", "abc", false},
		{"file unterminated line", "\n|abc", "abc\n", true},
		{"file terminal newline", "\n|abc\n", "abc\n\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			if tc.file {
				path := filepath.Join(t.TempDir(), "input.dl")
				fpMust(t, os.WriteFile(path, []byte(tc.input), 0600))
				fpMust(t, fp.ParseDatalogFile(f.context, path))
			} else {
				fpMust(t, fp.ParseDatalogString(f.context, tc.input))
			}
			predicates, err := f.context.Predicates()
			fpMust(t, err)
			if len(predicates) != 1 {
				t.Fatal("predicate count", len(predicates))
			}
			manager, err := f.context.Manager()
			fpMust(t, err)
			sym, err := manager.GetASTName(predicates[0])
			fpMust(t, err)
			got, err := manager.Symbols.Display(sym)
			fpMust(t, err)
			if got != tc.want {
				t.Fatalf("predicate name = %q, want %q", got, tc.want)
			}
		})
	}
}
