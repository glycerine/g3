package protomodel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ProtoModel) uninterpretedConstant(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if kind, err := p.manager.GetKind(h); err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	if n, err := p.manager.GetNumArgs(h); err != nil || n != 0 {
		return ast.ASTHandle{}, false, err
	}
	d, err := p.manager.GetDecl(h)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	fid, err := p.manager.GetFamilyID(d)
	return d, fid == ast.NullFamilyID, err
}

func (p *ProtoModel) cleanFunctionElse(fi *model.FunctionInterpretation, found *ast.ObjMap) (err error) {
	root, err := fi.Else()
	if err != nil {
		return err
	}
	// The source's explicit todo stack: every uncached argument is pushed
	// (so the last argument is rewritten first), and a node is rewritten
	// once all its arguments are cached.
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	// owned is the source's trail (expr_ref_vector): it holds only the
	// rewritten results that differ from their input and releases them
	// front to back.
	var owned []ast.ASTRef
	defer func() {
		for i := range owned {
			err = errors.Join(err, p.manager.DelRef(&owned[i]))
		}
	}()
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		d, constant, e := p.uninterpretedConstant(a)
		if e != nil {
			return e
		}
		if constant {
			todo = todo[:len(todo)-1]
			v, e := p.core.Core().ConstantInterpretation(d)
			if e != nil {
				return e
			}
			if v.IsNull() {
				s, e := p.manager.GetRange(d)
				if e != nil {
					return e
				}
				v, e = p.SomeValue(s)
				if e != nil {
					return e
				}
				if e = p.core.Core().RegisterConstant(d, v); e != nil {
					return e
				}
			}
			cache[a] = v
			continue
		}
		kind, e := p.manager.GetKind(a)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			cache[a] = a
			todo = todo[:len(todo)-1]
			continue
		}
		n, e := p.manager.GetNumArgs(a)
		if e != nil {
			return e
		}
		visited := true
		args := make([]ast.ASTHandle, 0, n)
		for i := uint32(0); i < n; i++ {
			arg, e := p.manager.GetArg(a, i)
			if e != nil {
				return e
			}
			if v, ok := cache[arg]; ok {
				args = append(args, v)
			} else {
				visited = false
				todo = append(todo, arg)
			}
		}
		if !visited {
			continue
		}
		d, e = p.manager.GetDecl(a)
		if e != nil {
			return e
		}
		if auxiliary, e := p.aux.Contains(d); e != nil {
			return e
		} else if auxiliary {
			if e = found.Insert(d, [2]ast.ASTHandle{}); e != nil {
				return e
			}
		}
		out, e := p.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		if e = p.rewriter.MakeApp(d, ast.SliceArguments(args), out); e != nil {
			return errors.Join(e, p.manager.DelRef(&out))
		}
		v, e := p.manager.RefGet(out)
		if e != nil {
			return errors.Join(e, p.manager.DelRef(&out))
		}
		if v != a {
			owned = append(owned, out)
		} else if e = p.manager.DelRef(&out); e != nil {
			return e
		}
		todo = todo[:len(todo)-1]
		cache[a] = v
	}
	cleaned := cache[root]
	// Like the source trail, owned must keep every rewritten expression alive
	// until the function interpretation has acquired its fallback reference.
	return fi.SetElse(cleaned)
}

func (p *ProtoModel) Cleanup() error {
	if err := p.open(); err != nil {
		return err
	}
	found := ast.NewObjMap(p.manager) // func_decl_set found_aux_fs
	declarations, err := p.core.Core().FunctionDeclarationsInTableOrder()
	if err != nil {
		return err
	}
	for _, d := range declarations {
		fi, err := p.core.Core().FunctionInterpretation(d)
		if err != nil {
			return err
		}
		partial, err := fi.IsPartial()
		if err != nil || partial {
			if err != nil {
				return err
			}
			continue
		}
		if err = p.cleanFunctionElse(fi, found); err != nil {
			return err
		}
	}
	// Unused auxiliary declarations, in m_aux_decls order; then
	// m_aux_decls.swap(found_aux_fs).
	if found.Len() != p.aux.Len() {
		for _, entry := range p.aux.Entries() {
			used, err := found.Contains(entry.Key)
			if err != nil {
				return err
			}
			if used {
				continue
			}
			if err = p.core.Core().UnregisterDeclaration(entry.Key); err != nil {
				return err
			}
		}
		p.aux = found
	}
	return nil
}

func (p *ProtoModel) FinalModel() (*model.Model, error) {
	result, err := p.FinalModelRetained()
	if err != nil {
		return nil, err
	}
	if err = p.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// FinalModelRetained is proto_model::mk_model without dropping the proto
// model: context::update_model keeps m_proto_model after m_model is made.
// The caller closes p later.
func (p *ProtoModel) FinalModelRetained() (*model.Model, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result, err := model.NewModel(p.manager)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (*model.Model, error) { return nil, errors.Join(e, result.Close()) }
	if err = result.Core().CopyConstantInterpretations(p.core.Core()); err != nil {
		return fail(err)
	}
	// m took the ownership of the func_interp's.
	if err = result.Core().TransferFunctionInterpretations(p.core.Core()); err != nil {
		return fail(err)
	}
	for _, s := range p.user.Sorts() {
		if err = result.RegisterUniverse(s, p.user.KnownUniverse(s)); err != nil {
			return fail(err)
		}
	}
	return result, nil
}

func (p *ProtoModel) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if p.closePhase == 0 {
		p.closePhase = 1
	}
	if p.closePhase == 1 {
		if p.evaluator != nil {
			if err := p.evaluator.Close(); err != nil {
				return err
			}
			p.evaluator = nil
		}
		p.closePhase = 2
	}
	if p.closePhase == 2 {
		if p.rewriter != nil {
			if err := p.rewriter.Close(); err != nil {
				return err
			}
			p.rewriter = nil
		}
		p.closePhase = 3
	}
	if p.closePhase == 3 {
		for len(p.factoryList) != 0 {
			i := len(p.factoryList) - 1
			if err := p.factoryList[i].Close(); err != nil {
				return err
			}
			p.factoryList[i] = nil
			p.factoryList = p.factoryList[:i]
		}
		p.factories = nil
		p.user = nil
		p.closePhase = 4
	}
	if p.closePhase == 4 {
		if p.core != nil {
			if err := p.core.Close(); err != nil {
				return err
			}
			p.core = nil
		}
		p.closePhase = 5
	}
	p.aux = nil
	p.closed = true
	return nil
}
