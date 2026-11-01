package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear indexed symbols.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// quantifiedLinear owns the temporary expressions of one compilation.
type quantifiedLinear struct {
	manager  *ast.ASTManager
	width    uint32
	index    ast.ASTHandle
	variable ast.ASTHandle
	one      ast.ASTHandle
	refs     []ast.ASTRef
}

func newQuantifiedLinear(m *ast.ASTManager, width uint32) (q *quantifiedLinear, err error) {
	if m == nil || width == 0 {
		return nil, ast.ErrASTDomain
	}
	q = &quantifiedLinear{manager: m, width: width}
	defer func() {
		if err != nil {
			err = errors.Join(err, q.close())
			q = nil
		}
	}()
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return q, err
	}
	q.index, err = q.hold(m.BitVectorSort(width))
	if err != nil {
		return q, err
	}
	q.variable, err = q.hold(m.MkVar(0, q.index))
	if err != nil {
		return q, err
	}
	q.one, err = q.hold(m.BitVectorNumeralRat(big.NewRat(1, 1), width))
	return q, err
}

func (q *quantifiedLinear) close() (err error) {
	for i := len(q.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, q.manager.DelRef(&q.refs[i]))
	}
	q.refs = nil
	return err
}

func (q *quantifiedLinear) hold(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := q.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q.refs = append(q.refs, ref)
	return value, nil
}

func (q *quantifiedLinear) declaration(predicate ast.ASTHandle, suffix string, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	name, err := q.manager.GetASTName(predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := q.manager.Symbols.Display(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := q.manager.Symbols.Intern(text + suffix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkFuncDecl(symbol, []ast.ASTHandle{q.index}, rangeSort, ast.ASTFuncDeclInfo{}))
}

func (q *quantifiedLinear) predicate(predicate ast.ASTHandle, at ast.ASTHandle) (ast.ASTHandle, error) {
	rangeSort, err := q.manager.GetRange(predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, "#", rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{at}))
}

func (q *quantifiedLinear) rule(predicate ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	boolean, err := q.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d", index), boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{q.variable}))
}

func (q *quantifiedLinear) argument(predicate ast.ASTHandle, column uint32, current bool) (ast.ASTHandle, error) {
	sort, err := q.manager.GetDomain(predicate, column)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d", column), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	index := q.variable
	if !current {
		index, err = q.hold(q.manager.MkBitVectorSub(index, q.one))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{index}))
}

func (q *quantifiedLinear) local(predicate, sort ast.ASTHandle, rule, index uint32) (ast.ASTHandle, error) {
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d_%d", rule, index), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{q.variable}))
}

func (q *quantifiedLinear) bindings(rule *base.Rule, predicate ast.ASTHandle, ruleIndex uint32) ([]ast.ASTHandle, error) {
	sorts, err := rule.VariableSorts()
	if err != nil {
		return nil, err
	}
	values := make([]ast.ASTHandle, len(sorts))
	bind := func(app, decl ast.ASTHandle, current bool) error {
		arity, e := q.manager.GetArity(decl)
		if e != nil {
			return e
		}
		for i := uint32(0); i < arity; i++ {
			arg, e := q.manager.GetArg(app, i)
			if e != nil {
				return e
			}
			kind, e := q.manager.GetKind(arg)
			if e != nil {
				return e
			}
			if kind != ast.ASTVar {
				continue
			}
			index, e := q.manager.GetVarIndex(arg)
			if e != nil || int(index) >= len(values) {
				return errors.Join(e, ast.ErrASTDomain)
			}
			if values[index].IsNull() {
				values[index], e = q.argument(decl, i, current)
				if e != nil {
					return e
				}
			}
		}
		return nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = bind(head, predicate, true); err != nil {
		return nil, err
	}
	count, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		tail, _, e := rule.Tail(i)
		if e != nil {
			return nil, e
		}
		decl, e := rule.TailDeclaration(i)
		if e != nil {
			return nil, e
		}
		if e = bind(tail, decl, false); e != nil {
			return nil, e
		}
	}
	var next uint32
	for i, sort := range sorts {
		if sort.IsNull() || !values[i].IsNull() {
			continue
		}
		values[i], err = q.local(predicate, sort, ruleIndex, next)
		if err != nil {
			return nil, err
		}
		next++
	}
	return values, nil
}
