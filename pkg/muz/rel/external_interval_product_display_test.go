package rel

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type externalIntervalDisplayRefusal struct{ err error }

func (w externalIntervalDisplayRefusal) Write([]byte) (int, error) { return 0, w.err }

func TestExternalIntervalProductDisplaySourceOrderAndFailures(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			f := newTableRelationFixture(t)
			family, err := f.ast.RegisterArithmeticPlugin()
			astTestMust(t, err)
			integer, err := f.ast.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			one, err := f.ast.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			astTestPin(t, f.ast, one)
			plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{})
			astTestMust(t, err)
			product, selected, err := NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integer}, kinds)
			astTestMust(t, err)
			if !selected || product == nil {
				t.Fatal("mixed product was not selected")
			}
			astTestMust(t, product.AddFact([]ast.ASTHandle{one}))

			var external, interval bytes.Buffer
			astTestMust(t, product.external.Display(&external))
			astTestMust(t, product.interval.Display(&interval))
			want := "Product of the following relations:\n"
			if kinds[0] == "external_relation" {
				want += external.String() + interval.String()
			} else {
				want += interval.String() + external.String()
			}
			var got bytes.Buffer
			astTestMust(t, product.Display(&got))
			if got.String() != want {
				t.Fatalf("display order/content\n got: %q\nwant: %q", got.String(), want)
			}

			refusal := errors.New("mixed product display refused")
			if err = product.Display(externalIntervalDisplayRefusal{err: refusal}); !errors.Is(err, refusal) {
				t.Fatalf("writer error = %v, want %v", err, refusal)
			}
			if err = product.Display(nil); !errors.Is(err, ErrTableRelation) {
				t.Fatalf("nil writer error = %v", err)
			}
			astTestMust(t, product.Close())
			if err = product.Display(io.Discard); !errors.Is(err, ErrTableRelation) {
				t.Fatalf("closed product display error = %v", err)
			}
			astTestMust(t, plugin.Close())
		})
	}
}
