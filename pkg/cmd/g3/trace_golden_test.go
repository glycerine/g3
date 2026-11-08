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
var goldenTraceTags = []string{"assign_core", "decide", "add_eq_detail", "mk_bool_var", "seq", "conflict"}

// goldenTraceSites lists the TRACE records the Go port emits, as
// "tag function file:line" with the source path relative to the repository.
// Records at other sites (z3 traces many more) are not compared. Add a site
// here when its record is ported.
var goldenTraceSites = map[string]bool{
	"assign_core assign_core src/smt/smt_context.cpp:293":                     true,
	"add_eq_detail add_eq src/smt/smt_context.cpp:497":                        true,
	"decide decide src/smt/smt_context.cpp:1829":                              true,
	"decide decide src/smt/smt_context.cpp:1885":                              true,
	"mk_bool_var mk_bool_var src/smt/smt_internalizer.cpp:825":                true,
	"seq add_axiom src/smt/theory_seq.cpp:3865":                               true,
	"seq deque_axiom src/smt/theory_seq.cpp:3098":                             true,
	"mk_enode mk_enode src/smt/smt_internalizer.cpp:939":                      true,
	"propagate_atoms propagate_atoms src/smt/smt_context.cpp:1380":            true,
	"propagate_relevancy propagate src/smt/smt_relevancy.cpp:463":             true,
	"propagate_relevancy relevant_eh src/smt/smt_context.cpp:1614":            true,
	"propagate_relevancy unmark_relevant_exprs src/smt/smt_relevancy.cpp:279": true,
	"propagate_relevancy_to_args propagate src/smt/smt_relevancy.cpp:462":     true,
	"seq get_automaton src/smt/theory_seq.cpp:4174":                           true,
}

// goldenIDMaskedSites print AST ids ("#123"). Until g3 allocates the same
// ids as z3 (TODO.md item 3) they are compared with the ids masked; every
// other field must still agree.
var goldenIDMaskedSites = map[string]bool{
	"conflict initialize_resolve src/smt/smt_conflict_resolution.cpp:403": true,
	"conflict initialize_resolve src/smt/smt_conflict_resolution.cpp:411": true,
	"conflict process_antecedent src/smt/smt_conflict_resolution.cpp:314": true,
	"conflict process_antecedent src/smt/smt_conflict_resolution.cpp:335": true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp:479":            true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp:487":            true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp:499":            true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp:507":            true,
	"conflict resolve src/smt/smt_conflict_resolution.cpp:573":            true,
	"conflict finalize_resolve src/smt/smt_conflict_resolution.cpp:432":   true,
	"conflict finalize_resolve src/smt/smt_conflict_resolution.cpp:439":   true,
	"conflict finalize_resolve src/smt/smt_conflict_resolution.cpp:462":   true,
}

var goldenASTID = regexp.MustCompile(`#[0-9]+`)

// goldenRecordText is the text a record is compared by.
func goldenRecordText(r traceRecord) string {
	if goldenIDMaskedSites[r.site] {
		return goldenASTID.ReplaceAllString(r.text, "#")
	}
	return r.text
}

// goldenVectorList names the vectors compared when G3_GOLDEN_VECTORS is
// unset, one per line. Each is expected to match z3 record for record at
// the sites above; add a vector once it does. seq_regex_length_disequality
// is block 12 of sequence_helper_inventory (see TODO.md item 11).
const goldenVectorList = "test_vectors/golden_trace_vectors.txt"

func goldenVectors(t *testing.T) []string {
	data, err := os.ReadFile(goldenVectorList)
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
	site string // "tag function file:line"
	text string // header and body as written
}

var buildTraceG3 struct {
	once sync.Once
	dir  string
	path string
	err  error
}

// cleanupTraceG3 removes the traced g3 build; TestTraceGolden registers it.
func cleanupTraceG3() {
	if buildTraceG3.dir != "" {
		_ = os.RemoveAll(buildTraceG3.dir)
	}
}

func goldenTraceG3(t *testing.T) string {
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
