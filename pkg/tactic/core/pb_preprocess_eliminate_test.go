package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPBPreprocessEliminatesOneSidedVariables(t *testing.T) {
	f := newPBPreprocessFixture(t)
	h := f.ge(f.vars[:2], []string{"1", "1"}, "1")
	g := f.goal(h)
	out := f.apply(g)
	if len(out.Goals) != 1 || out.ModelConverter == nil {
		t.Fatal("missing result/converter")
	}
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal("one-sided constraints survived", n, e)
	}
	if d, e := g.Depth(); e != nil || d != 1 {
		t.Fatal(d, e)
	}
}

func TestPBPreprocessNegativeLiteralChoosesFalse(t *testing.T) {
	f := newPBPreprocessFixture(t)
	notX, e := f.m.MkNot(f.vars[0])
	simplifyMust(t, e)
	g := f.goal(notX)
	f.apply(g)
	if n, e := g.Size(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
}

// AUDIT.md tactic_core_02-F4: pb_preprocess_tactic::operator() has no
// tactic_report; replace prints its IF_VERBOSE(3) line.
func TestPBPreprocessVerboseReplaceLine(t *testing.T) {
	b := arena.NewBudget(1 << 16)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: b})
	simplifyMust(t, err)
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	simplifyMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	simplifyMust(t, err)
	simplifyMust(t, diagnostic.SetProcessVerboseStream(stream))
	t.Cleanup(func() {
		simplifyMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		simplifyMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		simplifyMust(t, stream.Close())
	})
	f := newPBPreprocessFixture(t)
	clause, err := f.m.MkOr(f.vars[0], f.vars[1])
	simplifyMust(t, err)
	g := f.goal(clause)
	simplifyMust(t, diagnostic.SetProcessVerbosityLevel(10))
	f.apply(g)
	simplifyMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
	got, err := stream.String()
	simplifyMust(t, err)
	if got != "replace (or pb-x pb-y) -> true\n" {
		t.Fatalf("verbose output %q", got)
	}
}
