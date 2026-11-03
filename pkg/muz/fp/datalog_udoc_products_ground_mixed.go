package fp

// Source dl_compiler.cpp builds a mixed ground head by joining unary constant
// registers, then instr_union infers and unions their product kind. Direct
// insertion into an ordinary table would preserve correlations that the source
// product's independent component unions deliberately lose.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (c *datalogUDocContext) evaluateProductMixedGround(fact *udocProductGroundRule, bitColumn int) (resultErr error) {
	p := c.products
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	sorts := p.program.sorts[fact.head]
	boolColumn := 1 - bitColumn
	// Admission guarantees all BV constant registers select the same raw DOC
	// kind, and all Bool registers select the same finite kind. Their source
	// cache may therefore share constants without changing this concrete join.
	doc, err := c.factory.NewEmpty([]ast.ASTHandle{sorts[bitColumn]})
	if doc != nil {
		a.docs = append(a.docs, doc)
	}
	if err != nil {
		return err
	}
	if _, err = doc.AddFact([]ast.ASTHandle{fact.values[bitColumn]}); err != nil {
		return err
	}
	signature, err := rel.NewSignature([]uint64{2})
	if err != nil {
		return err
	}
	table, err := p.manager.NewDefaultFiniteTable(signature)
	if table != nil {
		a.tables = append(a.tables, table)
	}
	if err != nil {
		return err
	}
	boolSorts := []ast.ASTHandle{sorts[boolColumn]}
	encoded, err := p.manager.EncodeFact(boolSorts, []ast.ASTHandle{fact.values[boolColumn]})
	if err != nil {
		return err
	}
	if _, err = table.Add(encoded); err != nil {
		return err
	}
	var working *rel.UDocTableProduct
	if bitColumn == 0 {
		working, err = rel.JoinUDocFinite(p.manager, doc, boolSorts, table, nil, nil)
	} else {
		working, err = rel.JoinFiniteUDoc(p.manager, boolSorts, table, doc, nil, nil)
	}
	if working != nil {
		a.values = append(a.values, working)
	}
	if err != nil {
		return err
	}
	return p.unionResult(fact.head, working, a, nil)
}
