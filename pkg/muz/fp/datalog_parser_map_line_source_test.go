package fp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserMapUsesSourceLineReaderBoundaries(t *testing.T) {
	cases := []struct {
		name, contents string
		want           []string
	}{
		{"empty", "", []string{""}},
		{"terminal newline", "alpha\n", []string{"alpha", ""}},
		{"no terminal newline", "alpha", []string{"alpha"}},
		{"crlf", "alpha\r\nbeta\r\n", []string{"alpha\r", "beta\r", ""}},
		{"embedded nul", "alpha\x00tail\n", []string{"alpha", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fpMust(t, os.WriteFile(filepath.Join(dir, "names.map"), []byte(tc.contents), 0600))
			main := filepath.Join(dir, "program.dl")
			fpMust(t, os.WriteFile(main, []byte("N 10 names.map\n\nP(x:N)\n"), 0600))
			f := newDatalogParserFixture(t)
			fpMust(t, fp.ParseDatalogFile(f.context, main))
			predicates, err := f.context.Predicates()
			fpMust(t, err)
			manager, err := f.context.Manager()
			fpMust(t, err)
			sort, err := manager.GetDomain(predicates[0], 0)
			fpMust(t, err)
			count, err := f.context.ConstantCount(sort)
			fpMust(t, err)
			if count != uint64(len(tc.want)) {
				t.Fatal("map entry count", count, tc.want)
			}
			for i, want := range tc.want {
				got, e := f.context.FormatConstant(sort, uint64(i))
				fpMust(t, e)
				if got != want {
					t.Fatalf("entry %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}
