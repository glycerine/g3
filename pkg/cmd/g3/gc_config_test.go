package main

import "testing"

func TestConfigureGCAppliesDefaultWhenGOGCUnset(t *testing.T) {
	applied := -1
	changed := configureGC(func(string) (string, bool) { return "", false }, func(percent int) int {
		applied = percent
		return 100
	})
	if !changed || applied != defaultGCPercent {
		t.Fatal(changed, applied)
	}
}

func TestConfigureGCRespectsAnyUserChoice(t *testing.T) {
	// An empty or "off" value is still the user's decision.
	for _, value := range []string{"50", "off", ""} {
		changed := configureGC(func(name string) (string, bool) {
			if name != "GOGC" {
				t.Fatal("looked up", name)
			}
			return value, true
		}, func(int) int {
			t.Fatal("overrode GOGC", value)
			return 0
		})
		if changed {
			t.Fatal("reported a change for GOGC", value)
		}
	}
}
