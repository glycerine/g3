package smtcore

import (
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Retain a small window of emitted matcher instructions as a code tree would.
func BenchmarkSMTMAMInstructionStorage(b *testing.B) {
	var manager smtMAMCodeTreeManager
	for _, op := range []string{"Init", "Check", "Bind", "Yield2", "Yield8"} {
		b.Run(op, func(b *testing.B) {
			var window [64]*smtMAMInstr
			var regs [8]uint32
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var instr *smtMAMInstr
				switch op {
				case "Init":
					instr = manager.mkInit(2)
				case "Check":
					instr = manager.mkCheck(2, nil)
				case "Bind":
					instr = manager.mkBind(ast.ASTHandle{}, 2, 1, 2)
				case "Yield2":
					instr = manager.mkYield(ast.ASTHandle{}, ast.ASTHandle{}, regs[:2])
				case "Yield8":
					instr = manager.mkYield(ast.ASTHandle{}, ast.ASTHandle{}, regs[:])
				}
				window[i%len(window)] = instr
			}
			runtime.KeepAlive(window)
		})
	}
}
