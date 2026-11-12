package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2FunctionDeclarationReferenceErrors(t *testing.T) {
	for _, example := range []struct {
		name, declarations, term, diagnostic string
	}{
		{"unknown", "", "(_ as-array missing)", "unknown function"},
		{"popped-declaration", "(push 1) (declare-fun f (Bool) Bool) (pop 1)", "(_ as-array f)", "unknown function"},
		{"ambiguous", "(declare-fun f (Bool) Bool) (declare-fun f (Int) Bool)", "(_ as-array f)", "ambiguous function declaration reference"},
		{"macro", "(define-fun f ((x Bool)) Bool x)", "(_ as-array f)", "macros"},
		{"signed-macro", "(define-fun f ((x Bool)) Bool x)", "(_ as-array (f (Bool) Bool))", "macros"},
		// func_decls::find returns a sole declaration without checking its
		// signature, so only an overloaded name rejects a wrong signature.
		{"wrong-signature", "(declare-fun f (Bool) Bool) (declare-fun f (Real) Bool)", "(_ as-array (f (Int) Bool))", "unknown function"},
		{"indexed-user-function", "(declare-fun f (Bool) Bool)", "(_ as-array ((_ f 1) (Bool) Bool))", "unknown builtin function"},
		{"empty-index-list", "", "(_ as-array)", "index expected"},
		{"empty-builtin-index", "", "(_ as-array ((_ extract) ((_ BitVec 4)) (_ BitVec 2)))", "index expected"},
	} {
		t.Run(example.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			var output bytes.Buffer
			script := "(set-logic ALL) " + example.declarations + " (assert (= " + example.term + " " + example.term + "))"
			err = ParseCommands(context, strings.NewReader(script), &output)
			if err == nil || !strings.Contains(err.Error(), example.diagnostic) {
				t.Fatalf("declaration reference diagnostic: %v, want %q", err, example.diagnostic)
			}
		})
	}
}
