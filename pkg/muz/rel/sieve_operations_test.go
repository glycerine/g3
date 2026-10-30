package rel

import (
	"errors"
	"testing"
)

func TestSieveJoinRetainedEqualityAndIgnoredOverApproximation(t *testing.T) {
	left, err := NewSieveRelation(sieveSignature(t, 2, 2), []bool{true, true})
	if err != nil {
		t.Fatal(err)
	}
	right, _ := NewSieveRelation(sieveSignature(t, 2, 2), []bool{true, false})
	closeSieve(t, left, right)
	_, _ = left.AddFact(Fact{0, 1})
	_, _ = left.AddFact(Fact{1, 0})
	_, _ = right.AddFact(Fact{1, 0})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, joined)
	if joined.NumInnerRows() != 1 {
		t.Fatal("retained equality rows", joined.NumInnerRows())
	}
	for ignored := Element(0); ignored < 2; ignored++ {
		requireSieveFact(t, joined, Fact{0, 1, 1, ignored}, true)
	}
	requireSieveFact(t, joined, Fact{1, 0, 1, 0}, false)

	approxRight, _ := NewSieveRelation(sieveSignature(t, 2), []bool{false})
	closeSieve(t, approxRight)
	_, _ = approxRight.AddFact(Fact{0})
	approximated, err := left.Join(approxRight, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, approximated)
	if approximated.NumInnerRows() != left.NumInnerRows() {
		t.Fatal("ignored equality should be skipped", approximated.NumInnerRows())
	}
}

func TestSieveProjectInnerAndIgnoredColumns(t *testing.T) {
	relation, err := NewSieveRelation(sieveSignature(t, 2, 3, 2, 2), []bool{true, false, true, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, relation)
	_, _ = relation.AddFact(Fact{1, 0, 0, 1})
	_, _ = relation.AddFact(Fact{0, 1, 1, 1})
	projected, err := relation.Project([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, projected)
	if mask := projected.InnerColumns(); len(mask) != 2 || !mask[0] || !mask[1] {
		t.Fatal("project mask", mask)
	}
	requireSieveFact(t, projected, Fact{1, 1}, true)
	requireSieveFact(t, projected, Fact{0, 1}, true)
	if _, err = relation.Project([]int{2, 1}); err == nil {
		t.Fatal("unordered projection accepted")
	}
}

func TestSieveRenameCycleMovesMaskDomainsAndRows(t *testing.T) {
	relation, err := NewSieveRelation(sieveSignature(t, 2, 3, 4), []bool{true, false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, relation)
	_, _ = relation.AddFact(Fact{1, 2, 3})
	renamed, err := relation.RenameCycle([]int{0, 1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, renamed)
	signature, _ := renamed.Signature()
	if got := signature.Domains(); len(got) != 3 || got[0] != 3 || got[1] != 4 || got[2] != 2 {
		t.Fatal("renamed domains", got)
	}
	if mask := renamed.InnerColumns(); mask[0] || !mask[1] || !mask[2] {
		t.Fatal("renamed mask", mask)
	}
	for ignored := Element(0); ignored < 3; ignored++ {
		requireSieveFact(t, renamed, Fact{ignored, 3, 1}, true)
	}
	if _, err = relation.RenameCycle([]int{0, 0}); err == nil {
		t.Fatal("duplicate rename accepted")
	}
}

func TestSieveUnionDeltaAndLayoutValidation(t *testing.T) {
	signature := sieveSignature(t, 2, 3)
	target, _ := NewSieveRelation(signature, []bool{true, false})
	source, _ := NewSieveRelation(signature, []bool{true, false})
	delta, _ := NewSieveRelation(signature, []bool{true, false})
	closeSieve(t, target, source, delta)
	_, _ = target.AddFact(Fact{0, 0})
	_, _ = source.AddFact(Fact{0, 2})
	_, _ = source.AddFact(Fact{1, 1})
	if err := target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	if target.NumInnerRows() != 2 || delta.NumInnerRows() != 1 {
		t.Fatal("union/delta rows", target.NumInnerRows(), delta.NumInnerRows())
	}
	requireSieveFact(t, delta, Fact{1, 0}, true)
	requireSieveFact(t, delta, Fact{0, 0}, false)
	wrong, _ := NewSieveRelation(signature, []bool{false, true})
	closeSieve(t, wrong)
	if err := target.UnionFrom(wrong, nil); !errors.Is(err, ErrSieve) {
		t.Fatal("layout mismatch", err)
	}
}

func TestSieveFiltersPreserveSourceApproximation(t *testing.T) {
	signature := sieveSignature(t, 2, 2, 2)
	relation, _ := NewSieveRelation(signature, []bool{true, false, true})
	closeSieve(t, relation)
	for _, fact := range []Fact{{0, 0, 0}, {0, 0, 1}, {1, 0, 0}, {1, 0, 1}} {
		_, _ = relation.AddFact(fact)
	}
	if err := relation.FilterEqual(1, 1); err != nil || relation.NumInnerRows() != 4 {
		t.Fatal("ignored equality must be identity", relation.NumInnerRows(), err)
	}
	if err := relation.FilterIdentical([]int{0, 1}); err != nil || relation.NumInnerRows() != 4 {
		t.Fatal("one retained identical column must be identity", relation.NumInnerRows(), err)
	}
	if err := relation.FilterIdentical([]int{0, 2}); err != nil || relation.NumInnerRows() != 2 {
		t.Fatal("retained identical filter", relation.NumInnerRows(), err)
	}
	if err := relation.FilterEqual(1, 0); err != nil || relation.NumInnerRows() != 1 {
		t.Fatal("retained equality filter", relation.NumInnerRows(), err)
	}
	requireSieveFact(t, relation, Fact{1, 0, 1}, true)
}

func TestSieveIgnoredFiltersDoNotValidateIrrelevantValuesOrDomains(t *testing.T) {
	relation, err := NewSieveRelation(sieveSignature(t, 2, 3), []bool{false, true})
	if err != nil {
		t.Fatal(err)
	}
	closeSieve(t, relation)
	if _, err = relation.AddFact(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err = relation.FilterEqual(2, 0); err != nil {
		t.Fatal("ignored-column equality should be identity", err)
	}
	if err = relation.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal("one retained column across different domains should be identity", err)
	}
	if relation.NumInnerRows() != 1 {
		t.Fatal("ignored filters changed the inner relation", relation.NumInnerRows())
	}
}

func TestSieveNegationFilterMappings(t *testing.T) {
	signature := sieveSignature(t, 2, 2)
	target, _ := NewSieveRelation(signature, []bool{true, false})
	negated, _ := NewSieveRelation(signature, []bool{true, false})
	closeSieve(t, target, negated)
	_, _ = target.AddFact(Fact{0, 0})
	_, _ = target.AddFact(Fact{1, 0})
	_, _ = negated.AddFact(Fact{1, 1})
	if err := target.FilterByNegation(negated, []int{0}, []int{0}); err != nil {
		t.Fatal(err)
	}
	requireSieveFact(t, target, Fact{0, 1}, true)
	requireSieveFact(t, target, Fact{1, 0}, false)

	identity, _ := NewSieveRelation(signature, []bool{false, true})
	retainedNeg, _ := NewSieveRelation(signature, []bool{true, false})
	closeSieve(t, identity, retainedNeg)
	_, _ = identity.AddFact(Fact{0, 1})
	_, _ = retainedNeg.AddFact(Fact{1, 0})
	if err := identity.FilterByNegation(retainedNeg, []int{0}, []int{0}); err != nil || identity.Empty() {
		t.Fatal("full target to retained negated must be identity", identity.Empty(), err)
	}
}
