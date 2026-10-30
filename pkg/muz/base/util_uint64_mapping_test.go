package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestParseUint64MappingPreservesRawName(t *testing.T) {
	number, name, present, err := base.ParseUint64Mapping("27 alpha beta # comment")
	ruleMust(t, err)
	if !present || number != 27 || name != "alpha beta " {
		t.Fatal("mapping", number, name, present)
	}
	if _, _, _, err = base.ParseUint64Mapping("27x name"); err == nil {
		t.Fatal("missing separator accepted")
	}
	if _, _, present, err = base.ParseUint64Mapping("# empty"); err != nil || present {
		t.Fatal("comment line", present, err)
	}
}
