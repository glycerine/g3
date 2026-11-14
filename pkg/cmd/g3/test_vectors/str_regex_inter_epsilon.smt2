(set-option :smt.string_solver z3str3)
(declare-const x String)
(assert (str.in.re x (re.inter (re.* (str.to.re "a")) (re.* (str.to.re "b")))))
(check-sat)
