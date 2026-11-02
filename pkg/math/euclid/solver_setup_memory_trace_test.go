//go:build g3debug && g3trace

package euclid

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func euclidImpMemoryRecord(prefix string, s *solverSetup, line int) string {
	record := prefix + fmt.Sprintf("%p", s) + " src/math/euclid/euclidean_solver.cpp:" + strconv.Itoa(line)
	if prefix == "alloc " {
		record += " imp " + strconv.FormatUint(uint64(reflect.TypeOf(solverSetup{}).Size()), 10)
	}
	return record + "\n"
}

func TestEuclideanImpMemoryResetRefusalAfterOldDeallocation(t *testing.T) {
	m := euclidNumeralManager(t)
	baseline := m.BudgetField.Used()
	s, err := NewSolver(m)
	euclidMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("memory")
	euclidMust(t, s.BindTrace(tags))
	old := s.setup
	m.BudgetField.FailAfter(0)
	err = s.Reset()
	m.BudgetField.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || s.setup != nil || s.stop == nil {
		t.Fatalf("reset replacement did not stop after old owner teardown: %v %#v", err, s.setup)
	}
	want := euclidImpMemoryRecord("dealloc ", old, 799)
	got := output.String()
	if strings.Count(got, want) != 1 || strings.Contains(got, "src/math/euclid/euclidean_solver.cpp:800 imp ") {
		t.Fatalf("reset refusal memory prefix: %q", got)
	}
	euclidMust(t, s.Close())
	if got := m.BudgetField.Used(); got != baseline {
		t.Fatalf("reset refusal retained solver bytes: %d, baseline %d", got, baseline)
	}
}

func TestEuclideanImpMemoryWrappersEncloseEquationLifetime(t *testing.T) {
	m := euclidNumeralManager(t)
	z := m.Integers()
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("memory")
	s, err := newSolverSetupWithDebugTrace(m, nil, tags, 784)
	euclidMust(t, err)
	_, err = s.mkVar(false)
	euclidMust(t, err)
	a, c := numeral.NewInt(2), numeral.NewInt(3)
	eq, err := s.addEquation([]numeral.Int{a}, []uint32{0}, &c, nil, nil, true)
	euclidMust(t, err)
	euclidMust(t, s.Close())
	euclidMust(t, z.Del(&a))
	euclidMust(t, z.Del(&c))
	got := output.String()
	impAlloc := euclidImpMemoryRecord("alloc ", s, 784)
	impDealloc := euclidImpMemoryRecord("dealloc ", s, 788)
	eqAlloc := "alloc " + fmt.Sprintf("%p", eq) + " src/math/euclid/euclidean_solver.cpp:155 equation "
	eqDealloc := "dealloc " + fmt.Sprintf("%p", eq) + " src/math/euclid/euclidean_solver.cpp:303\n"
	if strings.Count(got, impAlloc) != 1 || strings.Count(got, impDealloc) != 1 ||
		strings.Count(got, eqAlloc) != 1 || strings.Count(got, eqDealloc) != 1 ||
		strings.Index(got, impAlloc) >= strings.Index(got, eqAlloc) ||
		strings.Index(got, eqAlloc) >= strings.Index(got, eqDealloc) ||
		strings.Index(got, eqDealloc) >= strings.Index(got, impDealloc) {
		t.Fatalf("source imp/equation memory order: %q", got)
	}
}

func TestEuclideanImpMemoryWrappersResetSites(t *testing.T) {
	m := euclidNumeralManager(t)
	s, err := NewSolver(m)
	euclidMust(t, err)
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("memory")
	euclidMust(t, s.BindTrace(tags))
	old := s.setup
	euclidMust(t, s.Reset())
	fresh := s.setup
	euclidMust(t, s.Close())
	got := output.String()
	oldRelease := euclidImpMemoryRecord("dealloc ", old, 799)
	freshAlloc := euclidImpMemoryRecord("alloc ", fresh, 800)
	freshRelease := euclidImpMemoryRecord("dealloc ", fresh, 788)
	if strings.Count(got, oldRelease) != 1 || strings.Count(got, freshAlloc) != 1 ||
		strings.Count(got, freshRelease) != 1 ||
		strings.Index(got, oldRelease) >= strings.Index(got, freshAlloc) ||
		strings.Index(got, freshAlloc) >= strings.Index(got, freshRelease) {
		t.Fatalf("source reset memory wrappers: %q", got)
	}
}
