package fp

// Source: src/muz/rel/dl_compiler.cpp compile_nonrecursive_stratum and
// do_compilation: one LOAD bank, dependency-first rules, then termination STORE.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type externalIntervalDatalogProgramRule struct {
	identity  *externalIntervalDatalogIdentityRule
	cartesian *externalIntervalDatalogCartesianRule
	stratum   uint32
}

func (r externalIntervalDatalogProgramRule) declarations() []ast.ASTHandle {
	if r.identity != nil {
		return []ast.ASTHandle{r.identity.head, r.identity.tail}
	}
	return []ast.ASTHandle{r.cartesian.head, r.cartesian.left, r.cartesian.right}
}

type externalIntervalDatalogProgram struct {
	rules        []externalIntervalDatalogProgramRule
	declarations []ast.ASTHandle
	schedule     []int
	registers    *externalIntervalDatalogRegisters
	next         int
	storing      bool
}

// Validate every rule before any callback. Ground rules remain on the existing
// single-rule path until their shared, program-wide singleton cache is lowered.
func (e *PropositionalEngine) externalIntervalProgramPlan() (*externalIntervalDatalogProgram, error) {
	count, err := e.set.Len()
	if err != nil || count < 2 {
		return nil, err
	}
	program := &externalIntervalDatalogProgram{}
	for index := 0; index < count; index++ {
		rule, cause := e.set.Rule(index)
		if cause != nil {
			return nil, cause
		}
		total, cause := rule.TailSize()
		plan := externalIntervalDatalogProgramRule{}
		if cause == nil {
			switch total {
			case 1:
				plan.identity, cause = e.externalIntervalIdentityRulePlan(rule)
			case 2:
				plan.cartesian, cause = e.externalIntervalCartesianRule(rule)
			default:
				cause = ErrExternalIntervalDatalogFragment
			}
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return nil, cause
		}
		program.rules = append(program.rules, plan)
		program.declarations = append(program.declarations, plan.declarations()...)
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return nil, err
	}
	if !finalized {
		valid, cause := e.set.Finalize()
		if cause != nil {
			return nil, cause
		}
		if !valid {
			return nil, ErrExternalIntervalDatalogFragment
		}
	}
	for index := range program.rules {
		plan := &program.rules[index]
		declarations := plan.declarations()
		plan.stratum, err = e.set.PredicateStratum(declarations[0])
		if err != nil {
			return nil, err
		}
		for _, tail := range declarations[1:] {
			stratum, cause := e.set.PredicateStratum(tail)
			if cause != nil {
				return nil, cause
			}
			if stratum >= plan.stratum {
				return nil, ErrExternalIntervalDatalogFragment
			}
		}
		program.schedule = append(program.schedule, index)
	}
	// A nonrecursive source stratum has one head; stable sorting preserves
	// the source rule vector order for multiple producers of that predicate.
	sort.SliceStable(program.schedule, func(i, j int) bool {
		return program.rules[program.schedule[i]].stratum < program.rules[program.schedule[j]].stratum
	})
	return program, nil
}

func (a *externalIntervalDatalogActivation) installExternalIntervalProgram(program *externalIntervalDatalogProgram) error {
	if a.program != nil {
		if len(a.program.rules) != len(program.rules) {
			return ErrExternalIntervalDatalogFragment
		}
		for index, left := range a.program.rules {
			right := program.rules[index]
			if left.stratum != right.stratum || !(sameExternalIntervalIdentityRule(left.identity, right.identity) || sameExternalIntervalCartesianRule(left.cartesian, right.cartesian)) {
				return ErrExternalIntervalDatalogFragment
			}
		}
		return nil
	}
	if a.pending != nil || a.ground != nil {
		return ErrExternalIntervalDatalogFragment
	}
	a.program = program
	a.identity, a.cartesian = nil, nil
	return nil
}

func (a *externalIntervalDatalogActivation) clearExternalIntervalProgramExecution() {
	a.program.registers = nil
	a.program.next = 0
	a.program.storing = false
	a.identity, a.cartesian = nil, nil
}

func (a *externalIntervalDatalogActivation) discardExternalIntervalProgramExecution() error {
	if a.program == nil || a.program.registers == nil {
		return nil
	}
	if err := a.program.registers.Close(); err != nil {
		failure := finishExternalIntervalDatalogRegisterFailure(err, a.program.registers)
		errors.As(failure, &a.pending)
		return failure
	}
	a.clearExternalIntervalProgramExecution()
	return nil
}

func (a *externalIntervalDatalogActivation) executeExternalIntervalProgram() error {
	program := a.program
	completed, err := a.resumeExternalIntervalIdentity()
	if err != nil {
		return err
	}
	if program.storing {
		a.clearExternalIntervalProgramExecution()
		return nil
	}
	if program.registers != nil && program.registers.closed {
		a.clearExternalIntervalProgramExecution()
	}
	if completed {
		program.next++
	}
	if program.registers == nil {
		registers, cause := a.loadExternalIntervalDatalogRegisters(program.declarations)
		if cause != nil {
			errors.As(cause, &a.pending)
			return cause
		}
		registers.deferStore = true
		program.registers = registers
	}
	for program.next < len(program.schedule) {
		plan := program.rules[program.schedule[program.next]]
		a.identity, a.cartesian = plan.identity, plan.cartesian
		if err := a.executeExternalIntervalIdentity(); err != nil {
			return err
		}
		program.next++
	}
	program.registers.deferStore = false
	program.storing = true
	if err := program.registers.store(); err != nil {
		var failure *externalIntervalDatalogRegisterFailure
		if errors.As(err, &failure) && failure.owner == program.registers {
			a.pending = failure
		} else {
			a.pending = newExternalIntervalDatalogIdentityFailure(err, program.registers, nil)
		}
		a.pendingCompletesRule = false
		return a.pending
	}
	a.clearExternalIntervalProgramExecution()
	return nil
}
