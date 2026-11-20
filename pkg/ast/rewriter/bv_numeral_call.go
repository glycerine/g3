package rewriter

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// A numerical child keeps its identity across calls, including a stopped
// normalization. Many rewriters never enter the scalar or addition bit reader.
type bvNumeralCall struct {
	frame *ast.ASTBitVectorNumeralInvocation
}

func (c *bvNumeralCall) acquire() *ast.ASTBitVectorNumeralInvocation {
	if c.frame == nil {
		c.frame = new(ast.ASTBitVectorNumeralInvocation)
	}
	return c.frame
}

// The AST manager checks its own state even for an unused child.
func (c *bvNumeralCall) cleanup(m *ast.ASTManager) error {
	if c.frame == nil {
		var empty ast.ASTBitVectorNumeralInvocation
		return m.CleanupBitVectorNumeralInvocation(&empty)
	}
	return m.CleanupBitVectorNumeralInvocation(c.frame)
}

func (c *bvNumeralCall) sourceReady(m *ast.ASTManager) error {
	if c.frame == nil {
		var empty ast.ASTBitVectorNumeralInvocation
		return m.BitVectorNumeralInvocationSourceReady(&empty)
	}
	return m.BitVectorNumeralInvocationSourceReady(c.frame)
}

// Reserve the full invocation with its owner, including before first use.
func bvNumeralCallExtraBytes() uintptr {
	return reflect.TypeOf(ast.ASTBitVectorNumeralInvocation{}).Size() - reflect.TypeOf(bvNumeralCall{}).Size()
}
