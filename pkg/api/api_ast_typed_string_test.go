package api

import (
	"strings"
	"testing"
)

func TestAPITypedASTStringFormsUseSelectedMode(t *testing.T) {
	f := newAPITypedASTFixture(t)
	for _, test := range []struct {
		name string
		call func(APIASTPrintMode) (string, error)
		want string
	}{
		{"ast", func(mode APIASTPrintMode) (string, error) { return f.context.ASTString(f.application, mode) }, "typed-f"},
		{"sort", func(mode APIASTPrintMode) (string, error) { return f.context.SortString(f.sort, mode) }, "TypedU"},
		{"function", func(mode APIASTPrintMode) (string, error) { return f.context.FunctionString(f.declaration, mode) }, "typed-f"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, mode := range []APIASTPrintMode{APIASTPrintSMTFull, APIASTPrintSMT2} {
				text, err := test.call(mode)
				if err != nil || !strings.Contains(text, test.want) {
					t.Fatal(mode, text, err)
				}
			}
		})
	}
}
