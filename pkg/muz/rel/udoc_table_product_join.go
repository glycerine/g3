package rel

// Source: dl_product_relation.cpp join_fn::init constructs the missing DOC
// component with sieve.full and the missing finite component with mk_full.

import "github.com/glycerine/g3/pkg/ast"

// JoinFiniteTables constructs the DOC/table product for two raw table inputs.
// The DOC side is full; the finite side performs the actual source join.
func JoinFiniteTables(manager *RelationManager, factory *UDocRelationFactory, leftSorts []ast.ASTHandle, left FiniteTable, rightSorts []ast.ASTHandle, right FiniteTable, leftColumns, rightColumns []uint32) (*UDocTableProduct, error) {
	if manager == nil || manager.open() != nil || factory == nil || factory.open() != nil || factory.manager != manager.manager || left == nil || right == nil || rawUDocProductFiniteKind(left) == 0 || rawUDocProductFiniteKind(right) == 0 || len(leftColumns) != len(rightColumns) {
		return nil, ErrUDocTableProduct
	}
	leftSignature, err := tableRelationSignature(manager, leftSorts)
	if err != nil {
		return nil, err
	}
	rightSignature, err := tableRelationSignature(manager, rightSorts)
	if err != nil {
		return nil, err
	}
	actual, err := left.Signature()
	if err != nil || !leftSignature.equal(actual) {
		return nil, ErrUDocTableProduct
	}
	actual, err = right.Signature()
	if err != nil || !rightSignature.equal(actual) {
		return nil, ErrUDocTableProduct
	}
	for i, column := range leftColumns {
		if column >= uint32(len(leftSorts)) || rightColumns[i] >= uint32(len(rightSorts)) || leftSorts[column] != rightSorts[rightColumns[i]] {
			return nil, ErrUDocTableProduct
		}
	}
	sorts := append(append([]ast.ASTHandle(nil), leftSorts...), rightSorts...)
	joinedLeft, joinedRight := make([]int, len(leftColumns)), make([]int, len(rightColumns))
	for i := range leftColumns {
		joinedLeft[i], joinedRight[i] = int(leftColumns[i]), int(rightColumns[i])
	}
	a := newUDocTableProductAttempt(manager)
	var cause error
	a.product.doc, cause = NewFullUDocSieve(factory, sorts)
	if cause == nil {
		a.product.table, cause = finiteTableJoin(left, right, joinedLeft, joinedRight)
	}
	return a.finish(cause)
}

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
	full, err := a.fullUnaryFinite(fullSignature, finite)
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

// Source get_full_tableish_relation preserves the actual table kind. The
// complement implementation is defined for at most one ordinary column;
// these two live mixed-join callers supply exactly the unary DOC signature.
// Every temporary is registered before the next fallible operation.
func (a *udocTableProductAttempt) fullUnaryFinite(signature Signature, like FiniteTable) (FiniteTable, error) {
	if signature.Len() != 1 || rawUDocProductFiniteKind(like) == 0 {
		return nil, ErrUDocTableProduct
	}
	if lazy, ok := like.(*LazySparseTable); ok && (a.manager.lazyFinite == nil || !a.manager.lazyFinite.Owns(lazy)) {
		return nil, ErrUDocTableProduct
	}
	empty, err := a.manager.NewDefaultFiniteTable(signature)
	if empty != nil {
		a.tables = append(a.tables, empty)
	}
	if err != nil {
		return nil, err
	}
	// get_full_tableish_relation passes the original table kind to the selected
	// plugin. This bounded path requires that concrete plugin to agree.
	if rawUDocProductFiniteKind(empty) != rawUDocProductFiniteKind(like) {
		return nil, ErrUDocTableProduct
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
	return full, err
}
