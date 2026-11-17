package rel

// Source: src/muz/rel/dl_instruction.cpp instruction::display_indented,
// instruction::log_verbose (IF_VERBOSE(2, display(ctx, verbose_stream()))),
// instr_while_loop's "looping ... " line and the display_head_impl of each
// instruction; src/util/container_util.h print_container;
// src/muz/rel/dl_relation_manager.cpp relation_manager::to_nice_string.
// AUDIT.md FUZZ-F5: the progress lines exist only in g3trace builds.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// instructionBody is display_body_impl for instructions that have one
// (instr_while_loop).
type instructionBody interface {
	DisplayBody(context *ExecutionContext, out io.Writer, indentation string) error
}

// SetConstantNames borrows the datalog context whose sort domains
// relation_manager::to_nice_string(sort, element) prints through
// context::print_constant_name. Without one, elements print as numbers.
func (c *ExecutionContext) SetConstantNames(names *base.DatalogContext) {
	if c != nil {
		c.constantNames = names
	}
}

// DisplayInstructionIndented is instruction::display_indented. The profile
// suffix (rel_context::output_profile, datalog.output_profile) is off by
// default and not ported.
func DisplayInstructionIndented(context *ExecutionContext, instruction Instruction, out io.Writer, indentation string) error {
	if _, err := io.WriteString(out, indentation); err != nil {
		return err
	}
	if err := instruction.DisplayHead(context, out); err != nil {
		return err
	}
	if _, err := io.WriteString(out, "\n"); err != nil {
		return err
	}
	if body, ok := instruction.(instructionBody); ok {
		return body.DisplayBody(context, out, indentation)
	}
	return nil
}

// logVerbose is instruction::log_verbose.
func logVerbose(context *ExecutionContext, instruction Instruction) error {
	if !diagnostic.VerboseProgress {
		return nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	var text bytes.Buffer
	if err = DisplayInstructionIndented(context, instruction, &text, ""); err != nil {
		return err
	}
	return diagnostic.ProgressVerbose(2, text.String())
}

// printColumns is print_container on a column vector: "(a,b,...)".
func printColumns(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprint(value)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// printRegisters is print_container on a register vector.
func printRegisters(values []uint32) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprint(value)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// registerPresent is ctx.reg(index) != nullptr.
func registerPresent(context *ExecutionContext, index uint32) bool {
	if context == nil || index == VoidRegister {
		return false
	}
	return uint64(index) < uint64(len(context.registers)) && context.registers[index] != nil
}

// registerShape is r->num_columns() and r->get_size_estimate_rows() of a
// live register.
func registerShape(context *ExecutionContext, index uint32) (columns, rows uint32, err error) {
	table := context.registers[index]
	signature, err := table.Signature()
	if err != nil {
		return 0, 0, err
	}
	rows, _, err = finiteTableSizeEstimates(table)
	return uint32(signature.Len()), rows, err
}

// joinedRegisterText is m_rel << (r ? ":" << num_columns << "-" << rows).
func joinedRegisterText(context *ExecutionContext, index uint32) (string, error) {
	text := fmt.Sprint(index)
	if !registerPresent(context, index) {
		return text, nil
	}
	columns, rows, err := registerShape(context, index)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d-%d", text, columns, rows), nil
}

// niceElementText is relation_manager::to_nice_string(sort, element): a
// numeral element prints through context::print_constant_name.
func niceElementText(context *ExecutionContext, sort ast.ASTHandle, value Element) (string, error) {
	if context != nil && context.constantNames != nil && !sort.IsNull() {
		var out bytes.Buffer
		if err := context.constantNames.WriteConstant(&out, sort, uint64(value)); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	return fmt.Sprint(uint64(value)), nil
}

// relMkPP writes mk_pp(h, m), the SMT2 printer.
func relMkPP(m *ast.ASTManager, h ast.ASTHandle) (_ string, result error) {
	context, globals, err := m.RewriterParameterServices()
	if err != nil {
		return "", err
	}
	var format ast.ASTHandle
	if context != nil && globals != nil {
		format, err = m.SMT2PPExprTreeWithParams(h, nil, context, globals, nil)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return "", err
	}
	formats, err := m.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if context != nil && globals != nil {
		empty := context.EmptyValue()
		err = m.PrintFormat(stream, format, context, &empty, globals)
	} else {
		err = m.PrintFormat(stream, format, nil, nil, nil)
	}
	if err != nil {
		return "", err
	}
	return stream.String()
}

// unsafeRuleColumnVerbose is compiler::make_add_unbound_column's
// IF_VERBOSE(3, "Compiling unsafe rule column " << col_idx << "\n" <<
// mk_ismt2_pp(rule formula) << "\n").
func unsafeRuleColumnVerbose(m *ast.ASTManager, rule *base.Rule, column int) (result error) {
	if !diagnostic.VerboseProgress {
		return nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(3)
	if err != nil || !enabled {
		return err
	}
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&formula)) }()
	handle, err := m.RefGet(formula)
	if err != nil {
		return err
	}
	text, err := relMkPP(m, handle)
	if err != nil {
		return err
	}
	return diagnostic.ProgressVerbose(3, fmt.Sprintf("Compiling unsafe rule column %d\n%s\n", column, text))
}

// finiteTablePlugin names the table plugin a finite table belongs to, or ""
// for tables that are not a table_relation's table.
func finiteTablePlugin(table FiniteTable) string {
	switch table.(type) {
	case *HashTable:
		return "hashtable"
	case *SparseTable:
		return "sparse"
	case *BitvectorTable:
		return "bitvector"
	case *LazySparseTable:
		return "lazy_sparse"
	}
	return ""
}

// newTableTypeVerbose is table_relation_plugin::tr_join_project_fn's
// IF_VERBOSE(1, "new type returned"): the table join returned a table of
// another plugin than the first relation's.
func newTableTypeVerbose(first, result FiniteTable) error {
	if !diagnostic.VerboseProgress || result == nil {
		return nil
	}
	before, after := finiteTablePlugin(first), finiteTablePlugin(result)
	if before == "" || after == "" || before == after {
		return nil
	}
	return diagnostic.ProgressVerbose(1, "new type returned\n")
}
