package qe

// Source: src/qe/nlqsat.cpp:80-102,178-223,312-423.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type nlqSearch struct {
	self            *nlqSearch
	solver          *nlsat.Solver
	state           *nlqState
	isTrue          sat.Literal
	boundReal       [][]uint32
	boundBoolean    [][]sat.BoolVar
	frontendBoolean map[sat.BoolVar]bool
	extra           []sat.Literal
	first, second   *nlsat.LiteralVector
	held            *nlsat.LiteralVector
	rounds          uint32
	closed          bool
}

func newNLQSearch(solver *nlsat.Solver, isTrue sat.Literal, realLevels []MaxLevel, boundReal [][]uint32, boundBoolean [][]sat.BoolVar, frontendBoolean map[sat.BoolVar]bool) (search *nlqSearch, err error) {
	if solver == nil {
		return nil, arena.ErrConfig
	}
	s := &nlqSearch{
		solver: solver, isTrue: isTrue,
		boundReal: append([][]uint32(nil), boundReal...), boundBoolean: append([][]sat.BoolVar(nil), boundBoolean...),
		frontendBoolean: make(map[sat.BoolVar]bool, len(frontendBoolean)),
	}
	s.self = s
	for variable, present := range frontendBoolean {
		s.frontendBoolean[variable] = present
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.close())
		}
	}()
	s.state, err = newNLQState(solver, realLevels)
	if err != nil {
		return s, err
	}
	for level, variables := range boundBoolean {
		value := EmptyMaxLevel()
		if level&1 == 0 {
			value.Existential = uint32(level)
		} else {
			value.Universal = uint32(level)
		}
		for _, variable := range variables {
			if err = s.state.setLevel(variable, value); err != nil {
				return s, err
			}
		}
	}
	s.first, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return s, err
	}
	s.second, err = nlsat.NewLiteralVector(solver)
	if err != nil {
		return s, err
	}
	return s, nil
}

func (s *nlqSearch) open() error {
	if s == nil || s.self != s || s.solver == nil || s.state == nil {
		return arena.ErrConfig
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.state.open()
}

func (s *nlqSearch) extractProjection(level uint32) (variables []uint32, freeBoolean map[sat.BoolVar]bool) {
	freeBoolean = make(map[sat.BoolVar]bool)
	for i, block := range s.boundReal {
		if uint32(i) >= level {
			variables = append(variables, block...)
		}
	}
	for i, block := range s.boundBoolean {
		if uint32(i) < level {
			for _, variable := range block {
				freeBoolean[variable] = true
			}
		}
	}
	return variables, freeBoolean
}

func (s *nlqSearch) projectCube(core []sat.Literal, projectionLevel uint32) (level MaxLevel, clause []sat.Literal, err error) {
	if err = s.state.restoreModel(); err != nil {
		return level, nil, fmt.Errorf("restore projection model: %w", err)
	}
	if err = s.first.Reset(); err != nil {
		return level, nil, err
	}
	if err = s.second.Reset(); err != nil {
		return level, nil, err
	}
	variables, freeBoolean := s.extractProjection(projectionLevel)
	for _, literal := range core {
		variable := literal.Variable()
		if !s.frontendBoolean[variable] || freeBoolean[variable] {
			if err = s.first.Push(literal); err != nil {
				return level, nil, err
			}
		}
	}
	current, next := s.first, s.second
	var values []sat.Literal
	for i := len(variables); i > 0; i-- {
		if err = current.Literals(&values); err != nil {
			return level, nil, err
		}
		// explain::project's DEBUG_CODE SASSERT(m_solver.value(ls[i]) ==
		// l_true): debug builds only, since value() evaluates the atom and
		// charges resource steps.
		for i := 0; ast.ASTSourceDebug && i < len(values); i++ {
			value, cause := s.solver.LiteralValue(values[i])
			if cause != nil {
				return level, nil, fmt.Errorf("read projection literal %v: %w", values[i], cause)
			}
			if value != lbool.True {
				return level, nil, fmt.Errorf("projection literal %v has model value %v: %w", values[i], value, arena.ErrConfig)
			}
		}
		if err = s.solver.Project(variables[i-1], values, next); err != nil {
			return level, nil, fmt.Errorf("project real variable %d: %w", variables[i-1], err)
		}
		current, next = next, current
	}
	if err = current.Literals(&values); err != nil {
		return level, nil, err
	}
	// mbp returns: its local new_result releases the previous projection
	// step's literals (nlqsat.cpp:205-226). current stands for the caller's
	// clause cl, released after the caller adds the clause and pops.
	if err = next.Reset(); err != nil {
		return level, nil, err
	}
	s.held = current
	level = EmptyMaxLevel()
	clause = make([]sat.Literal, len(values), len(values)+1)
	for i, literal := range values {
		clause[i] = literal.Negated()
		literalLevel, cause := s.state.getLevel(clause[i])
		if cause != nil {
			return level, nil, fmt.Errorf("read projected literal level: %w", cause)
		}
		level.Merge(literalLevel)
	}
	return level, clause, nil
}

func (s *nlqSearch) enforceParity(clause []sat.Literal) []sat.Literal {
	if nlqExists(s.state.level()) {
		clause = append(clause, s.isTrue.Negated())
	} else {
		clause = append(clause, s.isTrue)
	}
	return clause
}

func (s *nlqSearch) projectCore(core []sat.Literal) (level MaxLevel, clause []sat.Literal, err error) {
	level, clause, err = s.projectCube(core, s.state.level()-1)
	if err == nil {
		clause = s.enforceParity(clause)
	}
	return level, clause, err
}

func (s *nlqSearch) projectConflict(core []sat.Literal) error {
	if !s.state.validModel {
		return s.state.pop(1)
	}
	if s.state.level() < 2 {
		return arena.ErrConfig
	}
	level, clause, err := s.projectCore(core)
	if err != nil {
		return err
	}
	if len(clause) == 0 {
		return arena.ErrConfig
	}
	if err = s.solver.AddClause(clause, nlsat.Assumption{}); err != nil {
		return err
	}
	var scopes uint32
	maximum := level.Max()
	if maximum == NoQuantifierLevel {
		scopes = 2 * (s.state.level() / 2)
	} else {
		if uint64(maximum)+2 > uint64(s.state.level()) {
			return arena.ErrConfig
		}
		scopes = s.state.level() - maximum
		if scopes&1 != 0 {
			scopes--
		}
		if scopes < 2 {
			return arena.ErrConfig
		}
	}
	if err = s.state.pop(scopes); err != nil {
		return err
	}
	return s.releaseHeld()
}

// releaseHeld ends project()'s clause cl, releasing the projected literals.
func (s *nlqSearch) releaseHeld() error {
	if s.held == nil {
		return nil
	}
	held := s.held
	s.held = nil
	return held.Reset()
}

func (s *nlqSearch) check() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	for {
		s.rounds++
		assumptions, err := s.state.initAssumptions(s.isTrue, s.extra)
		if err != nil {
			return lbool.Undef, fmt.Errorf("nlqsat initialize assumptions at level %d: %w", s.state.level(), err)
		}
		input := append([]sat.Literal(nil), assumptions...)
		status, err := s.solver.CheckAssumptions(&input)
		if err != nil {
			return lbool.Undef, fmt.Errorf("nlqsat check assumptions at level %d: %w", s.state.level(), err)
		}
		switch status {
		case lbool.True:
			if err = s.state.saveModel(); err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat save model at level %d: %w", s.state.level(), err)
			}
			if err = s.state.push(); err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat push level %d: %w", s.state.level(), err)
			}
		case lbool.False:
			if s.state.level() == 0 {
				return lbool.False, nil
			}
			if s.state.level() == 1 {
				return lbool.True, nil
			}
			if err = s.projectConflict(input); err != nil {
				return lbool.Undef, fmt.Errorf("nlqsat project at level %d: %w", s.state.level(), err)
			}
		case lbool.Undef:
			return lbool.Undef, nil
		}
	}
}

func (s *nlqSearch) close() (err error) {
	if s == nil || s.self != s {
		return arena.ErrConfig
	}
	if s.closed {
		return nil
	}
	if s.second != nil {
		if err = s.second.Close(); err != nil {
			return err
		}
		s.second = nil
	}
	if s.first != nil {
		if err = s.first.Close(); err != nil {
			return err
		}
		s.first = nil
	}
	if s.state != nil {
		if err = s.state.close(); err != nil {
			return err
		}
		s.state = nil
	}
	s.solver = nil
	s.closed = true
	return nil
}
