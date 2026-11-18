package smtcore

// Source: `smt_params fp; smt::kernel k(m, fp);` as written by callers
// such as doc_manager::is_empty_complete and check_relation_plugin::
// check_equiv (src/muz/rel): a kernel over freshly constructed smt_params
// (the process parameters' smt module) and no logic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// DefaultParamsKernel is such a kernel and its smt_params owner.
type DefaultParamsKernel struct {
	*SMTKernel
	params *SMTParams
}

// NewDefaultParamsKernel builds smt_params fp from the manager's process
// parameter services and the kernel(m, fp) over them.
func NewDefaultParamsKernel(m *ast.ASTManager) (*DefaultParamsKernel, error) {
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return nil, err
	}
	if context == nil || globals == nil {
		// No parameter services: the bare kernel.
		kernel, err := NewSMTKernel(m, nil)
		if err != nil {
			return nil, err
		}
		return &DefaultParamsKernel{SMTKernel: kernel}, nil
	}
	local := context.EmptyValue()
	params, err := NewSMTParams(context, &local, globals, m.Budget())
	if err != nil {
		// A constructor failure returns the partly constructed params.
		if params != nil {
			err = errors.Join(err, params.Close())
		}
		return nil, err
	}
	kernel, err := NewSMTKernel(m, nil)
	if err != nil {
		return nil, errors.Join(err, params.Close())
	}
	result := &DefaultParamsKernel{SMTKernel: kernel, params: params}
	if err = kernel.configurePreprocessingParams(context, globals, &local, params, true); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err = smtInstallRetainedSetup(kernel, m, ""); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if manager := kernel.internalizer.quantifierManager; manager != nil {
		config := SMTSolverConfig{Params: context, Globals: globals}
		if err = manager.SetAuxiliarySolverFactory(func() (*SMTSolver, error) {
			return newKernelAuxiliarySolver(m, kernel, config)
		}); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

// Close releases the kernel before the smt_params it borrows.
func (k *DefaultParamsKernel) Close() error {
	if k == nil {
		return nil
	}
	err := k.SMTKernel.Close()
	if k.params != nil {
		err = errors.Join(err, k.params.Close())
		k.params = nil
	}
	return err
}
