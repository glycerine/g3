package rel

// Source: dl_product_relation.cpp join_fn::init matches the actual DOC kind
// through a sieve and supplies a full finite relation only for the unmatched
// table component. This is the live raw-DOC/product case, not product/product.

// JoinUDocProduct joins raw unary DOC with an existing two-component product.
// The product's borrowed manager and shared DOC factory must remain live.
func JoinUDocProduct(doc *TypedUDocRelation, product *UDocTableProduct, docColumns, productColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocProduct(doc, product, docColumns, productColumns, true)
}

// JoinProductUDoc preserves the opposite source operand and column order.
func JoinProductUDoc(product *UDocTableProduct, doc *TypedUDocRelation, productColumns, docColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocProduct(doc, product, docColumns, productColumns, false)
}

func joinUDocProduct(doc *TypedUDocRelation, product *UDocTableProduct, docColumns, productColumns []uint32, docFirst bool) (*UDocTableProduct, error) {
	if product.open() != nil || doc == nil || doc.open() != nil || doc.FormulaChecked() || doc.factory != product.doc.inner.factory || len(doc.sorts) != 1 || len(docColumns) != len(productColumns) || rawUDocProductFiniteKind(product.table) == 0 {
		return nil, ErrUDocTableProduct
	}
	// Full table construction is limited to the existing table-encodable
	// unary DOC domain. The product may already have projected away its Bool
	// columns; preserve its actual mask instead of reclassifying the sorts.
	signature, err := tableRelationSignature(product.manager, doc.sorts)
	if err != nil {
		return nil, err
	}
	left, right := make([]int, len(docColumns)), make([]int, len(docColumns))
	for i, column := range docColumns {
		other := productColumns[i]
		if column >= uint32(len(doc.sorts)) || other >= uint32(len(product.doc.sorts)) || doc.sorts[column] != product.doc.sorts[other] {
			return nil, ErrUDocTableProduct
		}
		left[i], right[i] = int(column), int(other)
	}
	a := newUDocTableProductAttempt(product.manager)
	if docFirst {
		a.product.doc, err = doc.JoinSieve(product.doc, docColumns, productColumns)
	} else {
		a.product.doc, err = product.doc.JoinUDoc(doc, productColumns, docColumns)
	}
	if err != nil {
		return a.finish(err)
	}
	full, err := a.fullUnaryFinite(signature, product.table)
	if err != nil {
		return a.finish(err)
	}
	if docFirst {
		a.product.table, err = finiteTableJoin(full, product.table, left, right)
	} else {
		a.product.table, err = finiteTableJoin(product.table, full, right, left)
	}
	return a.finish(err)
}
