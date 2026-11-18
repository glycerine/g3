package qe

// Source: src/qe/nlarith_util.{h,cpp} nlarith::util.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type NonlinearArithmeticUtil struct {
	self         *NonlinearArithmeticUtil
	manager      *ast.ASTManager
	term         *ArithmeticQETermUtil
	family       ast.FamilyID
	enableLinear bool
	closed       bool
	// imp's bool_rewriter m_bs, arith_rewriter m_rw, m_zero, m_one and
	// m_trail; the rewriters exist only when the util has parameter
	// services (NewNonlinearArithmeticUtilConfig).
	bs         *rewriter.BoolRewriter
	arith      *rewriter.ArithRewriter
	zero, unit ast.ASTRef
	trail      []ast.ASTRef
}

func NewNonlinearArithmeticUtil(manager *ast.ASTManager) (*NonlinearArithmeticUtil, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	term, err := NewArithmeticQETermUtil(manager, Config{})
	if err != nil {
		return nil, err
	}
	u := &NonlinearArithmeticUtil{manager: manager, term: term, family: term.family}
	u.self = u
	return u, nil
}

// NewNonlinearArithmeticUtilConfig is nlarith::util(m): imp's m_bs and m_rw
// read their parameters through config, and m_zero/m_one are num(0) and
// num(1).
func NewNonlinearArithmeticUtilConfig(manager *ast.ASTManager, config Config) (_ *NonlinearArithmeticUtil, err error) {
	u, err := NewNonlinearArithmeticUtil(manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, u.Close())
		}
	}()
	zero, err := manager.ArithmeticRealNumeralText("0")
	if err != nil {
		return nil, err
	}
	if u.zero, err = manager.NewRef(zero); err != nil {
		return nil, err
	}
	one, err := manager.ArithmeticRealNumeralText("1")
	if err != nil {
		return nil, err
	}
	if u.unit, err = manager.NewRef(one); err != nil {
		return nil, err
	}
	if u.bs, err = rewriter.NewBoolRewriter(manager, config.Context, config.Globals, nil); err != nil {
		return nil, err
	}
	if u.arith, err = rewriter.NewArithRewriter(manager, config.Context, config.Globals, nil); err != nil {
		return nil, err
	}
	return u, nil
}
func (u *NonlinearArithmeticUtil) open() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return arena.ErrClosed
	}
	if u.manager == nil || u.term == nil {
		return ast.ErrASTHandle
	}
	return nil
}
func (u *NonlinearArithmeticUtil) SetEnableLinear(enable bool) error {
	if err := u.open(); err != nil {
		return err
	}
	u.enableLinear = enable
	return nil
}
func (u *NonlinearArithmeticUtil) Close() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	err := u.resetTrail()
	if u.arith != nil {
		if err = errors.Join(err, u.arith.Close()); err == nil {
			u.arith = nil
		}
	}
	if u.bs != nil {
		if err = errors.Join(err, u.bs.Close()); err == nil {
			u.bs = nil
		}
	}
	for _, r := range []*ast.ASTRef{&u.unit, &u.zero} {
		if *r != (ast.ASTRef{}) {
			err = errors.Join(err, u.manager.DelRef(r))
		}
	}
	err = errors.Join(err, u.term.Close())
	if err == nil {
		u.closed = true
		u.manager = nil
		u.term = nil
	}
	return err
}

// numeral is A.is_numeral(h, n).
func (u *NonlinearArithmeticUtil) numeral(h ast.ASTHandle, n *unumeral.Rational) (bool, error) {
	return u.term.numeral(h, n)
}
