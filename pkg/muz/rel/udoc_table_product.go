package rel

// Source: dl_product_relation.cpp product_relation storage, membership,
// componentwise projection and formula. This concrete product joins raw DOC
// with an ordinary finite table; it is not finite_product_relation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrUDocTableProduct = errors.New("muz/rel: invalid DOC/table product")

// UDocTableProduct owns two independent abstractions over the same signature.
// The DOC sieve pins every original sort, including its ignored Bool columns.
// The relation manager and DOC factory are borrowed and outlive this owner.
type UDocTableProduct struct {
	manager *RelationManager
	doc     *UDocSieve
	table   FiniteTable
}

func (p *UDocTableProduct) open() error {
	if p == nil || p.manager == nil || p.manager.open() != nil || p.doc == nil || p.doc.open() != nil || p.table == nil {
		return ErrUDocTableProduct
	}
	return nil
}

func (p *UDocTableProduct) Sorts() []ast.ASTHandle {
	if p.open() != nil {
		return nil
	}
	return p.doc.Sorts()
}

// Doc and Table return borrowed component owners for concrete source dispatch.
func (p *UDocTableProduct) Doc() *UDocSieve {
	if p.open() != nil {
		return nil
	}
	return p.doc
}
func (p *UDocTableProduct) Table() FiniteTable {
	if p.open() != nil {
		return nil
	}
	return p.table
}

func (p *UDocTableProduct) Close() error {
	if p == nil {
		return nil
	}
	if p.table != nil {
		if err := p.table.Close(); err != nil {
			return err
		}
		p.table = nil
	}
	if p.doc != nil {
		if err := p.doc.Close(); err != nil {
			return err
		}
		p.doc = nil
	}
	p.manager = nil
	return nil
}

// Empty preserves source product_relation::empty: any empty component proves
// emptiness, but conflicting nonempty components are not intersected here.
func (p *UDocTableProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	empty, err := p.doc.Empty()
	if err != nil || empty {
		return empty, err
	}
	count, err := p.table.Len()
	return count == 0, err
}

func (p *UDocTableProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	found, err := p.doc.ContainsFact(values)
	if err != nil || !found {
		return found, err
	}
	fact, err := p.manager.EncodeFact(p.doc.sorts, values)
	if err != nil {
		return false, err
	}
	return p.table.Contains(fact)
}

func (p *UDocTableProduct) Clone() (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product.doc, err = p.doc.Clone()
	if err == nil {
		a.product.table, err = p.table.CloneFinite()
	}
	return a.finish(err)
}

func (p *UDocTableProduct) Project(removed []uint32) (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product.doc, err = p.doc.Project(removed)
	if err == nil {
		columns := make([]int, len(removed))
		for i, column := range removed {
			columns[i] = int(column)
		}
		a.product.table, err = finiteTableProject(p.table, columns)
	}
	return a.finish(err)
}
