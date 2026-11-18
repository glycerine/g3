package numeral

// rational(char const*) throws on allocation failure in z3, and C++
// destructors cannot fail. In g3 a parse stopped by a native accounting
// failure can leave a parser temporary whose release is pending; once memory
// is available again the next operation finishes that release instead of
// refusing every later operation with ErrRationalLexicalPending.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestRationalSetStringStoppedParseResumesOnNextOperation(t *testing.T) {
	b := arena.NewBudget(1 << 28)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64})
	serviceMust(t, e)
	m, e := NewArbitraryFloatManager(IntegerConfig{Memory: r, Cells: arena.Config{Budget: b, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
	serviceMust(t, e)
	q := m.rational
	text := strings.Repeat("9", 3000)
	// Warm the parse: its scratch and the output reach the native runtime.
	a := NewRational()
	serviceMust(t, q.SetString(&a, text))
	serviceMust(t, q.Del(&a))
	a = NewRational()
	contextTLSNearMinimum(t, r)
	lane, e := r.Allocate(8)
	serviceMust(t, e)
	delta := int64(-99950) - (1 << 16) - r.State().LocalSize
	if e = r.Reallocate(&lane, uint64(16)+uint64(delta)-8); !errors.Is(e, arena.ErrBounds) {
		t.Fatal(e)
	}
	if e = q.SetString(&a, text+"9"); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("parse did not stop at a release", e)
	}
	if !q.parserPending() {
		t.Fatal("stopped parse left no pending temporary: the fixture no longer reaches the case")
	}
	recovery, e := r.Allocate(400000)
	serviceMust(t, e)
	x, y := NewRational(), NewRational()
	serviceMust(t, q.SetInt64(&x, 5))
	if e = q.Add(&x, &x, &y); e != nil || q.parserPending() {
		t.Fatal("operation after the stopped parse", e, q.parserPending())
	}
	serviceMust(t, q.Reset(&a))
	serviceMust(t, q.SetString(&a, text))
	if bits, e := q.Bitsize(&a); e != nil || bits < 9000 {
		t.Fatal("parse after the resumed cleanup", bits, e)
	}
	for _, v := range []*Rational{&a, &x, &y} {
		serviceMust(t, q.Del(v))
	}
	contextTLSFinish(t, r, m.Close)
	for _, block := range []*umemory.Block{&lane, &recovery} {
		contextTLSFinish(t, r, func() error {
			if *block == (umemory.Block{}) {
				return nil
			}
			return r.Deallocate(block)
		})
	}
	contextTLSFinish(t, r, r.Close)
	if b.Used() != 0 {
		t.Fatal("refunds")
	}
}
