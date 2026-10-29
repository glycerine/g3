package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRetainedDecimalValueEndpointSelection(t *testing.T) {
	f := newRankedValueFixture(t, true)
	interval, err := NewBinaryInterval(f.alpha.root.owner.intervals)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = interval.Close() })
	lower, upper := numeral.NewBinaryInt(1), numeral.NewBinaryIntPower(3, 1)
	if err := interval.SetFinite(&lower, &upper, false, false); err != nil {
		t.Fatal(err)
	}
	selected, err := decimalAlgebraicEndpoint(interval)
	if err != nil || selected != &interval.upper {
		t.Fatal("integer lower must select upper", err)
	}
	lower = numeral.NewBinaryIntPower(5, 2)
	if err := interval.SetFinite(&lower, &upper, false, false); err != nil {
		t.Fatal(err)
	}
	selected, err = decimalAlgebraicEndpoint(interval)
	if err != nil || selected != &interval.lower {
		t.Fatal("fractional lower must be selected", err)
	}
}
