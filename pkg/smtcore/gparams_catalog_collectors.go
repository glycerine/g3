package smtcore

// Parameter collectors remain with the C++ definitions they translate.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Source: src/cmd_context/context_params.cpp
var contextParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "timeout", Kind: uparams.ParamUInt, Description: "default timeout (in milliseconds) used for solvers", Default: "4294967295"},
	uparams.NamedParamDescription{Name: "rlimit", Kind: uparams.ParamUInt, Description: "default resource limit used for solvers. Unrestricted when set to 0.", Default: "0"},
	uparams.NamedParamDescription{Name: "well_sorted_check", Kind: uparams.ParamBool, Description: "type checker", Default: "false"},
	uparams.NamedParamDescription{Name: "type_check", Kind: uparams.ParamBool, Description: "type checker (alias for well_sorted_check)", Default: "true"},
	uparams.NamedParamDescription{Name: "auto_config", Kind: uparams.ParamBool, Description: "use heuristics to automatically select solver and configure it", Default: "true"},
	uparams.NamedParamDescription{Name: "model_validate", Kind: uparams.ParamBool, Description: "validate models produced by solvers", Default: "false"},
	uparams.NamedParamDescription{Name: "dump_models", Kind: uparams.ParamBool, Description: "dump models whenever check-sat returns sat", Default: "false"},
	uparams.NamedParamDescription{Name: "trace", Kind: uparams.ParamBool, Description: "trace generation for VCC", Default: "false"},
	uparams.NamedParamDescription{Name: "trace_file_name", Kind: uparams.ParamString, Description: "trace out file name (see option 'trace')", Default: "z3.log"},
	uparams.NamedParamDescription{Name: "dot_proof_file", Kind: uparams.ParamString, Description: "file in which to output graphical proofs", Default: "proof.dot"},
	uparams.NamedParamDescription{Name: "debug_ref_count", Kind: uparams.ParamBool, Description: "debug support for AST reference counting", Default: "false"},
	uparams.NamedParamDescription{Name: "smtlib2_compliant", Kind: uparams.ParamBool, Description: "enable/disable SMT-LIB 2.0 compliance", Default: "false"},
)

func catalogCollectContextParams(d *uparams.ParamDescrs) error {
	if err := d.InsertGroup(contextParamsDescriptions); err != nil {
		return err
	}
	return catalogCollectContextSolverParams(d)
}

// Source: src/cmd_context/context_params.cpp
var contextSolverParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "proof", Kind: uparams.ParamBool, Description: "proof generation, it must be enabled when the Z3 context is created", Default: "false"},
	uparams.NamedParamDescription{Name: "model", Kind: uparams.ParamBool, Description: "model generation for solvers, this parameter can be overwritten when creating a solver", Default: "true"},
	uparams.NamedParamDescription{Name: "unsat_core", Kind: uparams.ParamBool, Description: "unsat-core generation for solvers, this parameter can be overwritten when creating a solver, not every solver in Z3 supports unsat core generation", Default: "false"},
)

func catalogCollectContextSolverParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(contextSolverParamsDescriptions)
}

// Source: src/ast/pattern/pattern_inference_params_helper.pyg
var patternInferenceParamsHelperDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "max_multi_patterns", Kind: uparams.ParamUInt, Description: "when patterns are not provided, the prover uses a heuristic to infer them, this option sets the threshold on the number of extra multi-patterns that can be created; by default, the prover creates at most one multi-pattern when there is no unary pattern", Default: "0", Module: "pi"},
	uparams.NamedParamDescription{Name: "block_loop_patterns", Kind: uparams.ParamBool, Description: "block looping patterns during pattern inference", Default: "true", Module: "pi"},
	uparams.NamedParamDescription{Name: "arith", Kind: uparams.ParamUInt, Description: "0 - do not infer patterns with arithmetic terms, 1 - use patterns with arithmetic terms if there is no other pattern, 2 - always use patterns with arithmetic terms", Default: "1", Module: "pi"},
	uparams.NamedParamDescription{Name: "use_database", Kind: uparams.ParamBool, Description: "use pattern database", Default: "false", Module: "pi"},
	uparams.NamedParamDescription{Name: "arith_weight", Kind: uparams.ParamUInt, Description: "default weight for quantifiers where the only available pattern has nested arithmetic terms", Default: "5", Module: "pi"},
	uparams.NamedParamDescription{Name: "non_nested_arith_weight", Kind: uparams.ParamUInt, Description: "default weight for quantifiers where the only available pattern has non nested arithmetic terms", Default: "10", Module: "pi"},
	uparams.NamedParamDescription{Name: "pull_quantifiers", Kind: uparams.ParamBool, Description: "pull nested quantifiers, if no pattern was found", Default: "true", Module: "pi"},
	uparams.NamedParamDescription{Name: "warnings", Kind: uparams.ParamBool, Description: "enable/disable warning messages in the pattern inference module", Default: "false", Module: "pi"},
)

func catalogCollectPatternInferenceParamsHelper(d *uparams.ParamDescrs) error {
	return d.InsertGroup(patternInferenceParamsHelperDescriptions)
}

// Source: src/interp/interp_params.pyg
var interpParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "profile", Kind: uparams.ParamBool, Description: "(INTERP) profile interpolation", Default: "false", Module: "interp"},
	uparams.NamedParamDescription{Name: "check", Kind: uparams.ParamBool, Description: "(INTERP) check interpolants", Default: "false", Module: "interp"},
)

func catalogCollectInterpParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(interpParamsDescriptions)
}

// Source: src/smt/params/smt_params_helper.pyg
var smtParamsHelperDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "auto_config", Kind: uparams.ParamBool, Description: "automatically configure solver", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "logic", Kind: uparams.ParamSymbol, Description: "logic used to setup the SMT solver", Default: "", Module: "smt"},
	uparams.NamedParamDescription{Name: "random_seed", Kind: uparams.ParamUInt, Description: "random seed for the smt solver", Default: "0", Module: "smt"},
	uparams.NamedParamDescription{Name: "relevancy", Kind: uparams.ParamUInt, Description: "relevancy propagation heuristic: 0 - disabled, 1 - relevancy is tracked by only affects quantifier instantiation, 2 - relevancy is tracked, and an atom is only asserted if it is relevant", Default: "2", Module: "smt"},
	uparams.NamedParamDescription{Name: "macro_finder", Kind: uparams.ParamBool, Description: "try to find universally quantified formulas that can be viewed as macros", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "quasi_macros", Kind: uparams.ParamBool, Description: "try to find universally quantified formulas that are quasi-macros", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "restricted_quasi_macros", Kind: uparams.ParamBool, Description: "try to find universally quantified formulas that are restricted quasi-macros", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "ematching", Kind: uparams.ParamBool, Description: "E-Matching based quantifier instantiation", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "phase_selection", Kind: uparams.ParamUInt, Description: "phase selection heuristic: 0 - always false, 1 - always true, 2 - phase caching, 3 - phase caching conservative, 4 - phase caching conservative 2, 5 - random, 6 - number of occurrences", Default: "3", Module: "smt"},
	uparams.NamedParamDescription{Name: "restart_strategy", Kind: uparams.ParamUInt, Description: "0 - geometric, 1 - inner-outer-geometric, 2 - luby, 3 - fixed, 4 - arithmetic", Default: "1", Module: "smt"},
	uparams.NamedParamDescription{Name: "restart_factor", Kind: uparams.ParamDouble, Description: "when using geometric (or inner-outer-geometric) progression of restarts, it specifies the constant used to multiply the currect restart threshold", Default: "1.1", Module: "smt"},
	uparams.NamedParamDescription{Name: "case_split", Kind: uparams.ParamUInt, Description: "0 - case split based on variable activity, 1 - similar to 0, but delay case splits created during the search, 2 - similar to 0, but cache the relevancy, 3 - case split based on relevancy (structural splitting), 4 - case split on relevancy and activity, 5 - case split on relevancy and current goal, 6 - activity-based case split with theory-aware branching activity", Default: "1", Module: "smt"},
	uparams.NamedParamDescription{Name: "delay_units", Kind: uparams.ParamBool, Description: "if true then z3 will not restart when a unit clause is learned", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "delay_units_threshold", Kind: uparams.ParamUInt, Description: "maximum number of learned unit clauses before restarting, ignored if delay_units is false", Default: "32", Module: "smt"},
	uparams.NamedParamDescription{Name: "pull_nested_quantifiers", Kind: uparams.ParamBool, Description: "pull nested quantifiers", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "refine_inj_axioms", Kind: uparams.ParamBool, Description: "refine injectivity axioms", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "timeout", Kind: uparams.ParamUInt, Description: "timeout (in milliseconds) (UINT_MAX and 0 mean no timeout)", Default: "4294967295", Module: "smt"},
	uparams.NamedParamDescription{Name: "rlimit", Kind: uparams.ParamUInt, Description: "resource limit (0 means no limit)", Default: "0", Module: "smt"},
	uparams.NamedParamDescription{Name: "max_conflicts", Kind: uparams.ParamUInt, Description: "maximum number of conflicts before giving up.", Default: "4294967295", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi", Kind: uparams.ParamBool, Description: "model based quantifier instantiation (MBQI)", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.max_cexs", Kind: uparams.ParamUInt, Description: "initial maximal number of counterexamples used in MBQI, each counterexample generates a quantifier instantiation", Default: "1", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.max_cexs_incr", Kind: uparams.ParamUInt, Description: "increment for MBQI_MAX_CEXS, the increment is performed after each round of MBQI", Default: "0", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.max_iterations", Kind: uparams.ParamUInt, Description: "maximum number of rounds of MBQI", Default: "1000", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.trace", Kind: uparams.ParamBool, Description: "generate tracing messages for Model Based Quantifier Instantiation (MBQI). It will display a message before every round of MBQI, and the quantifiers that were not satisfied", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.force_template", Kind: uparams.ParamUInt, Description: "some quantifiers can be used as templates for building interpretations for functions. Z3 uses heuristics to decide whether a quantifier will be used as a template or not. Quantifiers with weight >= mbqi.force_template are forced to be used as a template", Default: "10", Module: "smt"},
	uparams.NamedParamDescription{Name: "mbqi.id", Kind: uparams.ParamString, Description: "Only use model-based instantiation for quantifiers with id's beginning with string", Default: "", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.profile", Kind: uparams.ParamBool, Description: "profile quantifier instantiation", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.profile_freq", Kind: uparams.ParamUInt, Description: "how frequent results are reported by qi.profile", Default: "4294967295", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.max_instances", Kind: uparams.ParamUInt, Description: "maximum number of quantifier instantiations", Default: "4294967295", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.eager_threshold", Kind: uparams.ParamDouble, Description: "threshold for eager quantifier instantiation", Default: "10.0", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.lazy_threshold", Kind: uparams.ParamDouble, Description: "threshold for lazy quantifier instantiation", Default: "20.0", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.cost", Kind: uparams.ParamString, Description: "expression specifying what is the cost of a given quantifier instantiation", Default: "(+ weight generation)", Module: "smt"},
	uparams.NamedParamDescription{Name: "qi.max_multi_patterns", Kind: uparams.ParamUInt, Description: "specify the number of extra multi patterns", Default: "0", Module: "smt"},
	uparams.NamedParamDescription{Name: "bv.reflect", Kind: uparams.ParamBool, Description: "create enode for every bit-vector term", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "bv.enable_int2bv", Kind: uparams.ParamBool, Description: "enable support for int2bv and bv2int operators", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.random_initial_value", Kind: uparams.ParamBool, Description: "use random initial values in the simplex-based procedure for linear arithmetic", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.solver", Kind: uparams.ParamUInt, Description: "arithmetic solver: 0 - no solver, 1 - bellman-ford based solver (diff. logic only), 2 - simplex based solver, 3 - floyd-warshall based solver (diff. logic only) and no theory combination", Default: "2", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.nl", Kind: uparams.ParamBool, Description: "(incomplete) nonlinear arithmetic support based on Groebner basis and interval propagation", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.nl.gb", Kind: uparams.ParamBool, Description: "groebner Basis computation, this option is ignored when arith.nl=false", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.nl.branching", Kind: uparams.ParamBool, Description: "branching on integer variables in non linear clusters", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.nl.rounds", Kind: uparams.ParamUInt, Description: "threshold for number of (nested) final checks for non linear arithmetic", Default: "1024", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.euclidean_solver", Kind: uparams.ParamBool, Description: "eucliean solver for linear integer arithmetic", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.propagate_eqs", Kind: uparams.ParamBool, Description: "propagate (cheap) equalities", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.propagation_mode", Kind: uparams.ParamUInt, Description: "0 - no propagation, 1 - propagate existing literals, 2 - refine bounds", Default: "2", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.reflect", Kind: uparams.ParamBool, Description: "reflect arithmetical operators to the congruence closure", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.branch_cut_ratio", Kind: uparams.ParamUInt, Description: "branch/cut ratio for linear integer arithmetic", Default: "2", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.int_eq_branch", Kind: uparams.ParamBool, Description: "branching using derived integer equations", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.ignore_int", Kind: uparams.ParamBool, Description: "treat integer variables as real", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.dump_lemmas", Kind: uparams.ParamBool, Description: "dump arithmetic theory lemmas to files", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "arith.greatest_error_pivot", Kind: uparams.ParamBool, Description: "Pivoting strategy", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "pb.conflict_frequency", Kind: uparams.ParamUInt, Description: "conflict frequency for Pseudo-Boolean theory", Default: "1000", Module: "smt"},
	uparams.NamedParamDescription{Name: "pb.learn_complements", Kind: uparams.ParamBool, Description: "learn complement literals for Pseudo-Boolean theory", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "pb.enable_compilation", Kind: uparams.ParamBool, Description: "enable compilation into sorting circuits for Pseudo-Boolean", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "pb.enable_simplex", Kind: uparams.ParamBool, Description: "enable simplex to check rational feasibility", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "array.weak", Kind: uparams.ParamBool, Description: "weak array theory", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "array.extensional", Kind: uparams.ParamBool, Description: "extensional array theory", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack", Kind: uparams.ParamUInt, Description: "0 - disable dynamic ackermannization, 1 - expand Leibniz's axiom if a congruence is the root of a conflict, 2 - expand Leibniz's axiom if a congruence is used during conflict resolution", Default: "1", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack.eq", Kind: uparams.ParamBool, Description: "enable dynamic ackermannization for transtivity of equalities", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack.factor", Kind: uparams.ParamDouble, Description: "number of instance per conflict", Default: "0.1", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack.gc", Kind: uparams.ParamUInt, Description: "Dynamic ackermannization garbage collection frequency (per conflict)", Default: "2000", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack.gc_inv_decay", Kind: uparams.ParamDouble, Description: "Dynamic ackermannization garbage collection decay", Default: "0.8", Module: "smt"},
	uparams.NamedParamDescription{Name: "dack.threshold", Kind: uparams.ParamUInt, Description: " number of times the congruence rule must be used before Leibniz's axiom is expanded", Default: "10", Module: "smt"},
	uparams.NamedParamDescription{Name: "theory_case_split", Kind: uparams.ParamBool, Description: "Allow the context to use heuristics involving theory case splits, which are a set of literals of which exactly one can be assigned True. If this option is false, the context will generate extra axioms to enforce this instead.", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "string_solver", Kind: uparams.ParamSymbol, Description: "solver for string/sequence theories. options are: 'z3str3' (specialized string solver), 'seq' (sequence solver), 'auto' (use static features to choose best solver)", Default: "seq", Module: "smt"},
	uparams.NamedParamDescription{Name: "core.validate", Kind: uparams.ParamBool, Description: "validate unsat core produced by SMT context", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.strong_arrangements", Kind: uparams.ParamBool, Description: "assert equivalences instead of implications when generating string arrangement axioms", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.aggressive_length_testing", Kind: uparams.ParamBool, Description: "prioritize testing concrete length values over generating more options", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.aggressive_value_testing", Kind: uparams.ParamBool, Description: "prioritize testing concrete string constant values over generating more options", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.aggressive_unroll_testing", Kind: uparams.ParamBool, Description: "prioritize testing concrete regex unroll counts over generating more options", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.fast_length_tester_cache", Kind: uparams.ParamBool, Description: "cache length tester constants instead of regenerating them", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.fast_value_tester_cache", Kind: uparams.ParamBool, Description: "cache value tester constants instead of regenerating them", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.string_constant_cache", Kind: uparams.ParamBool, Description: "cache all generated string constants generated from anywhere in theory_str", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.use_binary_search", Kind: uparams.ParamBool, Description: "use a binary search heuristic for finding concrete length values for free variables in theory_str (set to False to use linear search)", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.binary_search_start", Kind: uparams.ParamUInt, Description: "initial upper bound for theory_str binary search", Default: "64", Module: "smt"},
	uparams.NamedParamDescription{Name: "theory_aware_branching", Kind: uparams.ParamBool, Description: "Allow the context to use extra information from theory solvers regarding literal branching prioritization.", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.finite_overlap_models", Kind: uparams.ParamBool, Description: "attempt a finite model search for overlapping variables instead of completely giving up on the arrangement", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.overlap_priority", Kind: uparams.ParamDouble, Description: "theory-aware priority for overlapping variable cases; use smt.theory_aware_branching=true", Default: "-0.1", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata", Kind: uparams.ParamBool, Description: "use automata-based reasoning for regular expressions (Z3str3 only)", Default: "true", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata_difficulty_threshold", Kind: uparams.ParamUInt, Description: "difficulty threshold for regex automata heuristics", Default: "1000", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata_intersection_difficulty_threshold", Kind: uparams.ParamUInt, Description: "difficulty threshold for regex intersection heuristics", Default: "1000", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata_failed_automaton_threshold", Kind: uparams.ParamUInt, Description: "number of failed automaton construction attempts after which a full automaton is automatically built", Default: "10", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata_failed_intersection_threshold", Kind: uparams.ParamUInt, Description: "number of failed automaton intersection attempts after which intersection is always computed", Default: "10", Module: "smt"},
	uparams.NamedParamDescription{Name: "str.regex_automata_length_attempt_threshold", Kind: uparams.ParamUInt, Description: "number of length/path constraint attempts before checking unsatisfiability of regex terms", Default: "10", Module: "smt"},
	uparams.NamedParamDescription{Name: "core.minimize", Kind: uparams.ParamBool, Description: "minimize unsat core produced by SMT context", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "core.extend_patterns", Kind: uparams.ParamBool, Description: "extend unsat core with literals that trigger (potential) quantifier instances", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "core.extend_patterns.max_distance", Kind: uparams.ParamUInt, Description: "limits the distance of a pattern-extended unsat core", Default: "4294967295", Module: "smt"},
	uparams.NamedParamDescription{Name: "core.extend_nonlocal_patterns", Kind: uparams.ParamBool, Description: "extend unsat cores with literals that have quantifiers with patterns that contain symbols which are not in the quantifier's body", Default: "false", Module: "smt"},
	uparams.NamedParamDescription{Name: "lemma_gc_strategy", Kind: uparams.ParamUInt, Description: "lemma garbage collection strategy: 0 - fixed, 1 - geometric, 2 - at restart, 3 - none", Default: "0", Module: "smt"},
)

func catalogCollectSmtParamsHelper(d *uparams.ParamDescrs) error {
	return d.InsertGroup(smtParamsHelperDescriptions)
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectContextParams", catalogCollectContextParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectContextSolverParams", catalogCollectContextSolverParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectPatternInferenceParamsHelper", catalogCollectPatternInferenceParamsHelper)
	uparams.RegisterGlobalParamsCollector("catalogCollectInterpParams", catalogCollectInterpParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectSmtParamsHelper", catalogCollectSmtParamsHelper)
}
