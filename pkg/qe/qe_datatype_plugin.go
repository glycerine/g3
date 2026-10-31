package qe

// Source: src/qe/qe_datatype_plugin.cpp, datatype_plugin construction,
// recognizer scanning, get_num_branches_nonrec and assign_nonrec.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// DatatypeSolverPlugin implements the non-recursive datatype branch of the
// QE solver plugin. Recursive branching and substitution are still pending.
type DatatypeSolverPlugin struct {
	self    *DatatypeSolverPlugin
	context *SolverContext
	manager *ast.ASTManager
	family  ast.FamilyID
	closed  bool
}

func NewDatatypeSolverPlugin(context *SolverContext) (*DatatypeSolverPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	family, err := m.RegisterDatatypePlugin()
	if err != nil {
		return nil, err
	}
	p := &DatatypeSolverPlugin{context: context, manager: m, family: family}
	p.self = p
	return p, nil
}

func (p *DatatypeSolverPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return p.context.open()
}
func (p *DatatypeSolverPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}
func (p *DatatypeSolverPlugin) MkAtom(ast.ASTHandle, bool) (ast.ASTHandle, bool, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return ast.ASTHandle{}, false, nil
}
func (p *DatatypeSolverPlugin) IsUninterpreted(h ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	kind, err := p.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	return true, nil
}
func (p *DatatypeSolverPlugin) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	p.closed = true
	return nil
}

func (p *DatatypeSolverPlugin) hasRecognizer(variable, formula ast.ASTHandle) (found bool, err error) {
	conjunction, err := NewConjunctionEnum(p.manager, formula)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, conjunction.Close()) }()
	count, err := conjunction.Len()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		h, e := conjunction.At(i)
		if e != nil {
			return false, e
		}
		kind, e := p.manager.GetKind(h)
		if e != nil {
			return false, e
		}
		if kind != ast.ASTApp {
			continue
		}
		family, e := p.manager.GetFamilyID(h)
		if e != nil {
			return false, e
		}
		if family != p.family {
			continue
		}
		decl, e := p.manager.GetDecl(h)
		if e != nil {
			return false, e
		}
		_, matched, e := p.manager.DatatypeRecognizerConstructor(decl)
		if e != nil {
			return false, e
		}
		if !matched {
			continue
		}
		argument, e := p.manager.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		if argument == variable {
			return true, nil
		}
	}
	return false, nil
}

// NonRecursiveBranches returns the source branch count and whether the target
// is non-recursive. Recursive datatypes are left for the pending source path.
func (p *DatatypeSolverPlugin) NonRecursiveBranches(variable, formula ast.ASTHandle) (uint32, bool, error) {
	if err := p.open(); err != nil {
		return 0, false, err
	}
	kind, err := p.manager.GetKind(variable)
	if err != nil {
		return 0, false, err
	}
	if kind != ast.ASTApp {
		return 0, false, ast.ErrASTDomain
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return 0, false, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return 0, false, err
	}
	if family != p.family {
		return 0, false, ast.ErrASTDomain
	}
	recursive, err := p.manager.DatatypeIsRecursive(sort)
	if err != nil || recursive {
		return 0, false, err
	}
	constructors, err := p.manager.DatatypeConstructors(sort)
	if err != nil {
		return 0, false, err
	}
	count := uint32(len(constructors))
	if count != 1 {
		found, e := p.hasRecognizer(variable, formula)
		if e != nil {
			return 0, false, e
		}
		if found {
			count = 1
		}
	}
	return count, true, nil
}

// NonRecursiveBranchConstraint returns an owned recognizer application when
// assign_nonrec would add one; a zero ref means no constraint is needed.
func (p *DatatypeSolverPlugin) NonRecursiveBranchConstraint(variable, formula ast.ASTHandle, branch uint32) (ast.ASTRef, error) {
	count, supported, err := p.NonRecursiveBranches(variable, formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !supported || branch >= count {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return ast.ASTRef{}, err
	}
	constructors, err := p.manager.DatatypeConstructors(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if len(constructors) == 1 {
		return ast.ASTRef{}, nil
	}
	found, err := p.hasRecognizer(variable, formula)
	if err != nil || found {
		return ast.ASTRef{}, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructors[branch])
	if err != nil {
		return ast.ASTRef{}, err
	}
	application, err := p.manager.MkApp(recognizer, []ast.ASTHandle{variable})
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(application)
}
