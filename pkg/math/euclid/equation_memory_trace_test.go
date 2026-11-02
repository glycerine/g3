//go:build g3debug && g3trace

package euclid

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestEuclideanEquationMemoryWrappersFollowTraceRebinding(t *testing.T) {
	m := euclidNumeralManager(t)
	z := m.Integers()
	s, err := NewSolver(m)
	euclidMust(t, err)
	_, err = s.MkVar()
	euclidMust(t, err)
	var before, after bytes.Buffer
	first := diagnostic.NewTraceTagsWithWriter(&before)
	first.Enable("memory")
	euclidMust(t, s.BindTrace(first))
	a, c := numeral.NewInt(2), numeral.NewInt(3)
	euclidMust(t, s.AssertEq([]numeral.Int{a}, []uint32{0}, &c, nullEquationIndex))
	eq := s.setup.equations[0]
	second := diagnostic.NewTraceTagsWithWriter(&after)
	second.Enable("memory")
	euclidMust(t, s.BindTrace(second))
	euclidMust(t, s.Close())
	euclidMust(t, z.Del(&a))
	euclidMust(t, z.Del(&c))
	alloc := "alloc " + fmt.Sprintf("%p", eq) + " src/math/euclid/euclidean_solver.cpp:155 equation " +
		strconv.FormatUint(uint64(reflect.TypeOf(equation{}).Size()), 10) + "\n"
	dealloc := "dealloc " + fmt.Sprintf("%p", eq) + " src/math/euclid/euclidean_solver.cpp:303\n"
	if strings.Count(before.String(), alloc) != 1 || strings.Contains(before.String(), dealloc) ||
		strings.Count(after.String(), dealloc) != 1 || strings.Contains(after.String(), alloc) {
		t.Fatalf("source equation wrapper records: before=%q after=%q", before.String(), after.String())
	}
}

func TestEuclideanEquationMemoryDeallocationSuppressedDuringFinalize(t *testing.T) {
	m := euclidNumeralManager(t)
	z := m.Integers()
	s, err := newSolverSetup(m)
	euclidMust(t, err)
	_, err = s.mkVar(false)
	euclidMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("memory")
	process := umemory.NewProcessAccounting()
	runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: arena.NewBudget(1024), Process: process})
	euclidMust(t, err)
	defer func() { euclidMust(t, runtime.Close()) }()
	euclidMust(t, tags.BindMemoryProcess(process))
	s.trace = tags
	a, c := numeral.NewInt(2), numeral.NewInt(3)
	_, err = s.addEquation([]numeral.Int{a}, []uint32{0}, &c, nil, nil, true)
	euclidMust(t, err)
	started, err := runtime.BeginSourceInitialize(math.MaxUint32)
	euclidMust(t, err)
	if !started {
		t.Fatal("fresh memory process was already initialized")
	}
	euclidMust(t, runtime.CompleteSourceInitialize())
	started, err = process.BeginFinalize()
	euclidMust(t, err)
	if !started {
		t.Fatal("fresh memory process was already finalizing")
	}
	euclidMust(t, s.Close())
	euclidMust(t, z.Del(&a))
	euclidMust(t, z.Del(&c))
	euclidMust(t, process.CompleteFinalize())
	if !strings.Contains(output.String(), "alloc ") || strings.Contains(output.String(), "dealloc ") {
		t.Fatalf("finalizing memory records = %q", output.String())
	}
}
