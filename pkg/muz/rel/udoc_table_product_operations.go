package rel

// Source: dl_product_relation.cpp diagonal aligned_union_fn and mutator_fn,
// with dl_sieve_relation.cpp's concrete DOC mask dispatch. Cross-component
// refinements require both directional unions; finite<-DOC is unavailable.

import "github.com/glycerine/g3/pkg/ast"

func (p *UDocTableProduct) EmptyLike() (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	inner, err := p.doc.inner.factory.NewEmpty(p.doc.inner.sorts)
	a.product.doc, err = finishUDocSieve(p.doc.sorts, p.doc.innerColumns, inner, err)
	if err == nil && p.table != nil {
		a.product.table, err = emptyFiniteLike(p.table)
	}
	return a.finish(err)
}

func (p *UDocTableProduct) sameLayout(other *UDocTableProduct) bool {
	if p.open() != nil || other.open() != nil || p.manager != other.manager || !p.doc.inner.sameShape(other.doc.inner) || len(p.doc.sorts) != len(other.doc.sorts) || rawUDocProductFiniteKind(p.table) != rawUDocProductFiniteKind(other.table) {
		return false
	}
	for i, sort := range p.doc.sorts {
		if sort != other.doc.sorts[i] || p.doc.innerColumns[i] != other.doc.innerColumns[i] {
			return false
		}
	}
	return true
}

func (p *UDocTableProduct) AddFact(values []ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.doc.AddFact(values); err != nil {
		return err
	}
	if p.table == nil {
		return nil
	}
	fact, err := p.manager.EncodeFact(p.doc.sorts, values)
	if err != nil {
		return err
	}
	_, err = p.table.Add(fact)
	return err
}

// UnionFrom requires aligned concrete components. Compiler delta registers
// are distinct owners; each receives the source's fresh component delta by
// union, preserving any facts already accumulated by earlier instructions.
func (p *UDocTableProduct) UnionFrom(source, delta *UDocTableProduct) (resultErr error) {
	if !p.sameLayout(source) || delta != nil && (!p.sameLayout(delta) || delta == p || delta == source) {
		return ErrUDocTableProduct
	}
	if delta == nil {
		if err := p.doc.inner.UnionFrom(source.doc.inner, nil); err != nil {
			return err
		}
		if p.table == nil {
			return nil
		}
		return finiteTableUnion(p.table, source.table, nil)
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product, err = p.EmptyLike()
	if err != nil {
		return a.discard(err)
	}
	defer func() { resultErr = a.discard(resultErr) }()
	if err = p.doc.inner.UnionFrom(source.doc.inner, a.product.doc.inner); err != nil {
		return err
	}
	if err = delta.doc.inner.UnionFrom(a.product.doc.inner, nil); err != nil {
		return err
	}
	if p.table != nil {
		if err = finiteTableUnion(p.table, source.table, a.product.table); err != nil {
			return err
		}
		return finiteTableUnion(delta.table, a.product.table, nil)
	}
	return nil
}

func (p *UDocTableProduct) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if column >= uint32(len(p.doc.sorts)) {
		return ErrUDocTableProduct
	}
	var encoded Element
	if p.table != nil {
		var err error
		encoded, err = p.manager.EncodeValue(p.doc.sorts[column], value)
		if err != nil {
			return err
		}
	}
	if inner := p.doc.sigToInner[column]; inner >= 0 {
		if err := p.doc.inner.FilterEqual(uint32(inner), value); err != nil {
			return err
		}
	}
	if p.table == nil {
		return nil
	}
	return finiteTableFilterEqual(p.table, encoded, int(column))
}

func (p *UDocTableProduct) FilterIdentical(columns []uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	var selected []uint32
	finite := make([]int, len(columns))
	for i, column := range columns {
		if column >= uint32(len(p.doc.sorts)) || p.doc.sorts[column] != p.doc.sorts[columns[0]] {
			return ErrUDocTableProduct
		}
		finite[i] = int(column)
		if inner := p.doc.sigToInner[column]; inner >= 0 {
			selected = append(selected, uint32(inner))
		}
	}
	if len(selected) >= 2 {
		if err := p.doc.inner.FilterIdentical(selected); err != nil {
			return err
		}
	}
	if len(columns) < 2 {
		return nil
	}
	if p.table == nil {
		return nil
	}
	return finiteTableFilterIdentical(p.table, finite)
}
