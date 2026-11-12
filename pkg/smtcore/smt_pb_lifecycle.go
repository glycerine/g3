package smtcore

// Source: theory_pb constructor/mk_fresh snapshots and context::copy_plugins.

import "github.com/glycerine/g3/pkg/ast"

// EnsurePseudoBooleanTheory installs the retained PB theory before the first
// solver scope or internalization. Optimization uses weighted PB bounds under
// arithmetic logics even when the input language did not contain PB terms.
func (s *SMTSolver) EnsurePseudoBooleanTheory() error {
	if err := s.open(); err != nil {
		return err
	}
	family, err := s.manager.RegisterPseudoBooleanPlugin()
	if err != nil {
		return err
	}
	// opt_solver::ensure_pb registers theory_pb before setup, so it leads
	// the plugin order and survives a profile without setup_card.
	if current := s.kernel.internalizer.theories[family]; current != nil {
		if _, ok := current.(*SMTPseudoBooleanTheory); !ok {
			return ast.ErrASTDomain
		}
		s.kernel.recordPresetupPlugin(family)
		return nil
	}
	theory, err := NewSMTPseudoBooleanTheory(family)
	if err != nil {
		return err
	}
	if err = s.kernel.RegisterTheory(theory); err != nil {
		return err
	}
	s.kernel.recordPresetupPlugin(family)
	return nil
}

func (t *SMTPseudoBooleanTheory) Setup() error {
	if _, err := t.State(); err != nil {
		return err
	}
	if !t.configured {
		if t.parameterOwner != nil {
			t.controls = t.parameterOwner.state
		}
		t.configured = true
		t.parameterOwner = nil
	}
	return nil
}

func (k *SMTKernel) copyPseudoBooleanPlugin(source *SMTKernel) error {
	if !source.preprocessingSetupDone {
		return nil
	}
	var previous *SMTPseudoBooleanTheory
	for _, plugin := range source.internalizer.theoryOrder {
		if pb, ok := plugin.(*SMTPseudoBooleanTheory); ok {
			previous = pb
			break
		}
	}
	if previous == nil {
		return nil
	}
	family, err := k.manager.RegisterPseudoBooleanPlugin()
	if err != nil {
		return err
	}
	if old := k.internalizer.theories[family]; old != nil {
		pb, ok := old.(*SMTPseudoBooleanTheory)
		if !ok || k.internalizer.theoriesSetup || len(k.internalizer.scopes) != 0 || len(pb.constraints) != 0 || len(pb.limits) != 0 {
			return ast.ErrASTDomain
		}
		pb.controls = previous.controls
		pb.configured = true
		pb.parameterOwner = nil
		return nil
	}
	pb, err := NewSMTPseudoBooleanTheory(family)
	if err != nil {
		return err
	}
	pb.controls = previous.controls
	pb.configured = true
	return k.RegisterTheory(pb)
}
