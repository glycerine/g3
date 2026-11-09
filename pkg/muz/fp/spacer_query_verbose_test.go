package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp:166-169,260-263. Empty
// ordinary queries print their model at verbosity two; from-level queries use
// verbosity one.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerQueryDisplaysEmptyProgramModelAtSourceVerbosity(t *testing.T) {
	for _, test := range []struct {
		name      string
		threshold uint32
		query     func(*pdrLifecycleFixture) (lbool.LBool, error)
	}{
		{name: "ordinary", threshold: 2, query: func(f *pdrLifecycleFixture) (lbool.LBool, error) {
			return f.engine.QueryPredicate(f.declarations["u"])
		}},
		{name: "from-level", threshold: 1, query: func(f *pdrLifecycleFixture) (lbool.LBool, error) {
			return f.engine.QueryPredicateFromLevel(f.declarations["u"], 4)
		}},
	} {
		for _, level := range []uint32{0, 1, 2, 3} {
			t.Run(fmt.Sprintf("%s/%d", test.name, level), func(t *testing.T) {
				f := newPDRLifecycleFixture(t)
				fragmentMust(t, f.engine.SelectEngine("spacer"))
				InstallContextModelConverter(f.engine) // a prior context::query installed m_mc
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
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(level))
				status, err := test.query(f)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("empty Spacer program query", status)
				}

				var expected bytes.Buffer
				before := f.manager.Stats().LiveReferences
				md, err := f.engine.Model(f.declarations["u"])
				fragmentMust(t, err)
				for _, name := range []string{"p", "q", "u"} {
					value, err := md.Core().ConstantInterpretation(f.declarations[name])
					fragmentMust(t, err)
					valid := f.manager.IsTrue(value)
					if name == "u" {
						valid = f.manager.IsFalse(value)
					}
					if !valid {
						t.Fatal("original predicate model", name, value)
					}
				}
				synthetic, err := md.Core().ConstantInterpretation(f.engine.spacerQuery)
				fragmentMust(t, err)
				if !synthetic.IsNull() {
					t.Fatal("synthetic query interpretation escaped conversion", synthetic)
				}
				if level >= test.threshold {
					fragmentMust(t, md.Core().WriteSMT2(&expected, f.engine.params, f.engine.globals, 0))
				}
				fragmentMust(t, md.Close())
				if got := f.manager.Stats().LiveReferences; got != before {
					t.Fatal("caller model leaked references", got, before)
				}
				got, err := stream.String()
				fragmentMust(t, err)
				if got != expected.String() {
					t.Fatalf("verbosity %d: got %q; want %q", level, got, expected.String())
				}

				// Repeated presentation owns and releases only temporary model
				// objects; the public model remains available after each pass.
				before = f.manager.Stats().LiveReferences
				for repeat := 0; repeat < 3; repeat++ {
					fragmentMust(t, stream.ResetText())
					fragmentMust(t, f.engine.writeSpacerEmptyProgramModel(test.threshold))
					got, err = stream.String()
					fragmentMust(t, err)
					if got != expected.String() {
						t.Fatalf("repeat %d: got %q; want %q", repeat, got, expected.String())
					}
					md, err = f.engine.Model(f.declarations["u"])
					fragmentMust(t, err)
					fragmentMust(t, md.Close())
					if got := f.manager.Stats().LiveReferences; got != before {
						t.Fatal("repeated model presentation leaked references", repeat, got, before)
					}
				}
			})
		}
	}
}
