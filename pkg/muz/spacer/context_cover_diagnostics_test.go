package spacer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerMissingCoverPredicateSourceDiagnostics(t *testing.T) {
	for _, level := range []uint32{9, 10} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			depthMust(t, c.Reset())
			name, err := m.GetASTName(f.other)
			depthMust(t, err)
			predicate, _, err := m.Symbols.BareString(name)
			depthMust(t, err)

			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			depthMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			depthMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
			depthMust(t, err)
			defer func() {
				depthMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				depthMust(t, stream.Close())
			}()
			depthMust(t, diagnostic.SetProcessVerbosityLevel(level))
			depthMust(t, diagnostic.SetProcessVerboseStream(stream))

			levels, err := c.NumLevels(f.other)
			depthMust(t, err)
			if levels != 0 {
				t.Fatal("missing predicate levels", levels)
			}
			falsity, err := m.MkFalse()
			depthMust(t, err)
			depthMust(t, c.AddCover(0, f.other, falsity))
			if _, found, err := c.Transformer(f.other); err != nil || !found {
				t.Fatal("cover-created transformer", found, err)
			}
			got, err := stream.String()
			depthMust(t, err)
			want := ""
			if level == 10 {
				want = strings.Repeat("did not find predicate "+predicate+"\n", 2)
			}
			if got != want {
				t.Fatalf("missing-predicate diagnostics = %q, want %q", got, want)
			}
		})
	}
}
