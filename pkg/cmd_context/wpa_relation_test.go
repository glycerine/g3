package cmd_context

import "testing"

func TestWPARelationLineWhitespaceCommentsAndErrors(t *testing.T) {
	values, present, err := parseWPARelationLine("  10  20 30 # tuple")
	if err != nil || !present || len(values) != 3 || values[0] != 10 || values[2] != 30 {
		t.Fatal(values, present, err)
	}
	if values, present, err = parseWPARelationLine(" # empty"); err != nil || present || values != nil {
		t.Fatal(values, present, err)
	}
	if _, _, err = parseWPARelationLine("1 nope"); err == nil {
		t.Fatal("accepted malformed relation tuple")
	}
}
