package api

// Public typed fixedpoint parameter keys.

import "testing"

func TestSMTContextTypedFixedpointParameters(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, context.Close()) })
	astTestMust(t, context.SetFixedpointSymbol(FixedpointEngine, "pdr"))
	astTestMust(t, context.SetFixedpointBool(FixedpointPDRFarkas, false))
	astTestMust(t, context.SetFixedpointBool(FixedpointTransformSlice, true))
	astTestMust(t, context.SetFixedpointBool(FixedpointSpacerReachDNF, false))
	astTestMust(t, context.SetFixedpointUInt(FixedpointPDRCacheMode, 2))
	astTestMust(t, context.SetFixedpointUInt(FixedpointSpacerMaximumLevel, 11))
	descriptors, err := context.FixedpointParameterDescriptors()
	astTestMust(t, err)
	known := make(map[string]bool, len(descriptors))
	for _, descriptor := range descriptors {
		known[descriptor.Name] = true
	}
	boolKeys := []FixedpointBoolParameter{
		FixedpointPDRFarkas, FixedpointPDRUseInductiveGeneralizer, FixedpointPDRUseMultiCoreGeneralizer,
		FixedpointPDRUseArithGeneralizer, FixedpointPDRInductiveReachability, FixedpointPDRUseConvexClosure,
		FixedpointPDRUseConvexInterior, FixedpointPDRUTVPI, FixedpointPDRUseModelGeneralizer,
		FixedpointPDRBreadthFirst, FixedpointPDRFlexibleTrace, FixedpointTransformSlice,
		FixedpointTransformInlineLinear, FixedpointTransformInlineEager, FixedpointTransformBitBlast,
		FixedpointTransformMagic, FixedpointTransformScale, FixedpointTransformKarr,
		FixedpointSpacerSkipPropagate, FixedpointSpacerResetObligations, FixedpointSpacerInitializeReach,
		FixedpointSpacerEliminateAuxiliary, FixedpointSpacerReachDNF, FixedpointSpacerRestarts,
		FixedpointSpacerInstantiate, FixedpointSpacerQuantifiedLemmas, FixedpointSpacerGroundCTI,
		FixedpointSpacerWeakAbstraction,
	}
	uintKeys := []FixedpointUIntParameter{
		FixedpointTimeout, FixedpointPDRMaximumContexts, FixedpointPDRCacheMode,
		FixedpointPDRFlexibleTraceDepth, FixedpointSpacerMaximumLevel, FixedpointSpacerOrderChildren,
		FixedpointSpacerRestartInitial, FixedpointSpacerRandomSeed,
	}
	for _, key := range boolKeys {
		if !known[string(key)] {
			t.Fatal("typed fixedpoint bool parameter is not catalogued", key)
		}
	}
	for _, key := range uintKeys {
		if !known[string(key)] {
			t.Fatal("typed fixedpoint uint parameter is not catalogued", key)
		}
	}
	if !known[string(FixedpointEngine)] {
		t.Fatal("typed fixedpoint engine parameter is not catalogued")
	}
}
