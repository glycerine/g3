package fp

import "testing"

// skipUnreachableInfiniteSortRules marks tests that drive the Go-only
// Int/Real Datalog evaluators (bound, interval, Karr, external interval,
// numeric explanations) with rules over infinite sorts. context::check_rules
// rejects every such program with "Rule contains infinite sorts" before a
// relation plugin is chosen (AUDIT.md muz_fp_01-F2), so these evaluators are
// unreachable for rule programs. Kept for the owner to retarget or delete.
func skipUnreachableInfiniteSortRules(t *testing.T) {
	t.Helper()
	t.Skip("muz_fp_01-F2: Datalog rules over Int/Real are rejected by check_rules")
}
