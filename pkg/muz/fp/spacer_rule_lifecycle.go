package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp check_reset/updt_params.

import "errors"

func (e *PropositionalEngine) closeSpacerForReplacement() (result error) {
	if e.spacer != nil {
		if err := e.spacer.Close(); err != nil {
			return err
		}
		e.spacer = nil
		e.spacerRevision = 0
	}
	if e.spacerRules != nil {
		result = errors.Join(result, e.spacerRules.Close())
		e.spacerRules = nil
	}
	return result
}

func (e *PropositionalEngine) spacerRulesCovered() (bool, error) {
	if e.spacer == nil || e.spacerRules == nil {
		return false, nil
	}
	count, err := e.spacerRules.Len()
	if err != nil || count == 0 {
		return false, err
	}
	// check_reset compares source rules before query preprocessing. Only
	// properties implied by rules that subsume every new rule may survive.
	return hornRuleSetSubsumes(e.spacerRules, e.set)
}
