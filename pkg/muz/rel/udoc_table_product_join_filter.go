package rel

// JoinUDocFilter applies a raw DOC relation membership condition to a product
// while preserving its finite component and original column layout.
func (p *UDocTableProduct) JoinUDocFilter(other *TypedUDocRelation, productColumns, docColumns []uint32) (*UDocTableProduct, error) {
	if p.open() != nil || other == nil || other.open() != nil || other.FormulaChecked() || p.doc.inner.factory != other.factory || len(productColumns) != len(docColumns) {
		return nil, ErrUDocTableProduct
	}
	if err := p.productJoinTableReady(); err != nil {
		return nil, err
	}
	for i, column := range productColumns {
		otherColumn := docColumns[i]
		if column >= uint32(len(p.doc.sorts)) || otherColumn >= uint32(len(other.sorts)) || p.doc.sorts[column] != other.sorts[otherColumn] {
			return nil, ErrUDocTableProduct
		}
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.fullDoc, err = p.doc.JoinUDoc(other, productColumns, docColumns)
	if err != nil {
		return a.finish(err)
	}
	removed := make([]uint32, len(other.sorts))
	base := uint32(len(p.doc.sorts))
	for i := range removed {
		removed[i] = base + uint32(i)
	}
	a.product.doc, err = a.fullDoc.Project(removed)
	if err != nil {
		return a.finish(err)
	}
	a.product.table, err = p.table.CloneFinite()
	return a.finish(err)
}
