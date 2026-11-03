# Recursive QBMC C++ reproducer

`recursive_qbmc.cpp` runs the same recursive Horn program and fixedpoint
transformation options as `TestBMCNumericProofIndependentValidation`'s
`qbmc/recursive` case. It expects the reachable query to return `sat` and
produce an answer. It uses the public C++ API and does not depend on g3.

From the g3 repository root, build and run it against a locally built Z3:

```sh
z3_src="$HOME/go/src/github.com/Z3Prover/z3"
c++ -std=c++17 -I "$z3_src/src/api" -I "$z3_src/src/api/c++" \
  pkg/muz/fp/test_vectors/qbmc_cpp/recursive_qbmc.cpp \
  -L "$z3_src/build" -Wl,-rpath,"$z3_src/build" -lz3 \
  -o /tmp/recursive_qbmc
/tmp/recursive_qbmc
```

On Z3 commit `a110bf8f94dc57ca785026805468a4ce96d0926c` (5.3.4.0), built
with `cmake -DCMAKE_BUILD_TYPE=Release`, the executable returned 1 on three
consecutive runs. The library printed an assertion violation at
`src/muz/bmc/dl_bmc_engine.cpp:313`:

```text
Failed to verify: unifier.unify_rules(*r0.get(), 0, *r2.get())
QBMC failed: unreachable
```

This reproduces a C++ proof-construction failure on the recursive case. It does
not establish that the positive-level-fact assumption caused the failure, or
that it explains the separate intermittent Go missing-selector error.
