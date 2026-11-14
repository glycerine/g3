(declare-const x Int)
(declare-const y Int)
(assert (= (mod x y) (div x y)))
(apply purify-arith :print-model-converter true)
