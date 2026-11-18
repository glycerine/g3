//go:build g3trace

package main

// Golden trace comparison between a z3 TRACE build and g3 built with
// -tags g3trace. Both write TRACE records to .z3-trace in their working
// directory for the tags enabled with -tr:tag. The test runs both on the same
// input in separate directories and compares the records, in order, for the
// trace sites the Go port implements (goldenTraceSites), stopping at the
// first difference with the preceding records of both sides for context.
//
//	go test -tags g3trace ./cmd/g3 -run TestTraceGolden -v
//	G3_GOLDEN_VECTORS=seq_negative_symbolic_mixed,lia1 go test -tags g3trace ./cmd/g3 -run TestTraceGolden
//
// Environment:
//
//	G3_Z3_TRACE        z3 TRACE binary (default ../../../build_trace/z3 from this directory)
//	G3_GOLDEN_VECTORS  comma-separated vector names in test_vectors, or paths to .smt2 files
//	G3_GOLDEN_TAGS     comma-separated trace tags (default goldenTraceTags)
//	G3_GOLDEN_SLOW     also compare test_vectors/golden_trace_slow_vectors.txt
//	                   (traced runs of tens of minutes)
//	G3_GOLDEN_TIMEOUT  per-run time limit as a Go duration (default 10m, or
//	                   90m with G3_GOLDEN_SLOW; the traced g3 run of
//	                   flush_bound_axioms_introsort_q3063 takes about 50
//	                   minutes, z3's about 25)
//
// The test is skipped under -short and when the z3 TRACE binary is missing.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// goldenTraceTags are enabled on both sides by default.
var goldenTraceTags = []string{"assign_core", "decide", "add_eq_detail", "mk_bool_var", "seq", "conflict", "sls"}

// goldenTraceSites lists the TRACE records the Go port emits, as
// "tag function file" with the source path relative to the repository. Line
// numbers are deliberately not part of a site (see parseTraceRecords).
// Records at other sites (z3 traces many more) are not compared. Add a site
// here when its record is ported.
var goldenTraceSites = map[string]bool{
	// AUDIT.md smtcore_02-F10, deletion order of the lemma GC (not in
	// goldenTraceTags; compare with G3_GOLDEN_TAGS=del_inactive_lemmas
	// G3_GOLDEN_VECTORS=lemma_gc_half_sorted_order).
	"del_inactive_lemmas del_inactive_lemmas1 src/smt/smt_context.cpp":         true,
	"assign_core assign_core src/smt/smt_context.cpp":                          true,
	"add_eq_detail add_eq src/smt/smt_context.cpp":                             true,
	"decide decide src/smt/smt_context.cpp":                                    true,
	"mk_bool_var mk_bool_var src/smt/smt_internalizer.cpp":                     true,
	"seq add_axiom src/smt/theory_seq.cpp":                                     true,
	"seq deque_axiom src/smt/theory_seq.cpp":                                   true,
	"mk_enode mk_enode src/smt/smt_internalizer.cpp":                           true,
	"propagate_atoms propagate_atoms src/smt/smt_context.cpp":                  true,
	"propagate_relevancy propagate src/smt/smt_relevancy.cpp":                  true,
	"propagate_relevancy relevant_eh src/smt/smt_context.cpp":                  true,
	"propagate_relevancy unmark_relevant_exprs src/smt/smt_relevancy.cpp":      true,
	"propagate_relevancy_to_args propagate src/smt/smt_relevancy.cpp":          true,
	"seq get_automaton src/smt/theory_seq.cpp":                                 true,
	"diff_logic_vars mk_var src/smt/theory_diff_logic_def.h":                   true,
	"non_diff_logic internalize_term src/smt/theory_diff_logic_def.h":          true,
	"non_diff_logic found_non_diff_logic_expr src/smt/theory_diff_logic_def.h": true,
	"non_diff_logic mk_var src/smt/theory_diff_logic_def.h":                    true,
	// AUDIT.md smtcore_10-F12, dense difference logic (not in
	// goldenTraceTags; compare with
	// G3_GOLDEN_TAGS=ddl,assign_profile,arith,del_atoms,ddl_model,epsilon,ddl_detail,non_diff_logic
	// G3_GOLDEN_VECTORS=dense_equality_axiom_order,dense_assume_eqs_all,dense_real_trace_records).
	"ddl internalize_atom src/smt/theory_dense_diff_logic_def.h":                     true,
	"arith internalize_atom src/smt/theory_dense_diff_logic_def.h":                   true,
	"ddl internalize_term src/smt/theory_dense_diff_logic_def.h":                     true,
	"ddl assign_eh src/smt/theory_dense_diff_logic_def.h":                            true,
	"assign_profile assign_eh src/smt/theory_dense_diff_logic_def.h":                 true,
	"ddl_detail assign_eh src/smt/theory_dense_diff_logic_def.h":                     true,
	"del_atoms del_atoms src/smt/theory_dense_diff_logic_def.h":                      true,
	"ddl get_antecedents src/smt/theory_dense_diff_logic_def.h":                      true,
	"ddl update_cells src/smt/theory_dense_diff_logic_def.h":                         true,
	"ddl propagate_using_cell src/smt/theory_dense_diff_logic_def.h":                 true,
	"ddl add_edge src/smt/theory_dense_diff_logic_def.h":                             true,
	"ddl_model init_model src/smt/theory_dense_diff_logic_def.h":                     true,
	"ddl_model fix_zero src/smt/theory_dense_diff_logic_def.h":                       true,
	"epsilon compute_epsilon src/smt/theory_dense_diff_logic_def.h":                  true,
	"non_diff_logic found_non_diff_logic_expr src/smt/theory_dense_diff_logic_def.h": true,
	// AUDIT.md smtcore_10-F12, sparse difference logic (compare with
	// G3_GOLDEN_TAGS=arith,epsilon,arith_final,diff_logic_vars
	// G3_GOLDEN_VECTORS=diff_logic_trace_records).
	"arith internalize_term src/smt/theory_diff_logic_def.h":     true,
	"arith internalize_atom src/smt/theory_diff_logic_def.h":     true,
	"arith internalize_eq_eh src/smt/theory_diff_logic_def.h":    true,
	"arith_final final_check_eh src/smt/theory_diff_logic_def.h": true,
	"arith propagate_atom src/smt/theory_diff_logic_def.h":       true,
	"arith mk_term src/smt/theory_diff_logic_def.h":              true,
	"arith mk_var src/smt/theory_diff_logic_def.h":               true,
	"epsilon compute_delta src/smt/theory_diff_logic_def.h":      true,
	"arith mk_value src/smt/theory_diff_logic_def.h":             true,
	"arith make_feasible src/smt/diff_logic.h":                   true,
	// AUDIT.md math_polynomial_01-F1 (not in goldenTraceTags; compare with
	// G3_GOLDEN_TAGS=algebraic G3_GOLDEN_VECTORS=nra_root_sort_order).
	"algebraic compare src/math/polynomial/algebraic_numbers.cpp": true,
	// AUDIT.md math_polynomial_08-F1 (compare with G3_GOLDEN_TAGS=rem_bug
	// G3_GOLDEN_VECTORS=nra_rem_bug_trace).
	"rem_bug rem src/math/polynomial/upolynomial.cpp": true,
	// AUDIT.md math_polynomial_11-F1 (G3_GOLDEN_TAGS=upolynomial
	// G3_GOLDEN_VECTORS=nra_translate_q_trace).
	"upolynomial translate_q src/math/polynomial/upolynomial.cpp":         true,
	"upolynomial translate_bq src/math/polynomial/upolynomial.cpp":        true,
	"upolynomial descartes_bound_a_b src/math/polynomial/upolynomial.cpp": true,
	// AUDIT.md math_polynomial_10-F1 (G3_GOLDEN_TAGS=upolynomial
	// G3_GOLDEN_VECTORS=nra_cube_root_two).
	"upolynomial drs_isolate_0_1_roots src/math/polynomial/upolynomial.cpp":  true,
	"upolynomial drs_isolate_roots src/math/polynomial/upolynomial.cpp":      true,
	"upolynomial add_isolating_interval src/math/polynomial/upolynomial.cpp": true,
	"upolynomial add_root src/math/polynomial/upolynomial.cpp":               true,
	"upolynomial sqf_isolate_roots src/math/polynomial/upolynomial.cpp":      true,
	"upolynomial isolate_roots src/math/polynomial/upolynomial.cpp":          true,
	// AUDIT.md qe_03-F4 (not in goldenTraceTags; compare with
	// G3_GOLDEN_TAGS=qe,model_evaluator
	// G3_GOLDEN_VECTORS=fp_spacer_array_project_eqs). The qe sites of
	// array_project_eqs_util::project and ::operator() share their function
	// names with unported sites, so only these are listed.
	"qe find_arr_eqs src/muz/spacer/spacer_qe_project.cpp":                          true,
	"qe find_subst_term src/muz/spacer/spacer_qe_project.cpp":                       true,
	"model_evaluator eval_array_eq src/muz/spacer/spacer_mev_array.cpp":             true,
	"model_evaluator extract_array_func_interp src/muz/spacer/spacer_mev_array.cpp": true,
	// AUDIT.md tactic_sls_ufbv_01-F2: SLS tracker and engine sites.
	"sls operator() src/tactic/sls/sls_tactic.cpp":       true,
	"sls initialize src/tactic/sls/sls_tracker.h":        true,
	"sls reset src/tactic/sls/sls_tracker.h":             true,
	"sls randomize src/tactic/sls/sls_tracker.h":         true,
	"sls randomize_local src/tactic/sls/sls_evaluator.h": true,
	"sls mk_random_move src/tactic/sls/sls_engine.cpp":   true,
	// AUDIT.md qe_02-F1 (not in goldenTraceTags; compare with
	// G3_GOLDEN_TAGS=qe_lite G3_GOLDEN_VECTORS=qe_lite_der_singleton).
	"qe_lite flatten_definitions src/qe/qe_lite.cpp": true,
	// AUDIT.md qe_05-F6 (not in goldenTraceTags; compare with
	// G3_GOLDEN_TAGS=qe G3_GOLDEN_VECTORS=qe_partition_block_parent).
	"qe operator() src/qe/qe.cpp":            true,
	"qe eliminate_block src/qe/qe.cpp":       true,
	"qe solve src/qe/qe.cpp":                 true,
	"qe reduce_quantifier src/qe/qe.cpp":     true,
	"qe operator() src/qe/qe_sat_tactic.cpp": true,
	// The search-tree, quant_elim_plugin, qe-sat and qe tactic sites
	// (AUDIT.md qe_05-F6). The qe_arith_plugin.cpp sites are not ported.
	"qe operator() src/qe/qe_tactic.cpp":                        true,
	"qe extract_alt_form src/qe/qe_sat_tactic.cpp":              true,
	"qe qt src/qe/qe_sat_tactic.cpp":                            true,
	"qe add_constraint src/qe/qe_sat_tactic.cpp":                true,
	"qe blast_or src/qe/qe_sat_tactic.cpp":                      true,
	"qe project_var_partial src/qe/qe_sat_tactic.cpp":           true,
	"qe project_var_full src/qe/qe_sat_tactic.cpp":              true,
	"qe project src/qe/qe_sat_tactic.cpp":                       true,
	"qe check_success src/qe/qe_sat_tactic.cpp":                 true,
	"qe nnf_strengthen src/qe/qe_sat_tactic.cpp":                true,
	"qe nnf_strengthening_extrapolate src/qe/qe_sat_tactic.cpp": true,
	"qe smt_test_extrapolate src/qe/qe_sat_tactic.cpp":          true,
	"qe get_partition src/qe/qe.cpp":                            true,
	"qe extract_equalities src/qe/qe.cpp":                       true,
	"qe get_leaves src/qe/qe.cpp":                               true,
	"qe reset src/qe/qe.cpp":                                    true,
	"qe reset_free_vars src/qe/qe.cpp":                          true,
	"qe add_child src/qe/qe.cpp":                                true,
	"qe check src/qe/qe.cpp":                                    true,
	"qe final_check src/qe/qe.cpp":                              true,
	"qe elim_var src/qe/qe.cpp":                                 true,
	"qe add_var src/qe/qe.cpp":                                  true,
	"qe add_constraint src/qe/qe.cpp":                           true,
	"qe update_current src/qe/qe.cpp":                           true,
	"qe normalize src/qe/qe.cpp":                                true,
	"qe constrain_assignment src/qe/qe.cpp":                     true,
	"qe process_partition src/qe/qe.cpp":                        true,
	"qe find_min_weight src/qe/qe.cpp":                          true,
	"qe solve_vars src/qe/qe.cpp":                               true,
	// elim-small-bv's rw_cfg (compare with G3_GOLDEN_TAGS=elim_small_bv,
	// elim_small_bv_app,elim_small_bv_pre G3_GOLDEN_VECTORS=
	// verbose_progress_elim_small_bv).
	"elim_small_bv pre_visit src/tactic/bv/elim_small_bv_tactic.cpp":         true,
	"elim_small_bv replace_var src/tactic/bv/elim_small_bv_tactic.cpp":       true,
	"elim_small_bv reduce_quantifier src/tactic/bv/elim_small_bv_tactic.cpp": true,
	"elim_small_bv_app reduce_app src/tactic/bv/elim_small_bv_tactic.cpp":    true,
	"elim_small_bv_pre pre_visit src/tactic/bv/elim_small_bv_tactic.cpp":     true,
}

// goldenPointerMaskedSites print search_tree node addresses ("node 0x...");
// they are compared with the addresses masked.
var goldenPointerMaskedSites = map[string]bool{
	"qe add_child src/qe/qe.cpp":  true,
	"qe get_leaves src/qe/qe.cpp": true,
	"qe check src/qe/qe.cpp":      true,
}

var goldenPointer = regexp.MustCompile(`node 0x[0-9a-f]+`)

// goldenIDMaskedSites print AST ids ("#123"). Until g3 allocates the same
// ids as z3 (TODO.md item 3) they are compared with the ids masked; every
// other field must still agree.
var goldenIDMaskedSites = map[string]bool{
	"conflict initialize_resolve src/smt/smt_conflict_resolution.cpp": true,
	"conflict process_antecedent src/smt/smt_conflict_resolution.cpp": true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp":            true,
	"conflict finalize_resolve src/smt/smt_conflict_resolution.cpp":   true,
}

var goldenASTID = regexp.MustCompile(`#[0-9]+`)

// goldenTraceLine is the ":line" suffix of a TRACE header's file.
var goldenTraceLine = regexp.MustCompile(`:[0-9]+$`)

// goldenRecordText is the text a record is compared by.
func goldenRecordText(r traceRecord) string {
	if goldenIDMaskedSites[r.site] {
		return goldenASTID.ReplaceAllString(r.text, "#")
	}
	if goldenPointerMaskedSites[r.site] {
		return goldenPointer.ReplaceAllString(r.text, "node 0x")
	}
	return r.text
}

// goldenVectorList names the vectors compared when G3_GOLDEN_VECTORS is
// unset, one per line. Each is expected to match z3 record for record at
// the sites above; add a vector once it does. seq_regex_length_disequality
// is block 12 of sequence_helper_inventory (see TODO.md item 11).
const goldenVectorList = "test_vectors/golden_trace_vectors.txt"

// goldenSlowVectorList holds golden vectors whose traced runs take tens of
// minutes (flush_bound_axioms_introsort_q3063: z3 about 25, g3 about 50).
// They are compared only when G3_GOLDEN_SLOW is set, so the default run
// stays short.
const goldenSlowVectorList = "test_vectors/golden_trace_slow_vectors.txt"

func goldenVectors(t *testing.T) []string {
	out := readGoldenVectorList(t, goldenVectorList)
	if os.Getenv("G3_GOLDEN_SLOW") != "" {
		out = append(out, readGoldenVectorList(t, goldenSlowVectorList)...)
	}
	return out
}

func readGoldenVectorList(t *testing.T, list string) []string {
	data, err := os.ReadFile(list)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

const goldenContext = 30

type traceRecord struct {
	site string // "tag function file" (the :line is dropped)
	text string // header and body as written
}

var buildTraceG3 struct {
	once *sync.Once
	dir  string
	path string
	err  error
}

// cleanupTraceG3 removes the traced g3 build; TestTraceGolden and
// TestASTEventOrder register it. It also resets the build state so a later
// test rebuilds instead of reusing the path of the removed binary.
func cleanupTraceG3() {
	if buildTraceG3.dir != "" {
		_ = os.RemoveAll(buildTraceG3.dir)
	}
	buildTraceG3.once = nil
	buildTraceG3.dir = ""
	buildTraceG3.path = ""
	buildTraceG3.err = nil
}

func goldenTraceG3(t *testing.T) string {
	if buildTraceG3.once == nil {
		buildTraceG3.once = new(sync.Once)
	}
	buildTraceG3.once.Do(func() {
		dir, err := os.MkdirTemp("", "g3trace")
		if err != nil {
			buildTraceG3.err = err
			return
		}
		buildTraceG3.dir = dir
		buildTraceG3.path = filepath.Join(dir, "g3")
		cmd := exec.Command("go", "build", "-tags", "g3trace", "-o", buildTraceG3.path, ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildTraceG3.err = fmt.Errorf("go build -tags g3trace: %v\n%s", err, out)
		}
	})
	if buildTraceG3.err != nil {
		t.Fatal(buildTraceG3.err)
	}
	return buildTraceG3.path
}

func goldenZ3Trace(t *testing.T) string {
	if p := os.Getenv("G3_Z3_TRACE"); p != "" {
		return p
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "build_trace", "z3")
}

// runTraced runs bin with -tr:tag for each tag on input in a fresh directory
// and returns the .z3-trace records.
func runTraced(t *testing.T, bin, input string, tags []string) []traceRecord {
	dir := t.TempDir()
	args := make([]string, 0, len(tags)+1)
	for _, tag := range tags {
		args = append(args, "-tr:"+tag)
	}
	args = append(args, input)
	ctx, cancel := context.WithTimeout(context.Background(), goldenTimeout(t))
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	// The solvers' answers are not compared here; exit status is ignored
	// because some vectors end with an error response by design.
	_ = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out on %s", bin, input)
	}
	f, err := os.Open(filepath.Join(dir, ".z3-trace"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	defer f.Close()
	return parseTraceRecords(t, f)
}

func goldenTimeout(t *testing.T) time.Duration {
	s := os.Getenv("G3_GOLDEN_TIMEOUT")
	if s == "" {
		if os.Getenv("G3_GOLDEN_SLOW") != "" {
			return 90 * time.Minute
		}
		return 10 * time.Minute
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatalf("G3_GOLDEN_TIMEOUT: %v", err)
	}
	return d
}

// parseTraceRecords splits a .z3-trace stream into records. The z3 build
// names sources relative to its build directory ("../src/..."); the prefix
// is removed so both sides name them from the repository root.
func parseTraceRecords(t *testing.T, f *os.File) []traceRecord {
	const footer = "------------------------------------------------"
	var records []traceRecord
	var cur *traceRecord
	var body strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		line := sc.Text()
		if cur == nil && strings.HasPrefix(line, "-------- [") && strings.HasSuffix(line, " ---------") {
			header := strings.TrimSuffix(strings.TrimPrefix(line, "-------- ["), " ---------")
			header = strings.Replace(header, " ../src/", " src/", 1)
			tag, rest, _ := strings.Cut(header, "] ")
			// Sites are identified by tag, function and file, not by line:
			// a line number moves whenever the fork's source file is edited
			// above the TRACE (as the relevancy backport did), and that must
			// not break the comparison. No ported function has two sites
			// with the same tag of which only some are ported.
			rest = goldenTraceLine.ReplaceAllString(rest, "")
			cur = &traceRecord{site: tag + " " + rest}
			body.Reset()
			body.WriteString("-------- [" + tag + "] " + rest + " ---------\n")
			continue
		}
		if cur != nil && line == footer {
			cur.text = body.String()
			records = append(records, *cur)
			cur = nil
			continue
		}
		if cur != nil {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func filterTraceRecords(records []traceRecord) []traceRecord {
	out := records[:0:0]
	for _, r := range records {
		if goldenTraceSites[r.site] || goldenIDMaskedSites[r.site] {
			out = append(out, r)
		}
	}
	return out
}

func goldenVectorPath(name string) string {
	if strings.HasSuffix(name, ".smt2") {
		if abs, err := filepath.Abs(name); err == nil {
			return abs
		}
		return name
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "test_vectors", name+".smt2")
}

func TestTraceGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("golden trace comparison runs only without -short")
	}
	z3 := goldenZ3Trace(t)
	if _, err := os.Stat(z3); err != nil {
		t.Skipf("z3 TRACE build not found at %s (set G3_Z3_TRACE)", z3)
	}
	g3 := goldenTraceG3(t)
	t.Cleanup(cleanupTraceG3)
	tags := goldenTraceTags
	if s := os.Getenv("G3_GOLDEN_TAGS"); s != "" {
		tags = strings.Split(s, ",")
	}
	vectors := goldenVectors(t)
	if s := os.Getenv("G3_GOLDEN_VECTORS"); s != "" {
		vectors = strings.Split(s, ",")
	}
	if len(vectors) == 0 {
		t.Skip("no golden vectors yet; set G3_GOLDEN_VECTORS to compare a vector")
	}
	for _, name := range vectors {
		name := name
		t.Run(filepath.Base(name), func(t *testing.T) {
			input := goldenVectorPath(name)
			if _, err := os.Stat(input); err != nil {
				t.Fatal(err)
			}
			want := filterTraceRecords(runTraced(t, z3, input, tags))
			got := filterTraceRecords(runTraced(t, g3, input, tags))
			compareTraceRecords(t, want, got)
		})
	}
}

func compareTraceRecords(t *testing.T, want, got []traceRecord) {
	n := len(want)
	if len(got) < n {
		n = len(got)
	}
	for i := 0; i < n; i++ {
		if goldenRecordText(want[i]) == goldenRecordText(got[i]) {
			continue
		}
		var b strings.Builder
		from := i - goldenContext
		if from < 0 {
			from = 0
		}
		fmt.Fprintf(&b, "traces differ at record %d of z3 %d / g3 %d (matching records %d..%d shown first)\n", i, len(want), len(got), from, i-1)
		for j := from; j < i; j++ {
			fmt.Fprintf(&b, "%06d both:\n%s", j, want[j].text)
		}
		fmt.Fprintf(&b, "%06d z3:\n%s%06d g3:\n%s", i, want[i].text, i, got[i].text)
		t.Fatal(b.String())
	}
	if len(want) != len(got) {
		var b strings.Builder
		fmt.Fprintf(&b, "traces agree on %d records but z3 has %d and g3 has %d\n", n, len(want), len(got))
		if len(want) > n {
			fmt.Fprintf(&b, "next z3 record:\n%s", want[n].text)
		} else {
			fmt.Fprintf(&b, "next g3 record:\n%s", got[n].text)
		}
		t.Fatal(b.String())
	}
}
