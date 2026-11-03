package cmd_context

import "testing"

func TestWPARelationLineWhitespaceCommentsAndErrors(t *testing.T) {
	values, present, err := parseWPARelationLine("  10  20 30 # tuple")
	if err != nil || !present || len(values) != 3 || values[0] != 10 || values[2] != 30 {
		t.Fatal(values, present, err)
	}
	if values, present, err = parseWPARelationLine(" # empty"); err != nil || !present || len(values) != 0 {
		t.Fatal(values, present, err)
	}
	if values, present, err = parseWPARelationLine("# absent"); err != nil || present || values != nil {
		t.Fatal(values, present, err)
	}
	if _, _, err = parseWPARelationLine("1 nope"); err == nil {
		t.Fatal("accepted malformed relation tuple")
	}
}

func TestWPARelationLineRejectsNonSourceSeparatorsAndOverflow(t *testing.T) {
	for _, line := range []string{"1\t2", "\t1", "1\v2", "+1", "18446744073709551616"} {
		if _, _, err := parseWPARelationLine(line); err == nil {
			t.Errorf("accepted non-source relation grammar %q", line)
		}
	}
	values, present, err := parseWPARelationLine("18446744073709551615\rignored")
	if err != nil || !present || len(values) != 1 || values[0] != ^uint64(0) {
		t.Fatal(values, present, err)
	}
}
