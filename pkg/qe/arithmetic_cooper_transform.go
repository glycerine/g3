package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) applyCooper(variable ast.ASTHandle, processor *ArithmeticBoundsProcessor, result ast.ASTRef) (applied bool, resultErr error) {
	runtime, applied, err := p.newCooperRuntime(variable, processor)
	if err != nil || !applied {
		return applied, err
	}
	defer func() {
		if closeErr := runtime.close(); resultErr == nil {
			resultErr = closeErr
		}
	}()
	// The C++ plugin rewrites nested modulo terms before it installs direct
	// divisibility equivalences and the common-residue guard.
	if err = runtime.rewriteNested(processor, result); err != nil {
		return true, err
	}
	if err = runtime.baseConstraints(result); err != nil {
		return true, err
	}
	if err = runtime.rewriteDirect(processor, result); err != nil {
		return true, err
	}
	if err = runtime.publish(); err != nil {
		return true, err
	}
	return true, nil
}
