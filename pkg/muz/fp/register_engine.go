package fp

// Source: src/muz/fp/dl_register_engine.cpp concrete fixedpoint engine factory.

import "fmt"

func (e *PropositionalEngine) selectBackend(kind EngineKind) {
	if e.backend != kind.String() {
		e.queryReady = false
	}
	e.backend = kind.String()
}

// SelectEngine applies the retained fixedpoint engine factory branch.
func (e *PropositionalEngine) SelectEngine(name string) error {
	if err := e.open(); err != nil {
		return err
	}
	if name == "auto-config" {
		kind := EngineDatalog
		for _, state := range e.predicates {
			if state.table == nil {
				if e.params == nil || e.globals == nil {
					return fmt.Errorf("muz/fp: fixedpoint auto-config requires parameter services for infinite-domain predicates")
				}
				kind = EnginePDR
				break
			}
		}
		e.selectBackend(kind)
		return nil
	}
	kind, err := parseEngineKind(name)
	if err != nil {
		return err
	}
	if !kind.Retained() {
		return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
	}
	switch kind {
	case EngineDatalog:
		e.selectBackend(kind)
		return nil
	case EnginePDR, EngineSpacer:
		if e.params == nil || e.globals == nil {
			return fmt.Errorf("muz/fp: fixedpoint engine %s requires parameter services", name)
		}
		e.selectBackend(kind)
		return nil
	case EngineDDNF:
		e.selectBackend(kind)
		return nil
	}
	return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
}
