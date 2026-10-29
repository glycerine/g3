package api

import "testing"

func TestAPIStatisticsStringUsesSMT2AndDropsTrailingNewline(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	object, err := newAPIStatistics(f.c.smtContextState, apiStatisticsFixture())
	astTestMust(t, err)
	defer func() { astTestMust(t, object.Close()) }()
	text, err := object.String()
	astTestMust(t, err)
	if text != "(:floating-counter 2.50\n :integer-counter  7)" {
		t.Fatalf("statistics display = %q", text)
	}
}
