package rel

// Source: sieve_relation_plugin::mk_rename_fn collects the selected-column
// sub-permutation, then relation_manager decomposes it into source cycles.

import "github.com/glycerine/g3/pkg/ast"

func (p *UDocTableProduct) Rename(cycle []uint32) (*UDocTableProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation := make([]uint32, len(p.doc.sorts))
	seen := make([]bool, len(permutation))
	finiteCycle := make([]int, len(cycle))
	for i := range permutation {
		permutation[i] = uint32(i)
	}
	for i, column := range cycle {
		if column >= uint32(len(permutation)) || seen[column] {
			return nil, ErrUDocTableProduct
		}
		seen[column], finiteCycle[i] = true, int(column)
	}
	for i, column := range cycle {
		permutation[column] = cycle[(i+1)%len(cycle)]
	}
	sorts := make([]ast.ASTHandle, len(permutation))
	mask := make([]bool, len(permutation))
	var innerPermutation []uint32
	for destination, source := range permutation {
		sorts[destination], mask[destination] = p.doc.sorts[source], p.doc.innerColumns[source]
		if mask[destination] {
			innerPermutation = append(innerPermutation, uint32(p.doc.sigToInner[source]))
		}
	}
	a := newUDocTableProductAttempt(p.manager)
	inner, err := p.doc.inner.Clone()
	if inner != nil {
		a.inners = append(a.inners, inner)
	}
	if err != nil {
		return a.finish(err)
	}
	for {
		local, found := removePermutationCycle(innerPermutation)
		if !found {
			break
		}
		next, cause := inner.Rename(local)
		if next != nil {
			a.inners = append(a.inners, next)
		}
		if cause != nil {
			return a.finish(cause)
		}
		inner = next
	}
	a.product.doc, err = AdoptUDocSieve(sorts, mask, inner)
	if err != nil {
		return a.finish(err)
	}
	// Only the final inner transfers into the new sieve. Earlier cycle
	// intermediates remain in the attempt until all construction has finished.
	a.inners = a.inners[:len(a.inners)-1]
	a.product.table, err = finiteTableRename(p.table, finiteCycle)
	return a.finish(err)
}
