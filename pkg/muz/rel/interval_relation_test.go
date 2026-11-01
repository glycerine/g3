package rel_test

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

type intervalRelationRefusingWriter struct{ err error }

func (w intervalRelationRefusingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestIntervalRelationFactAndJoin(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	r := rel.NewIntervalEmpty(2)
	if !r.Empty() {
		t.Fatal("new empty relation")
	}
	if err := r.AddFact([]*big.Rat{n(1), n(2)}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddFact([]*big.Rat{n(3), n(4)}); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{n(2), n(3)}) || r.ContainsFact([]*big.Rat{n(0), n(3)}) {
		t.Fatal("fact hull")
	}
	if err := r.FilterEqual(0, n(2)); err != nil {
		t.Fatal(err)
	}
	if err := r.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{n(2), n(2)}) || r.ContainsFact([]*big.Rat{n(2), n(3)}) {
		t.Fatal("equal columns")
	}
	other := rel.NewIntervalEmpty(1)
	if err := other.AddFact([]*big.Rat{n(2)}); err != nil {
		t.Fatal(err)
	}
	joined, err := rel.JoinIntervalRelations(r, other, []uint32{0}, []uint32{0})
	if err != nil {
		t.Fatal(err)
	}
	if !joined.ContainsFact([]*big.Rat{n(2), n(2), n(2)}) {
		t.Fatal("joined fact")
	}
	projected, err := joined.Project([]uint32{0, 1})
	if err != nil || !projected.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatalf("projected fact: %v", err)
	}
	lo, _, hi, _, err := projected.Bounds(0)
	if err != nil || lo.Cmp(n(2)) != 0 || hi.Cmp(n(2)) != 0 {
		t.Fatal("projected bounds")
	}
	lo.SetInt64(99)
	if !projected.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatal("bounds escaped mutable state")
	}
	emptyProjection, err := rel.NewIntervalEmpty(2).Project([]uint32{0})
	if err != nil || !emptyProjection.Empty() {
		t.Fatal("projection of empty relation must remain empty")
	}
}

func TestIntervalRelationFull(t *testing.T) {
	if !rel.NewIntervalFull(2).IsFull() {
		t.Fatal("full interval relation reported bounded")
	}
	if !rel.NewIntervalEmpty(2).IsFull() {
		t.Fatal("empty relation columns should retain full interval bounds")
	}
	bounded := rel.NewIntervalFull(1)
	if err := bounded.FilterEqual(0, big.NewRat(3, 1)); err != nil {
		t.Fatal(err)
	}
	if bounded.IsFull() {
		t.Fatal("bounded interval relation reported full")
	}
	if (*rel.IntervalRelation)(nil).IsFull() {
		t.Fatal("nil interval relation reported full")
	}
}

func TestIntervalRelationRename(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	r := rel.NewIntervalEmpty(3)
	if err := r.AddFact([]*big.Rat{n(1), n(2), n(3)}); err != nil {
		t.Fatal(err)
	}
	renamed, err := r.Rename([]uint32{0, 1, 2})
	if err != nil || !renamed.ContainsFact([]*big.Rat{n(3), n(1), n(2)}) {
		t.Fatalf("cycle rename: %v", err)
	}
	if !r.ContainsFact([]*big.Rat{n(1), n(2), n(3)}) {
		t.Fatal("rename changed source")
	}
	if _, err := r.Rename([]uint32{0, 0}); err == nil {
		t.Fatal("duplicate cycle column accepted")
	}
	if _, err := r.Rename([]uint32{3}); err == nil {
		t.Fatal("out-of-range cycle column accepted")
	}
	empty, err := rel.NewIntervalEmpty(2).Rename([]uint32{0, 1})
	if err != nil || !empty.Empty() {
		t.Fatal("rename must preserve emptiness")
	}
}

func TestIntervalRelationUnionDelta(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	target, source, delta := rel.NewIntervalEmpty(1), rel.NewIntervalEmpty(1), rel.NewIntervalEmpty(1)
	if err := source.AddFact([]*big.Rat{n(1)}); err != nil {
		t.Fatal(err)
	}
	changed, err := target.UnionWithDelta(source, delta, false)
	if err != nil || !changed || !delta.ContainsFact([]*big.Rat{n(1)}) {
		t.Fatalf("initial delta: %v", err)
	}
	if err := source.AddFact([]*big.Rat{n(2)}); err != nil {
		t.Fatal(err)
	}
	changed, err = target.UnionWithDelta(source, delta, false)
	if err != nil || !changed || !delta.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatalf("changed delta: %v", err)
	}
	changed, err = target.UnionWithDelta(source, delta, false)
	if err != nil || changed || !delta.ContainsFact([]*big.Rat{n(1)}) {
		t.Fatalf("unchanged delta: %v", err)
	}
	changed, err = target.UnionWithDelta(rel.NewIntervalEmpty(1), delta, false)
	if err != nil || changed || !delta.Empty() {
		t.Fatalf("empty source delta: %v", err)
	}
	if _, err = target.UnionWithDelta(source, target, false); err == nil {
		t.Fatal("aliased delta accepted")
	}
}

func TestIntervalRelationCloneMatchesSourceCopy(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	source := rel.NewIntervalEmpty(2)
	for _, value := range []int64{0, 10} {
		if err := source.AddFact([]*big.Rat{n(value), n(value)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := source.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	clone, err := source.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err := clone.FilterEqual(0, n(5)); err != nil {
		t.Fatal(err)
	}
	if !clone.ContainsFact([]*big.Rat{n(5), n(4)}) {
		t.Fatal("source vector_relation::copy separates equality classes in the destination")
	}
	if !source.ContainsFact([]*big.Rat{n(6), n(6)}) {
		t.Fatal("clone mutation changed source relation")
	}
	empty, err := rel.NewIntervalEmpty(2).Clone()
	if err != nil || !empty.Empty() {
		t.Fatal("clone must preserve empty state", err)
	}
	if _, err := (*rel.IntervalRelation)(nil).Clone(); err == nil {
		t.Fatal("nil relation clone accepted")
	}
}

func TestIntervalRelationDisplaySourceFormat(t *testing.T) {
	var output bytes.Buffer
	full := rel.NewIntervalFull(3)
	if err := full.Display(&output); err != nil {
		t.Fatal(err)
	}
	const fullWant = "0 in (-oo, oo)\n1 in (-oo, oo)\n2 in (-oo, oo)\n\n"
	if output.String() != fullWant {
		t.Fatalf("full display: got %q, want %q", output.String(), fullWant)
	}

	output.Reset()
	if err := rel.NewIntervalEmpty(2).Display(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "empty\n" {
		t.Fatalf("empty display: %q", output.String())
	}

	output.Reset()
	joined := rel.NewIntervalFull(2)
	if err := joined.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := joined.Display(&output); err != nil {
		t.Fatal(err)
	}
	const joinedWant = "0 = 1 1 in (-oo, oo)\n\n"
	if output.String() != joinedWant {
		t.Fatalf("equality-class display: got %q, want %q", output.String(), joinedWant)
	}

	refusal := errors.New("refused")
	if err := full.Display(intervalRelationRefusingWriter{refusal}); !errors.Is(err, refusal) {
		t.Fatalf("writer error: %v", err)
	}
	if err := full.Display(nil); err == nil {
		t.Fatal("nil writer accepted")
	}
}
