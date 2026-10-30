package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTContextQuantifierIdentifiersPreserveSourceSymbols(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	qid, skid, err := f.context.QuantifierIdentifiers(f.value)
	if err != nil || qid != "source-qid" || skid != "source-skid" {
		t.Fatal(qid, skid, err)
	}
	plain, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{f.sort},
		Names: []usymbol.Symbol{usymbol.Numerical(9)}, Body: f.truth,
	})
	plain = f.pin(t, plain, err)
	qid, skid, err = f.context.QuantifierIdentifiers(plain)
	if err != nil || qid != "" || skid != "" {
		t.Fatal(qid, skid, err)
	}
	if _, _, err = f.context.QuantifierIdentifiers(f.truth); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}
