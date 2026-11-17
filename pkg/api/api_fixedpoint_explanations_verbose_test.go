//go:build g3trace

package api

// The -v 1 lines of the explanation programs (rule transformations, the
// sparse tables' verbose actions under mk_explanations' fact transfer and
// the evaluation) against the fixed fork's: test_vectors/explanations/
// expl.c <mode> 1 and scen.c <scenario> 1, standard error, as
// verbose_mode_N.txt and verbose_scenario_N.txt. Times are compared as
// 0s (the fork's are below the millisecond it rounds to 0).

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var explanationVerboseTime = regexp.MustCompile(`(?m)[0-9][0-9.e+-]*s(\)?)$`)

func explanationVerbose(t *testing.T, run func() string) string {
	t.Helper()
	var buf bytes.Buffer
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: arena.NewBudget(1 << 24), BorrowedWriter: &buf})
	astTestMust(t, err)
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	astTestMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	astTestMust(t, err)
	astTestMust(t, diagnostic.SetProcessVerboseStream(stream))
	astTestMust(t, diagnostic.SetProcessVerbosityLevel(1))
	func() {
		defer func() {
			astTestMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
			astTestMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		}()
		run()
	}()
	astTestMust(t, stream.Close())
	return explanationVerboseTime.ReplaceAllString(buf.String(), "0s$1")
}

func TestAPIFixedpointExplanationsVerbose(t *testing.T) {
	for _, mode := range []int{0, 1, 2, 5, 6, 8, 9, 11, 12, 14, 15, 16} {
		t.Run("mode"+strconv.Itoa(mode), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("test_vectors", "explanations", "verbose_mode_"+strconv.Itoa(mode)+".txt"))
			astTestMust(t, err)
			got := explanationVerbose(t, func() string { return runExplanationsMode(t, mode) })
			if w := explanationVerboseTime.ReplaceAllString(string(want), "0s$1"); got != w {
				t.Fatalf("mode %d verbose:\n%s\nwant\n%s", mode, got, w)
			}
		})
	}
	for _, s := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28} {
		t.Run("scenario"+strconv.Itoa(s), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("test_vectors", "explanations", "verbose_scenario_"+strconv.Itoa(s)+".txt"))
			astTestMust(t, err)
			got := explanationVerbose(t, func() string { return runExplanationScenario(t, s) })
			if w := explanationVerboseTime.ReplaceAllString(string(want), "0s$1"); got != w {
				t.Fatalf("scenario %d verbose:\n%s\nwant\n%s", s, got, w)
			}
		})
	}
}
