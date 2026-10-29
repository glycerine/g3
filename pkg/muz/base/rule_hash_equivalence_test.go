package base_test

import "testing"

func TestRuleStructuralEqualityAndSourceHash(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	first := f.rule("P", []string{"Q"}, []bool{false})
	defer func() { ruleMust(t, first.Close()) }()
	second := f.rule("P", []string{"Q"}, []bool{false})
	defer func() { ruleMust(t, second.Close()) }()
	negative := f.rule("P", []string{"Q"}, []bool{true})
	defer func() { ruleMust(t, negative.Close()) }()
	if equal, err := first.StructurallyEqual(second); err != nil || !equal {
		t.Fatalf("equal rules = %v,%v", equal, err)
	}
	firstHash, err := first.Hash()
	ruleMust(t, err)
	secondHash, err := second.Hash()
	ruleMust(t, err)
	if firstHash != secondHash {
		t.Fatalf("equal hashes = %d,%d", firstHash, secondHash)
	}
	if equal, err := first.StructurallyEqual(negative); err != nil || equal {
		t.Fatalf("sign-sensitive equality = %v,%v", equal, err)
	}
	negativeHash, err := negative.Hash()
	ruleMust(t, err)
	if negativeHash == firstHash {
		t.Fatal("tail sign did not contribute to source hash")
	}
}
