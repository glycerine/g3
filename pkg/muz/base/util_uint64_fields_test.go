package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestParseUint64FieldsSpacingCommentsAndEmptyLines(t *testing.T) {
	values, present, err := base.ParseUint64Fields("  1 2  3 # ignored")
	ruleMust(t, err)
	if !present || len(values) != 3 || values[0] != 1 || values[1] != 2 || values[2] != 3 {
		t.Fatal("fields", values, present)
	}
	values, present, err = base.ParseUint64Fields("# comment")
	ruleMust(t, err)
	if present || values != nil {
		t.Fatal("comment-only", values, present)
	}
	values, present, err = base.ParseUint64Fields("   ")
	ruleMust(t, err)
	if !present || len(values) != 0 {
		t.Fatal("space-only source semantics", values, present)
	}
}
