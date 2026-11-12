; seq_rewriter::mk_seq_extract: a constant position past a constant base is
; empty whatever the length; positions and lengths must be 32-bit unsigned,
; added with wraparound; unit prefixes may precede non-unit words.
(declare-const c Int)
(declare-const x Int)
(declare-const s (Seq Int))
(declare-const t String)
(simplify (str.substr "" 2 x))
(simplify (str.replace (str.substr "" 2 x) "ba" (str.at t 0)))
(simplify (str.substr "abc" 3 x))
(simplify (str.substr "abc" 1 x))
(simplify (str.substr "abc" 1 5000000000))
(simplify (str.substr "abc" 1 4294967295))
(simplify (str.substr "abc" 1 2147483648))
(simplify (str.substr "abc" 1 1))
(simplify (seq.extract (seq.++ (seq.unit c) s) 0 1))
(simplify (seq.extract (seq.++ (seq.unit c) s) 1 1))
(simplify (seq.extract (seq.unit c) 5 1))
