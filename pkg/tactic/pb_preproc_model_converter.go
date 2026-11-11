package tactic

// Source: src/tactic/core/pb_preprocess_tactic.cpp:42-83
// pb_preproc_model_converter: an ordered list of (constant, value) pairs that
// operator() registers into the model, later pairs replacing earlier ones.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type pbPreprocModelConverter struct {
	tacticConverterBase
	manager *ast.ASTManager
	// constants and values are m_const; refs is m_refs (e, v, e, v, ...).
	constants, values []ast.ASTHandle
	refs              []ast.ASTHandle
}

// PBPreprocModelConverter is pb_preproc_model_converter.
type PBPreprocModelConverter = pbPreprocModelConverter

func NewPBPreprocModelConverter(m *ast.ASTManager) (*PBPreprocModelConverter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	c := &pbPreprocModelConverter{manager: m}
	if e := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); e != nil {
		return nil, e
	}
	return c, nil
}

// SetValue is set_value(e, p): negations flip p, then set_value_p.
func (c *pbPreprocModelConverter) SetValue(e ast.ASTHandle, positive bool) error {
	if err := c.open(); err != nil {
		return err
	}
	m := c.manager
	for {
		kind, err := m.GetKind(e)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			break
		}
		family, err := m.GetFamilyID(e)
		if err != nil {
			return err
		}
		if family != m.BasicFamilyID() {
			break
		}
		op, err := m.GetDeclKind(e)
		if err != nil {
			return err
		}
		if op != ast.ASTOpNot {
			break
		}
		if e, err = m.GetArg(e, 0); err != nil {
			return err
		}
		positive = !positive
	}
	var value ast.ASTHandle
	var err error
	if positive {
		value, err = m.MkTrue()
	} else {
		value, err = m.MkFalse()
	}
	if err != nil {
		return err
	}
	return c.setValueP(e, value)
}

// setValueP appends the pair and m_refs.push_back(e), push_back(v).
func (c *pbPreprocModelConverter) setValueP(e, v ast.ASTHandle) error {
	if err := c.manager.IncRef(e); err != nil {
		return err
	}
	c.refs = append(c.refs, e)
	if err := c.manager.IncRef(v); err != nil {
		return err
	}
	c.refs = append(c.refs, v)
	c.constants = append(c.constants, e)
	c.values = append(c.values, v)
	return nil
}

func (c *pbPreprocModelConverter) ApplyModel(out *ModelRef, _ uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	md, err := out.Get()
	if err != nil {
		return err
	}
	if md == nil || md.Manager() != c.manager {
		return ast.ErrASTHandle
	}
	for i, e := range c.constants {
		d, err := c.manager.GetDecl(e)
		if err != nil {
			return err
		}
		if err = md.RegisterConstant(d, c.values[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *pbPreprocModelConverter) ApplyUnindexed(out *ModelRef) error {
	return c.ApplyModel(out, 0)
}

func (c *pbPreprocModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error { return c.open() }
func (c *pbPreprocModelConverter) Cancel() error                              { return c.open() }

// Display is converter::display's empty default.
func (c *pbPreprocModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *pbPreprocModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if translator == nil || translator.From() != c.manager || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewPBPreprocModelConverter(translator.To())
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for i, e := range c.constants {
		// mc->set_value_p(translator(e), translator(v)): GCC evaluates the
		// value first.
		v, err := translator.Translate(c.values[i])
		if err != nil {
			return nil, err
		}
		te, err := translator.Translate(e)
		if err != nil {
			return nil, err
		}
		if err = result.setValueP(te, v); err != nil {
			return nil, err
		}
	}
	published = true
	return result, nil
}

func (c *pbPreprocModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	// m_refs (expr_ref_vector) releases forward; m_const holds no refs.
	for i, h := range c.refs {
		if h.IsNull() {
			continue
		}
		consumed, e := c.manager.DecRefConsumed(h)
		if consumed {
			c.refs[i] = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	c.refs, c.constants, c.values = nil, nil, nil
	c.release()
	return nil
}
