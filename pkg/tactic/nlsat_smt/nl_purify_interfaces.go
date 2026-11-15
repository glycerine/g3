package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, rw_cfg interface-var mode.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlInterfaceRewriter owns introduced interface constants, definitions, and
// rewritten terms until Close. Definitions are separated by the solver that
// receives them in the source tactic.
type nlInterfaceRewriter struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	real       ast.ASTHandle
	cache      map[ast.ASTHandle]ast.ASTHandle
	cacheOrder *ast.ObjMap // obj_map m_interface_cache table order
	// rw: the rewriter_tpl<rw_cfg> in interface-var mode.
	config               nlInterfaceConfig
	traversal            *rewriter.Traversal
	out                  ast.ASTRef
	refs                 []ast.ASTRef
	newReals             []ast.ASTHandle
	interfaceValues      []ast.ASTHandle
	nonlinearDefinitions []ast.ASTHandle
	smtDefinitions       []ast.ASTHandle
}

func newNLInterfaceRewriter(m *ast.ASTManager) (*nlInterfaceRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return nil, err
	}
	r := &nlInterfaceRewriter{manager: m, arithmetic: family, real: real, cache: make(map[ast.ASTHandle]ast.ASTHandle), cacheOrder: ast.NewObjMap(m)}
	r.config.owner = r
	if r.out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return nil, err
	}
	if r.traversal, err = rewriter.NewTraversal(m, &r.config); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// nlInterfaceConfig is rw_cfg in mode_interface_var.
type nlInterfaceConfig struct {
	rewriter.DefaultConfiguration
	owner *nlInterfaceRewriter
}

func (c *nlInterfaceConfig) ReduceQuantifier(ast.ASTHandle, ast.ASTHandle, ast.AppArguments, ast.AppArguments, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, tactic.NewTacticException("quantifiers are not supported in mixed-mode nlsat engine")
}

// ReduceApp is reduce_app_real.
func (c *nlInterfaceConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	r := c.owner
	m := r.manager
	family, err := m.GetFamilyID(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	count := args.Count()
	if count == 0 {
		// OP_NUM and OP_IRRATIONAL_ALGEBRAIC_NUM; constants have no arguments.
		return rewriter.RewriteFailed, nil
	}
	isArith := family == r.arithmetic
	values := make([]ast.ASTHandle, count)
	hasInterface := false
	for i := uint32(0); i < count; i++ {
		arg, e := args.At(i)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		values[i] = arg
		var needs bool
		if isArith {
			kind, e := m.GetKind(arg)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			needs = kind != ast.ASTApp
			if !needs {
				argFamily, e := m.GetFamilyID(arg)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				needs = argFamily != r.arithmetic
			}
		} else {
			sort, e := m.GetSort(arg)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			needs = sort == r.real
		}
		if needs {
			hasInterface = true
			if values[i], e = r.interfaceVar(arg); e != nil {
				return rewriter.RewriteFailed, e
			}
		}
	}
	if !hasInterface {
		return rewriter.RewriteFailed, nil
	}
	value, err := m.MkApp(d, values)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, m.SetRef(out, value)
}

func (r *nlInterfaceRewriter) hold(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.refs = append(r.refs, ref)
	}
	return err
}

func (r *nlInterfaceRewriter) interfaceVar(arg ast.ASTHandle) (ast.ASTHandle, error) {
	if value, ok := r.cache[arg]; ok {
		return value, nil
	}
	family, err := r.manager.GetFamilyID(arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := r.manager.GetNumArgs(arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == ast.NullFamilyID && count == 0 {
		if err = r.hold(arg); err != nil {
			return ast.ASTHandle{}, err
		}
		r.cache[arg] = arg
		if err = r.cacheOrder.Insert(arg, [2]ast.ASTHandle{arg}); err != nil {
			return ast.ASTHandle{}, err
		}
		r.interfaceValues = append(r.interfaceValues, arg)
		return arg, nil
	}
	value, err := r.manager.MkFreshConstNullable(nil, r.real)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(value); err != nil {
		return ast.ASTHandle{}, err
	}
	eq, err := r.manager.MkEq(value, arg)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(eq); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(arg); err != nil {
		return ast.ASTHandle{}, err
	}
	r.cache[arg] = value
	if err = r.cacheOrder.Insert(arg, [2]ast.ASTHandle{value}); err != nil {
		return ast.ASTHandle{}, err
	}
	r.interfaceValues = append(r.interfaceValues, value)
	r.newReals = append(r.newReals, value)
	if family == r.arithmetic {
		r.nonlinearDefinitions = append(r.nonlinearDefinitions, eq)
	} else {
		r.smtDefinitions = append(r.smtDefinitions, eq)
	}
	return value, nil
}

// interfaceValuesInCacheOrder lists the interface variables (the
// m_interface_cache values) in the obj_map's table order, which
// enforce_equalities iterates.
func (r *nlInterfaceRewriter) interfaceValuesInCacheOrder() []ast.ASTHandle {
	entries := r.cacheOrder.Entries()
	values := make([]ast.ASTHandle, len(entries))
	for i, entry := range entries {
		values[i] = entry.Value[0]
	}
	return values
}

func (r *nlInterfaceRewriter) rewrite(term ast.ASTHandle) (ast.ASTHandle, error) {
	if r == nil || r.manager == nil || r.traversal == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if err := r.traversal.Rewrite(term, r.out); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := r.manager.RefGet(r.out)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if value != term {
		if err = r.hold(value); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return value, nil
}

// rewriteGoal performs the source's first goal rewrite and publishes newly
// introduced arithmetic definitions to the nonlinear goal. SMT definitions
// remain available in smtDefinitions for the cooperating solver.
func (r *nlInterfaceRewriter) rewriteGoal(g, nonlinear *tactic.Goal) error {
	if r == nil || r.manager == nil || g == nil || nonlinear == nil || g == nonlinear || g.Manager() != r.manager || nonlinear.Manager() != r.manager {
		return ast.ErrASTDomain
	}
	if err := g.CheckOwner(); err != nil {
		return err
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		return tactic.NewTacticException("qfufnra-purify does not support proof generation")
	}
	// rewrite_goal starts with r.reset(): the cache is cleared.
	if err = r.traversal.Reset(); err != nil {
		return err
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		original, err := g.Form(i)
		if err != nil {
			return err
		}
		start := len(r.nonlinearDefinitions)
		value, err := r.rewrite(original)
		if err != nil {
			return err
		}
		for _, definition := range r.nonlinearDefinitions[start:] {
			if err = nonlinear.AssertWithProof(definition, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
				return err
			}
		}
		dependency, err := g.Dependency(i)
		if err != nil {
			return err
		}
		if err = g.Update(i, value, ast.ASTHandle{}, dependency); err != nil {
			return err
		}
	}
	return nil
}

func (r *nlInterfaceRewriter) Close() (result error) {
	if r == nil || r.manager == nil {
		return nil
	}
	if r.traversal != nil {
		result = r.traversal.Close()
		r.traversal = nil
	}
	if r.out != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.out))
	}
	for i := len(r.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.manager = nil
	r.refs = nil
	r.cache = nil
	return result
}
