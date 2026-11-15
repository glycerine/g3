package polynomial

import (
	"fmt"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
	"testing"
)

func factorVerboseFixture(t *testing.T, level uint32, out *utext.NumericTextStream) {
	t.Helper()
	oldLevel, e := diagnostic.ProcessVerbosityLevel()
	serviceMust(t, e)
	oldOut, e := diagnostic.BorrowProcessVerboseStream()
	serviceMust(t, e)
	serviceMust(t, diagnostic.SetProcessVerboseStream(out))
	serviceMust(t, diagnostic.SetProcessVerbosityLevel(level))
	t.Cleanup(func() {
		serviceMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		serviceMust(t, diagnostic.SetProcessVerboseStream(oldOut))
	})
}
func TestUPolynomialFactorReleaseOmitsVerbose(t *testing.T) {
	for _, f := range []struct {
		name  string
		coeff []int32
		count uint32
	}{{"zero", nil, 0}, {"constant", []int32{7}, 0}, {"quadratic", []int32{-1, 0, 1}, 2}} {
		for _, level := range []uint32{999, 1000, 1001} {
			t.Run(f.name+fmt.Sprint(level), func(t *testing.T) {
				q, s := preparationPhysicalFixture(t)
				m := s.manager
				p := upolySourceNew(t, m, f.coeff)
				fs, e := m.NewFactors()
				serviceMust(t, e)
				t.Cleanup(func() { serviceMust(t, s.CleanupRetained()); serviceMust(t, fs.Close()) })
				stream, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: q.Budget()})
				serviceMust(t, e)
				t.Cleanup(func() { serviceMust(t, stream.Close()) })
				factorVerboseFixture(t, level, stream)
				ok, e := s.Factor(mustUPolyView(t, m, p), fs, DefaultUPolynomialFactorParams())
				serviceMust(t, e)
				if !ok || fs.count != f.count {
					t.Fatal("verbose changed factorization", ok, fs.count)
				}
				text, e := stream.String()
				serviceMust(t, e)
				// AUDIT.md FUZZ-F7: the :distinct-factors line is
				// #ifndef _EXTERNAL_RELEASE; g3 follows the release build.
				if text != "" {
					t.Fatal("release-profile factor verbose", text)
				}
			})
		}
	}
}
