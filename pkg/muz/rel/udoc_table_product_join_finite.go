package rel

// Source: dl_product_relation.cpp join_fn::init, where a product's DOC
// component is paired with a full DOC relation and its table component is
// paired with an ordinary finite input.

import "github.com/glycerine/g3/pkg/ast"

// JoinProductFinite joins a product input with a raw finite-table input. The
// product/table component is joined with the table; a full DOC component over
// the finite input signature supplies the unmatched side of the product.
func JoinProductFinite(product *UDocTableProduct, finiteSorts []ast.ASTHandle, finite FiniteTable, productColumns, finiteColumns []uint32) (*UDocTableProduct, error) {
	return joinProductFinite(product, finiteSorts, finite, productColumns, finiteColumns, true)
}

// JoinFiniteProduct preserves the opposite source operand and column order.
func JoinFiniteProduct(manager *RelationManager, finiteSorts []ast.ASTHandle, finite FiniteTable, product *UDocTableProduct, finiteColumns, productColumns []uint32) (*UDocTableProduct, error) {
	if product == nil || manager != product.manager {
		return nil, ErrUDocTableProduct
	}
	return joinProductFinite(product, finiteSorts, finite, productColumns, finiteColumns, false)
}

func joinProductFinite(product *UDocTableProduct, finiteSorts []ast.ASTHandle, finite FiniteTable, productColumns, finiteColumns []uint32, productFirst bool) (*UDocTableProduct, error) {
	if product == nil || product.open() != nil || finite == nil || rawUDocProductFiniteKind(finite) == 0 || len(productColumns) != len(finiteColumns) {
		return nil, ErrUDocTableProduct
	}
	if err := product.productJoinTableReady(); err != nil {
		return nil, err
	}
	if lazy, ok := finite.(*LazySparseTable); ok && (product.manager.lazyFinite == nil || !product.manager.lazyFinite.Owns(lazy)) {
		return nil, ErrUDocTableProduct
	}
	signature, err := tableRelationSignature(product.manager, finiteSorts)
	if err != nil {
		return nil, err
	}
	actual, err := finite.Signature()
	if err != nil || !signature.equal(actual) {
		return nil, ErrUDocTableProduct
	}
	left, right := make([]int, len(productColumns)), make([]int, len(finiteColumns))
	for i, column := range productColumns {
		other := finiteColumns[i]
		if column >= uint32(len(product.doc.sorts)) || other >= uint32(len(finiteSorts)) || product.doc.sorts[column] != finiteSorts[other] {
			return nil, ErrUDocTableProduct
		}
		left[i], right[i] = int(column), int(other)
	}
	a := newUDocTableProductAttempt(product.manager)
	a.fullDoc, err = NewFullUDocSieve(product.doc.inner.factory, finiteSorts)
	if err != nil {
		return a.finish(err)
	}
	if productFirst {
		a.product.doc, err = product.doc.Join(a.fullDoc, productColumns, finiteColumns)
		if err == nil {
			if product.table == nil {
				var signature Signature
				signature, err = tableRelationSignature(product.manager, product.doc.sorts)
				if err == nil {
					var full FiniteTable
					full, err = a.fullUnaryFinite(signature, finite)
					if err == nil {
						a.product.table, err = finiteTableJoin(full, finite, left, right)
					}
				}
			} else {
				a.product.table, err = finiteTableJoin(product.table, finite, left, right)
			}
		}
	} else {
		a.product.doc, err = a.fullDoc.Join(product.doc, finiteColumns, productColumns)
		if err == nil {
			if product.table == nil {
				var signature Signature
				signature, err = tableRelationSignature(product.manager, product.doc.sorts)
				if err == nil {
					var full FiniteTable
					full, err = a.fullUnaryFinite(signature, finite)
					if err == nil {
						a.product.table, err = finiteTableJoin(finite, full, right, left)
					}
				}
			} else {
				a.product.table, err = finiteTableJoin(finite, product.table, right, left)
			}
		}
	}
	return a.finish(err)
}
