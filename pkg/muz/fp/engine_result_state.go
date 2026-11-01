package fp

// Source: src/muz/base/dl_engine_base.h result contract,
// src/muz/pdr/pdr_dl_interface.{h,cpp}, and
// src/muz/spacer/spacer_dl_interface.{h,cpp}.

import "github.com/glycerine/g3/pkg/util/lbool"

// recordResultSnapshot publishes a result only after its query returned. The
// revision and engine kind prevent stale models/certificates from escaping
// after rules, parameters, or engine selection change.
func (e *PropositionalEngine) recordResultSnapshot(result lbool.LBool, queryErr error) {
	if queryErr != nil {
		e.queryReady = false
		e.lastResult = lbool.Undef
		return
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil || !kind.Retained() {
		e.queryReady = false
		e.lastResult = lbool.Undef
		return
	}
	e.queryReady = true
	e.queryRevision = e.revision
	e.queryEngine = kind
	e.lastResult = result
}

func (e *PropositionalEngine) requireResultSnapshot(kind EngineKind) error {
	if err := e.open(); err != nil {
		return err
	}
	selected, err := parseEngineKind(e.backend)
	if err != nil || selected != kind || !e.queried || !e.queryReady || e.queryEngine != kind || e.queryRevision != e.revision {
		return ErrPropositionalEngine
	}
	switch kind {
	case EnginePDR, EngineQPDR:
		if e.pdr == nil || e.pdrRevision != e.revision {
			return ErrPropositionalEngine
		}
	case EngineSpacer:
		if e.spacer == nil || e.spacerRevision != e.revision {
			return ErrPropositionalEngine
		}
	}
	return nil
}

func (e *PropositionalEngine) requirePDRSnapshot() error {
	kind, err := parseEngineKind(e.backend)
	if err != nil || (kind != EnginePDR && kind != EngineQPDR) {
		return ErrPropositionalEngine
	}
	return e.requireResultSnapshot(kind)
}

func (e *PropositionalEngine) requireBMCSnapshot() error {
	kind, err := parseEngineKind(e.backend)
	if err != nil || (kind != EngineBMC && kind != EngineQBMC) {
		return ErrPropositionalEngine
	}
	return e.requireResultSnapshot(kind)
}

// LastResult returns the retained source result for the latest non-stale
// query, including unknown. Mutations invalidate the snapshot.
func (e *PropositionalEngine) LastResult() (lbool.LBool, error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	kind, err := parseEngineKind(e.backend)
	if err != nil {
		return lbool.Undef, err
	}
	if e.queryClone != nil {
		if _, _, err = e.expressionQueryState(); err != nil {
			return lbool.Undef, err
		}
		return e.lastResult, nil
	}
	if err = e.requireResultSnapshot(kind); err != nil {
		return lbool.Undef, err
	}
	return e.lastResult, nil
}
