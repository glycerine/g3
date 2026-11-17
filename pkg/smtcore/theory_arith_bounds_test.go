package smtcore

import (
	"math/big"
	"testing"
)

func TestSMTArithmeticValueLexicographicAndAtomComplements(t *testing.T) {
	finite := testArithValue(big.NewRat(4, 1), nil)
	strict := testArithValue(big.NewRat(4, 1), big.NewRat(1, 1))
	infinite := testArithValue(nil, nil)
	infinite.infinity = smtQInt(1)
	if c1, _ := finite.cmp(testQ(), &strict); c1 >= 0 {
		t.Fatal("arithmetic value lexicographic order")
	}
	if c2, _ := strict.cmp(testQ(), &infinite); c2 >= 0 {
		t.Fatal("arithmetic value lexicographic order")
	}
	epsilon := testArithValue(nil, big.NewRat(1, 1))
	lower, err := newSMTArithmeticAtom(testQ(), 3, 7, &finite, smtArithmeticLowerAtom)
	if err != nil {
		t.Fatal(err)
	}
	if err = lower.assign(testQ(), false, &epsilon); err != nil {
		t.Fatal(err)
	}
	if lower.kind != smtArithmeticUpperBound || testBig(&lower.value.rational).Cmp(big.NewRat(4, 1)) != 0 || testBig(&lower.value.infinitesimal).Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("false lower atom complement")
	}
	upper, err := newSMTArithmeticAtom(testQ(), 4, 8, &finite, smtArithmeticUpperAtom)
	if err != nil {
		t.Fatal(err)
	}
	if err = upper.assign(testQ(), false, &epsilon); err != nil {
		t.Fatal(err)
	}
	if upper.kind != smtArithmeticLowerBound || testBig(&upper.value.infinitesimal).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("false upper atom complement")
	}
}

func TestSMTArithmeticAntecedentAndDerivedCoefficients(t *testing.T) {
	derived := &smtArithmeticJustifiedDerivedBound{}
	literal := NewSMTLiteral(5, false)
	if err := derived.pushLiteral(testQ(), literal, testQR(big.NewRat(2, 1))); err != nil {
		t.Fatal(err)
	}
	if err := derived.pushLiteral(testQ(), literal, testQR(big.NewRat(3, 1))); err != nil {
		t.Fatal(err)
	}
	var out smtArithmeticAntecedents
	if err := derived.pushJustification(testQ(), &out, testQR(big.NewRat(-2, 1)), true); err != nil {
		t.Fatal(err)
	}
	if len(out.literals) != 1 || out.literals[0] != literal || testBig(&out.literalCoefficients[0]).Cmp(big.NewRat(-10, 1)) != 0 || out.numParameters() != 2 {
		t.Fatal("scaled derived antecedent")
	}
	out.reset(testQ())
	if len(out.literals) != 0 || out.numParameters() != 0 {
		t.Fatal("antecedent reset")
	}
	atom, err := newSMTArithmeticAtom(testQ(), 6, 1, newSMTArithmeticValuePtr(0, 0), smtArithmeticLowerAtom)
	if err != nil {
		t.Fatal(err)
	}
	if err = atom.assign(testQ(), true, newSMTArithmeticValuePtr(0, 1)); err != nil {
		t.Fatal(err)
	}
	if err = atom.pushJustification(testQ(), &out, testQR(big.NewRat(7, 1)), false); err != nil {
		t.Fatal(err)
	}
	if len(out.literals) != 1 || out.literals[0] != NewSMTLiteral(6, false) || out.numParameters() != 0 {
		t.Fatal("atom antecedent")
	}
}

func newSMTArithmeticValuePtr(rational, infinitesimal int64) *smtArithmeticValue {
	value := testArithValue(big.NewRat(rational, 1), big.NewRat(infinitesimal, 1))
	return &value
}

func TestSMTArithmeticBoundTrailEncoding(t *testing.T) {
	bound := &smtArithmeticBound{variable: 9}
	trail, err := newSMTArithmeticBoundTrail(9, bound, true)
	if err != nil {
		t.Fatal(err)
	}
	if !trail.upper() || trail.variable() != 9 || trail.old != bound {
		t.Fatal("upper bound trail encoding")
	}
	trail, err = newSMTArithmeticBoundTrail(10, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if trail.upper() || trail.variable() != 10 {
		t.Fatal("lower bound trail encoding")
	}
}

func TestSMTArithmeticEqualityBoundAndAccumulation(t *testing.T) {
	root := &SMTEnode{}
	left, right := &SMTEnode{root: root}, &SMTEnode{root: root}
	value := testArithValue(big.NewRat(3, 1), nil)
	equality, err := newSMTArithmeticEqualityBound(testQ(), 2, &value, smtArithmeticLowerBound, left, right)
	if err != nil {
		t.Fatal(err)
	}
	var out smtArithmeticAntecedents
	if err = equality.pushJustification(testQ(), &out, testQR(big.NewRat(4, 1)), true); err != nil {
		t.Fatal(err)
	}
	if len(out.equalities) != 1 || testBig(&out.equalityCoefficients[0]).Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("equality antecedent")
	}
	wrong := &SMTEnode{}
	wrong.root = wrong
	if _, err = newSMTArithmeticEqualityBound(testQ(), 2, &value, smtArithmeticLowerBound, left, wrong); err == nil {
		t.Fatal("different equality roots accepted")
	}
	atom, err := newSMTArithmeticAtom(testQ(), 8, 2, &value, smtArithmeticLowerAtom)
	if err != nil {
		t.Fatal(err)
	}
	if err = atom.assign(testQ(), true, newSMTArithmeticValuePtr(0, 1)); err != nil {
		t.Fatal(err)
	}
	target := &smtArithmeticDerivedBound{}
	seenLiterals := make(map[int32]bool)
	seenEqualities := make(map[SMTEnodePair]bool)
	if err = smtAccumulateArithmeticJustification(testQ(), atom, target, testQR(big.NewRat(2, 1)), false, seenLiterals, seenEqualities); err != nil {
		t.Fatal(err)
	}
	if err = smtAccumulateArithmeticJustification(testQ(), atom, target, testQR(big.NewRat(3, 1)), false, seenLiterals, seenEqualities); err != nil {
		t.Fatal(err)
	}
	if len(target.literals) != 1 {
		t.Fatal("non-proof accumulation did not suppress duplicate")
	}
}

func TestSMTNormalizeArithmeticIntegerBound(t *testing.T) {
	for _, row := range []struct {
		rational *big.Rat
		epsilon  int64
		kind     smtArithmeticBoundKind
		want     int64
	}{
		{big.NewRat(3, 2), 0, smtArithmeticLowerBound, 2},
		{big.NewRat(3, 2), 0, smtArithmeticUpperBound, 1},
		{big.NewRat(-3, 2), 0, smtArithmeticLowerBound, -1},
		{big.NewRat(-3, 2), 0, smtArithmeticUpperBound, -2},
		{big.NewRat(2, 1), 1, smtArithmeticLowerBound, 3},
		{big.NewRat(2, 1), -1, smtArithmeticUpperBound, 1},
	} {
		value := testArithValue(row.rational, big.NewRat(row.epsilon, 1))
		got := newSMTArithmeticValue()
		err := smtNormalizeArithmeticIntegerBound(testQ(), &value, row.kind, &got)
		if err != nil {
			t.Fatal(err)
		}
		if testBig(&got.rational).Cmp(big.NewRat(row.want, 1)) != 0 || got.infinitesimal.Sign() != 0 {
			t.Fatalf("normalize %s eps %d kind %d = %s", row.rational, row.epsilon, row.kind, testBig(&got.rational))
		}
	}
}
