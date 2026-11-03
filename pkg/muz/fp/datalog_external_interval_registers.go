package fp

// Source: src/muz/rel/dl_instruction.cpp instr_io and
// src/muz/rel/dl_compiler.cpp compiler::do_compilation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type externalIntervalDatalogRegisterSlot struct {
	declaration ast.ASTHandle
	entry       *externalIntervalDatalogEntry
	persistent  *rel.ExternalIntervalProduct
	product     *rel.ExternalIntervalProduct
}

// externalIntervalDatalogRegisters owns the execution copies loaded from one
// product activation. A nil product is the source execution-context encoding
// of an empty register.
type externalIntervalDatalogRegisters struct {
	activation *externalIntervalDatalogActivation
	runtime    *memory.Runtime
	slots      []externalIntervalDatalogRegisterSlot
	indices    map[ast.ASTHandle]int
	nested     memory.ConstructionFailure
	retired    []*rel.ExternalIntervalProduct
	retireNext int
	published  bool
	deferStore bool
	closed     bool
}

type externalIntervalDatalogRegisterFailure struct {
	original, cleanup, reported error
	text                        string
	owner                       *externalIntervalDatalogRegisters
}

var _ memory.ConstructionFailure = (*externalIntervalDatalogRegisterFailure)(nil)

func (f *externalIntervalDatalogRegisterFailure) Error() string        { return f.text }
func (f *externalIntervalDatalogRegisterFailure) Unwrap() error        { return f.reported }
func (f *externalIntervalDatalogRegisterFailure) OriginalCause() error { return f.original }
func (f *externalIntervalDatalogRegisterFailure) CleanupCause() error  { return f.cleanup }
func (f *externalIntervalDatalogRegisterFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *externalIntervalDatalogRegisterFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExternalIntervalDatalogRegisterFailure(cause error, owner *externalIntervalDatalogRegisters) error {
	if owner == nil {
		return cause
	}
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &externalIntervalDatalogRegisterFailure{
		original: original, cleanup: cleanup, reported: reported,
		text: reported.Error(), owner: owner,
	}
}

// loadExternalIntervalDatalogRegisters mirrors instr_io LOAD. Declarations
// are supplied in compiler first-encounter order; repeated declarations reuse
// the existing register. Nonempty persistent products are cloned, while empty
// products occupy a nil register.
func (a *externalIntervalDatalogActivation) loadExternalIntervalDatalogRegisters(declarations []ast.ASTHandle) (_ *externalIntervalDatalogRegisters, resultErr error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	if a.program != nil && a.program.registers != nil {
		registers := a.program.registers
		if err := registers.open(); err != nil {
			return nil, err
		}
		for _, declaration := range declarations {
			if _, found := registers.indices[declaration]; !found {
				return nil, ErrExternalIntervalDatalogFragment
			}
		}
		return registers, nil
	}
	registers := &externalIntervalDatalogRegisters{
		activation: a,
		runtime:    a.manager.NumeralService().RuntimeOwner(),
		indices:    make(map[ast.ASTHandle]int, len(declarations)),
	}
	defer func() {
		if resultErr != nil {
			resultErr = finishExternalIntervalDatalogRegisterFailure(resultErr, registers)
		}
	}()
	// Source ensure_predicate_loaded validates/allocates every unique register
	// before execution begins. Match that boundary before invoking callbacks.
	for _, declaration := range declarations {
		if _, found := registers.indices[declaration]; found {
			continue
		}
		entry := a.entries[declaration]
		if entry == nil || entry.product == nil {
			return nil, ErrExternalIntervalDatalogFragment
		}
		registers.indices[declaration] = len(registers.slots)
		registers.slots = append(registers.slots, externalIntervalDatalogRegisterSlot{
			declaration: declaration,
			entry:       entry,
			persistent:  entry.product,
		})
	}
	for i := range registers.slots {
		slot := &registers.slots[i]
		empty, err := slot.persistent.Empty()
		if err != nil {
			return nil, err
		}
		if empty {
			continue
		}
		slot.product, err = slot.persistent.Clone()
		if err != nil {
			return nil, err
		}
	}
	return registers, nil
}

func (r *externalIntervalDatalogRegisters) open() error {
	if r == nil || r.closed || r.activation == nil || r.indices == nil {
		return ErrExternalIntervalDatalogFragment
	}
	return r.activation.open()
}

// relation returns the borrowed register product. Nil denotes an empty source
// register and is not an error.
func (r *externalIntervalDatalogRegisters) relation(declaration ast.ASTHandle) (*rel.ExternalIntervalProduct, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	index, found := r.indices[declaration]
	if !found || r.published {
		return nil, ErrExternalIntervalDatalogFragment
	}
	return r.slots[index].product, nil
}

// ensureRelation materializes the source UNION target when its loaded
// register is nil. The fresh product uses the persistent predicate's exact
// signature and ordered component kinds.
func (r *externalIntervalDatalogRegisters) ensureRelation(declaration ast.ASTHandle) (*rel.ExternalIntervalProduct, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	index, found := r.indices[declaration]
	if !found || r.published {
		return nil, ErrExternalIntervalDatalogFragment
	}
	slot := &r.slots[index]
	if slot.product != nil {
		return slot.product, nil
	}
	product, selected, err := rel.NewSelectedExternalIntervalProduct(r.activation.plugin, slot.entry.sorts, slot.entry.kinds)
	if err != nil {
		errors.As(err, &r.nested)
		return nil, err
	}
	if !selected || product == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	slot.product = product
	return product, nil
}

func (r *externalIntervalDatalogRegisters) cleanupNested() error {
	if r.nested == nil {
		return nil
	}
	if err := r.nested.Cleanup(); err != nil {
		return err
	}
	r.nested = nil
	return nil
}

// store mirrors instr_io STORE for every loaded predicate. Nil registers are
// first materialized as fresh empty products. After all construction and stale
// entry checks succeed, every replacement is published as one transaction;
// the displaced products are then retired. A retirement refusal therefore
// leaves activation entries on independent usable replacements and returns a
// retry owner without replaying EMPTY or CLONE callbacks.
func (r *externalIntervalDatalogRegisters) store() error {
	if err := r.open(); err != nil {
		return err
	}
	// A complete program owns one LOAD/STORE boundary. Individual rule
	// continuations finish their temporaries while the program keeps registers.
	if r.deferStore {
		return nil
	}
	if err := r.cleanupNested(); err != nil {
		return err
	}
	if !r.published {
		for i := range r.slots {
			slot := &r.slots[i]
			entry := r.activation.entries[slot.declaration]
			if entry != slot.entry || entry == nil || entry.product != slot.persistent {
				return ErrExternalIntervalDatalogFragment
			}
		}
		for i := range r.slots {
			slot := &r.slots[i]
			entry := slot.entry
			if slot.product != nil {
				continue
			}
			product, selected, err := rel.NewSelectedExternalIntervalProduct(r.activation.plugin, entry.sorts, entry.kinds)
			if err != nil {
				errors.As(err, &r.nested)
				return err
			}
			if !selected || product == nil {
				return ErrExternalIntervalDatalogFragment
			}
			slot.product = product
		}
		r.retired = make([]*rel.ExternalIntervalProduct, len(r.slots))
		for i := range r.slots {
			slot := &r.slots[i]
			r.retired[i] = slot.persistent
			slot.entry.product = slot.product
			slot.product = nil
		}
		r.retireNext = len(r.retired) - 1
		r.published = true
	}
	if err := r.retire(); err != nil {
		return &externalIntervalDatalogRegisterFailure{
			original: err, cleanup: err, reported: err,
			text: err.Error(), owner: r,
		}
	}
	r.finish()
	return nil
}

func (r *externalIntervalDatalogRegisters) retire() error {
	for r.retireNext >= 0 {
		if r.retired[r.retireNext] != nil {
			if err := r.retired[r.retireNext].Close(); err != nil {
				return err
			}
			r.retired[r.retireNext] = nil
		}
		r.retireNext--
	}
	return nil
}

func (r *externalIntervalDatalogRegisters) finish() {
	r.closed = true
	r.activation = nil
	r.slots = nil
	r.indices = nil
	r.retired = nil
	r.nested = nil
}

func (r *externalIntervalDatalogRegisters) close() error {
	if r == nil || r.closed {
		return nil
	}
	if err := r.cleanupNested(); err != nil {
		return err
	}
	if r.published {
		if err := r.retire(); err != nil {
			return err
		}
		r.finish()
		return nil
	}
	for i := len(r.slots) - 1; i >= 0; i-- {
		if r.slots[i].product != nil {
			if err := r.slots[i].product.Close(); err != nil {
				return err
			}
			r.slots[i].product = nil
		}
	}
	r.finish()
	return nil
}

func (r *externalIntervalDatalogRegisters) Close() error { return r.close() }
