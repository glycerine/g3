package fp

// Source: src/muz/fp/dl_register_engine.cpp concrete fixedpoint engine factory.

import "fmt"

func (e *PropositionalEngine) selectBackend(kind EngineKind) error {
	if e.backend != kind.String() {
		e.queryReady = false
		if e.backend == EngineBMC.String() {
			if err := e.clearBMCTrace(); err != nil {
				return err
			}
			e.bmcStatistics.Reset()
		}
		if e.backend == EngineDDNF.String() {
			if err := e.ddnf.closeTrail(e.manager); err != nil {
				return err
			}
			if e.ddnf.registry != nil && !e.ddnf.registryBorrowed {
				e.ddnf.registry.Close()
			}
			e.ddnf = ddnfEngineState{output: e.ddnf.output}
		}
	}
	e.backend = kind.String()
	return nil
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
		return e.selectBackend(kind)
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
		return e.selectBackend(kind)
	case EnginePDR, EngineSpacer, EngineBMC, EngineCLP:
		if e.params == nil || e.globals == nil {
			return fmt.Errorf("muz/fp: fixedpoint engine %s requires parameter services", name)
		}
		return e.selectBackend(kind)
	case EngineDDNF:
		return e.selectBackend(kind)
	}
	return fmt.Errorf("muz/fp: fixedpoint engine %s is not ported", name)
}
