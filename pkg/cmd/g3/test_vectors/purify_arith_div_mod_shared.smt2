(declare-const x Int)
(declare-const y Int)
(assert (= (div x y) (mod x y)))
(apply purify-arith :print-model-converter true)
