package rel

// Source: src/muz/rel/dl_instruction.cpp instr_while_loop.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// whileInstruction owns its body and repeats it while at least one control
// register contains a non-empty table.
type whileInstruction struct {
	accounting *base.AccountedObject
	controls   []uint32
	body       *InstructionBlock
	closed     bool
}

// NewWhileLoopInstruction transfers ownership of body to the returned instruction.
func NewWhileLoopInstruction(controls []uint32, body *InstructionBlock) Instruction {
	return &whileInstruction{
		accounting: base.NewAccountedObject(),
		controls:   append([]uint32(nil), controls...),
		body:       body,
	}
}

func (i *whileInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil || i.body == nil {
		return ErrInstruction
	}
	return i.body.open()
}

func (i *whileInstruction) controlIsEmpty(context *ExecutionContext) (bool, error) {
	for _, register := range i.controls {
		if hasRelationExplanationRegister(context, register) {
			product, err := context.RelationExplanationRegister(register)
			if err != nil {
				return false, err
			}
			empty, err := product.Empty()
			if err != nil {
				return false, err
			}
			if !empty {
				return false, nil
			}
			continue
		}
		if hasExplanationInstructionRegister(context, register) {
			product, err := context.ExplanationRegister(register)
			if err != nil {
				return false, err
			}
			count, err := product.Len()
			if err != nil {
				return false, err
			}
			if count != 0 {
				return false, nil
			}
			continue
		}
		table, err := context.FiniteRegister(register)
		if err != nil {
			return false, err
		}
		if table == nil {
			continue
		}
		count, err := table.Len()
		if err != nil {
			return false, err
		}
		if count != 0 {
			return false, nil
		}
	}
	return true, nil
}

func (i *whileInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if err := logVerbose(context, i); err != nil {
		return false, err
	}
	count := 0
	for {
		empty, err := i.controlIsEmpty(context)
		if err != nil || empty {
			return empty, err
		}
		if diagnostic.VerboseProgress {
			// IF_VERBOSE(10, "looping ... " << count++).
			if err = diagnostic.ProgressVerbose(10, fmt.Sprintf("looping ... %d\n", count)); err != nil {
				return false, err
			}
			count++
		}
		complete, err := i.body.Perform(context)
		if err != nil || !complete {
			return complete, err
		}
	}
}

func (i *whileInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	return i.body.MakeAnnotations(context)
}

func (i *whileInstruction) DisplayHead(_ *ExecutionContext, out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	_, err := io.WriteString(out, "while"+printRegisters(i.controls))
	return err
}

// DisplayBody is display_body_impl: m_body->display_indented(ctx, out,
// indentation+"    ").
func (i *whileInstruction) DisplayBody(context *ExecutionContext, out io.Writer, indentation string) error {
	if err := i.open(); err != nil {
		return err
	}
	return i.body.DisplayIndented(context, out, indentation+"    ")
}

func (i *whileInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *whileInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	if err := i.accounting.ProcessCosts(); err != nil {
		return err
	}
	return i.body.ProcessAllCosts()
}

func (i *whileInstruction) CollectStatistics(out *statistics.Statistics) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	cost, err := i.accounting.TotalCosts()
	if err != nil {
		return err
	}
	out.UpdateUint("instruction", cost.Instructions)
	out.UpdateUint("instruction-time", cost.Milliseconds)
	return nil
}

func (i *whileInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	return errors.Join(i.body.Close(), i.accounting.Close())
}
