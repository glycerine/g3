package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// fmaMnemonic matches the fused multiply-add instructions that the Go
// disassembler prints for arm64, ppc64le, s390x, riscv64 and amd64 (FMA3).
var fmaMnemonic = regexp.MustCompile(`^(V?F(N)?M(ADD|SUB)[0-9A-Z.]*|W?FN?M[AS][DE]?B?R?|N?M[AS][DE]BR?)$`)

// TestNoFMAContraction cross-builds g3 for FMA-capable targets and fails if
// the compiler fused a multiply with an add in any g3 function. z3's x86-64
// build rounds every product, so g3 must round explicitly with float64() to
// produce identical floating-point results on every platform.
//
// G3_FMACHECK_TARGETS overrides the default linux/arm64 target list, for
// example "linux/arm64 linux/ppc64le linux/s390x linux/riscv64 linux/amd64/v3".
func TestNoFMAContraction(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-builds g3; skipped under -short")
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
				target, len(sites), strings.Join(sites, "\n"))
		}
	}
}

// fusedSites disassembles bin and returns "file:line FUNC MNEMONIC" for each
// fused multiply-add instruction inside a github.com/glycerine/g3 function,
// plus the count of such instructions in the whole binary.
func fusedSites(goBin, bin string) (sites []string, total int, err error) {
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
		site := strings.TrimSpace(cols[0]) + " " + fn + " " + op[0]
		if !seen[site] {
			seen[site] = true
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
	sort.Strings(sites)
	return sites, total, nil
}
