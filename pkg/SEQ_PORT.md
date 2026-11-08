# Porting `theory_seq` faithfully

Goal: replace Go's sequence theory (`smtcore/theory_seq*.go`,
`SMTSequenceTheory`) with a mechanical port of the C++
`src/smt/theory_seq.h` (603 lines) and `theory_seq.cpp` (4,794 lines), as was
done for `theory_arith` (ARITH_PORT.md).

## Why

TODO.md item 2. Logics missing from `setup::setup_logic` (QF_NIA, ALL, ...)
go to `setup_unknown`, which registers theory_seq. Its `final_check_eh`
draws from the context random generator in `reduce_length_eq` and
`branch_variable` even when there are no sequence constraints, so every
later random choice in z3 depends on it. Go's sequence theory is a different
design (its own word-equation solver, length splits, regex stages), so these
draws, the order of the final-check steps, and the seq statistics cannot
match. Thirteen seq/str vectors also differ.

## Scope

- `theory_seq` and its nested types: `solution_map`, `exclusion_table`,
  `eq`, `ne`, `nc`, the replay actions, `cell`, stats.
- `theory_seq_empty` is already ported (`theory_seq_empty.go`) and stays.
- `theory_str` (z3str3) is separate and out of scope.
- Reused as they are, after checking each against the source:
  `ast/rewriter/seq_rewriter*.go` (seq_rewriter), `string_regex_automaton.go`
  (re2automaton, eautomaton), `ast` dependency manager
  (scoped_dependency_manager), `util/backtrack/union_find.go`,
  `proto_model/sequence_factory.go` (seq_factory), `ast_seq_*.go` (seq_util).

## Acceptance

On all `cmd/g3/test_vectors` and the fuzzers:

1. Verdicts and models agree with C++.
2. Seq statistics (`seq-*`) and the arith and Boolean statistics of
   seq-bearing vectors match C++ exactly; vectors under `setup_unknown` match
   their random draws.
3. `make` stays green.

## Ground rules (from ARITH_PORT.md)

- One type, `SMTTheorySeq`, is `theory_seq`. It embeds `SMTTheoryBase` and
  calls the context through `t.state`, where the source calls
  `get_context()`. No adapters.
- Files mirror the source: `smt_theory_seq.go` (theory_seq.h types and
  fields), `smt_theory_seq_core.go`, ... in the order of theory_seq.cpp.
  Function names are the C++ names in lowerCamelCase.
- Port only what retained callers reach; no STL ports.
- The old implementation stays selectable until the port passes, then it
  and its tests are deleted.

## Phases

1. Types and fields (theory_seq.h), solution_map, exclusion_table,
   eq/ne/nc, dependency helpers, replay and trail.
2. Internalization, axioms (`add_*_axiom`, `enque_axiom`, `deque_axiom`),
   `mk_literal`, skolems, length handling.
3. Equalities: canonize/expand, solve_eqs and friends, branch_* functions,
   length coherence, fixed length.
4. Disequalities and non-contains: solve_nqs/solve_ne/solve_nc,
   check_extensionality, check_contains.
5. Automata: propagate_in_re, accept/reject/step, propagate_automata.
6. int/string conversion, model generation (seq_value_proc), statistics.
7. Switch over in setup, delete the old theory and its tests.

## Status

Not started.
