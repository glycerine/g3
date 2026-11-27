package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// fmaMnemonic matches the fused multiply-add instructions that the Go
// disassembler prints for arm64, ppc64le, s390x, riscv64 and amd64 (FMA3).
var fmaMnemonic = regexp.MustCompile(`^(V?F(N)?M(ADD|SUB)[0-9A-Z.]*|W?FN?M[AS][DE]?B?R?|N?M[AS][DE]BR?)$`)

// TestNoFMAContraction cross-builds g3, and the util/lp and util/numeral test
// binaries, for FMA-capable targets and fails if the compiler fused a multiply
// with an add in any g3 function (test code may fuse only explicit math.FMA
// calls). z3's x86-64
// build rounds every product, so g3 must round explicitly with float64() to
// produce identical floating-point results on every platform.
//
// G3_FMACHECK_TARGETS overrides the default linux/arm64 target list, for
// example "linux/arm64 linux/ppc64le linux/s390x linux/riscv64 linux/amd64/v3".
func TestNoFMAContraction(t *testing.T) {
	if !testing.Short() {
		t.Skip("cross-builds g3, a long test; runs only under -short")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		var lookErr error
		if goBin, lookErr = exec.LookPath("go"); lookErr != nil {
			t.Skip("go tool unavailable")
		}
	}
	targets := strings.Fields(os.Getenv("G3_FMACHECK_TARGETS"))
	if len(targets) == 0 {
		targets = []string{"linux/arm64"}
	}
	// Keep the archives used by the cross-builds until the last binary is
	// linked, even if go clean -cache runs against the shared build cache.
	t.Setenv("GOCACHE", t.TempDir())
	for _, target := range targets {
		parts := strings.Split(target, "/")
		if len(parts) < 2 {
			t.Fatalf("bad target %q, want GOOS/GOARCH[/level]", target)
		}
		env := append(os.Environ(), "GOOS="+parts[0], "GOARCH="+parts[1], "CGO_ENABLED=0")
		if len(parts) == 3 {
			env = append(env, "GO"+strings.ToUpper(parts[1])+"="+parts[2])
		}
		bin := filepath.Join(t.TempDir(), "g3")
		build := exec.Command(goBin, "build", "-o", bin, ".")
		build.Env = env
		if out, err := build.CombinedOutput(); err != nil {
			t.Skipf("cross-compiling for %s unavailable: %v\n%s", target, err, out)
		}
		sites, total, err := fusedSites(goBin, bin)
		if err != nil {
			t.Fatalf("%s: objdump: %v", target, err)
		}
		// Positive control: runtime and math use FMA on these targets.
		if total == 0 {
			t.Fatalf("%s: no fused multiply-add found anywhere; has the objdump format changed?", target)
		}
		if len(sites) > 0 {
			t.Errorf("%s: %d fused multiply-add sites in g3 code; wrap the product in float64():\n%s",
				target, len(sites), formatFMASites(sites))
		}

		// Test oracles must round where the product code rounds, so the
		// floating-point test binaries are checked too. Only explicit
		// math.FMA calls, which test g3's software fma, may stay fused.
		for _, pkg := range fmaTestPackages {
			pkgDir := filepath.Join("..", "..", filepath.FromSlash(pkg))
			testBin := filepath.Join(t.TempDir(), filepath.Base(pkg)+".test")
			build := exec.Command(goBin, "test", "-c", "-o", testBin, ".")
			build.Dir = pkgDir
			build.Env = env
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("%s: building %s tests: %v\n%s", target, pkg, err, out)
			}
			sites, _, err := fusedSites(goBin, testBin)
			if err != nil {
				t.Fatalf("%s: objdump %s tests: %v", target, pkg, err)
			}
			var bad []fmaSite
			allowed := 0
			for _, s := range sites {
				if callsMathFMA(pkgDir, s.loc) {
					allowed++
				} else {
					bad = append(bad, s)
				}
			}
			if pkg == "util/numeral" && allowed == 0 {
				t.Errorf("%s: %s: no fused math.FMA oracle found; has the objdump format changed?", target, pkg)
			}
			if len(bad) > 0 {
				t.Errorf("%s: %s: %d fused multiply-add sites in test code; wrap the product in float64():\n%s",
					target, pkg, len(bad), formatFMASites(bad))
			}
		}
	}
}

// fmaTestPackages lists the packages, relative to pkg/, whose test binaries
// compute floating-point oracles that must round like the product code.
var fmaTestPackages = []string{"util/lp", "util/numeral"}

// fmaSite is one fused multiply-add instruction in a g3 function.
type fmaSite struct {
	loc string // source "file.go:line" as objdump prints it
	fn  string
	op  string
}

func formatFMASites(sites []fmaSite) string {
	lines := make([]string, len(sites))
	for i, s := range sites {
		lines[i] = s.loc + " " + s.fn + " " + s.op
	}
	return strings.Join(lines, "\n")
}

// callsMathFMA reports whether the source line at loc ("file.go:line",
// resolved in dir) contains an explicit math.FMA call.
func callsMathFMA(dir, loc string) bool {
	i := strings.LastIndex(loc, ":")
	if i < 0 {
		return false
	}
	n, err := strconv.Atoi(loc[i+1:])
	if err != nil || n < 1 {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.Base(loc[:i])))
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	return n <= len(lines) && strings.Contains(lines[n-1], "math.FMA(")
}

// fusedSites disassembles bin and returns each distinct fused multiply-add
// instruction inside a github.com/glycerine/g3 function, sorted, plus the
// count of such instructions in the whole binary.
func fusedSites(goBin, bin string) (sites []fmaSite, total int, err error) {
	cmd := exec.Command(goBin, "tool", "objdump", bin)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	seen := make(map[string]bool)
	inG3 := false
	fn := ""
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "TEXT ") {
			fields := strings.Fields(line)
			fn = ""
			if len(fields) > 1 {
				fn = fields[1]
			}
			inG3 = strings.HasPrefix(fn, "github.com/glycerine/g3/")
			continue
		}
		// Instruction lines are tab separated: location, address, encoding, text.
		cols := strings.Split(line, "\t")
		text := ""
		for i := len(cols) - 1; i >= 0; i-- {
			if c := strings.TrimSpace(cols[i]); c != "" {
				text = c
				break
			}
		}
		op := strings.Fields(text)
		if len(op) == 0 || !fmaMnemonic.MatchString(strings.ToUpper(op[0])) {
			continue
		}
		total++
		if !inG3 {
			continue
		}
		site := fmaSite{loc: strings.TrimSpace(cols[0]), fn: fn, op: op[0]}
		if key := site.loc + " " + site.fn + " " + site.op; !seen[key] {
			seen[key] = true
			sites = append(sites, site)
		}
	}
	scanErr := sc.Err()
	if err := cmd.Wait(); err != nil {
		return nil, 0, err
	}
	if scanErr != nil {
		return nil, 0, scanErr
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		if a.loc != b.loc {
			return a.loc < b.loc
		}
		if a.fn != b.fn {
			return a.fn < b.fn
		}
		return a.op < b.op
	})
	return sites, total, nil
}
