package api

// Typed names for fixedpoint parameters consumed by retained Datalog, PDR,
// Spacer, and Horn preprocessing paths.

type FixedpointBoolParameter string
type FixedpointUIntParameter string
type FixedpointSymbolParameter string

const (
	FixedpointPDRFarkas                  FixedpointBoolParameter = "fixedpoint.pdr.farkas"
	FixedpointPDRUseInductiveGeneralizer FixedpointBoolParameter = "fixedpoint.pdr.use_inductive_generalizer"
	FixedpointPDRUseMultiCoreGeneralizer FixedpointBoolParameter = "fixedpoint.pdr.use_multicore_generalizer"
	FixedpointPDRUseArithGeneralizer     FixedpointBoolParameter = "fixedpoint.pdr.use_arith_inductive_generalizer"
	FixedpointPDRInductiveReachability   FixedpointBoolParameter = "fixedpoint.pdr.inductive_reachability_check"
	FixedpointPDRUseConvexClosure        FixedpointBoolParameter = "fixedpoint.pdr.use_convex_closure_generalizer"
	FixedpointPDRUseConvexInterior       FixedpointBoolParameter = "fixedpoint.pdr.use_convex_interior_generalizer"
	FixedpointPDRUTVPI                   FixedpointBoolParameter = "fixedpoint.pdr.utvpi"
	FixedpointPDRUseModelGeneralizer     FixedpointBoolParameter = "fixedpoint.pdr.use_model_generalizer"
	FixedpointPDRBreadthFirst            FixedpointBoolParameter = "fixedpoint.pdr.bfs_model_search"
	FixedpointPDRFlexibleTrace           FixedpointBoolParameter = "fixedpoint.pdr.flexible_trace"
	FixedpointTransformSlice             FixedpointBoolParameter = "fixedpoint.xform.slice"
	FixedpointTransformInlineLinear      FixedpointBoolParameter = "fixedpoint.xform.inline_linear"
	FixedpointTransformInlineEager       FixedpointBoolParameter = "fixedpoint.xform.inline_eager"
	FixedpointTransformBitBlast          FixedpointBoolParameter = "fixedpoint.xform.bit_blast"
	FixedpointTransformMagic             FixedpointBoolParameter = "fixedpoint.xform.magic"
	FixedpointTransformScale             FixedpointBoolParameter = "fixedpoint.xform.scale"
	FixedpointTransformKarr              FixedpointBoolParameter = "fixedpoint.xform.karr"
	FixedpointSpacerSkipPropagate        FixedpointBoolParameter = "fixedpoint.spacer.skip_propagate"
	FixedpointSpacerResetObligations     FixedpointBoolParameter = "fixedpoint.spacer.reset_obligation_queue"
	FixedpointSpacerInitializeReach      FixedpointBoolParameter = "fixedpoint.spacer.init_reach_facts"
	FixedpointSpacerEliminateAuxiliary   FixedpointBoolParameter = "fixedpoint.spacer.elim_aux"
	FixedpointSpacerReachDNF             FixedpointBoolParameter = "fixedpoint.spacer.reach_dnf"
	FixedpointSpacerRestarts             FixedpointBoolParameter = "fixedpoint.spacer.restarts"
	FixedpointSpacerInstantiate          FixedpointBoolParameter = "fixedpoint.spacer.instantiate"
	FixedpointSpacerQuantifiedLemmas     FixedpointBoolParameter = "fixedpoint.spacer.qlemmas"
	FixedpointSpacerGroundCTI            FixedpointBoolParameter = "fixedpoint.spacer.ground_cti"
	FixedpointSpacerWeakAbstraction      FixedpointBoolParameter = "fixedpoint.spacer.weak_abs"
)

const (
	FixedpointTimeout               FixedpointUIntParameter = "fixedpoint.timeout"
	FixedpointPDRMaximumContexts    FixedpointUIntParameter = "fixedpoint.pdr.max_num_contexts"
	FixedpointPDRCacheMode          FixedpointUIntParameter = "fixedpoint.pdr.cache_mode"
	FixedpointPDRFlexibleTraceDepth FixedpointUIntParameter = "fixedpoint.pdr.flexible_trace_depth"
	FixedpointSpacerMaximumLevel    FixedpointUIntParameter = "fixedpoint.spacer.max_level"
	FixedpointSpacerOrderChildren   FixedpointUIntParameter = "fixedpoint.spacer.order_children"
	FixedpointSpacerRestartInitial  FixedpointUIntParameter = "fixedpoint.spacer.restart_initial_threshold"
	FixedpointSpacerRandomSeed      FixedpointUIntParameter = "fixedpoint.spacer.random_seed"
)

const FixedpointEngine FixedpointSymbolParameter = "fixedpoint.engine"

func (c *SMTContext) SetFixedpointBool(parameter FixedpointBoolParameter, value bool) error {
	return c.SetBoolOption(string(parameter), value)
}

func (c *SMTContext) SetFixedpointUInt(parameter FixedpointUIntParameter, value uint32) error {
	return c.SetUIntOption(string(parameter), value)
}

func (c *SMTContext) SetFixedpointSymbol(parameter FixedpointSymbolParameter, value string) error {
	return c.SetSymbolOption(string(parameter), value)
}
