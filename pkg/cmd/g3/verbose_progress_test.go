//go:build g3trace

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The IF_VERBOSE progress lines ported for AUDIT.md FUZZ-F5 are compiled
// only into g3trace builds (owner decision 2026-11-15), so this test is
// tagged. It runs the g3 front end in a child process (timeit reports go to
// std::cerr, the verbose lines to verbose_stream, both standard error in
// the C++) and compares the whole standard error, with timeit's :time and
// memory fields removed, against what build/z3 printed for the same
// arguments: test_vectors/verbose_progress_<name>.stderr, and with
// G3_CPP_REFERENCE set also the live reference binary.

type verboseProgressCase struct {
	name, vector string
	args         []string
}

var verboseProgressCases = []verboseProgressCase{
	// muz_pipeline2: filter_interpreted removes rows (remove_fact) in place;
	// a negated tail's unbound column is a total relation with no nullary
	// join; mk_full's aux table recycles; filter_interpreted_and_project
	// leaks its clone; reset_negated_tables removes rows.
	{"rel_negation_v2", "rel_negation", []string{"-v:2"}},
	// plugins_doc: udoc_plugin join result/orig/union/delta, dst/neg,
	// filter project result and filter_interpreted displays.
	{"rel_doc_v3", "rel_doc", []string{"-v:3"}},
	// check_relation over doc: the <objective> verified lines.
	{"rel_doc_checked_v3", "rel_doc_checked", []string{"-v:3"}},
	// Explicit table kinds next to udoc relations; xform.bit_blast.
	{"rel_doc_explicit_table_v3", "rel_doc_explicit_table", []string{"-v:3"}},
	{"rel_doc_bit_blast_v3", "rel_doc_bit_blast", []string{"-v:3"}},
	// default_table_negation_filter_fn's contains_fact; reset_negated_tables
	// on the relation get_relation created (no favourite migration).
	{"rel_negated_table_lifetime_v3", "rel_negated_table_lifetime", []string{"-v:3"}},
	{"rel_doc_bool_tables_v3", "rel_doc_bool_tables", []string{"-v:3"}},
	{"rel_filter_v2", "rel_filter", []string{"-v:2"}},
	// Relations exist only once get_relation creates them.
	{"horn_empty_model_v2", "horn_empty_model", []string{"-v:2"}},
	// (started "check-sat"), combined-solver, the three timeit lines,
	// (smt.configuring), (smt.collecting-features), (smt.searching),
	// (smt.final-check ...), the level-11 model dump.
	{"arith", "arith", []string{"-v:100"}},
	{"arith_v11", "arith", []string{"-v:11"}},
	// final-check per theory, (smt.mbqi) after the proto model dump, the
	// MBQI auxiliary context's own setup/timeit lines.
	{"quantified", "quantified", []string{"-v:100"}},
	{"quantified_no_mbqi", "quantified", []string{"-v:100", "smt.mbqi=false"}},
	// (smt.restarting ...), (smt.delete-inactive-clauses ...); at -v:100
	// the agility is fixed with two decimals because ~timeit left
	// std::fixed/setprecision(2) on std::cerr.
	{"pigeon8_v3", "pigeon8", []string{"-v:3"}},
	{"pigeon8_v100", "pigeon8", []string{"-v:100"}},
	// CFG_BASIC (setup_unknown) and :restart-outer for RS_IN_OUT_GEOMETRIC.
	{"pigeon8_basic", "pigeon8", []string{"-v:3", "auto_config=false"}},
	// (smt.working ...) every smt.tick conflicts of one bounded_search.
	{"pigeon9_v3", "pigeon9", []string{"-v:3"}},
	{"mixed_model", "mixed_model", []string{"-v:100"}},
	{"datatype", "datatype", []string{"-v:100"}},
	{"qf_s", "qf_s", []string{"-v:100"}},
	{"string", "string", []string{"-v:100"}},
	{"qf_bv", "qf_bv", []string{"-v:100"}},
	{"qf_lia", "qf_lia", []string{"-v:100"}},
	{"incremental", "incremental", []string{"-v:100"}},
	// CFG_LOGIC (push before check-sat): setup_default per logic, and
	// setup_seq_str's setup_str calling setup_arith again for z3str3.
	{"logic_incremental", "logic_incremental", []string{"-v:100"}},
	{"string_push", "string_push", []string{"-v:100"}},
	{"string_push_z3str3", "string_push", []string{"-v:100", "smt.string_solver=z3str3"}},
	{"array", "array", []string{"-v:100"}},
	{"usort", "usort", []string{"-v:100"}},
	{"unsat", "unsat", []string{"-v:100"}},
	// complete_model's (model.completion <name>) at level 12.
	{"model_completion", "model_completion", []string{"-v:12"}},
	// (get-consequences inc-limits) and the per-iteration progress line.
	{"consequences_v1", "consequences", []string{"-v:1"}},
	{"consequences_v100", "consequences", []string{"-v:100"}},
	// opt: (maxsmt), (opt.maxsat mutex ...), trace_bounds of maxres,
	// pd-maxres, wmax and sortmax, found optimum, is-sat/display_answer,
	// new assumption/core lines of maxres, theory_pb's no propagation,
	// is_maxsat's folded soft terms and the optsmt bound lines.
	{"opt_ms_v3", "opt_ms", []string{"-v:3"}},
	{"opt_ms_v100", "opt_ms", []string{"-v:100"}},
	{"opt_ms_pd_v100", "opt_ms", []string{"-v:100", "opt.maxsat_engine=pd-maxres"}},
	{"opt_ms_wmax_v10", "opt_ms", []string{"-v:10", "opt.maxsat_engine=wmax"}},
	{"opt_ms_sortmax_v10", "opt_ms", []string{"-v:10", "opt.maxsat_engine=sortmax"}},
	{"opt_ms2_wmax_v100", "opt_ms2", []string{"-v:100", "opt.maxsat_engine=wmax"}},
	{"opt_ms2_sortmax_v100", "opt_ms2", []string{"-v:100", "opt.maxsat_engine=sortmax"}},
	{"opt_conv_v10", "opt_conv", []string{"-v:10"}},
	{"opt_fold_v1", "opt_fold", []string{"-v:1"}},
	{"opt_soft_max_sortmax", "opt_soft_max", []string{"-v:1", "opt.maxsat_engine=sortmax"}},
	{"opt_box2_symba", "opt_box2", []string{"-v:2", "opt.optsmt_engine=symba"}},
	{"opt_box2_farkas", "opt_box2", []string{"-v:2", "opt.optsmt_engine=farkas"}},
	{"opt_lex2_basic", "opt_lex2", []string{"-v:2", "opt.optsmt_engine=basic"}},
	// theory_utvpi's parity conflict and set_conflict lines.
	{"utvpi_v2", "utvpi", []string{"-v:2"}},
	// theory_arith's Max. non linear arithmetic rounds.
	{"nl_rounds_v3", "nl_rounds", []string{"-v:3", "smt.arith.nl.rounds=1"}},
	// ctx_solver_simplify_tactic's (ctx-solver-simplify :num-steps N).
	{"ctx_solver_simplify_v10", "ctx_solver_simplify", []string{"-v:10"}},
	// muz/tab: run()'s instructions, apply_rule, select scores, the
	// subsumption index's setup-match/try-match/check lines, the
	// certificate with get_proof's body instance.
	{"tab_inv2_v2", "horn_inv2", []string{"-v:2", "fixedpoint.engine=tab"}},
	{"tab_count_v1", "horn_count", []string{"-v:1", "fixedpoint.engine=tab"}},
	{"tab_count_v2", "horn_count", []string{"-v:2", "fixedpoint.engine=tab"}},
	// bmc::query's display_rules at level 2.
	{"bmc_count_v2", "horn_count", []string{"-v:2", "fixedpoint.engine=bmc"}},
	// pdr farkas_learner's combined lemma.
	{"pdr_inv2_v2", "horn_inv2", []string{"-v:2", "fixedpoint.engine=pdr"}},
	// spacer: expand:/is-reachable:/create_child with the stopwatch times
	// masked, Entering level, Propagating:, the check_reachability and
	// propagate timeit lines, Itp Core:, the inductive property on l_false.
	{"spacer_inv2_v1", "horn_inv2", []string{"-v:1", "fixedpoint.engine=spacer"}},
	{"spacer_inv2_v2", "horn_inv2", []string{"-v:2", "fixedpoint.engine=spacer"}},
	{"spacer_count_v1", "horn_count", []string{"-v:1", "fixedpoint.engine=spacer"}},
	// ddnf: (query), (ddnf.preprocess), the insert/root/child lines,
	// (ddnf.compile) and the level-15 node display.
	{"ddnf_v10", "ddnf", []string{"-v:10", "fixedpoint.engine=ddnf"}},
	{"ddnf_v15", "ddnf", []string{"-v:15", "fixedpoint.engine=ddnf"}},
	// compile_eq's (or (= w id) ...) follows the ptr_hashtable of the
	// node's descendants (level-20 rule display).
	{"ddnf_v20", "ddnf", []string{"-v:20", "fixedpoint.engine=ddnf"}},
	// mk_bit_blast keeps only the bit-blasted query as output.
	{"spacer_bit_blast_v2", "bit_blast", []string{"-v:2"}},
	// horn_tactic's tactic_report("horn") line at level 10.
	{"spacer_inv2_v10", "horn_inv2", []string{"-v:10", "fixedpoint.engine=spacer"}},
	// with smt.mbqi on, mk_proto_model runs for checks that want no model:
	// its values shift the pdr lemma ids.
	{"pdr_count_v1", "horn_count", []string{"-v:1", "fixedpoint.engine=pdr"}},
	// spacer pob ids: create_next_child's post, ~derivation and normalize's
	// adhoc rewriter release their terms where the source does.
	{"spacer_projection_v1", "horn_projection", []string{"-v:1", "fixedpoint.engine=spacer"}},
	{"spacer_eq_prop_off_v1", "spacer_eq_prop_off", []string{"-v:1"}},
	// pdr create_children's rep, child_states and n_cube lifetimes.
	{"pdr_projection_v1", "horn_projection", []string{"-v:1"}},
}

// verboseProgressFiltered keeps, for each case, only the lines starting
// with the given prefix: the rest of those runs' output (opt) is not
// ported yet.
var verboseProgressFiltered = map[string][]string{
	// mus::get_mus1's (mus reducing core: ...) from maxres's minimize_core.
	"mus": {"(mus "},
	// opt::context::optimize's (optimize:check-sat), (optimize:sat) and
	// execute_lex's (opt :lex).
	"opt_lex":    {"(optimize:", "(opt :lex)"},
	"opt_pareto": {"(optimize:", "(opt :lex)"},
	"opt_box":    {"(optimize:", "(opt :lex)"},
	"opt_unsat":  {"(optimize:", "(opt :lex)"},
	// read_datalog's front-end lines, the parser's include: line and, at
	// level 10, original_rules.display_deps. The rest of the -dl output
	// (instruction log, relation code) is not ported yet.
	"dl_path_v1":    {"Z3 Datalog Engine", "parsing finished", "running saturation", "path -> ", "edge -> "},
	"dl_path_v10":   {"Z3 Datalog Engine", "parsing finished", "running saturation", "path -> ", "edge -> "},
	"dl_include_v2": {"Z3 Datalog Engine", "include: ", "parsing finished", "running saturation"},
	// instruction::log_verbose's displays (level 2) and the while loop's
	// "looping ... " (level 10); verbose_action timings and recycling are
	// not in these filters.
	"dl_path_v2_instructions":       {"store ", "load ", "union ", "clone ", "move ", "while", "    ", "join ", "join_project ", "mk_unary_singleton", "mark_saturated"},
	"dl_path_v10_instructions":      {"load ", "union ", "clone ", "move ", "while", "    ", "join ", "join_project ", "mk_unary_singleton", "mark_saturated", "looping ... "},
	"rel_bit_blast_v2_instructions": {"load ", "union ", "clone ", "move ", "while", "    ", "join ", "join_project ", "mk_unary_singleton", "mark_saturated"},
	// join_planner::run emits the rules in source order (fork fix, owner
	// decision 2026-11-15; z3 4.7.1 walked a rule-pointer address map), so
	// the relations are loaded and registered in that order.
	"rel_query_sequence_v2_instructions": {"load ", "union ", "clone ", "move ", "while", "    ", "join ", "join_project ", "mk_unary_singleton", "mark_saturated"},
	// get_grounded_concats' "concats N" lines (FUZZ-F5): z3str3 grounds a
	// variable through the concat of lowest AST id (fork fix, owner decision
	// 2026-11-15; z3 4.7.1 took the lowest heap address).
	"str_concats_order": {"concats "},
	// compute_contains walks contain_pair_bool_map, an obj_pair_map, in
	// hash-table order.
	"str_contains_order": {"concats "},
}

var verboseProgressFilteredCases = []verboseProgressCase{
	{"mus", "mus", []string{"-v:12"}},
	{"opt_lex", "opt_lex", []string{"-v:1"}},
	{"opt_pareto", "opt_pareto", []string{"-v:1"}},
	{"opt_box", "opt_box", []string{"-v:1"}},
	{"opt_unsat", "opt_unsat", []string{"-v:1"}},
	{"dl_path_v1", "path", []string{"-v:1", "-dl"}},
	{"dl_path_v10", "path", []string{"-v:10", "-dl"}},
	{"dl_include_v2", "include", []string{"-v:2", "-dl"}},
	// verbose_action's sparse_table lines: join_project (1), recycle and
	// remove_fact (2), add_fact (10), get_key_indexer, union, project, ...
	// (11).
	// table_relation's join returning a sparse table for bitvector
	// operands: "new type returned" (level 1).
	{"rel_new_type_v2", "rel_bitvector_join", []string{"-v:2"}},
	// rel_context::saturate's report_big_relations(1000) at level 10.
	{"rel_report_v10", "rel_report", []string{"-v:10"}},
	{"dl_path_v2_actions", "path", []string{"-v:2", "-dl"}},
	{"dl_path_v10_actions", "path", []string{"-v:10", "-dl"}},
	{"dl_path_v11_actions", "path", []string{"-v:11", "-dl"}},
	{"dl_path_v2_instructions", "path", []string{"-v:2", "-dl"}},
	{"dl_path_v10_instructions", "path", []string{"-v:10", "-dl"}},
	{"rel_bit_blast_v2_instructions", "rel_bit_blast", []string{"-v:2"}},
	{"rel_query_sequence_v2_instructions", "query_sequence", []string{"-v:2"}},
	{"str_concats_order", "str_concats_order", []string{"-v:100"}},
	{"str_contains_order", "str_contains_order", []string{"-v:100"}},
}

func filterVerboseProgress(name, log string) string {
	prefixes, found := verboseProgressFiltered[name]
	if !found {
		return log
	}
	var out strings.Builder
	for _, line := range strings.SplitAfter(log, "\n") {
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				out.WriteString(line)
				break
			}
		}
	}
	return out.String()
}

var verboseProgressMeasures = regexp.MustCompile(` :time [0-9.]+| :(before-|after-)?memory [0-9.]+`)

// verboseProgressTimes masks spacer's per-node stopwatch seconds and the rule
// transformer's elapsed seconds, which vary from run to run.
var verboseProgressNodeTimes = regexp.MustCompile(`(?m) (F|T|X|U|K|UNDEF) [0-9]+\.[0-9]+$`)
var verboseProgressTransformTimes = regexp.MustCompile(`(no-op|rules) [0-9.e-]+s\)`)

// verboseProgressActions masks verbose_action's elapsed seconds ("msg...0s",
// or the seconds alone when a nested action's line came in between).
var verboseProgressActions = regexp.MustCompile(`\.\.\.[0-9][0-9.e+-]*s`)
var verboseProgressActionTail = regexp.MustCompile(`(?m)^[0-9][0-9.e+-]*s$`)

// normalizeVerboseProgress drops timeit's time and memory fields, and the
// SAT solver's "checking model" notices: build/z3 is a release build, where
// that IF_VERBOSE sits under #ifndef _EXTERNAL_RELEASE, while g3 keeps the
// check (a separate release-profile question, not a FUZZ-F5 site).
func normalizeVerboseProgress(log string) string {
	log = verboseProgressMeasures.ReplaceAllString(log, "")
	log = verboseProgressNodeTimes.ReplaceAllString(log, " $1 t")
	log = verboseProgressTransformTimes.ReplaceAllString(log, "$1 Ts)")
	log = verboseProgressActions.ReplaceAllString(log, "...Ts")
	log = verboseProgressActionTail.ReplaceAllString(log, "Ts")
	var out strings.Builder
	for _, line := range strings.SplitAfter(log, "\n") {
		if !strings.HasPrefix(line, "\"checking model") {
			out.WriteString(line)
		}
	}
	return out.String()
}

const verboseProgressChildEnv = "G3_TEST_VERBOSE_PROGRESS_ARGS"

// runVerboseProgressChild runs g3 with args in a child copy of the test
// binary and returns its standard error.
func runVerboseProgressChild(t *testing.T, args []string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestVerboseProgressMessages$")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), verboseProgressChildEnv+"="+strings.Join(args, "\x1f"))
	var out, diagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatal(err, out.String(), diagnostics.String())
	}
	return diagnostics.String()
}

func TestVerboseProgressMessages(t *testing.T) {
	if args := os.Getenv(verboseProgressChildEnv); args != "" {
		os.Args = append([]string{"g3"}, strings.Split(args, "\x1f")...)
		main()
		return
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, tc := range append(append([]verboseProgressCase(nil), verboseProgressCases...), verboseProgressFilteredCases...) {
		t.Run(tc.name, func(t *testing.T) {
			extension := ".smt2"
			if strings.HasPrefix(tc.name, "dl_") {
				extension = ".datalog"
			}
			path, err := filepath.Abs(filepath.Join("test_vectors", "verbose_progress_"+tc.vector+extension))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join("test_vectors", "verbose_progress_"+tc.name+".stderr"))
			if err != nil {
				t.Fatal(err)
			}
			args := append(append([]string(nil), tc.args...), path)
			got := filterVerboseProgress(tc.name, normalizeVerboseProgress(runVerboseProgressChild(t, args)))
			if got != string(expected) {
				t.Fatalf("g3 standard error\n%s\nwant (build/z3)\n%s", got, expected)
			}
			if reference == "" {
				return
			}
			var native, cppLog bytes.Buffer
			cmd := exec.Command(reference, args...)
			cmd.Stdout = &native
			cmd.Stderr = &cppLog
			if err := cmd.Run(); err != nil {
				t.Fatal(err, native.String(), cppLog.String())
			}
			if cpp := filterVerboseProgress(tc.name, normalizeVerboseProgress(cppLog.String())); got != cpp {
				t.Fatalf("Go/C++ standard error differ\nGo:\n%s\nC++:\n%s", got, cpp)
			}
		})
	}
}

// verboseStatisticsCases check read_datalog's display_statistics sections,
// which IF_VERBOSE(2) writes to std::cout. The Time/Parsing lines are
// dropped. The generated rules list only the parser's output predicates.
var verboseStatisticsCases = []struct {
	name, vector string
}{
	{"dl_family_stats_v2", "family"},
	{"dl_path_stats_v2", "path"},
}

func filterVerboseStatistics(text string) string {
	var out strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(line, "Time: ") || strings.HasPrefix(line, "Parsing: ") {
			continue
		}
		out.WriteString(line)
	}
	return out.String()
}

func TestVerboseProgressDatalogStatistics(t *testing.T) {
	if os.Getenv(verboseProgressChildEnv) != "" {
		return
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, tc := range verboseStatisticsCases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("test_vectors", "verbose_progress_"+tc.vector+".datalog"))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(filepath.Join("test_vectors", "verbose_progress_"+tc.name+".stdout"))
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"-v:2", "-dl", path}
			cmd := exec.Command(os.Args[0], "-test.run=^TestVerboseProgressMessages$")
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), verboseProgressChildEnv+"="+strings.Join(args, "\x1f"))
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err = cmd.Run(); err != nil {
				t.Fatal(err, stdout.String(), stderr.String())
			}
			got := filterVerboseStatistics(stdout.String())
			// The child test binary reports PASS on its own stdout.
			got = strings.TrimSuffix(got, "PASS\n")
			if got != string(expected) {
				t.Fatalf("g3 standard output\n%s\nwant (build/z3)\n%s", got, expected)
			}
			if reference == "" {
				return
			}
			var native bytes.Buffer
			z3 := exec.Command(reference, args...)
			z3.Stdout = &native
			if err = z3.Run(); err != nil {
				t.Fatal(err, native.String())
			}
			if cpp := filterVerboseStatistics(native.String()); got != cpp {
				t.Fatalf("Go/C++ standard output differ\nGo:\n%s\nC++:\n%s", got, cpp)
			}
		})
	}
}
