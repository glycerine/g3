package smtcore

import (
	"testing"
)

func TestSMTExtendedRationalArithmeticAndUndefinedInfinity(t *testing.T) {
	minusInfinity := smtInfiniteExtended(false)
	plusInfinity := smtInfiniteExtended(true)
	if minusInfinity.testCmp(&plusInfinity) >= 0 || minusInfinity.testString() != "-oo" || plusInfinity.testString() != "oo" {
		t.Fatal("extended infinity ordering or formatting")
	}
	if err := plusInfinity.add(testQ(), &minusInfinity); err == nil {
		t.Fatal("opposite infinity addition accepted")
	}
	plusInfinity = smtInfiniteExtended(true)
	if err := plusInfinity.sub(testQ(), &plusInfinity); err == nil {
		t.Fatal("same infinity subtraction accepted")
	}
	value := oiFinite(-3, 2)
	other := oiFinite(5, 3)
	if err := value.add(testQ(), &other); err != nil || !oiIs(&value.value, 1, 6) {
		t.Fatalf("finite addition = %s, err %v", value.testString(), err)
	}
	value.mul(testQ(), &other)
	if !oiIs(&value.value, 5, 18) {
		t.Fatalf("finite multiplication = %s", value.testString())
	}
	if err := value.inverse(testQ()); err != nil || !oiIs(&value.value, 18, 5) {
		t.Fatalf("finite inverse = %s, err %v", value.testString(), err)
	}
}

func TestSMTExtendedRationalZeroInfinityPowerAndNegation(t *testing.T) {
	zero := smtExtendedZero()
	infinity := smtInfiniteExtended(false)
	infinity.mul(testQ(), &zero)
	if !infinity.zero() {
		t.Fatal("zero times infinity must be zero")
	}
	if err := infinity.inverse(testQ()); err == nil {
		t.Fatal("zero inverse accepted")
	}
	infinity = smtInfiniteExtended(false)
	infinity.power(testQ(), 2)
	if infinity.kind != smtExtendedPlusInfinity {
		t.Fatal("even power of minus infinity")
	}
	infinity = smtInfiniteExtended(false)
	infinity.power(testQ(), 3)
	if infinity.kind != smtExtendedMinusInfinity {
		t.Fatal("odd power of minus infinity")
	}
	value := oiFinite(-2, 3)
	value.power(testQ(), 3)
	if !oiIs(&value.value, -8, 27) {
		t.Fatalf("finite power = %s", value.testString())
	}
	value.neg(testQ())
	if !oiIs(&value.value, 8, 27) {
		t.Fatalf("finite negation = %s", value.testString())
	}
}
