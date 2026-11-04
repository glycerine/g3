package fp_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"
)

func TestDatalogParserInvocationArgumentBoundaries(t *testing.T) {
	for _, input := range []string{
		"N 2\n\nP\nP().\n",
		"N 2\n\nQ(x:N)\nQ(\"a\",).\n",
	} {
		t.Run(input, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			err := fp.ParseDatalogString(f.context, input)
			var parseErr *fp.DatalogParseError
			if !errors.As(err, &parseErr) || parseErr.Message != "too many arguments passed to predicate" {
				t.Fatal("invocation diagnostic", err)
			}
			if count := f.ruleCount(t); count != 0 {
				t.Fatal("invalid fact published", count)
			}
		})
	}
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nP\nQ(x:N)\nP.\nQ(\"a\").\n"))
	if count := f.ruleCount(t); count != 2 {
		t.Fatal("valid fact count", count)
	}
}
