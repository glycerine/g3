package tactic

// Source: src/tactic/arith/elim01_tactic.cpp, bool2int_model_converter
// (z3 4.7): elim01's only model converter. It always builds a new model:
// each original integer gets the numeral spelled by its Boolean bits, the
// other constants and every function interpretation are copied, and the
// fresh Boolean bits are dropped.

import (
	"errors"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type Bool2IntModelConverter struct {
	tacticConverterBase
	manager    *ast.ASTManager
	refs       []ast.ASTHandle // m_refs: each holds one declaration reference
	bools      map[ast.ASTHandle]struct{}
	numsAsBool [][]ast.ASTHandle
	numsAsInt  []ast.ASTHandle
}

func NewBool2IntModelConverter(m *ast.ASTManager) (*Bool2IntModelConverter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	c := &Bool2IntModelConverter{manager: m, bools: map[ast.ASTHandle]struct{}{}}
	if err := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Bool2IntModelConverter) holdDecl(d ast.ASTHandle) error {
	kind, err := c.manager.GetKind(d)
	if err != nil {
		return err
	}
	if kind != ast.ASTFuncDecl {
		return ast.ErrASTDomain
	}
	if err = c.manager.IncRef(d); err != nil {
		return err
	}
	c.refs = append(c.refs, d)
	return nil
}

// Insert is insert(x_old, sz, x_new): x_old is spelled by the bits x_new,
// least significant first.
func (c *Bool2IntModelConverter) Insert(old ast.ASTHandle, bits []ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.holdDecl(old); err != nil {
		return err
	}
	c.numsAsInt = append(c.numsAsInt, old)
	group := make([]ast.ASTHandle, 0, len(bits))
	for _, bit := range bits {
		if err := c.holdDecl(bit); err != nil {
			c.numsAsBool = append(c.numsAsBool, group)
			return err
		}
		group = append(group, bit)
		c.bools[bit] = struct{}{}
	}
	c.numsAsBool = append(c.numsAsBool, group)
	return nil
}

func (c *Bool2IntModelConverter) ApplyModel(out *ModelRef, goal uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if goal != 0 {
		return errTacticModelConverterGoal
	}
	source, err := out.Get()
	if err != nil {
		return err
	}
	if source == nil || source.Manager() != c.manager {
		return ast.ErrASTHandle
	}
	m := c.manager
	target, err := model.NewModel(m)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, target.Close())
		}
	}()
	for i, old := range c.numsAsInt {
		value := new(big.Int)
		power := big.NewInt(1)
		isValue := true
		for _, bit := range c.numsAsBool[i] {
			interp, e := source.ConstantInterpretation(bit)
			if e != nil {
				return e
			}
			switch {
			case interp.IsNull():
				isValue = false
			case m.IsTrue(interp):
				value.Add(value, power)
			case !m.IsFalse(interp):
				isValue = false
			}
			if !isValue {
				break
			}
			power = new(big.Int).Lsh(power, 1)
		}
		if isValue {
			numeral, e := m.ArithmeticIntegerNumeralText(value.String())
			if e != nil {
				return e
			}
			if e = target.Core().RegisterConstant(old, numeral); e != nil {
				return e
			}
		}
	}
	count, err := source.NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		decl, e := source.Constant(i)
		if e != nil {
			return e
		}
		if _, isBit := c.bools[decl]; isBit {
			continue
		}
		value, e := source.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		if e = target.Core().RegisterConstant(decl, value); e != nil {
			return e
		}
	}
	if err = target.Core().CopyFunctionInterpretations(source); err != nil {
		return err
	}
	if derived := source.DerivedModel(); derived != nil {
		if err = target.CopyUniverseInterpretations(derived); err != nil {
			return err
		}
	}
	if err = out.Set(target.Core()); err != nil {
		published = out.holds(target.Core())
		return err
	}
	published = true
	return nil
}

func (c *Bool2IntModelConverter) ApplyUnindexed(out *ModelRef) error         { return c.ApplyModel(out, 0) }
func (c *Bool2IntModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error { return c.open() }
func (c *Bool2IntModelConverter) Cancel() error                              { return c.open() }
func (c *Bool2IntModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *Bool2IntModelConverter) Translate(t *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if t == nil || t.From() != c.manager || t.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewBool2IntModelConverter(t.To())
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for i, old := range c.numsAsInt {
		a, e := t.Translate(old)
		if e != nil {
			return nil, e
		}
		bits := make([]ast.ASTHandle, len(c.numsAsBool[i]))
		for j, bit := range c.numsAsBool[i] {
			if bits[j], e = t.Translate(bit); e != nil {
				return nil, e
			}
		}
		if e = result.Insert(a, bits); e != nil {
			return nil, e
		}
	}
	published = true
	return result, nil
}

func (c *Bool2IntModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	// m_refs is a func_decl_ref_vector: released front to back.
	for len(c.refs) > 0 {
		consumed, e := c.manager.DecRefConsumed(c.refs[0])
		if consumed {
			c.refs = c.refs[1:]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	c.refs, c.bools, c.numsAsBool, c.numsAsInt = nil, nil, nil, nil
	c.release()
	return nil
}
