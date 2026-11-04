package fp_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserReportsSourceLocation(t *testing.T) {
	f := newDatalogParserFixture(t)
	err := fp.ParseDatalogString(f.context, "N 2\n\nP(x:N)\nP(X) :- !.\n")
	var parseError *fp.DatalogParseError
	if !errors.As(err, &parseError) || parseError.Line != 4 || parseError.Column == 0 {
		t.Fatal("diagnostic", err)
	}
}

func TestDatalogParserRejectsArityAndSortMismatch(t *testing.T) {
	for _, input := range []string{
		"N 2\n\nP(x:N)\nP(X,X).\n",
		"N 2\nI int\n\nP(x:N)\nQ(i:I)\nP(X) :- Q(X).\n",
	} {
		f := newDatalogParserFixture(t)
		err := fp.ParseDatalogString(f.context, input)
		if err == nil || (!strings.Contains(err.Error(), "arguments") && !strings.Contains(err.Error(), "sort")) {
			t.Fatal("unexpected error", err)
		}
	}
}
