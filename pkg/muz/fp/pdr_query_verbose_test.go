package fp

// Source: pdr_dl_interface.cpp:153, IF_VERBOSE(2, m_ctx.display_rules(...)).

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRQueryDisplaysTransformedRulesAtSourceVerbosity(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			fragmentMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			fragmentMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
			fragmentMust(t, err)
			defer func() {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				fragmentMust(t, stream.Close())
			}()
			fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
			for _, level := range []uint32{2, 0, 1, 2, 3, 0} {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(level))
				fragmentMust(t, stream.ResetText())
				status, err := f.engine.QueryPredicate(f.declarations["q"])
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("query", status)
				}
				var expectedRules bytes.Buffer
				if level >= 2 {
					fragmentMust(t, f.engine.pdrRules.WriteDisplay(&expectedRules))
				}
				got, err := stream.String()
				fragmentMust(t, err)
				if !strings.HasPrefix(got, expectedRules.String()) {
					t.Fatalf("verbosity %d transformed-rule prefix: got %q; want prefix %q", level, got, expectedRules.String())
				}
				got = strings.TrimPrefix(got, expectedRules.String())
				if level == 0 {
					if got != "" {
						t.Fatalf("verbosity %d: got %q after transformed rules", level, got)
					}
					continue
				}
				// context::solve has its own IF_VERBOSE(1) reachable-search
				// diagnostic after pdr_dl_interface's IF_VERBOSE(2) rule list.
				if !strings.HasPrefix(got, "\n0 pdr-lifecycle-q closed\n  true ") || !strings.HasSuffix(got, "\ngoals 0\n") {
					t.Fatalf("verbosity %d reachable-search suffix: %q", level, got)
				}
			}
		})
	}
}

// The empty transformed program still has its output predicate, so source
// query reaches the level-1 model branch after the level-2 rule listing.
func TestPDRQueryDisplaysEmptyProgramModelAtSourceVerbosity(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			fragmentMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			fragmentMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
			fragmentMust(t, err)
			defer func() {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				fragmentMust(t, stream.Close())
			}()
			fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
			var queryReferences uint32
			for iteration, level := range []uint32{0, 1, 2, 1, 0} {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(level))
				fragmentMust(t, stream.ResetText())
				status, err := f.engine.QueryPredicate(f.declarations["u"])
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("empty program query", status)
				}
				if iteration == 0 {
					queryReferences = f.manager.Stats().LiveReferences
				} else if f.manager.Stats().LiveReferences != queryReferences {
					t.Fatal("verbose query retained temporary model references")
				}
				n, err := f.engine.pdrRules.Len()
				fragmentMust(t, err)
				if n != 0 {
					t.Fatal("fixture did not eliminate all rules", n)
				}
				var expected bytes.Buffer
				if level >= 2 {
					fragmentMust(t, f.engine.pdrRules.WriteDisplay(&expected))
				}
				before := f.manager.Stats().LiveReferences
				md, err := f.engine.Model(f.declarations["u"])
				fragmentMust(t, err)
				for _, name := range []string{"p", "q", "u"} {
					value, err := md.Core().ConstantInterpretation(f.declarations[name])
					fragmentMust(t, err)
					valid := f.manager.IsTrue(value)
					if name == "u" {
						// Source get_level_property excludes the query predicate.
						valid = value.IsNull()
					}
					if !valid {
						t.Fatal("original predicate model", name)
					}
				}
				if level >= 1 {
					fragmentMust(t, md.Core().WriteSMT2(&expected, f.engine.params, f.engine.globals, 0))
				}
				fragmentMust(t, md.Close())
				if f.manager.Stats().LiveReferences != before {
					t.Fatal("caller model leaked references")
				}
				got, err := stream.String()
				fragmentMust(t, err)
				if got != expected.String() {
					t.Fatalf("verbosity %d: got %q; want %q", level, got, expected.String())
				}
			}
		})
	}
}

// pdr_dl_interface marks a zero-rule transformed program unsatisfiable
// without entering context::solve, so even enabled post-solve validation is
// not reached by a repeated empty-program query.
func TestPDREmptyProgramBypassesSolveValidation(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
			fragmentMust(t, f.engine.globals.SourceSetName("fixedpoint.pdr.validate_result", "true"))
			status, err := f.engine.QueryPredicate(f.declarations["u"])
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("warm empty-program query", status)
			}
			f.manager.Budget().FailAfter(0)
			status, err = f.engine.QueryPredicate(f.declarations["u"])
			f.manager.Budget().FailAfter(-1)
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("empty-program query entered solve validation", status)
			}
		})
	}
}
