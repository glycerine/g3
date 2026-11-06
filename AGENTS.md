git use: Do not amend or rebase commits in git. Make a corrective additional commit instead. We push
in the background and have seen an accidental git fork and we do not want to lose data.

C++ additions, tests, benchmarks, and positive controls belong in this repository's
src/ tree by default. This is the C++ fork being ported. Do not use the separate
~/go/src/github.com/Z3Prover/z3 checkout as a project source or destination unless
the user explicitly requests it.

A rule for storing test vectors and associated test data: do not create a 
directory named testdata. Avoid this as a directory name. Use test_vectors instead. 
The reason is that the Go fuzzer uses the directory name testdata/ for its ephemeral
storage and the directory will get wiped and we could lose the test vectors by 
accident and that would be bad.


Porting verification: run fast, focused tests for the features being ported.
Run the full native test suite sparingly, roughly once per hour, since it takes
about eight minutes. Keep commits frequent and do not commit known failing
focused checks. No Go generics, C/cgo, source hashing, wasm/wasi builds, or race
checks for the current single-goroutine porting work. Preserve the pkg/ split.

Insertion-ordered dictionaries: use github.com/glycerine/insdict at v0.14.6
when a dictionary/hashmap must iterate keys in insertion order. Its generics
are an explicitly approved exception to the general no-generics rule.

Never port any of the C++ standard library. Use Go standard-library string
handling exclusively; fidelity applies to project algorithms, not STL internals.

Port only C++ code reached by retained source consumers. Do not port dead C++
code or add universal Go adapters merely to represent an unused template. For
live template instantiations, use concrete typed Go implementations for the
actual numeral, pointer, or value types instead of any-valued callback layers.
