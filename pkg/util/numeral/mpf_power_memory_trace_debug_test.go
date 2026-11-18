//go:build g3debug && g3trace

package numeral

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func mpfPowerMemoryTraceOwner(t *testing.T, p *umemory.ProcessAccounting) (*ArbitraryFloatManager, *umemory.Runtime, *arena.Budget) {
	t.Helper()
	b := arena.NewBudget(1 << 25)
	r, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64, Process: p})
	serviceMust(t, err)
	m, err := NewArbitraryFloatManager(IntegerConfig{Memory: r,
		Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells:   arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	serviceMust(t, err)
	return m, r, b
}

func TestMPFPowerWrapperMemoryTraceMissHitAndSourceDeletion(t *testing.T) {
	m, r, b := mpfPowerMemoryTraceOwner(t, nil)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("memory")
	serviceMust(t, m.BindTrace(tags))
	a, err := m.powerTwoWrapper(30, false)
	serviceMust(t, err)
	before := out.String()
	again, err := m.powerTwoWrapper(30, false)
	serviceMust(t, err)
	if again != a || out.String() != before {
		t.Fatal("cache hit allocated a second wrapper", out.String())
	}
	c, err := m.powerTwoMinusOneWrapper(30, false)
	serviceMust(t, err)
	for _, entry := range []struct {
		wrapper *arbitraryFloatPowerWrapper
		line    int
	}{{a, 268}, {c, 282}} {
		alloc := "alloc " + fmt.Sprintf("%p", entry.wrapper) + " src/util/mpf.h:" + strconv.Itoa(entry.line) + " mpz 16\n"
		if strings.Count(out.String(), alloc) != 1 {
			t.Fatal("source wrapper allocation", alloc, out.String())
		}
	}
	serviceMust(t, m.SourceFinalize())
	for _, w := range []*arbitraryFloatPowerWrapper{a, c} {
		dealloc := "dealloc " + fmt.Sprintf("%p", w) + " src/util/mpf.h:258\n"
		if strings.Count(out.String(), dealloc) != 1 {
			t.Fatal("source wrapper deallocation", dealloc, out.String())
		}
	}
	serviceMust(t, m.Close())
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("wrapper owner retained storage", b.Used())
	}
}

func TestMPFPowerWrapperMemoryTraceFailedPayloadAndFinalizeGuard(t *testing.T) {
	p := umemory.NewProcessAccounting()
	m, r, b := mpfPowerMemoryTraceOwner(t, p)
	if first, err := r.BeginSourceInitialize(math.MaxUint32); err != nil || !first {
		t.Fatal(first, err)
	}
	serviceMust(t, r.CompleteSourceInitialize())
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("memory")
	serviceMust(t, m.BindTrace(tags))
	b.FailAfter(1)
	if _, err := m.powerTwoWrapper(100, false); err == nil {
		t.Fatal("payload refusal did not reach retained wrapper")
	}
	b.FailAfter(-1)
	w := m.PowerWrappers
	if w == nil || !strings.Contains(out.String(), "alloc "+fmt.Sprintf("%p", w)+" src/util/mpf.h:268 mpz ") {
		t.Fatal("failed payload lost wrapper allocation record", out.String())
	}
	if active, err := p.BeginFinalize(); err != nil || !active {
		t.Fatal(active, err)
	}
	serviceMust(t, m.Close())
	if strings.Contains(out.String(), "dealloc "+fmt.Sprintf("%p", w)+" src/util/mpf.h:258\n") {
		t.Fatal("source finalization traced wrapper deletion", out.String())
	}
	serviceMust(t, p.CompleteFinalize())
	serviceMust(t, r.Close())
	if b.Used() != 0 {
		t.Fatal("failed wrapper retained storage", b.Used())
	}
}

// AUDIT.md util_numeral_03-F2: ~powers2 disposes each u_map in table order:
// keys 5 then 2 sit in slots 5 and 2, so 2 is freed first.
func TestMPFPowerWrapperMemoryTraceUMapDisposeOrder(t *testing.T) {
	m, r, _ := mpfPowerMemoryTraceOwner(t, nil)
	var out bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&out)
	tags.Enable("memory")
	serviceMust(t, m.BindTrace(tags))
	five, err := m.powerTwoWrapper(5, false)
	serviceMust(t, err)
	two, err := m.powerTwoWrapper(2, false)
	serviceMust(t, err)
	serviceMust(t, m.SourceFinalize())
	text := out.String()
	i := strings.Index(text, "dealloc "+fmt.Sprintf("%p", two))
	j := strings.Index(text, "dealloc "+fmt.Sprintf("%p", five))
	if i < 0 || j < 0 || i > j {
		t.Fatal("u_map dispose order", text)
	}
	serviceMust(t, m.Close())
	serviceMust(t, r.Close())
}
