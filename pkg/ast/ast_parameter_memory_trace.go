package ast

// Source: src/ast/ast.h:124; src/ast/ast.cpp:38,53,60;
// src/util/memory_manager.cpp:202-211.

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) traceParameterRationalAllocated(r *astParameterRecord, file string, line int) {
	m.rationalTraceSerial++
	// The source rational* survives parameter moves. A slot address does not,
	// so the concrete Go owner carries one stable allocation identity. Its
	// text (manager address/serial) is formatted only when traced.
	r.rationalTraceID = m.rationalTraceSerial
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "allocate", "src/util/memory_manager.cpp", 209,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{
				"alloc ", m.rationalTraceName(r.rationalTraceID), " ", file, ":", strconv.Itoa(line),
				" rational ", strconv.FormatUint(uint64(reflect.TypeOf(unumeral.OwnedRational{}).Size()), 10), "\n",
			} {
				_, _ = io.WriteString(w, part)
			}
			return nil
		})
}

func (m *ASTManager) traceParameterRationalDeallocated(r *astParameterRecord, line int) {
	id := r.rationalTraceID
	r.rationalTraceID = 0
	if id == 0 {
		return
	}
	tags := m.astMemoryTraceTags()
	if !udiag.DebugBuild || !tags.ShouldTrace("memory") {
		return
	}
	_ = tags.EmitConditional("memory", "deallocate", "src/util/memory_manager.cpp", 204,
		func() bool { return !m.astMemoryFinalizing(tags) }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "dealloc "+m.rationalTraceName(id)+" src/ast/ast.cpp:"+strconv.Itoa(line)+"\n")
			return nil
		})
}

// rationalTraceName is the memory trace's name of rational allocation id.
func (m *ASTManager) rationalTraceName(id uint64) string {
	return fmt.Sprintf("%p/%d", m, id)
}
