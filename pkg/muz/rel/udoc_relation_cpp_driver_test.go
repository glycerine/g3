package rel

// Source: src/test/udoc_relation.cpp test_join(1000), test_filter_neg.
import (
	"testing"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestUDocRelationSourceRandomizedJoinAndAntiJoin(t *testing.T) {
	if !testing.Short() {
		t.Skip("long source stress test runs with -short")
	}
	rng := z3rand.New(0)
	leftManager, _ := newTestDocManager(t, 5)
	rightManager, _ := newTestDocManager(t, 5)
	joinedManager, _ := newTestDocManager(t, 10)
	left, err := NewUDocRelation(leftManager, []uint32{2, 3})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewUDocRelation(rightManager, []uint32{2, 3})
	if err != nil {
		_ = left.Close()
		t.Fatal(err)
	}
	closeUDoc(t, left, right)

	for round := 0; round < 1000; round++ {
		if err = left.Reset(); err != nil {
			t.Fatal(err)
		}
		if err = right.Reset(); err != nil {
			t.Fatal(err)
		}
		for n := 0; n < 3; n++ {
			if err = addRandomSourceDoc(t, left, rng, 3); err != nil {
				t.Fatal(err)
			}
			if err = addRandomSourceDoc(t, right, rng, 3); err != nil {
				t.Fatal(err)
			}
		}
		joined, e := left.Join(right, []uint32{0}, []uint32{0}, joinedManager)
		if e != nil {
			t.Fatalf("round %d join: %v", round, e)
		}
		for lv := uint64(0); lv < 32; lv++ {
			leftHas := sourceUDocContains(t, left, lv)
			for rv := uint64(0); rv < 32; rv++ {
				want := leftHas && (lv&3) == (rv&3) && sourceUDocContains(t, right, rv)
				got := sourceUDocContains(t, joined, lv|(rv<<5))
				if got != want {
					leftSummary, rightSummary, joinedSummary := sourceRelationSummary(t, left), sourceRelationSummary(t, right), sourceRelationSummary(t, joined)
					_ = joined.Close()
					t.Fatalf("round %d join tuple (%d,%d): got %t want %t (left=%t right=%t; L=%s R=%s J=%s)", round, lv, rv, got, want, leftHas, sourceUDocContains(t, right, rv), leftSummary, rightSummary, joinedSummary)
				}
			}
		}
		if e = joined.Close(); e != nil {
			t.Fatal(e)
		}
	}

	antiManager, _ := newTestDocManager(t, 3)
	antiProductManager, _ := newTestDocManager(t, 6)
	target, err := NewUDocRelation(antiManager, []uint32{1, 1, 1})
	if err != nil {
		t.Fatal(err)
	}
	negative, err := NewUDocRelation(antiManager, []uint32{1, 1, 1})
	if err != nil {
		_ = target.Close()
		t.Fatal(err)
	}
	closeUDoc(t, target, negative)
	for round := 0; round < 300; round++ {
		if err = target.Reset(); err != nil {
			t.Fatal(err)
		}
		if err = negative.Reset(); err != nil {
			t.Fatal(err)
		}
		targetDoc, e := randomSourceSetRandom(t, antiManager, rng)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = target.elems.InsertOwned(targetDoc); e != nil {
			t.Fatal(e)
		}
		negativeDoc, e := randomSourceSetRandom(t, antiManager, rng)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = negative.elems.InsertOwned(negativeDoc); e != nil {
			t.Fatal(e)
		}
		columns := []uint32{0}
		if round >= 100 {
			columns = append(columns, 1)
		}
		before := make([]bool, 8)
		negativeRows := make([]bool, 8)
		for value := uint64(0); value < 8; value++ {
			before[value] = sourceUDocContains(t, target, value)
			negativeRows[value] = sourceUDocContains(t, negative, value)
		}
		if err = target.FilterByNegation(negative, columns, columns, antiProductManager); err != nil {
			t.Fatalf("round %d anti-join: %v", round, err)
		}
		for value := uint64(0); value < 8; value++ {
			blocked := false
			for other := uint64(0); other < 8; other++ {
				if !negativeRows[other] {
					continue
				}
				match := true
				for _, col := range columns {
					if (value>>col)&1 != (other>>col)&1 {
						match = false
						break
					}
				}
				blocked = blocked || match
			}
			got := sourceUDocContains(t, target, value)
			if got != (before[value] && !blocked) {
				t.Fatalf("round %d anti-join value %d: got %t want %t", round, value, got, before[value] && !blocked)
			}
		}
	}
}

func randomSourceSetRandom(t *testing.T, manager *DocManager, rng *z3rand.Generator) (*Doc, error) {
	t.Helper()
	document, err := manager.AllocateX()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < 2*manager.NumTBits()/3; i++ {
		bit := rng.Bounded(manager.NumTBits())
		value := TBit0
		if rng.Bounded(2) != 0 {
			value = TBit1
		}
		if err = manager.Set(document, bit, value); err != nil {
			_ = manager.Deallocate(document)
			return nil, err
		}
	}
	return document, nil
}

func sourceRelationSummary(t *testing.T, relation *UDocRelation) string {
	t.Helper()
	result := ""
	for _, document := range relation.elems.values {
		positive, err := relation.manager.vectors.String(document.positive)
		if err != nil {
			t.Fatal(err)
		}
		result += positive + "\\"
		for _, negative := range document.negative.values {
			text, e := relation.manager.vectors.String(negative)
			if e != nil {
				t.Fatal(e)
			}
			result += text + ","
		}
		result += ";"
	}
	return result
}

func addRandomSourceDoc(t *testing.T, relation *UDocRelation, rng *z3rand.Generator, numDiff int) error {
	t.Helper()
	document, err := randomSourceCube(t, relation.manager, int(relation.NumBits()), rng)
	if err != nil {
		return err
	}
	for i := 0; i < numDiff; i++ {
		negative, e := relation.manager.vectors.Allocate()
		if e != nil {
			_ = relation.manager.Deallocate(document)
			return e
		}
		sameAsPositive := true
		for bit := uint32(0); bit < relation.NumBits(); bit++ {
			pos, e := relation.manager.vectors.Get(document.positive, bit)
			if e != nil {
				_ = relation.manager.vectors.Deallocate(negative)
				_ = relation.manager.Deallocate(document)
				return e
			}
			value := pos
			if pos == TBitX {
				value = sourceRandomTBit(rng)
			}
			if e = relation.manager.vectors.Set(negative, bit, value); e != nil {
				_ = relation.manager.vectors.Deallocate(negative)
				_ = relation.manager.Deallocate(document)
				return e
			}
			if value != pos {
				sameAsPositive = false
			}
		}
		if sameAsPositive {
			_ = relation.manager.vectors.Deallocate(negative)
			_ = relation.manager.Deallocate(document)
			return nil
		}
		if err = relation.manager.AddNegativeOwned(document, negative); err != nil {
			_ = relation.manager.vectors.Deallocate(negative)
			_ = relation.manager.Deallocate(document)
			return err
		}
	}
	_, err = relation.elems.InsertOwned(document)
	return err
}

func randomSourceCube(t *testing.T, manager *DocManager, bits int, rng *z3rand.Generator) (*Doc, error) {
	t.Helper()
	document, err := manager.AllocateX()
	if err != nil {
		return nil, err
	}
	for bit := 0; bit < bits; bit++ {
		if err = manager.Set(document, uint32(bit), sourceRandomTBit(rng)); err != nil {
			_ = manager.Deallocate(document)
			return nil, err
		}
	}
	return document, nil
}

func sourceRandomTBit(rng *z3rand.Generator) TBit {
	switch rng.Bounded(3) {
	case 0:
		return TBit0
	case 1:
		return TBit1
	default:
		return TBitX
	}
}

func sourceUDocContains(t *testing.T, relation *UDocRelation, value uint64) bool {
	t.Helper()
	for _, document := range relation.elems.values {
		positive := true
		for bit := uint32(0); bit < relation.NumBits(); bit++ {
			want, err := relation.manager.vectors.Get(document.positive, bit)
			if err != nil {
				t.Fatal(err)
			}
			assigned := TBit0
			if (value>>bit)&1 != 0 {
				assigned = TBit1
			}
			if want != TBitX && want != assigned {
				positive = false
				break
			}
		}
		if !positive {
			continue
		}
		excluded := false
		for _, negative := range document.negative.values {
			matches := true
			for bit := uint32(0); bit < relation.NumBits(); bit++ {
				want, err := relation.manager.vectors.Get(negative, bit)
				if err != nil {
					t.Fatal(err)
				}
				assigned := TBit0
				if (value>>bit)&1 != 0 {
					assigned = TBit1
				}
				if want != TBitX && want != assigned {
					matches = false
					break
				}
			}
			if matches {
				excluded = true
				break
			}
		}
		if !excluded {
			return true
		}
	}
	return false
}
