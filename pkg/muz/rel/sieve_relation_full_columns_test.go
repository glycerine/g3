package rel

import "testing"

func TestSieveFullPreservesSelectedColumnLayout(t *testing.T) {
	signature := sieveSignature(t, 2, 3, 2)
	full, err := NewFullSieveWithColumns(signature, []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, full)
	if got := full.InnerColumns(); len(got) != 3 || !got[0] || got[1] || !got[2] {
		t.Fatal("selected layout", got)
	}
	if full.NumInnerRows() != 4 {
		t.Fatal("selected full inner product rows", full.NumInnerRows())
	}
	enumerateSignatureFacts(t, signature, func(fact Fact) {
		requireSieveFact(t, full, fact, true)
	})

	empty, err := full.EmptyLike()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, empty)
	if !empty.Empty() {
		t.Fatal("empty-like relation is nonempty")
	}
	if got := empty.InnerColumns(); len(got) != 3 || !got[0] || got[1] || !got[2] {
		t.Fatal("empty-like layout", got)
	}
	if full.Empty() {
		t.Fatal("empty-like construction mutated source")
	}
}

func TestSieveFullSelectedColumnsNullaryIdentity(t *testing.T) {
	signature := sieveSignature(t)
	full, err := NewFullSieveWithColumns(signature, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, full)
	if full.NumInnerRows() != 1 {
		t.Fatal("nullary full must contain its identity tuple", full.NumInnerRows())
	}
	requireSieveFact(t, full, Fact{}, true)
	empty, err := full.EmptyLike()
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, empty)
	requireSieveFact(t, empty, Fact{}, false)
}
