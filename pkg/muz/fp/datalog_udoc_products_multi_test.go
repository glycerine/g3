package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUDocProductsThreeWayProductDocFiniteJoin(t *testing.T) {
	for _, productFirst := range []bool{true, false} {
		name := "doc-finite-product"
		if productFirst {
			name = "product-doc-finite"
		}
		t.Run(name, func(t *testing.T) {
			f := newPublicUDocProductFixture(t, "hashtable", false, false)
			newPredicate := func(name string, sorts []ast.ASTHandle) ast.ASTHandle {
				n, err := f.m.Symbols.Intern(name)
				fragmentMust(t, err)
				declaration := f.pin(f.m.MkFuncDecl(n, sorts, f.boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, f.e.RegisterPredicate(declaration))
				return declaration
			}
			intermediate := newPredicate("wide-product-intermediate", []ast.ASTHandle{f.bits, f.boolean})
			doc2 := newPredicate("wide-product-doc-2", []ast.ASTHandle{f.bits})
			finite2 := newPredicate("wide-product-finite-2", []ast.ASTHandle{f.boolean, f.bits})
			fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1}))
			fragmentMust(t, f.e.AddTableFact(doc2, []uint32{1}))
			fragmentMust(t, f.e.AddTableFact(finite2, []uint32{1, 1}))

			// Build an actual product dependency from the first DOC/table pair.
			firstBody := f.pin(f.m.MkAnd(f.app(f.doc, f.x), f.app(f.finite, f.b, f.x)))
			firstRule := f.pin(f.m.MkImplies(firstBody, f.app(intermediate, f.x, f.b)))
			fragmentMust(t, f.e.AddRule(firstRule, "wide-product-first"))

			productTail := f.app(intermediate, f.x, f.b)
			docTail := f.app(doc2, f.x)
			finiteTail := f.app(finite2, f.b, f.x)
			left, right := productTail, docTail
			if !productFirst {
				left, right = docTail, finiteTail
			}
			body := f.pin(f.m.MkAnd(left, right))
			last := finiteTail
			if !productFirst {
				last = productTail
			}
			body = f.pin(f.m.MkAnd(body, last))
			wideRule := f.pin(f.m.MkImplies(body, f.app(f.head, f.x, f.b)))
			fragmentMust(t, f.e.AddRule(wideRule, "wide-product-second"))

			f.checkRows(lbool.True, [2]uint32{1, 1})
			program := f.e.udocDatalog
			if program == nil || program.products == nil || program.products.result[f.head] == nil {
				t.Fatal("three-way body did not execute through a concrete DOC/table product")
			}
		})
	}
}
