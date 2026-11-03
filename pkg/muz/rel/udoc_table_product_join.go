package rel

// Source: dl_product_relation.cpp join_fn::init constructs the missing DOC
// component with sieve.full and the missing finite component with mk_full.

import "github.com/glycerine/g3/pkg/ast"

func rawUDocProductFiniteKind(table FiniteTable) uint8 {
	switch table.(type) {
	case *HashTable:
		return 1
	case *SparseTable:
		return 2
	case *LazySparseTable:
		return 3
	default:
		return 0
	}
}

// JoinUDocFinite keeps DOC as the first operand. Its unary signature is the
// source table_base::complement boundary for the opposite full finite table.
func JoinUDocFinite(manager *RelationManager, doc *TypedUDocRelation, finiteSorts []ast.ASTHandle, finite FiniteTable, docColumns, finiteColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocFinite(manager, doc, finiteSorts, finite, docColumns, finiteColumns, true)
}

func JoinFiniteUDoc(manager *RelationManager, finiteSorts []ast.ASTHandle, finite FiniteTable, doc *TypedUDocRelation, finiteColumns, docColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocFinite(manager, doc, finiteSorts, finite, docColumns, finiteColumns, false)
}

func joinUDocFinite(manager *RelationManager, doc *TypedUDocRelation, finiteSorts []ast.ASTHandle, finite FiniteTable, docColumns, finiteColumns []uint32, docFirst bool) (*UDocTableProduct, error) {
	if manager == nil || manager.open() != nil || doc == nil || doc.open() != nil || doc.FormulaChecked() || doc.factory.manager != manager.manager || len(doc.sorts) != 1 || rawUDocProductFiniteKind(finite) == 0 || len(docColumns) != len(finiteColumns) {
		return nil, ErrUDocTableProduct
	}
	signature, err := tableRelationSignature(manager, finiteSorts)
	if err != nil {
		return nil, err
	}
	actual, err := finite.Signature()
	if err != nil || !signature.equal(actual) {
		return nil, ErrUDocTableProduct
	}
	fullSignature, err := tableRelationSignature(manager, doc.sorts)
	if err != nil {
		return nil, err
	}
	hasBool := false
	boolean, err := manager.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	for _, sort := range finiteSorts {
		hasBool = hasBool || sort == boolean
	}
	if !hasBool {
		return nil, ErrUDocTableProduct
	}
	left, right := make([]int, len(docColumns)), make([]int, len(docColumns))
	for i, column := range docColumns {
		other := finiteColumns[i]
		if column >= uint32(len(doc.sorts)) || other >= uint32(len(finiteSorts)) || doc.sorts[column] != finiteSorts[other] {
			return nil, ErrUDocTableProduct
		}
		left[i], right[i] = int(column), int(other)
	}
	a := newUDocTableProductAttempt(manager)
	a.fullDoc, err = NewFullUDocSieve(doc.factory, finiteSorts)
	if err != nil {
		return a.finish(err)
	}
	if docFirst {
		a.product.doc, err = doc.JoinSieve(a.fullDoc, docColumns, finiteColumns)
	} else {
		a.product.doc, err = a.fullDoc.JoinUDoc(doc, finiteColumns, docColumns)
	}
	if err != nil {
		return a.finish(err)
	}
	empty, err := manager.NewDefaultFiniteTable(fullSignature)
	if empty != nil {
		a.tables = append(a.tables, empty)
	}
	if err != nil {
		return a.finish(err)
	}
	// get_full_tableish_relation passes the original table kind to the selected
	// plugin. This bounded path requires that concrete plugin to agree.
	if rawUDocProductFiniteKind(empty) != rawUDocProductFiniteKind(finite) {
		return a.finish(ErrUDocTableProduct)
	}
	var full FiniteTable
	switch table := empty.(type) {
	case *HashTable:
		value, cause := table.Complement()
		err = cause
		if value != nil {
			full = value
		}
	case *SparseTable:
		value, cause := table.ComplementFunctional(nil)
		err = cause
		if value != nil {
			full = value
		}
	case *LazySparseTable:
		value, cause := table.Complement()
		err = cause
		if value != nil {
			full = value
		}
	default:
		err = ErrUDocTableProduct
	}
	if full != nil {
		a.tables = append(a.tables, full)
	}
	if err != nil {
		return a.finish(err)
	}
	if docFirst {
		a.product.table, err = finiteTableJoin(full, finite, left, right)
	} else {
		a.product.table, err = finiteTableJoin(finite, full, right, left)
	}
	return a.finish(err)
}
