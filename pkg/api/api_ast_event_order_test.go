//go:build g3trace

package api

// AST id fidelity of the C API paths no SMT-LIB vector reaches
// (explanations, magic sets, wide udoc relations, external relations):
// TestASTEventOrder (cmd/g3) compares the mk_var_bug trace's mk_ast/del_ast
// events of the smt2 vectors with z3's; this test does the same for the C
// API reference programs in test_vectors (expl.c, scen.c, wide.c, ext.c)
// and the g3 API calls that mirror them (runExplanationsMode,
// runExplanationScenario, runUDocWide, runExternalPluginMode).
//
// z3's events come from the reference program compiled with the system C
// compiler against a z3 TRACE build's libz3.so (with a constructor that
// calls Z3_enable_trace("mk_var_bug")), run in a scratch directory; g3's
// from this test binary re-run on the one program with
// EnableTrace("mk_var_bug"). Set G3_CPP_TRACE_LIB to the directory of the
// TRACE build's libz3.so (<repo>/build_trace) to run it:
//
//	G3_CPP_TRACE_LIB=<repo>/build_trace go test -tags g3trace ./api -run TestAPIASTEventOrder
//
// expl.c, scen.c and ext.c end without deleting their context, so their
// events are compared with the same number of g3's first events (g3's test
// cleanup then releases its context); wide.c deletes its context and is
// compared whole.

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const apiASTEventCaseEnv = "G3_API_AST_EVENT_CASE"

type apiASTEventCase struct {
	program string // test_vectors/<program>
	args    []string
	whole   bool // the program deletes its context
	run     func(t *testing.T)
}

func (c apiASTEventCase) name() string {
	return strings.TrimSuffix(filepath.Base(c.program), ".c") + "_" + strings.Join(c.args, "_")
}

func apiASTEventCases() []apiASTEventCase {
	var cases []apiASTEventCase
	for _, mode := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15, 16} {
		cases = append(cases, apiASTEventCase{program: "explanations/expl.c", args: []string{strconv.Itoa(mode)},
			run: func(t *testing.T) { runExplanationsMode(t, mode) }})
	}
	for s := 1; s <= 28; s++ {
		cases = append(cases, apiASTEventCase{program: "explanations/scen.c", args: []string{strconv.Itoa(s)},
			run: func(t *testing.T) { runExplanationScenario(t, s) }})
	}
	for _, spec := range []struct {
		bits     uint32
		relation string
	}{{160, "doc"}, {65, "doc"}, {64, "doc"}, {8, "doc"}, {160, "hashtable"}, {200, "doc"}} {
		cases = append(cases, apiASTEventCase{program: "udoc_wide/wide.c", args: []string{strconv.Itoa(int(spec.bits)), spec.relation}, whole: true,
			run: func(t *testing.T) { runUDocWide(t, spec.bits, spec.relation) }})
	}
	for mode := 0; mode < len(externalPluginExpected); mode++ {
		cases = append(cases, apiASTEventCase{program: "external_relation/ext.c", args: []string{strconv.Itoa(mode)},
			run: func(t *testing.T) { runExternalPluginMode(t, mode) }})
	}
	return cases
}

// TestAPIASTEventOrderCase runs one case's g3 API calls with the
// mk_var_bug trace (the child process of TestAPIASTEventOrder).
func TestAPIASTEventOrderCase(t *testing.T) {
	name := os.Getenv(apiASTEventCaseEnv)
	if name == "" {
		t.Skip("run by TestAPIASTEventOrder")
	}
	for _, c := range apiASTEventCases() {
		if c.name() == name {
			astTestMust(t, EnableTrace("mk_var_bug"))
			c.run(t)
			return
		}
	}
	t.Fatalf("no case %s", name)
}

// apiTraceEvents reads the mk_ast/del_ast events of dir/.z3-trace.
func apiTraceEvents(t *testing.T, dir string) []string {
	f, err := os.Open(filepath.Join(dir, ".z3-trace"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<28)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "mk_ast: ") || strings.HasPrefix(line, "del_ast: ") {
			events = append(events, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestAPIASTEventOrder(t *testing.T) {
	lib := os.Getenv("G3_CPP_TRACE_LIB")
	if lib == "" {
		t.Skip("set G3_CPP_TRACE_LIB to a z3 TRACE build's libz3.so directory")
	}
	lib, err := filepath.Abs(lib)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("no C compiler")
	}
	include, err := filepath.Abs(filepath.Join("..", "..", "src", "api"))
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	shim := filepath.Join(bin, "shim.c")
	if err := os.WriteFile(shim, []byte("#include \"z3.h\"\n__attribute__((constructor)) static void g3_ast_trace(void) { Z3_enable_trace(\"mk_var_bug\"); }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	built := map[string]string{}
	for _, c := range apiASTEventCases() {
		t.Run(c.name(), func(t *testing.T) {
			exe := built[c.program]
			if exe == "" {
				src, err := filepath.Abs(filepath.Join("test_vectors", c.program))
				if err != nil {
					t.Fatal(err)
				}
				exe = filepath.Join(bin, strings.ReplaceAll(c.program, "/", "_"))
				out, err := exec.Command(cc, "-o", exe, src, shim, "-I"+include, "-L"+lib, "-lz3", "-Wl,-rpath,"+lib).CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v\n%s", c.program, err, out)
				}
				built[c.program] = exe
			}
			zdir, gdir := t.TempDir(), t.TempDir()
			cmd := exec.Command(exe, c.args...)
			cmd.Dir = zdir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("z3 %s: %v\n%s", c.name(), err, out)
			}
			cmd = exec.Command(self, "-test.run", "^TestAPIASTEventOrderCase$", "-test.count=1")
			cmd.Dir = gdir
			cmd.Env = append(os.Environ(), apiASTEventCaseEnv+"="+c.name())
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("g3 %s: %v\n%s", c.name(), err, out)
			}
			want, got := apiTraceEvents(t, zdir), apiTraceEvents(t, gdir)
			if !c.whole && len(got) > len(want) {
				got = got[:len(want)]
			}
			n := min(len(got), len(want))
			for j := 0; j < n; j++ {
				if got[j] != want[j] {
					lo := max(j-8, 0)
					t.Fatalf("event %d differs: g3 %q, z3 %q\ng3 events %d..%d: %q\nz3 events %d..%d: %q",
						j+1, got[j], want[j], lo+1, j+1, got[lo:j+1], lo+1, j+1, want[lo:j+1])
				}
			}
			if len(got) != len(want) {
				t.Fatalf("g3 has %d events, z3 %d", len(got), len(want))
			}
			t.Log(fmt.Sprintf("%d events", len(want)))
		})
	}
}
