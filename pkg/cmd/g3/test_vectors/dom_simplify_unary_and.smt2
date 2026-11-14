(declare-const p Bool)
(assert (not (and p)))
(apply dom-simplify)
