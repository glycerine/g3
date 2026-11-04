package fp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

// Source read_string checks char_reader.eof while consuming string contents.
// The last unterminated physical line is already marked EOF by line_reader.
func TestDatalogParserFileStringOnLastPhysicalLine(t *testing.T) {
	cases := []struct {
		name, input    string
		file, accepted bool
	}{
		{"stream", "N 2\n\nP(x:N)\nP(\"a\").", false, true},
		{"file last line", "N 2\n\nP(x:N)\nP(\"a\").", true, false},
		{"file terminated line", "N 2\n\nP(x:N)\nP(\"a\").\n", true, true},
		{"file empty string", "N 2\n\nP(x:N)\nP(\"\").", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			var err error
			if tc.file {
				path := filepath.Join(t.TempDir(), "program.dl")
				fpMust(t, os.WriteFile(path, []byte(tc.input), 0600))
				err = fp.ParseDatalogFile(f.context, path)
			} else {
				err = fp.ParseDatalogString(f.context, tc.input)
			}
			if (err == nil) != tc.accepted {
				t.Fatal("string acceptance", err)
			}
			count := f.ruleCount(t)
			want := 0
			if tc.accepted {
				want = 1
			}
			if count != want {
				t.Fatal("rule publication", count, want)
			}
		})
	}
}
